BINARY := unifi-externaldns
MOCK   := mockunifi

.PHONY: build mock run fmt vet test clean

build:
	go build -o bin/$(BINARY) ./cmd/unifi-externaldns

mock:
	go build -o bin/$(MOCK) ./cmd/mockunifi

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
