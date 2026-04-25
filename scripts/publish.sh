#!/usr/bin/env bash
# scripts/publish.sh — release docops in one shot.
#
# What it does (in order, abort-on-error):
#   1. Validate VERSION (X.Y.Z), clean tree, on dev (or --force-main).
#   2. Sync dev: git pull --ff-only origin dev.
#   3. Fast-forward main from dev: git merge --ff-only dev; git push origin main.
#   4. Run `make release DRY_RUN=1` and show the preview.
#   5. Prompt to confirm.
#   6. Run `make release` for real (creates VERSION commit, tag, pushes).
#   7. Fast-forward dev to main and push.
#   8. Optionally `gh run watch`.
#
# Usage:
#   scripts/publish.sh 0.5.2
#   scripts/publish.sh 0.5.2 --watch       # also tail the release workflow
#   scripts/publish.sh 0.5.2 --yes         # skip the confirm prompt (CI use)
#
# All git pushes go to `origin`. Tag format is `v$VERSION`.

set -euo pipefail

# --- helpers ------------------------------------------------------------------

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }
bold()   { printf '\033[1m%s\033[0m\n' "$*"; }

die() {
	red "publish: $*" >&2
	exit 1
}

confirm() {
	local prompt="$1"
	if [ "${YES:-0}" = "1" ]; then
		return 0
	fi
	printf '%s [y/N] ' "$prompt"
	read -r reply
	case "$reply" in
		y|Y|yes|YES) return 0 ;;
		*) return 1 ;;
	esac
}

# --- args ---------------------------------------------------------------------

VERSION="${1:-}"
shift || true
WATCH=0
YES=0
for arg in "$@"; do
	case "$arg" in
		--watch) WATCH=1 ;;
		--yes|-y) YES=1 ;;
		*) die "unknown flag: $arg" ;;
	esac
done

if [ -z "$VERSION" ]; then
	die "usage: scripts/publish.sh X.Y.Z [--watch] [--yes]"
fi
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	die "VERSION must match X.Y.Z (got $VERSION)"
fi

# --- preflight ----------------------------------------------------------------

# Repo root sanity.
[ -f Makefile ]      || die "no Makefile in $(pwd) — run from repo root"
[ -d cmd/docops ]    || die "no cmd/docops/ in $(pwd) — wrong repo?"

# Clean working tree.
if ! git diff-index --quiet HEAD --; then
	die "tracked files have uncommitted changes; commit or stash first"
fi

# Branch.
branch=$(git rev-parse --abbrev-ref HEAD)
if [ "$branch" != "dev" ]; then
	die "expected to be on 'dev', currently on '$branch' — switch first"
fi

# Tag must not already exist locally or on origin.
if git rev-parse "v$VERSION" >/dev/null 2>&1; then
	die "tag v$VERSION already exists locally"
fi
git fetch origin --tags --quiet
if git ls-remote --exit-code --tags origin "refs/tags/v$VERSION" >/dev/null 2>&1; then
	die "tag v$VERSION already exists on origin"
fi

bold "publish: docops v$VERSION"
echo

# --- 1. sync dev with origin --------------------------------------------------

yellow "→ syncing dev with origin/dev"
git pull --ff-only origin dev

# --- 2. fast-forward main from dev -------------------------------------------

yellow "→ fast-forwarding main from dev"
git checkout main
git pull --ff-only origin main || true   # remote may not have main; tolerated
if ! git merge --ff-only dev; then
	red "main is not a fast-forward of dev — diverged. Resolve manually." >&2
	git checkout dev
	exit 1
fi
git push origin main

# --- 3. dry run release ------------------------------------------------------

echo
yellow "→ running dry run preview"
echo
make release VERSION="$VERSION" DRY_RUN=1
echo

# --- 4. confirm and release --------------------------------------------------

if ! confirm "release v$VERSION for real (this writes VERSION, tags, pushes)?"; then
	yellow "publish: aborted before release. main has been fast-forwarded but no tag was cut."
	yellow "to roll back the main fast-forward, run: git checkout main && git reset --hard origin/main"
	git checkout dev
	exit 1
fi

echo
yellow "→ cutting release v$VERSION"
make release VERSION="$VERSION"

# --- 5. resync dev with main -------------------------------------------------

echo
yellow "→ resyncing dev with main (picks up the VERSION-bump commit)"
git checkout dev
git merge --ff-only main
git push origin dev

# --- 6. done -----------------------------------------------------------------

echo
green "published docops v$VERSION"
echo
echo "Watch the release workflow:"
echo "  gh run watch"
echo

if [ "$WATCH" = "1" ] && command -v gh >/dev/null 2>&1; then
	yellow "→ tailing release workflow (ctrl-c to stop)"
	gh run watch
fi
