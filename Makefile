# router – skupper-router wrapper for Eclipse ioFog / Datasance PoT

BINARY      := router
BINARY_PATH := bin/$(BINARY)
LDFLAGS     := -trimpath -ldflags="-s -w"

GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

export PATH := $(GOBIN):$(PATH)

GOLANGCI_LINT_VERSION ?= v2.12.2
GOLANGCI_LINT         := $(GOBIN)/golangci-lint

GOVULNCHECK_VERSION ?= v1.1.4
GOSEC_SCOPE           := ./...
# Legacy skupper AMQP + container paths: K8s secret names (G101), AMQP type coercion (G115/G109),
# skrouterd subprocess (G204), env/volume config paths (G304), container file modes (G301/G306), tar utils (G305/G110).
GOSEC_EXCLUDE         := G101,G109,G110,G115,G204,G301,G304,G305,G306

.PHONY: all build test lint lint-fix install-lint fmt fmt-check vulncheck security-code clean

all: build test

build:
	@mkdir -p bin
	go build $(LDFLAGS) -o $(BINARY_PATH) .

test:
	go test ./...

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) -> $(GOBIN)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)

install-lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) version

lint: $(GOLANGCI_LINT) fmt-check
	@echo "Running golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@$(GOLANGCI_LINT) run --config .golangci.yaml

lint-fix: $(GOLANGCI_LINT)
	@echo "Running golangci-lint $(GOLANGCI_LINT_VERSION) with --fix..."
	@$(GOLANGCI_LINT) run --config .golangci.yaml --fix

fmt:
	go fmt ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Run 'make fmt' or 'gofmt -w .'"; gofmt -l .; exit 1)

vulncheck:
	@echo "Running govulncheck..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); \
	fi
	@chmod +x scripts/vulncheck.sh
	@scripts/vulncheck.sh
	@echo "Verifying module integrity..."
	@go mod verify

security-code:
	@echo "Running Go static security analysis..."
	@if ! command -v gosec >/dev/null 2>&1; then \
		echo "Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi
	@gosec -exclude-generated -exclude-dir=build -exclude-dir=bin -exclude=$(GOSEC_EXCLUDE) $(GOSEC_SCOPE)

clean:
	rm -rf bin/
