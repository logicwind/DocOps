# Configuring zundd

zundd itself is configured separately from fleet YAML. Fleet YAML is
portable — it describes *what you want*. zundd config is environment-specific
— it describes *where you're running*. Different machines running the same
fleet will have different zundd configs.

---

## Config file

Config is loaded from, in priority order:

1. `--config <path>` flag
2. `~/.zund/config.yaml`
3. `/etc/zund/config.yaml`

```yaml
# ~/.zund/config.yaml

# Incus connection
incus:
  socket: /var/lib/incus/unix.socket     # Linux default
  # socket: /Users/nix/.colima/incus/incus.sock  # macOS + Colima
  project: zund                           # Incus project for fleet isolation

# Container defaults
containers:
  defaultImage: zund/base                 # base image for agents

# Data root — all managed data lives under this directory
data:
  dir: ~/.zund/data

# API server
api:
  unixSocket: ~/.zund/zundd.sock          # CLI connects here
  tcp:
    enabled: true
    host: 0.0.0.0
    port: 4000                            # console + remote access

# Logging
log:
  level: info                             # debug | info | warn | error
```

Environment variable overrides:

- `ZUND_TCP_PORT` — overrides `api.tcp.port`
- `ZUND_HOST_API_URL` — override for container-to-daemon reachback

---

## Managed data layout

zundd auto-creates and auto-mounts all per-agent data. Users don't
configure paths — everything lives under `data.dir`.

```
~/.zund/data/
├── sessions/
│   ├── writer/                 # Pi session JSONL files (per agent)
│   │   └── session-abc.jsonl
│   └── reviewer/
│       └── session-def.jsonl
├── memory.db                   # single SQLite (facts + working memory, scoped by row)
├── sessions.db                 # ephemeral index of JSONL files, GC'd per retention
├── workspace/
│   ├── writer/                 # persistent workspace per agent
│   └── reviewer/
├── extensions/                 # Pi extensions (shared across agents)
│   └── zund-fleet.ts
├── skills/
│   ├── brand-voice/            # local — copied from fleet dir on apply
│   │   ├── SKILL.md
│   │   └── references/
│   ├── fleet-tools/            # builtin — shipped with zundd
│   └── social-post/            # git — fetched and cached
│       ├── SKILL.md
│       └── resources/
├── artifacts/
│   └── blobs/<sha[0:2]>/<sha>  # content-addressed blob store
├── _cache/
│   ├── git/                    # raw git clones (shared, deduped)
│   │   └── github.com/company/agent-skills/
│   └── registry/               # registry downloads (versioned)
│       └── company/deploy-to-vercel/
│           ├── 3.0.0/
│           └── 2.1.0/
└── state/
    └── fleet.json              # zundd internal state (container IDs, health)
```

---

## Auto-mounted into every container

Users do not configure these mounts — zundd adds them to every agent
container automatically.

| Host path | Container path | Purpose |
|-----------|---------------|---------|
| `~/.zund/data/sessions/{agent}/` | `/root/.pi/agent/sessions` | Pi session persistence (ADR 0009) |
| `~/.zund/data/workspace/{agent}/` | `/workspace` | Persistent agent workspace |
| `~/.zund/data/extensions/` | `/root/.pi/agent/extensions` | Zund fleet Pi extension |
| `~/.zund/data/skills/{name}/` | `/skills/{name}/` | Per-agent, only assigned skills (readonly) |

Only skills assigned to an agent are mounted into that agent's container.
An agent with `skills: [brand-voice, social-post]` gets two readonly mounts,
not the whole skills directory.

**Memory is NOT mounted.** It runs inside the zundd process. Pi agents call
memory tools via the `zund-fleet` extension, which calls back to zundd's
API. See ADRs 0010 and 0016.

---

## Skill provisioning

Skills are sourced from four places. zundd handles fetch, cache, and mount
on every apply.

| Source | On apply | Stored at |
|--------|----------|-----------|
| `local` | Copied from fleet directory | `skills/{name}/` |
| `builtin` | Already present (shipped with zundd) | `skills/{name}/` |
| `git` | Cloned to `_cache/git/`, extracted | `skills/{name}/` |
| `registry` | Downloaded to `_cache/registry/`, extracted | `skills/{name}/` |

---

## User-configured mounts (optional)

For extra data beyond what zundd manages, agents can declare additional
mounts in their YAML:

```yaml
kind: agent
name: writer
mounts:
  - name: brand-assets
    host: /shared/brand-assets
    readonly: true
```

This mounts the host path at `/data/brand-assets` inside the container.

---

## Zero-config experience

Create an agent, apply, it gets sessions + workspace + extensions + memory
automatically. No paths to configure in the common case.

Configuration is only needed when:

- Running on a non-standard Incus socket (macOS with Colima)
- Changing the API port (`ZUND_TCP_PORT`)
- Relocating the data directory (unusual)
- Restricting TCP exposure (e.g., localhost-only deployments)

---

## Related

- [`reference/daemon.md`](../daemon.md) — full daemon internals, apply
  pipeline, state lifecycle
- [`reference/decisions/0008-dual-bun-serve-instances.md`](../decisions/0008-dual-bun-serve-instances.md) — why Unix + TCP
- [`reference/decisions/0009-session-storage-host-mounted.md`](../decisions/0009-session-storage-host-mounted.md) — session mounts
