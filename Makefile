.PHONY: build docker-build docker-run release test test-coverage clean install uninstall help

BINDIR ?= $(GOPATH)/bin

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the walship binary
	go build -o walship ./cmd/walship

install: ## Install walship to $$GOPATH/bin
	go install ./cmd/walship

uninstall: ## Uninstall walship
	rm -f $(BINDIR)/walship

test: ## Run all tests
	go test -v ./...

test-coverage: ## Run tests with coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean: ## Clean build artifacts and test outputs
	rm -f walship
	rm -f coverage.out coverage.html
	rm -rf dist/

docker-build:
	docker build -t walship .

docker-run:
	docker run --rm \
		-e WALSHIP_SERVICE_URL=http://localhost:8080 \
		-e WALSHIP_AUTH_KEY=test \
		walship

release:
	git tag $(v)
	git push origin $(v)
