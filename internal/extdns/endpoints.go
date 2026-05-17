// Package extdns translates Unifi DHCP leases into ExternalDNS DNSEndpoint
// custom resources and reconciles them against a Kubernetes API server.
package extdns

import (
	"fmt"
	"regexp"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/mnorrsken/unifi-externaldns/internal/networkapi"
)

const (
	SiteIDLabel = "unifi-externaldns/site-id"
	defaultTTL  = 300
)

var invalidHostRun = regexp.MustCompile(`[^a-z0-9-]+`)

func sanitizeHost(raw string) string {
	host := invalidHostRun.ReplaceAllString(strings.ToLower(raw), "-")
	return strings.Trim(host, "-")
}

func (r *Reconciler) buildDNSEndpoint(name, host, ip string) *v1alpha1.DNSEndpoint {
	return &v1alpha1.DNSEndpoint{
		TypeMeta: v1.TypeMeta{
			APIVersion: "externaldns.k8s.io/v1alpha1",
			Kind:       "DNSEndpoint",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: r.Namespace,
			Labels: map[string]string{
				SiteIDLabel: r.SiteID,
			},
		},
		Spec: v1alpha1.DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{
				{
					DNSName:    fmt.Sprintf("%s.%s", host, r.DomainSuffix),
					RecordTTL:  endpoint.TTL(defaultTTL),
					RecordType: "A",
					Targets:    []string{ip},
				},
			},
		},
	}
}

// Desired produces the DNSEndpoint set that should exist for the given leases.
// Leases without a usable hostname or IP are skipped; the map key is the
// sanitized hostname (used as metadata.name).
func (r *Reconciler) Desired(leases []networkapi.Lease) map[string]*v1alpha1.DNSEndpoint {
	items := make(map[string]*v1alpha1.DNSEndpoint)
	for _, l := range leases {
		name := sanitizeHost(l.Hostname)
		if name == "" || l.IP == "" {
			continue
		}

		cr := r.buildDNSEndpoint(name, l.Hostname, l.IP)
		items[cr.Name] = cr
	}
	return items
}
