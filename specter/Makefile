.PHONY: build test test-race lint fmt fmt-check vulncheck check clean dogfood prerelease install-hooks version-sync publish-vscode release-check doc-style doc-style-changed doc-style-grades

BINARY := bin/specter
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/specter/

test:
	go test ./...

test-verbose:
	go test -v ./...

# Race condition detector, slower but catches concurrency bugs
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

# Documentation style gate (shared Hanalyx checker, v6).
# Enforces the prohibited list from dev/DEVELOPER_DOCUMENTATION_STYLE_GUIDE: em dashes, emojis,
# AI speak, British spellings, and reading level. Grade 10 is the writing target; the failing
# gate is 11.0, derived from this repo's own corpus (see the comment in the script).
#
# doc-style          the whole tree. 537 em-dash findings of existing debt, so it fails today.
# doc-style-changed  only files changed against main. This is the ratchet, and what CI should run:
#                    nobody has to clean the whole tree, but nobody can edit a file and leave it
#                    below standard.
# doc-style-grades   the reading-level distribution, for re-deriving the gate.
doc-style:
	python3 scripts/check-doc-style.py --selftest
	python3 scripts/check-doc-style.py --all

# MUST run from the repository root. `git diff --name-only` returns paths relative to the repo
# root (`specter/README.md`), and the checker opens them relative to its own working directory.
# Run it from `specter/` and every path resolves to nothing, whereupon it reports the files clean
# instead of failing. Measured 2026-08-09: identical input gave "2 file(s) clean" from `specter/`
# and 66 findings from the root. Filed as bugs/SP-HP-026.
doc-style-changed:
	python3 scripts/check-doc-style.py --selftest
	cd .. && python3 specter/scripts/check-doc-style.py --changed

doc-style-grades:
	python3 scripts/check-doc-style.py --grades

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

# Run specter against its own specs (full pipeline).
# This is the lightweight annotation-counting gate: coverage and sync run
# with --strictness annotation so the target stays independent of
# .specter-results.json (the manifest default `threshold` routes both
# commands through the strict path, which requires the results file,
# spec-coverage C-31 / spec-sync C-06). The strict-pipeline variant is
# `make dogfood-strict` below.
dogfood: build
	./$(BINARY) parse specs/*.spec.yaml
	./$(BINARY) resolve
	./$(BINARY) check
	./$(BINARY) coverage --strictness annotation
	./$(BINARY) sync --strictness annotation

# Mechanical eval gate. Specter dogfoods its own --strict on itself.
# Runs go test -json and jest (with jest-junit reporter), feeds both
# streams into specter ingest, then runs specter coverage --strict.
# Every annotated AC must have a passing test result or the gate fails.
# Artifacts (.specter-results.json, junit.xml) are gitignored.
dogfood-strict: build
	@echo "--- Go tests → go test -json ---"
	go test -json ./... 2>/dev/null > .specter-go-results.json || true
	@echo ""
	@echo "--- VS Code extension tests → jest + jest-junit ---"
	cd vscode-extension && npm test
	@echo ""
	@echo "--- specter ingest (merge both streams) ---"
	./$(BINARY) ingest --go-test .specter-go-results.json --junit vscode-extension/junit.xml --output .specter-results.json
	@echo ""
	@echo "--- specter coverage --strict ---"
	./$(BINARY) coverage --strict

# Full pre-release gate. Run this before every release
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

# Release gate. Packages the VSIX and prints the mandatory human-verification
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
	echo "     (the specter/ repo itself, 15 dogfood specs)."; \
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
	echo "Skipping a step is not a shortcut, it's a commitment to"; \
	echo "ship a broken version. See RELEASING.md for the full gate."; \
	echo "============================================================"
