.PHONY: build test test-race lint fmt fmt-check vulncheck check clean dogfood prerelease install-hooks version-sync publish-vscode release-check

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

# Install git hooks (pre-commit + commit-msg) into the local repo
install-hooks:
	cp scripts/pre-commit ../.git/hooks/pre-commit
	chmod +x ../.git/hooks/pre-commit
	cp scripts/commit-msg ../.git/hooks/commit-msg
	chmod +x ../.git/hooks/commit-msg
	@echo "pre-commit and commit-msg hooks installed"

# Sync VERSION into the VS Code extension's package.json
version-sync:
	@VER=$$(cat VERSION); \
	node -e "const fs=require('fs'),p='vscode-extension/package.json',j=JSON.parse(fs.readFileSync(p));j.version='$$VER';fs.writeFileSync(p,JSON.stringify(j,null,2)+'\n');" && \
	echo "Synced version $$VER → vscode-extension/package.json"

# Publish VS Code extension to Marketplace
# Usage: make publish-vscode PAT=<your-token>
publish-vscode:
	@if [ -z "$(PAT)" ]; then \
		echo "Usage: make publish-vscode PAT=<your-personal-access-token>"; \
		exit 1; \
	fi
	$(MAKE) version-sync
	cd vscode-extension && npm run build
	cd vscode-extension && npx vsce publish --pat $(PAT)

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

# Release gate — packages the VSIX and prints the mandatory human-verification
# checklist. Does NOT run vsce publish under any circumstance.
# See CLAUDE.md > "Release discipline" for why every step matters.
release-check: prerelease
	@VER=$$(cat VERSION); \
	VSIX=vscode-extension/specter-vscode-$$VER.vsix; \
	if [ ! -f "$$VSIX" ]; then \
		echo "ERROR: expected $$VSIX after prerelease; not found."; exit 1; \
	fi; \
	echo ""; \
	echo "============================================================"; \
	echo "  VSIX built: $$VSIX"; \
	echo "============================================================"; \
	echo ""; \
	echo "Before running 'vsce publish', the human operator MUST:"; \
	echo ""; \
	echo "  1. Install locally:"; \
	echo "       code --install-extension $$VSIX --force"; \
	echo ""; \
	echo "  2. Reload VS Code and open a KNOWN-WORKING workspace"; \
	echo "     (the specter/ repo itself — 14 dogfood specs)."; \
	echo "     Verify: Coverage sidebar populates with spec entries."; \
	echo ""; \
	echo "  3. Reload VS Code and open a KNOWN-FAILING workspace"; \
	echo "     (one whose specs do not parse Specter's schema)."; \
	echo "     Verify: sidebar shows the empty-state message AND"; \
	echo "     status bar shows the error state AND Output channel"; \
	echo "     contains the parse errors."; \
	echo ""; \
	echo "  4. Exercise every changed code path end-to-end."; \
	echo ""; \
	echo "  5. Check the Output channel for unexplained red entries."; \
	echo ""; \
	echo "  6. Sign off."; \
	echo ""; \
	echo "  7. Publish:"; \
	echo "       npx vsce publish --packagePath $$VSIX"; \
	echo "     For non-trivial changes, prefer --pre-release first."; \
	echo ""; \
	echo "Skipping a step is not a shortcut — it's a commitment to"; \
	echo "ship a broken version. See RELEASING.md for the full gate."; \
	echo "============================================================"
