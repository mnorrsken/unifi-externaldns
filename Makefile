BINARY := unifi-externaldns

.PHONY: build run fmt vet test clean

build:
	go build -o bin/$(BINARY) ./cmd/unifi-externaldns

run:
	go run ./cmd/unifi-externaldns

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf bin
