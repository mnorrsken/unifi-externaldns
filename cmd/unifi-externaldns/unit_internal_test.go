package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("desiredResources", func() {
	It("filters invalid names and ips and builds endpoints", func() {
		cfg := Config{DomainSuffix: "example.com", Namespace: "default", SiteID: "site"}
		clients := []client{
			{Name: "valid-host", IPAddress: "10.0.0.1"},
			{Name: "invalid@name", IPAddress: "10.0.0.2"},
			{Name: "valid-host noip", IPAddress: ""},
		}

		res := desiredResources(cfg, clients)
		Expect(res).To(HaveLen(1))
		ep, ok := res["valid-host"]
		Expect(ok).To(BeTrue())
		Expect(ep.Spec.Endpoints).To(HaveLen(1))
		Expect(ep.Spec.Endpoints[0].DNSName).To(Equal("valid-host.example.com"))
		Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf("10.0.0.1"))
	})
})
