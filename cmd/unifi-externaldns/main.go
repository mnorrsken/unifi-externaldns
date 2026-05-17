package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mnorrsken/unifi-externaldns/internal/extdns"
	"github.com/mnorrsken/unifi-externaldns/internal/networkapi"
)

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

func pollOnce(parent context.Context, apiClient *networkapi.Client, cfg Config) error {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	leases, err := apiClient.FetchLeases(ctx)
	if err != nil {
		return err
	}

	kc, err := extdns.NewKubeClient()
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	r := &extdns.Reconciler{
		Client:       kc,
		Namespace:    cfg.Namespace,
		SiteID:       cfg.SiteID,
		DomainSuffix: cfg.DomainSuffix,
	}

	stats, err := r.Reconcile(ctx, r.Desired(leases))
	if err != nil {
		return err
	}

	log.Printf("reconcile complete: created=%d updated=%d deleted=%d", stats.Created, stats.Updated, stats.Deleted)
	return nil
}

func main() {
	cfg, err := loadConfig(os.Args)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	apiClient := &networkapi.Client{
		APIURL: cfg.APIURL,
		SiteID: cfg.SiteID,
		APIKey: cfg.APIKey,
		HTTP:   networkapi.NewHTTPClient(cfg.InsecureSkipVerify),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	if err := pollOnce(ctx, apiClient, cfg); err != nil {
		log.Printf("poll error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("received shutdown signal, exiting")
			return
		case <-ticker.C:
			if err := pollOnce(ctx, apiClient, cfg); err != nil {
				log.Printf("poll error: %v", err)
			}
		}
	}
}
