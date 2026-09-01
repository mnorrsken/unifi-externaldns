# unifi-externaldns

Go poller that reads the Unifi `active-leases` API and reconciles Kubernetes
`DNSEndpoint` resources (for external-dns) from DHCP leases that have a
hostname and IP. Multi-arch image and Helm chart on GHCR.

## Commands

`make build`, `make run`, `make test`, `make vet`, `make fmt`. Flags and the
`UNIFI_API_KEY` env var are documented in README "Configuration". For a local
run without a router, use the stub in `internal/mock`.

## Verify before done

`go build ./... && go vet ./... && go test ./...`. For a change to the
reconcile logic in `internal/extdns`, run it once against a kind or lab
cluster and check that create, update and delete of `DNSEndpoint` all happen.

## Layout

`internal/networkapi` talks to the Unifi API, `internal/extdns` builds and
reconciles the `DNSEndpoint` CRs, `internal/mock` fakes the API for tests.
Keep API parsing tolerant: the Unifi payload changes between controller
versions.

## Release notes (what differs from the wiki "GitHub Release Process" skill)

- Changelog heading: `## vX.Y.Z`.
- On `v*` tags: `release.yml` builds and pushes the multi-arch image;
  `helm.yml` sets `Chart.yaml` versions from the tag (never bump by hand) and
  pushes the chart to `ghcr.io/mnorrsken/charts/unifi-externaldns`.
- Commit message for the release: `Add <feature> (vX.Y.Z)`.
