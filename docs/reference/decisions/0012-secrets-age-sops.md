---
id: "0012"
title: Secrets — age + sops with pluggable backend
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0015"]
tags: [state, secrets, l2]
---

# 0012 · Secrets — age + sops with pluggable backend

Date: 2026-04-16
Status: accepted
Related: ADR 0015 (L2 pluggability)

## Context

Agents need API keys (Anthropic, OpenAI, OpenRouter, third-party APIs for
skills). Requirements:

- Secrets must be committable to git (or at least visible in the fleet
  folder) so teams can version-control them.
- The encrypted form must be safe to leak (commit, share, etc.).
- Backend must be swappable: age for local/CI, cloud KMS for managed
  deployments, Vault for enterprise.
- UX: users never touch crypto primitives. `zund secret set X Y`, done.
- Secrets are scoped to roles, skills, and agents with clear declaration
  of what each needs.

## Decision

Use **age** (encryption primitive) + **sops** (key management layer).

**Defaults for v0.3:**

- age key auto-generated at init → `~/.zund/age.key`
- `fleet/.sops.yaml` declares which recipients can decrypt
- `fleet/secrets/keys.yaml` holds encrypted values, git-safe

**User experience:**

```bash
zund init                           # creates age key + .sops.yaml
zund secret set anthropic-key "sk-..."  # → encrypted in keys.yaml
zund secret list                    # shows names, not values
zund secret rm anthropic-key
```

**Declaration model:**

- **Role** declares what secrets the persona needs.
- **Skill** declares what secrets it needs.
- **Agent** maps declared names to encrypted store entries.

Validation at apply time: if a role/skill declares a secret that no agent
maps, apply fails loudly.

**Apply flow:** daemon decrypts at apply time using the age key; injects
plaintext as env vars into containers. Containers never see the encrypted
store.

**Backend switching:** `fleet/.sops.yaml` controls the backend.

```yaml
# age → AWS KMS (change the recipient)
creation_rules:
  - kms: "arn:aws:kms:us-east-1:123:key/abc"

# age → Vault
creation_rules:
  - hc_vault_transit_uri: "https://vault.example.com/v1/transit/keys/zund"
```

`zund secret rotate` re-encrypts under the new backend. No zundd code changes.

**Remote server setup:** paste the age key once on a new server; subsequent
applies decrypt locally.

**CI:** provide the age key via env var.

## Consequences

**Makes easier:**

- Secrets in git are safe by default (encrypted).
- Backend migration is a config change.
- Standard tools (age, sops) — no bespoke crypto.
- Validation catches misconfiguration at apply time, not runtime.

**Makes harder:**

- age key loss = secret loss. Backup story must be documented.
- sops adds one more tool to install. Acceptable — it's widely packaged.
- First-time users encounter crypto concepts (recipients, key rotation).
  Hidden by the `zund secret` commands but surfaces on advanced workflows.

## Implementation notes

- `SecretStore` contract: `packages/core/src/contracts/secrets.ts`
  (including `SecretStoreError` base class for backend-agnostic error
  handling).
- Default impl: `packages/plugins/secrets-age-sops/src/store.ts`
  (SopsSecretStore) wrapping `vault.ts` (age/sops spawn).
- Pure resolver + consumer helpers: `packages/core/src/secrets.ts`
  (no backend knowledge — any SecretStore feeds the same logic).
- API route: `apps/daemon/src/api/secrets-routes.ts`.
- CLI: `apps/cli/src/commands/secret/{set,get,list,remove,rotate}.ts`.
  `rotate` hits the daemon's `/v1/fleet/:name/secrets/rotate` endpoint
  to recreate running consumers with fresh values.
- Uses age + sops binaries; the plugin spawns them as subprocesses.
- The pluggable backend is the sops provider interface, not a
  Zund-defined abstraction. Zund holds the convention; sops owns the
  primitive. Hosted/KMS backends implement `SecretStore` directly and
  bypass sops entirely.
- Operator docs: [`docs/reference/guides/secrets.md`](../guides/secrets.md)
  covers age key backup, remote server bootstrap, and sops recipient
  rotation.
