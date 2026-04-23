BINARY := docops
PKG    := github.com/logicwind/docops
VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
  -X $(PKG)/internal/version.Version=$(VERSION) \
  -X $(PKG)/internal/version.Commit=$(COMMIT) \
  -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: build install test lint clean tidy release-snapshot release

build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/docops

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/docops

test:
	go test -race ./...

lint:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin dist

release-snapshot:
	goreleaser release --snapshot --clean

# make release VERSION=X.Y.Z
#   Bumps the VERSION file (read by docops update-check via raw.githubusercontent.com),
#   commits the bump, creates an annotated v$VERSION tag, and pushes both.
#   The tag push triggers .github/workflows/release.yml → goreleaser builds and
#   publishes the GitHub Release. Pass DRY_RUN=1 to print what would happen
#   without writing or pushing anything.
release:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "usage: make release VERSION=X.Y.Z [DRY_RUN=1]"; exit 2; \
	fi
	@echo "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || \
		(echo "VERSION must match X.Y.Z (got $(VERSION))" && exit 2)
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "working tree not clean; commit or stash first"; exit 2; \
	fi
	@branch=$$(git rev-parse --abbrev-ref HEAD); \
	 if [ "$$branch" != "main" ]; then \
		echo "refusing to release from '$$branch' — switch to main first"; exit 2; \
	 fi
	@if git rev-parse "v$(VERSION)" >/dev/null 2>&1; then \
		echo "tag v$(VERSION) already exists locally"; exit 2; \
	fi
	@if git ls-remote --exit-code --tags origin "refs/tags/v$(VERSION)" >/dev/null 2>&1; then \
		echo "tag v$(VERSION) already exists on origin"; exit 2; \
	fi
	@if [ -n "$(DRY_RUN)" ]; then \
		echo "[dry-run] would write '$(VERSION)' to VERSION"; \
		echo "[dry-run] would commit: chore: release v$(VERSION)"; \
		echo "[dry-run] would tag: v$(VERSION)"; \
		echo "[dry-run] would push: main + v$(VERSION) to origin"; \
		exit 0; \
	fi
	echo "$(VERSION)" > VERSION
	git add VERSION
	git commit -m "chore: release v$(VERSION)"
	git tag -a "v$(VERSION)" -m "v$(VERSION)"
	git push origin main
	git push origin "v$(VERSION)"
	@echo
	@echo "v$(VERSION) tagged and pushed. Watch the release workflow:"
	@echo "  gh run watch"
