package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	v1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
)

type lease struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

type leaseResponse struct {
	DHCPLeaseInfo []lease `json:"dhcp_lease_info"`
}

type Config struct {
	APIURL             string
	SiteID             string
	APIKey             string
	DomainSuffix       string
	PollInterval       time.Duration
	Namespace          string
	InsecureSkipVerify bool
}

const (
	defaultNamespace = "default"
	defaultSiteID    = "default"
	defaultTTL       = 300
)

func loadConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	apiURLFlag := fs.String("api-url", "", "Base Unifi network API URL, e.g. https://router.internal/proxy/network")
	siteIDFlag := fs.String("site-id", defaultSiteID, "Unifi site identifier")
	domainFlag := fs.String("domain-suffix", "", "DNS suffix to append, e.g. example.com")
	pollFlag := fs.String("poll-interval", "1m", "Poll interval, e.g. 30s or 2m")
	nsFlag := fs.String("namespace", defaultNamespace, "Kubernetes namespace for generated CRs")
	insecureFlag := fs.Bool("insecure", false, "Skip TLS verification (lab only)")

	if err := fs.Parse(args[1:]); err != nil {
		return Config{}, err
	}

	cfg := Config{}
	cfg.APIURL = strings.TrimSpace(*apiURLFlag)
	cfg.SiteID = strings.TrimSpace(*siteIDFlag)
	cfg.DomainSuffix = strings.TrimPrefix(strings.TrimSpace(*domainFlag), ".")
	cfg.Namespace = strings.TrimSpace(*nsFlag)
	cfg.InsecureSkipVerify = *insecureFlag
	cfg.APIKey = strings.TrimSpace(os.Getenv("UNIFI_API_KEY"))

	if cfg.Namespace == "" {
		cfg.Namespace = defaultNamespace
	}
	if cfg.SiteID == "" {
		cfg.SiteID = defaultSiteID
	}

	if cfg.APIKey == "" {
		return Config{}, errors.New("UNIFI_API_KEY env var is required")
	}
	if cfg.APIURL == "" || cfg.DomainSuffix == "" {
		return Config{}, errors.New("flags --api-url and --domain-suffix are required")
	}

	var err error
	cfg.PollInterval, err = time.ParseDuration(strings.TrimSpace(*pollFlag))
	if err != nil {
		return Config{}, fmt.Errorf("invalid --poll-interval: %w", err)
	}

	return cfg, nil
}

func newHTTPClient(insecure bool) *http.Client {
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Transport: transport}
}

func newKubeClient() (ctrlclient.Client, error) {
	cfg, err := loadKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add core scheme: %w", err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add externaldns scheme: %w", err)
	}
	return ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
}

func loadKubeConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	kubeconfig := clientcmd.RecommendedHomeFile
	if env := strings.TrimSpace(os.Getenv("KUBECONFIG")); env != "" {
		kubeconfig = env
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func buildLeasesURL(baseURL, siteID string) (string, error) {
	endpoint, err := url.JoinPath(strings.TrimSuffix(baseURL, "/"), "v2", "api", "site", siteID, "active-leases")
	if err != nil {
		return "", fmt.Errorf("build URL: %w", err)
	}
	return endpoint, nil
}

func fetchLeases(ctx context.Context, httpClient *http.Client, cfg Config) ([]lease, error) {
	endpoint, err := buildLeasesURL(cfg.APIURL, cfg.SiteID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-API-KEY", cfg.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var payload leaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return payload.DHCPLeaseInfo, nil
}

var invalidHostRun = regexp.MustCompile(`[^a-z0-9-]+`)

func sanitizeHost(raw string) string {
	host := invalidHostRun.ReplaceAllString(strings.ToLower(raw), "-")
	return strings.Trim(host, "-")
}

func buildDNSEndpoint(cfg Config, name, host, ip string) *v1alpha1.DNSEndpoint {
	return &v1alpha1.DNSEndpoint{
		TypeMeta: v1.TypeMeta{
			APIVersion: "externaldns.k8s.io/v1alpha1",
			Kind:       "DNSEndpoint",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"unifi-externaldns.snosr.se/site-id": cfg.SiteID,
			},
		},
		Spec: v1alpha1.DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{
				{
					DNSName:    fmt.Sprintf("%s.%s", host, cfg.DomainSuffix),
					RecordTTL:  endpoint.TTL(defaultTTL),
					RecordType: "A",
					Targets:    []string{ip},
				},
			},
		},
	}
}

