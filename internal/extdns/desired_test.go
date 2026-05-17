package extdns_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mnorrsken/unifi-externaldns/internal/extdns"
	"github.com/mnorrsken/unifi-externaldns/internal/networkapi"
)

var _ = Describe("Reconciler.Desired", func() {
	It("sanitizes hostnames, skips empties, and builds endpoints", func() {
		r := &extdns.Reconciler{DomainSuffix: "example.com", Namespace: "default", SiteID: "site"}
		leases := []networkapi.Lease{
			{Hostname: "MARTIN-PC", IP: "10.0.0.1"},
			{Hostname: "my_device", IP: "10.0.0.4"},
			{Hostname: "", IP: "10.0.0.2"},
			{Hostname: "noip-host", IP: ""},
		}

		res := r.Desired(leases)
		Expect(res).To(HaveLen(2))

		ep := res["martin-pc"]
		Expect(ep).NotTo(BeNil())
		Expect(ep.Spec.Endpoints[0].DNSName).To(Equal("MARTIN-PC.example.com"))
		Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf("10.0.0.1"))

		ep = res["my-device"]
		Expect(ep).NotTo(BeNil())
		Expect(ep.Spec.Endpoints[0].DNSName).To(Equal("my_device.example.com"))
		Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf("10.0.0.4"))
	})
})
