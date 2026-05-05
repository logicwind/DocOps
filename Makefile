BINARY := docops
PKG    := github.com/logicwind/docops
VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
  -X $(PKG)/internal/version.Version=$(VERSION) \
  -X $(PKG)/internal/version.Commit=$(COMMIT) \
  -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: build install test lint clean tidy release-snapshot release publish beta

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

# make publish VERSION=X.Y.Z
#   End-to-end release: sync dev, ff main, dry-run preview, confirm, cut tag,
#   push, resync dev. Wraps scripts/publish.sh. Pass YES=1 to skip the
#   confirmation prompt (CI), WATCH=1 to tail the release workflow.
publish:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "usage: make publish VERSION=X.Y.Z [YES=1] [WATCH=1]"; exit 2; \
	fi
	@flags=""; \
	 if [ "$(YES)" = "1" ]; then flags="$$flags --yes"; fi; \
	 if [ "$(WATCH)" = "1" ]; then flags="$$flags --watch"; fi; \
	 ./scripts/publish.sh "$(VERSION)" $$flags

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
	@if ! git diff-index --quiet HEAD --; then \
		echo "tracked files have uncommitted changes; commit or stash first"; exit 2; \
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
	@set -e; \
	if [ -n "$(DRY_RUN)" ]; then \
		echo "[dry-run] would write '$(VERSION)' to VERSION"; \
		echo "[dry-run] would commit: chore: release v$(VERSION)"; \
		echo "[dry-run] would tag:    v$(VERSION)"; \
		echo "[dry-run] would push:   main + v$(VERSION) to origin"; \
		echo; \
		echo "[dry-run] re-run without DRY_RUN=1 to perform the release."; \
		exit 0; \
	fi; \
	echo "$(VERSION)" > VERSION; \
	git add VERSION; \
	git commit -m "chore: release v$(VERSION)"; \
	git tag -a "v$(VERSION)" -m "v$(VERSION)"; \
	git push origin main; \
	git push origin "v$(VERSION)"; \
	echo; \
	echo "v$(VERSION) tagged and pushed. Watch the release workflow:"; \
	echo "  gh run watch"

# make beta VERSION=X.Y.Z-(alpha|beta|rc).N
#   Cuts a prerelease tag from the current branch. Does NOT bump VERSION
#   (that file tracks latest stable; prereleases must not move it). CI's
#   release workflow detects the prerelease tag, lets goreleaser route the
#   formula write to docops@beta / docops-beta only — stable users see
#   nothing. See ADR-0032. Pass DRY_RUN=1 to preview.
beta:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "usage: make beta VERSION=X.Y.Z-(alpha|beta|rc).N [DRY_RUN=1]"; exit 2; \
	fi
	@echo "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+-(alpha|beta|rc)\.[0-9]+$$' || \
		(echo "VERSION must match X.Y.Z-(alpha|beta|rc).N (got $(VERSION))" && exit 2)
	@if ! git diff-index --quiet HEAD --; then \
		echo "tracked files have uncommitted changes; commit or stash first"; exit 2; \
	fi
	@if git rev-parse "v$(VERSION)" >/dev/null 2>&1; then \
		echo "tag v$(VERSION) already exists locally"; exit 2; \
	fi
	@if git ls-remote --exit-code --tags origin "refs/tags/v$(VERSION)" >/dev/null 2>&1; then \
		echo "tag v$(VERSION) already exists on origin"; exit 2; \
	fi
	@set -e; \
	branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ -n "$(DRY_RUN)" ]; then \
		echo "[dry-run] branch:        $$branch"; \
		echo "[dry-run] would tag:     v$(VERSION) (annotated)"; \
		echo "[dry-run] would push:    v$(VERSION) to origin"; \
		echo "[dry-run] VERSION file:  unchanged (stable channel)"; \
		echo; \
		echo "[dry-run] re-run without DRY_RUN=1 to cut the prerelease."; \
		exit 0; \
	fi; \
	git tag -a "v$(VERSION)" -m "v$(VERSION) (prerelease)"; \
	git push origin "v$(VERSION)"; \
	echo; \
	echo "v$(VERSION) tagged and pushed from $$branch."; \
	echo "Watch the release workflow:  gh run watch"; \
	echo "Then:  brew upgrade logicwind/tap/docops@beta  (or scoop update docops-beta)"
