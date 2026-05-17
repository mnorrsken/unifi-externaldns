// Package mock generates fake Unifi /v2 active-leases responses for use in
// end-to-end tests.
package mock

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
)

type Fingerprint struct {
	HasOverride bool `json:"has_override"`
}

type Lease struct {
	ClientType          string      `json:"client_type"`
	DisplayName         string      `json:"display_name"`
	Fingerprint         Fingerprint `json:"fingerprint"`
	FixedIP             string      `json:"fixed_ip,omitempty"`
	Hostname            string      `json:"hostname,omitempty"`
	IconResolutions     []string    `json:"icon_resolutions"`
	IP                  string      `json:"ip"`
	LeaseExpirationTime int64       `json:"lease_expiration_time"`
	MAC                 string      `json:"mac"`
	Name                string      `json:"name,omitempty"`
	NetworkID           string      `json:"network_id"`
	OUI                 string      `json:"oui"`
	Status              string      `json:"status"`
	UseFixedIP          bool        `json:"use_fixedip"`
}

type Response struct {
	DHCPLeaseInfo []Lease `json:"dhcp_lease_info"`
}

type template struct {
	clientType string
	hostname   string // empty means the lease has no hostname field
	fixedIP    bool
	online     bool
	oui        string
}

var fixtures = []template{
	{"WIRED", "device-1", true, true, "VendorA"},
	{"WIRED", "device-2", true, true, "VendorB"},
	{"WIRED", "device-3", true, false, "VendorB"},
	{"WIRED", "DESKTOP-A", false, true, "VendorC"},
	{"WIRED", "Laptop-1", true, true, "VendorC"},
	{"WIRELESS", "Laptop", false, true, ""},
	{"WIRELESS", "", false, true, ""}, // no hostname — should be skipped by consumer
	{"WIRELESS", "plug-1", false, true, "VendorD"},
	{"WIRELESS", "Phone--1", false, true, ""},         // double-hyphen
	{"WIRELESS", "my_device", false, true, "VendorD"}, // underscore — name sanitized, host raw
	{"WIRELESS", "plug-2", false, true, "VendorD"},
	{"WIRELESS", "", false, true, "VendorE"}, // no hostname
	{"WIRELESS", "TV-1", true, true, "VendorF"},
}

const (
	wiredNet    = "1111111111111111111111aa"
	wirelessNet = "2222222222222222222222bb"
)

// Generate returns a deterministic Response derived from seed.
func Generate(seed uint64) Response {
	r := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	out := Response{DHCPLeaseInfo: make([]Lease, 0, len(fixtures))}
	for i, t := range fixtures {
		var ip string
		if t.fixedIP {
			ip = fmt.Sprintf("10.123.0.%d", 10+i)
		} else if t.clientType == "WIRED" {
			ip = fmt.Sprintf("10.123.1.%d", 100+r.IntN(150))
		} else {
			ip = fmt.Sprintf("10.123.2.%d", 10+r.IntN(240))
		}

		netID := wiredNet
		if t.clientType == "WIRELESS" {
			netID = wirelessNet
		}

		l := Lease{
			ClientType:      t.clientType,
			Fingerprint:     Fingerprint{HasOverride: false},
			Hostname:        t.hostname,
			IconResolutions: []string{},
			IP:              ip,
			MAC:             fmt.Sprintf("aa:bb:cc:%02x:%02x:%02x", i, r.IntN(256), r.IntN(256)),
			NetworkID:       netID,
			OUI:             t.oui,
			UseFixedIP:      t.fixedIP,
		}

		if t.fixedIP {
			l.FixedIP = ip
			l.Name = strings.ToLower(t.hostname)
			l.DisplayName = strings.ToLower(t.hostname)
			l.LeaseExpirationTime = 0
		} else {
			l.DisplayName = displayNameFor(t.hostname, l.MAC)
			l.LeaseExpirationTime = 1779100000 + int64(r.IntN(100000))
		}

		l.Status = "offline"
		if t.online {
			l.Status = "online"
		}

		out.DHCPLeaseInfo = append(out.DHCPLeaseInfo, l)
	}
	return out
}

func displayNameFor(hostname, mac string) string {
	suffix := mac[len(mac)-5:]
	if hostname == "" {
		return "unknown device " + suffix
	}
	return hostname + " " + suffix
}

// Handler returns an http.Handler that serves a generated Response at
// GET /v2/api/site/{site}/active-leases. If apiKey is non-empty the handler
// requires a matching X-API-KEY header.
func Handler(apiKey string, seed uint64) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/api/site/{site}/active-leases", func(w http.ResponseWriter, req *http.Request) {
		if apiKey != "" && req.Header.Get("X-API-KEY") != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Generate(seed))
	})
	return mux
}