func desiredResources(cfg Config, leases []lease) map[string]*v1alpha1.DNSEndpoint {
	items := make(map[string]*v1alpha1.DNSEndpoint)
	for _, l := range leases {
		name := sanitizeHost(l.Hostname)
		if name == "" || l.IP == "" {
			continue
		}

		cr := buildDNSEndpoint(cfg, name, l.Hostname, l.IP)
		items[cr.Name] = cr
	}
	return items
}

type reconcileStats struct {
	Created int
	Updated int
	Deleted int
}

func specsEqual(a, b v1alpha1.DNSEndpointSpec) bool {
	return reflect.DeepEqual(a, b)
}

func labelsEqual(a, b map[string]string) bool {
	return reflect.DeepEqual(a, b)
}

func reconcileEndpoints(ctx context.Context, c ctrlclient.Client, cfg Config, desired map[string]*v1alpha1.DNSEndpoint) (reconcileStats, error) {
	var stats reconcileStats

	list := v1alpha1.DNSEndpointList{}
	selector := labels.Set{"unifi-externaldns.snosr.se/site-id": cfg.SiteID}.AsSelector()
	if err := c.List(ctx, &list, ctrlclient.InNamespace(cfg.Namespace), ctrlclient.MatchingLabelsSelector{Selector: selector}); err != nil {
		return stats, fmt.Errorf("list existing: %w", err)
	}

	existing := make(map[string]*v1alpha1.DNSEndpoint)
	for i := range list.Items {
		item := list.Items[i]
		existing[item.Name] = &item
	}

	// Handle updates and deletions
	for name, obj := range existing {
		d, ok := desired[name]
		if !ok {
			if err := c.Delete(ctx, obj); err != nil {
				return stats, fmt.Errorf("delete %s: %w", name, err)
			}
			stats.Deleted++
			continue
		}

		aLabels := d.GetLabels()
		dSpec := d.Spec

		if !specsEqual(dSpec, obj.Spec) || !labelsEqual(aLabels, obj.GetLabels()) {
			obj.Spec = dSpec
			obj.SetLabels(aLabels)
			if err := c.Update(ctx, obj); err != nil {
				return stats, fmt.Errorf("update %s: %w", name, err)
			}
			stats.Updated++
		}

		delete(desired, name)
	}

	// Create remaining desired
	for name, obj := range desired {
		if err := c.Create(ctx, obj); err != nil {
			return stats, fmt.Errorf("create %s: %w", name, err)
		}
		stats.Created++
		delete(desired, name)
	}

	return stats, nil
}

func pollOnce(parent context.Context, httpClient *http.Client, cfg Config) error {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	leases, err := fetchLeases(ctx, httpClient, cfg)
	if err != nil {
		return err
	}

	kc, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	desired := desiredResources(cfg, leases)
	reconStats, err := reconcileEndpoints(ctx, kc, cfg, desired)
	if err != nil {
		return err
	}

	log.Printf("reconcile complete: created=%d updated=%d deleted=%d", reconStats.Created, reconStats.Updated, reconStats.Deleted)
	return nil
}

func main() {
	cfg, err := loadConfig(os.Args)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	httpClient := newHTTPClient(cfg.InsecureSkipVerify)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	if err := pollOnce(ctx, httpClient, cfg); err != nil {
		log.Printf("poll error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("received shutdown signal, exiting")
			return
		case <-ticker.C:
			if err := pollOnce(ctx, httpClient, cfg); err != nil {
				log.Printf("poll error: %v", err)
			}
		}
	}
}
