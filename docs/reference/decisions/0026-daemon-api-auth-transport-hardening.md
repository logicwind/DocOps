---
id: "0026"
title: "Daemon API — auth and transport hardening (loopback + socket perms + bearer token + Origin guard)"
date: 2026-04-18
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0008", "0012", "0015", "0020"]
tags: [security, access, transport, l4]
---

# 0026 · Daemon API — auth and transport hardening

Date: 2026-04-18
Status: accepted
Implementation: done — all six measures landed on dev branch
2026-04-18 (loopback bind, socket 0600, bearer token, Origin guard,
audit log, logging-discipline test).
Evidence: review of `apps/daemon/src/api/secrets-routes.ts` and
`apps/daemon/src/api/server.ts` while scoping the CLI plugin-decoupling
refactor (see `roadmap/next.md`). Confirmed exposure paths in conversation
2026-04-18.

## Context

ADR 0008 ships the daemon over two transports: a Unix socket and a TCP
port. Its consequences section explicitly flags a deferred concern:

> Security distinction between transports must be enforced at the router
> level when needed (e.g., admin-only routes). Currently all routes are
> available on both.

Since 0008 shipped, the route surface has grown to include endpoints that
handle plaintext secrets:

- `GET /v1/fleet/:name/secrets` — list keys
- `POST /v1/fleet/:name/secrets` — set (plaintext value in body)
- `DELETE /v1/fleet/:name/secrets/:key` — remove
- `GET /v1/fleet/:name/secrets/:key` — **reveal plaintext**
- `POST /v1/fleet/:name/secrets/rotate` — force-recreate consumers

Plus full agent lifecycle control (start, stop, message, restart) and
fleet apply. The file header on `secrets-routes.ts` already states:

> ⚠ SECURITY WARNING: These endpoints expose plaintext secret values
> (reveal endpoint) and allow mutation of the vault. They MUST only be
> served on the Unix socket + 127.0.0.1 TCP interface.

Audit on 2026-04-18 found three concrete gaps against that policy:

