package extdns_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	v1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"

	"github.com/mnorrsken/unifi-externaldns/internal/extdns"
	"github.com/mnorrsken/unifi-externaldns/internal/mock"
	"github.com/mnorrsken/unifi-externaldns/internal/networkapi"
)

var _ = Describe("end-to-end with mock server and fake k8s client", Ordered, func() {
	var (
		server    *httptest.Server
		apiClient *networkapi.Client
	)

	BeforeAll(func() {
		server = httptest.NewServer(mock.Handler("test-key", 1))
		apiClient = &networkapi.Client{
			APIURL: server.URL,
			SiteID: "default",
			APIKey: "test-key",
			HTTP:   http.DefaultClient,
		}
	})

	AfterAll(func() {
		if server != nil {
			server.Close()
		}
	})

	It("rejects requests without the configured API key", func() {
		bad := *apiClient
		bad.APIKey = "wrong"
		_, err := bad.FetchLeases(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("401"))
	})

	It("fetches leases from the mock and reconciles DNSEndpoints through create/update/delete", func() {
		ctx := context.Background()
		k8sClient := newFakeClient()
		r := &extdns.Reconciler{
			Client:       k8sClient,
			Namespace:    "default",
			SiteID:       "default",
			DomainSuffix: "lan",
		}

		leases, err := apiClient.FetchLeases(ctx)
		Expect(err).NotTo(HaveOccurred())

		// Mock returns 13 fixtures; 2 have no hostname so the consumer keeps 11.
		desired := r.Desired(leases)
		Expect(desired).To(HaveLen(11))

		// Sanitization: "my_device" → metadata.name "my-device", DNSName keeps the raw "my_device".
		ep, ok := desired["my-device"]
		Expect(ok).To(BeTrue())
		Expect(ep.Spec.Endpoints[0].DNSName).To(Equal("my_device.lan"))
		Expect(ep.Spec.Endpoints[0].Targets).To(HaveLen(1))

		// Mixed-case hostname keeps original casing in DNSName.
		ep, ok = desired["laptop-1"]
		Expect(ok).To(BeTrue())
		Expect(ep.Spec.Endpoints[0].DNSName).To(Equal("Laptop-1.lan"))

		listBySite := func() []v1alpha1.DNSEndpoint {
			var list v1alpha1.DNSEndpointList
			Expect(k8sClient.List(ctx, &list, ctrlclient.MatchingLabels{extdns.SiteIDLabel: r.SiteID})).To(Succeed())
			return list.Items
		}

		// Create
		stats, err := r.Reconcile(ctx, copyDesired(desired))
		Expect(err).NotTo(HaveOccurred())
		Expect(stats).To(Equal(extdns.Stats{Created: 11}))
		Expect(listBySite()).To(HaveLen(11))

		// Re-running with the same desired set is a no-op.
		stats, err = r.Reconcile(ctx, copyDesired(desired))
		Expect(err).NotTo(HaveOccurred())
		Expect(stats).To(Equal(extdns.Stats{}))

		// Change a single target IP — only the Update branch should fire.
		mutated := copyDesired(desired)
		mutated["my-device"].Spec.Endpoints[0].Targets = []string{"10.99.99.99"}
		stats, err = r.Reconcile(ctx, mutated)
		Expect(err).NotTo(HaveOccurred())
		Expect(stats).To(Equal(extdns.Stats{Updated: 1}))

		var obj v1alpha1.DNSEndpoint
		Expect(k8sClient.Get(ctx, ctrlclient.ObjectKey{Name: "my-device", Namespace: r.Namespace}, &obj)).To(Succeed())
		Expect(obj.Spec.Endpoints[0].Targets).To(ConsistOf("10.99.99.99"))

		// Emptying the desired set deletes everything we created.
		stats, err = r.Reconcile(ctx, map[string]*v1alpha1.DNSEndpoint{})
		Expect(err).NotTo(HaveOccurred())
		Expect(stats).To(Equal(extdns.Stats{Deleted: 11}))
		Expect(listBySite()).To(BeEmpty())
	})
})

// copyDesired returns a deep copy of the map. Reconcile mutates its input
// (deletes consumed keys), so callers that need to reuse the set must pass a
// fresh map each time.
func copyDesired(in map[string]*v1alpha1.DNSEndpoint) map[string]*v1alpha1.DNSEndpoint {
	out := make(map[string]*v1alpha1.DNSEndpoint, len(in))
	for k, v := range in {
		out[k] = v.DeepCopy()
	}
	return out
}
