---
title: Publish docops to npm via per-platform optionalDependencies (esbuild/biome/turbo pattern)
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0012]
depends_on: [TP-024]
---

# Publish docops to npm via per-platform optionalDependencies (esbuild/biome/turbo pattern)

## Goal

Implement the npm distribution path committed to in ADR-0012's
"npm distribution addendum": one thin meta-package depended on by
five per-platform packages, where npm's resolver picks exactly the
binary tarball that matches the host. This lets users in
JavaScript-heavy repos do:

```
npm i -D docops      # or pnpm/yarn/bun
npx docops init
```

without us writing a `postinstall` shim that downloads from GitHub
Releases at install time. Match the pattern used by `esbuild`,
`@biomejs/biome`, and `turbo` — those are well-trodden, ci-cache
friendly, and don't trigger npm's `--ignore-scripts` hardening.

Depends on TP-024 because the publishing workflow needs to be
solid (one source of release truth, one tag) before adding a
second distribution channel.

## Acceptance

- Decision recorded (in this task or a thin ADR — see Notes) on:
  - **Package names.** `docops` if the npm name is available,
    otherwise `@logicwind/docops`. Per-platform packages follow
    `@docops/cli-<os>-<arch>` if the `@docops` org is available,
    else `@logicwind/docops-<os>-<arch>`. Verify availability
    before locking the choice.
  - **Bin entry point.** A tiny `bin/docops.js` shim that
    `require()`s the platform package and `execve`s into the
    embedded binary, OR a directly-referenced bin from the
    platform package — match whatever `@biomejs/biome@latest`
    does as of task-start.
- `package.json` (in a new `npm/` subtree, not the Go module root)
  for the meta-package and one per platform-arch combo:
  - darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-x64.
- Each platform package's `package.json` carries `os`/`cpu`
  fields so npm only installs the matching one.
- The meta-package's `optionalDependencies` lists all five with the
  exact same version as the goreleaser release.
- goreleaser publishes the npm packages on tag push. Either:
  (a) goreleaser `publishers:` stage shells out to `npm publish`
  with the per-platform tarballs, or
  (b) a separate post-release GitHub Actions step pulls the
  GitHub-Release tarballs and runs `npm publish`. Pick whichever
  is simpler given goreleaser v2's npm support state at task time.
- `NPM_TOKEN` secret added to the repo with publish scope on the
  chosen org/scope.
- A v0.2.x or v0.3.0 tag publishes successfully:
  - `npm view docops version` returns the tag version.
  - `npx docops@<version> --version` on darwin-arm64 prints the
    matching commit/build line — proves the right platform package
    was selected and `bin` works.
- README "Installation" section gains an npm row alongside brew/scoop.

## Notes

- **Naming sub-decision** — if `docops` and `@docops` are both
  free, lock both today and squat. If either is taken, draft a
  one-page ADR confirming the fallback (`@logicwind/docops`) so
  future contributors don't second-guess.
- **Why not postinstall download:** breaks `npm ci --ignore-scripts`
  (increasingly common in security-conscious orgs), invisible to
  npm audit / lockfile, harder to mirror behind a private registry.
  optionalDependencies sidesteps all three.
- **Lockstep versioning is a hard constraint.** Every release tag
  must publish all six packages at the same version or `npm i`
  will install a meta-package whose optionalDeps point at a missing
  version. goreleaser's templating handles this naturally if we
  use `{{ .Version }}` in the package.json synthesis.
- **No TypeScript dependency.** The meta-package's `bin/docops.js`
  is plain CommonJS. We do not ship a TypeScript build step here;
  this is a delivery channel, not application code.
- **Release smoke test idea:** add a CI job that, after the
  release workflow succeeds, runs `npm install docops@<tag>` on
  ubuntu-latest in a clean container and invokes
  `node_modules/.bin/docops --version`. Catches "platform package
  didn't get published" silently.
- **Out of scope:** TypeScript types (no public Node API),
  programmatic JS API for docops commands, sourcemaps. Users
  interact with the CLI; the npm package is just a delivery
  vehicle for the binary.