1. **TCP server bound to `0.0.0.0`** (Bun's default for `Bun.serve({ port })`).
   Anyone on the LAN — coffee-shop wifi, shared office network, hotel —
   could `curl http://<your-laptop>:4000/v1/fleet/X/secrets/MY_KEY` and
   exfiltrate every secret in the active fleet.
2. **Unix socket created with default umask** (typically `0755` or `0775`).
   On a multi-user host, any other local user could read/write secrets and
   control agents through the socket.
3. **No authentication on TCP at all.** Any process on the host (including
   a malicious npm script in another project the user is editing) can hit
   `localhost:4000` and operate the daemon. Loopback is *not* an
   authentication mechanism.

A fourth concern is forward-looking: as the console UI matures, browser
contexts pointing at the daemon become a real CSRF target. A malicious
website the user happens to be visiting can `fetch("http://localhost:4000/...")`
without a custom header or preflight if we don't actively reject it.

These are pre-existing exposure paths — the trigger to address them now
is the planned CLI plugin-decoupling refactor (`roadmap/next.md`), which
will move secret `set`/`get`/`list`/`remove` from direct vault import to
HTTP. The HTTP path must be safe to carry plaintext secrets routinely
before that refactor lands.

## Decision

Four hardening measures, layered. Each closes one of the gaps above.

### 1. TCP listens on loopback only

```typescript
Bun.serve({
  port,
  hostname: "127.0.0.1",  // never 0.0.0.0
  fetch: router,
});
```

Until measure (3) lands, this is non-negotiable. Once bearer-token auth is
in place, an `--insecure-bind-all` escape hatch can be considered for
narrow use cases (LAN dashboard from a phone), but defaults stay loopback.

### 2. Unix socket is `chmod 0600` immediately after bind

```typescript
const socketServer = Bun.serve({ unix: socketPath, fetch: router });
chmodSync(socketPath, 0o600);
```

The socket is the primary auth boundary for local CLI use (per ADR 0008:
"if you can access the Unix socket, you're authorized"). For that
statement to hold, the socket must actually be owner-only.

### 3. Per-user bearer-token auth on TCP

On first boot, the daemon writes a random 32-byte token (base64) to
`~/.zund/auth.token` with mode `0600`. Every TCP request must carry
`Authorization: Bearer <token>` or get `401 Unauthorized`. The Unix
socket is exempt — its filesystem perms are the auth.

```typescript
function requireAuth(req: Request, transport: "unix" | "tcp"): Response | null {
  if (transport === "unix") return null; // socket perms = auth
  const header = req.headers.get("authorization");
  if (header !== `Bearer ${state.authToken}`) {
    return Response.json({ error: "unauthorized" }, { status: 401 });
  }
  return null;
}
```

CLI and console read the token from the same file and add the header
automatically via the shared `transport/client.ts` HTTP client. Users
never see the token in normal operation.

Token rotation: deleting the file and restarting the daemon regenerates
it. Documented but not automated — explicit ops action.

### 4. Origin guard on browser-mutable routes

Reject TCP requests where the `Origin` header is present and not in an
allow-list (default: empty; opt-in via config). Blocks CSRF from
arbitrary websites the user happens to visit:

```typescript
function checkOrigin(req: Request): Response | null {
  const origin = req.headers.get("origin");
  if (origin === null) return null; // non-browser client (curl, CLI)
  if (state.originAllowList.has(origin)) return null;
  return Response.json({ error: "origin not allowed" }, { status: 403 });
}
```

The console adds its own origin to the allow-list at boot. CLI and other
non-browser clients omit `Origin` entirely, so they pass.

### 5. Audit log for secret operations

Append-only file at `~/.zund/audit.log`, one JSON line per mutation/reveal:

```json
{"ts":"2026-04-18T12:34:56Z","fleet":"work","op":"reveal","key":"OPENAI_KEY","transport":"unix"}
```

Separate from the structured logger so it survives log rotation and is
easy to ship to an external SIEM later. Never includes the value.

### 6. Logging discipline test

A snapshot test that scans every route handler under `/v1/fleet/*/secrets/*`
and asserts no `log.*` call passes the request body or response value.
Prevents future careless `log.debug(req.body)` from regressing.

## Implementation

**All six measures shipped 2026-04-18:**

- Measure 1 — `apps/daemon/src/api/server.ts` binds TCP to `127.0.0.1`.
- Measure 2 — same file, `chmodSync(socketPath, 0o600)` immediately after
  socket bind.
- Measure 3 — `apps/daemon/src/auth.ts` (`loadOrCreateAuthToken`,
  `checkTcpAuth`); `apps/daemon/src/api/server.ts` builds a separate
  router per transport so the Unix-socket router skips auth and the TCP
  router enforces it. CLI client (`apps/cli/src/transport/client.ts`)
  reads `~/.zund/auth.token` and attaches `Authorization: Bearer`. The
  console proxy (`apps/console/server.ts`) does the same and strips the
  browser's `Origin` header server-side.
- Measure 4 — `checkTcpAuth` in `apps/daemon/src/auth.ts` rejects TCP
  requests with disallowed `Origin` (allow-list from
  `ZUND_ALLOWED_ORIGINS` env var; empty by default since the console
  proxy strips Origin and is the trusted client).
- Measure 5 — `apps/daemon/src/audit.ts` (`recordSecretOp`) writes
  one JSON line per mutation/reveal/rotate to `~/.zund/audit.log`.
  Wired into all four mutating handlers in `secrets-routes.ts`.
- Measure 6 — `apps/daemon/test/unit/auth.test.ts` includes a static
  scan that fails if any `log.*` call in `secrets-routes.ts` mentions
  `body`, `value`, `plaintext`, or `req.json`.

**Out of scope for this ADR (deferred):**

- TLS on TCP. Loopback-only + bearer token is sufficient for v1; TLS
  matters when we revisit LAN/remote access.
- Token rotation automation, multi-token support, scoped tokens
  (read-only vs admin). Single per-user token is the v1.
- mTLS for cross-host scenarios. Belongs to a future federation ADR.
- Replacing bearer tokens with PASETO/JWT. Bearer token is simpler and
  the daemon never federates outward.

## Consequences

**Makes easier:**

- The CLI plugin-decoupling refactor (`roadmap/next.md`) becomes safe to
  pursue — plaintext secrets transit a properly authenticated channel,
  not the open internet via accidental `0.0.0.0` exposure.
- `secrets-routes.ts:6`'s file-header policy ("MUST only be served on the
  Unix socket + 127.0.0.1 TCP interface") becomes machine-enforced rather
  than aspirational.
- The console can move from "trust loopback" to "present a token" with no
  protocol change — same HTTP, just one more header.
- A clear story for "how do I know this daemon is mine" exists for future
  multi-tenant or shared-host scenarios.

**Makes harder:**

- LAN/phone access to the console no longer works out of the box. Users
  who relied on `http://laptop-ip:4000` from a phone need to SSH-tunnel
  or wait for the post-token `--insecure-bind-all` escape hatch.
- Docker-on-Mac/Linux dev workflows that called the daemon from a
  container via `host.docker.internal:4000` now need to mount the socket
  (`-v ~/.zund/zundd.sock:/zundd.sock`) or use `--network=host`.
- Multi-user shared-machine setups (one zundd per host, multiple OS
  users) no longer work via the socket. This was a security hole before;
  it is now an explicit configuration error.
- Two new failure modes: (a) missing/unreadable token file → CLI gets
  401 from TCP and must fall back to socket or surface a clear error;
  (b) misconfigured Origin allow-list → console requests get 403 with
  no obvious cause. Both need clear error messages.

**Breaking changes for users on this branch:**

- Anyone hitting the daemon over TCP from a non-loopback address breaks
  immediately. Connection refused, no helpful message — just a network
  error. Mitigation: changelog note + a startup log line that prints the
  bind address explicitly so operators see it.
- Once measure 3 ships, anyone using a custom HTTP client that doesn't
  read `~/.zund/auth.token` breaks. CLI and console handle this
  transparently; bespoke integrations need to be updated.

## Alternatives considered

- **TLS + client certs.** Heavier-weight, useful when the daemon is
  exposed beyond loopback. Deferred to a federation ADR.
- **Unix-socket-only (drop TCP entirely).** Would simplify auth (file
  perms only) but breaks the console — Next.js dev server can't easily
  proxy through a Unix socket on all platforms, and browsers can't speak
  Unix sockets. Keeping TCP is the right call.
- **PAM / system auth.** Overkill for a single-user developer tool. The
  per-user bearer token achieves the same outcome with one file.
- **Capability-scoped tokens (read-only, admin, secrets-only).** Useful
  later when the daemon serves multiple human roles or scripted CI.
  Single token now; scopes when the use case appears.
