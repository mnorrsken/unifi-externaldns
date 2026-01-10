# Unifi ExternalDNS client poller

A small Go utility that queries the Unifi API for connected clients once per minute and emits Kubernetes `DNSEndpoint` resources for those whose names start with a valid DNS label. Future work will create/update Kubernetes objects for each client.

## Requirements
- Go 1.22+

## Configuration
Parameters:

- `--api-url` (required): Base API URL, e.g. `https://unifi.example.com` (no trailing slash).
- `--site-id` (required): Site identifier.
- `--domain-suffix` (required): Domain appended to DNS names, e.g. `example.com`.
- `--poll-interval` (optional): Duration between polls, default `1m` (e.g. `30s`, `2m`).
- `--namespace` (optional): Kubernetes namespace for generated CRs, default `default`.
- `--insecure` (optional): Skip TLS verification (lab setups only).
- `UNIFI_API_KEY` env var (required): API key used in the `X-API-KEY` header.

## Usage
```bash
UNIFI_API_KEY=your-api-key \
go run ./... \
	--api-url=https://unifi.example.com \
	--site-id=default \
	--domain-suffix=example.com
```

On each poll the tool now reconciles `DNSEndpoint` resources in Kubernetes (in-cluster if available, otherwise via kubeconfig). It creates/updates/deletes CRs in the chosen namespace, labelled with `unifi-externaldns.snosr.se/site-id` and containing a single `A` record pointing to the client IP. The DNS name is `<first-token>.<domain-suffix>` where the first whitespace-delimited name token is a valid DNS label; metadata.name is the full client name lowercased with invalid characters collapsed to `-`. The program listens for `SIGINT`/`SIGTERM` and exits cleanly.

## Make targets
- `make build` - Build binary to `bin/`
- `make run` - Run the program
- `make fmt` - Format code
- `make vet` - Go vet
- `make test` - Run tests (none yet)

## API reference
This follows the example in `api-example.txt` for `/v1/sites/<site-id>/clients?limit=200` using the `X-API-KEY` header.
