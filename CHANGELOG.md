# Changelog

All notable changes to this project are documented here. Versions follow `vMAJOR.MINOR.PATCH`.

## v0.1.2

### Changed
- **Go 1.26** — `go.mod` now requires Go 1.26.1; Docker build image bumped from `golang:1.25-bookworm` to `golang:1.26-bookworm`.
- **Kubernetes deps** — `k8s.io/client-go` and `k8s.io/apimachinery` 0.34.2 → 0.36.1, `sigs.k8s.io/controller-runtime` 0.22.4 → 0.24.1, `sigs.k8s.io/external-dns` 0.20.0 → 0.21.0.
- **Test deps** — `ginkgo/v2` 2.25.3 → 2.29.0, `gomega` 1.38.2 → 1.40.0.
- **CI** — `actions/checkout` 6 → 7.

## v0.1.1

### Added
- **Helm chart documentation** — README section covering OCI install from `ghcr.io/mnorrsken/charts/unifi-externaldns`, required values, and secret wiring.
- **CHANGELOG.md** — initial changelog with retroactive `v0.1.0` baseline.

## v0.1.0

### Added
- **Unifi active-leases poller** — queries `GET /v2/api/site/<site-id>/active-leases` once per `--poll-interval` using `X-API-KEY`.
- **DNSEndpoint reconciler** — creates/updates/deletes `externaldns.k8s.io/v1alpha1` `DNSEndpoint` resources from leases that have both `hostname` and `ip`, labelled `unifi-externaldns/site-id`.
- **In-cluster + kubeconfig auth** — uses in-cluster config when available, falls back to kubeconfig for local runs.
- **Helm chart** — `charts/unifi-externaldns` with deployment, ServiceAccount, Role/RoleBinding, configurable image, resources, and security context.
- **Release pipeline** — `release.yml` builds multi-arch (`linux/amd64`, `linux/arm64`) images to `ghcr.io/mnorrsken/unifi-externaldns` and cuts a GitHub release on every `v*` tag.
- **Helm pipeline** — `helm.yml` lints on PRs and publishes the packaged chart to `oci://ghcr.io/mnorrsken/charts` on every `v*` tag.
