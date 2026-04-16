.PHONY: build test test-race lint fmt fmt-check vulncheck check clean dogfood prerelease install-hooks version-sync

BINARY := bin/specter
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/specter/

test:
	go test ./...

test-verbose:
	go test -v ./...

# Race condition detector — slower but catches concurrency bugs
test-race:
	go test -race ./...

# Format all Go source files in place
fmt:
	gofmt -w $$(find . -name "*.go" | grep -v vendor)

# Fail if any file needs formatting (used in CI)
fmt-check:
	@unformatted=$$(gofmt -l $$(find . -name "*.go" | grep -v vendor)); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files need gofmt:"; \
		echo "$$unformatted"; \
		echo "Run: make fmt"; \
		exit 1; \
	fi

# Full lint: gofmt check + go vet + golangci-lint
lint: fmt-check
	go vet ./...
	golangci-lint run ./...

# Check known vulnerabilities in dependencies
vulncheck:
	govulncheck ./...

check: lint test build

# Install the pre-commit hook into the local git repo
install-hooks:
	cp scripts/pre-commit ../.git/hooks/pre-commit
	chmod +x ../.git/hooks/pre-commit
	@echo "pre-commit hook installed"

# Sync VERSION into the VS Code extension's package.json
version-sync:
	@VER=$$(cat VERSION); \
	node -e "const fs=require('fs'),p='vscode-extension/package.json',j=JSON.parse(fs.readFileSync(p));j.version='$$VER';fs.writeFileSync(p,JSON.stringify(j,null,2)+'\n');" && \
	echo "Synced version $$VER → vscode-extension/package.json"

clean:
	rm -rf bin/ specter-linux-* specter-darwin-* specter-windows-*

# Cross-compilation
build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-linux-amd64 ./cmd/specter/
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-darwin-amd64 ./cmd/specter/
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-darwin-arm64 ./cmd/specter/
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-windows-amd64.exe ./cmd/specter/

# Run specter against its own specs (full pipeline)
dogfood: build
	./$(BINARY) parse specs/*.spec.yaml
	./$(BINARY) resolve
	./$(BINARY) check
	./$(BINARY) coverage
	./$(BINARY) sync

# Full pre-release gate — run this before every release
prerelease: check test-race vulncheck dogfood
	@echo ""
	@echo "--- Cross-compiling for all platforms ---"
	$(MAKE) build-all
	@echo ""
	@echo "--- VS Code extension: typecheck + lint + tests + package ---"
	cd vscode-extension && npm run typecheck && npm run lint && npm test
	cd vscode-extension && npm run build && npm run package
	@echo ""
	@echo "--- All pre-release checks passed ---"
