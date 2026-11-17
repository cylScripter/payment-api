.PHONY: tools proto run build build-linux

tools:
	GOBIN=$(CURDIR)/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	GOBIN=$(CURDIR)/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

proto:
	PATH="$(CURDIR)/bin:$$PATH" protoc -I . -I third_party \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(shell find api -name '*.proto' -type f | sort)

run:
	go run ./cmd/server

build:
	go build -o bin/payment-api ./cmd/server

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/payment-api ./cmd/server
