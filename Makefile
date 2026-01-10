BINARY := unifi-externaldns

.PHONY: build run fmt vet test clean

build:
	go build -o bin/$(BINARY) ./...

run:
	go run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf bin
