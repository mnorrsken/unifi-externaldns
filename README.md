# Unifi ExternalDNS client poller

A small Go utility that queries the Unifi active-leases API once per minute and emits Kubernetes `DNSEndpoint` resources for DHCP leases that have a `hostname` and `ip`.

## Requirements
- Go 1.22+

## Configuration
Parameters:

- `--api-url` (required): Base network API URL, e.g. `https://router.internal/proxy/network` (no trailing slash).
- `--site-id` (optional): Site identifier, default `default`.
- `--domain-suffix` (required): Domain appended to DNS names, e.g. `example.com`.
- `--poll-interval` (optional): Duration between polls, default `1m` (e.g. `30s`, `2m`).
- `--namespace` (optional): Kubernetes namespace for generated CRs, default `default`.
- `--insecure` (optional): Skip TLS verification (lab setups only).
- `UNIFI_API_KEY` env var (required): API key used in the `X-API-KEY` header.

## Usage
```bash
UNIFI_API_KEY=your-api-key \
go run ./cmd/unifi-externaldns \
	--api-url=https://router.internal/proxy/network \
	--site-id=default \
	--domain-suffix=example.com
```

On each poll the tool reconciles `DNSEndpoint` resources in Kubernetes (in-cluster if available, otherwise via kubeconfig). It creates/updates/deletes CRs in the chosen namespace, labelled with `unifi-externaldns.snosr.se/site-id` and containing a single `A` record pointing to the lease IP. The DNS name and metadata.name are both the lowercased `hostname`; leases without a hostname or IP are skipped. The program listens for `SIGINT`/`SIGTERM` and exits cleanly.

## Make targets
- `make build` - Build the main binary to `bin/unifi-externaldns`
- `make mock` - Build the mock server to `bin/mockunifi` (see below)
- `make run` - Run the program
- `make fmt` - Format code
- `make vet` - Go vet
- `make test` - Run tests

## API reference
Calls the undocumented `GET <api-url>/v2/api/site/<site-id>/active-leases` endpoint with the `X-API-KEY` header and reads `dhcp_lease_info[].hostname` and `dhcp_lease_info[].ip`.

## Tests
`make test` runs the unit and end-to-end suite. The e2e test drives the real `fetchLeases` against a `httptest` instance of the mock server and reconciles the result into an in-process fake Kubernetes client (`controller-runtime/pkg/client/fake`). No external binaries are required.

## Mock server
`cmd/mockunifi` serves (or prints) a fake response matching the active-leases shape, useful for local testing.

```bash
# Print one fake response to stdout
go run ./cmd/mockunifi --print

# Serve it on :8080
go run ./cmd/mockunifi --addr :8080

# Point the real binary at it
UNIFI_API_KEY=ignored go run ./cmd/unifi-externaldns --api-url=http://localhost:8080 --domain-suffix=lan
```
