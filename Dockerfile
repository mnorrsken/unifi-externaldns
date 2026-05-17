ARG BUILDPLATFORM
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" -o /unifi-externaldns ./cmd/unifi-externaldns

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /unifi-externaldns /usr/local/bin/unifi-externaldns
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/unifi-externaldns"]
