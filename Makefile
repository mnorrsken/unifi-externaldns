BINARY := unifi-externaldns

.PHONY: build run fmt vet vuln test clean

build:
	go build -o bin/$(BINARY) ./cmd/unifi-externaldns

run:
	go run ./cmd/unifi-externaldns

fmt:
	gofmt -w .

vet:
	go vet ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

test:
	go test ./...

clean:
	rm -rf bin
