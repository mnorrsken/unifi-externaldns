# Unifi ExternalDNS client poller

A small Go utility that queries the Unifi active-leases API once per minute and emits Kubernetes `DNSEndpoint` resources for DHCP leases that have a `hostname` and `ip`.

## Features
- Polls the Unifi `active-leases` API on a configurable interval.
- Reconciles `DNSEndpoint` resources (create/update/delete) in a target namespace.
- Ships as a multi-arch container image (`linux/amd64`, `linux/arm64`) on GHCR.
- Deployable via the bundled Helm chart published to GHCR as an OCI artifact.

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

On each poll the tool reconciles `DNSEndpoint` resources in Kubernetes (in-cluster if available, otherwise via kubeconfig). It creates/updates/deletes CRs in the chosen namespace, labelled with `unifi-externaldns/site-id` and containing a single `A` record pointing to the lease IP. The DNS name and metadata.name are both the lowercased `hostname`; leases without a hostname or IP are skipped. The program listens for `SIGINT`/`SIGTERM` and exits cleanly.

## Helm chart

The chart in `charts/unifi-externaldns` is published as an OCI artifact to `oci://ghcr.io/mnorrsken/charts/unifi-externaldns` on every `v*` tag.

### Install

Store the Unifi API key in a Secret first:

```bash
kubectl create secret generic unifi-api-key \
  --from-literal=api-key=YOUR_UNIFI_API_KEY
```

Install the chart:

```bash
helm install unifi-externaldns \
  oci://ghcr.io/mnorrsken/charts/unifi-externaldns \
  --version 0.1.1 \
  --set unifi.apiUrl=https://router.internal/proxy/network \
  --set unifi.existingSecret.name=unifi-api-key \
  --set domainSuffix=example.com
```

### Key values

| Value | Default | Description |
| --- | --- | --- |
| `image.repository` | `ghcr.io/mnorrsken/unifi-externaldns` | Container image. |
| `image.tag` | `""` | Overrides `Chart.appVersion`. |
| `unifi.apiUrl` | `""` | **Required.** Base Unifi network API URL. |
| `unifi.siteId` | `default` | Unifi site identifier. |
| `unifi.insecure` | `false` | Skip TLS verification (lab use only). |
| `unifi.existingSecret.name` | `""` | **Required.** Secret holding the API key. |
| `unifi.existingSecret.key` | `api-key` | Key inside the Secret. |
| `domainSuffix` | `""` | **Required.** Suffix appended to lease hostnames. |
| `pollInterval` | `1m` | Polling cadence. |
| `targetNamespace` | `""` | Namespace for generated `DNSEndpoint` resources (defaults to the release namespace). |
| `rbac.create` | `true` | Create `Role`/`RoleBinding` for `DNSEndpoint` management. |
| `serviceAccount.create` | `true` | Create a dedicated ServiceAccount. |

See [charts/unifi-externaldns/values.yaml](charts/unifi-externaldns/values.yaml) for the full set, including resources, security context, and scheduling controls.

## Make targets
- `make build` - Build the main binary to `bin/unifi-externaldns`
- `make run` - Run the program
- `make fmt` - Format code
- `make vet` - Go vet
- `make test` - Run tests

## API reference
Calls the undocumented `GET <api-url>/v2/api/site/<site-id>/active-leases` endpoint with the `X-API-KEY` header and reads `dhcp_lease_info[].hostname` and `dhcp_lease_info[].ip`.

## Tests
`make test` runs the unit and end-to-end suite. The e2e test drives the real `fetchLeases` against a `httptest` instance of the mock server and reconciles the result into an in-process fake Kubernetes client (`controller-runtime/pkg/client/fake`). No external binaries are required.
