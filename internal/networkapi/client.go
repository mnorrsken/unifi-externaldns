// Package networkapi is a minimal client for the Unifi network controller's
// undocumented /v2 active-leases endpoint.
package networkapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Lease struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

type leaseResponse struct {
	DHCPLeaseInfo []Lease `json:"dhcp_lease_info"`
}

type Client struct {
	APIURL string
	SiteID string
	APIKey string
	HTTP   *http.Client
}

func NewHTTPClient(insecure bool) *http.Client {
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Transport: transport}
}

func (c *Client) leasesURL() (string, error) {
	u, err := url.JoinPath(strings.TrimSuffix(c.APIURL, "/"), "v2", "api", "site", c.SiteID, "active-leases")
	if err != nil {
		return "", fmt.Errorf("build URL: %w", err)
	}
	return u, nil
}

func (c *Client) FetchLeases(ctx context.Context) ([]Lease, error) {
	u, err := c.leasesURL()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-API-KEY", c.APIKey)
	req.Header.Set("Accept", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
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
