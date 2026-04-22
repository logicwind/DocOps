# Secret Management in Zund

This document covers how secrets flow from an encrypted vault into running
agent containers. For the architectural decision behind age + sops and the
pluggable-backend direction see
[`reference/decisions/0012-secrets-age-sops.md`](../decisions/0012-secrets-age-sops.md).

---

## 1. Goal and non-goals

**Goal:** `zund apply` decrypts `fleet/secrets/keys.yaml` once at apply time,
validates that every active agent's required env vars are satisfied, and
injects the resolved plaintext values as container environment variables.
Agents read keys from `process.env` like any normal CLI tool. No secrets
appear in logs, fleet YAML, or rollback state.

**Non-goals (explicitly parked):**

- Multi-provider secret registries (HashiCorp Vault, AWS Secrets Manager).
- The `exec` source provider (1Password CLI / `op run` pattern).
- Re-decryption on live config reload (not a zund flow today).
- Partial-apply when some agents have resolution errors (all-or-nothing).

---

## 2. Lifecycle

```
zund init
  └─► generates ~/.zund/age.key  (AGE-SECRET-KEY-1... + pubkey comment)
      writes fleet/.sops.yaml    (age recipient = pubkey)

zund secret set anthropic-key sk-ant-test-123
  └─► readAllSecrets() decrypts fleet/secrets/keys.yaml (if it exists)
      adds/updates the key-value pair
      writeVault() re-encrypts the whole file with SOPS

fleet/secrets/keys.yaml  ← SOPS-encrypted, git-safe
  ANTHROPIC_API_KEY: ENC[AES256_GCM,data:...,iv:...,tag:...,type:str]
  GITHUB_TOKEN: ENC[AES256_GCM,...]

zund apply
  └─► readAllSecrets({ fleetDir }) — decrypt once, get plaintext map
      resolveFleetSecrets(activeAgents, vault, roles, skills, envLookup)
        └─► build snapshot: Record<agentName, Record<envVar, plaintext>>
            collect errors: ResolutionError[]
      if errors → abort, report all errors, launch nothing
      per agent: launchAgent(..., envVars: { ZUND_API_URL, ...snapshot[name] })
        └─► setContainerConfig environment.ENV_VAR = plaintext
            writePiConfig pulls ANTHROPIC_API_KEY from envVars
```

**Directory layout:**

```
~/.zund/age.key              private age key (mode 0600, never committed)
fleet/
  .sops.yaml                 { creation_rules: [{ age: "<pubkey>" }] }
  secrets/
    keys.yaml                SOPS-encrypted flat map (committed to git)
  agents/
    writer.yaml              references secrets by name
```

---

## 3. Resource schema

### Role / skill declaring requirements

```yaml
kind: role
name: writer-role
secrets:
  required:
    - ANTHROPIC_API_KEY     # agent MUST map this or apply aborts
  optional:
    - OPENAI_API_KEY        # missing mapping is silently skipped
```

### Agent mapping requirements to secret names

String shorthand — coerces to `{ source: vault, id: anthropic-key }`:

```yaml
kind: agent
name: writer
role: writer-role
secrets:
  ANTHROPIC_API_KEY: anthropic-key   # looks up "anthropic-key" in vault
```

Explicit object form — read from host environment instead:

```yaml
secrets:
  STRIPE_KEY:
    source: env
    id: STRIPE_API_KEY               # reads process.env.STRIPE_API_KEY on host
```

### Defaults inheritance

```yaml
kind: defaults
agent:
  secrets:
    ANTHROPIC_API_KEY: anthropic-key  # inherited by every agent unless overridden
```

`applyDefaults` merges these before `resolveFleetSecrets` runs, so the
resolver only ever sees the fully-merged agent.

---

## 4. Resolution algorithm

`resolveFleetSecrets` in `packages/core/src/secrets.ts` is pure and
synchronous — all I/O happens before it is called. It lives in `@zund/core`
so any `SecretStore` implementation (age+sops, Vault, KMS) feeds into the
same resolution logic once it's produced a plaintext `vault` snapshot.

Steps for each active agent:

1. **Build required set** — union of `role.secrets.required` (agent's role)
   and `skill.secrets.required` for each skill the agent actually uses.
   Skills not referenced by the agent contribute nothing.
2. **Build optional set** — same union logic for `.optional` lists.
3. **For each required env var:**
   - If no mapping in `agent.secrets` → `ResolutionError` with reason
     matching `/no mapping/i`.
   - If mapping present: resolve the ref (vault lookup or `envLookup` call).
     Empty or missing → `ResolutionError`.
4. **For each optional env var:**
   - No mapping → silently skipped (no error).
   - Mapping present but resolution fails → `ResolutionError`.
5. **Extra ad-hoc mappings** on the agent (not in any required/optional list)
   are resolved like optional entries.
6. **Any error for an agent** → that agent is absent from `snapshot`.
   All errors from all agents are collected before returning (never fail-fast).

The executor aborts the entire apply if `errors.length > 0`. No containers
are created or destroyed until all secrets are confirmed resolvable.

---

## 5. Provider model

Two sources are supported today:

| Source  | Schema                              | Resolution                              |
| ------- | ----------------------------------- | --------------------------------------- |
| `vault` | `"secret-name"` or `{ source: "vault", id: "secret-name" }` | Look up `id` in the decrypted SOPS map. |
| `env`   | `{ source: "env", id: "ENV_VAR" }`  | Call `envLookup("ENV_VAR")` (executor passes `process.env[n]`). |

**Future `exec` provider (parked):** `{ source: "exec", id: "op://vault/item/field" }`
would shell out to a provider CLI (1Password `op`, Vault CLI, etc.) using a
JSON-over-stdio protocol similar to OpenClaw's `openclaw secrets exec` design.
The `SecretRef` discriminated union in `fleet/types.ts` has room for this
extension without a schema revision — add `"exec"` to the `source` union and
a new branch in `resolver.ts::resolveRef`.

### Environment gotchas

Three things commonly trip people up and none of them are visible in the
schema:

1. **`source: env` reads the daemon's environment, not the CLI's.** The
   resolver runs inside `zundd`, so `envLookup` dereferences
   `process.env` on the daemon process. If you `export STRIPE_API_KEY=…`
   in the shell that runs `zund apply`, but the daemon was started
   earlier in a different shell, the var is invisible. You must set
   env-source secrets in the environment that launches `zundd` — a
   systemd unit, a shell rc file, or a foreground `SOPS_AGE_KEY_FILE=…
   STRIPE_API_KEY=… bun apps/daemon/src/index.ts`. Rule of thumb:
   if you can't see the var in `ps e <pid-of-zundd>`, the resolver
   can't either.

2. **`SOPS_AGE_KEY_FILE` is also a daemon-side concern.** `sops -d`
   runs inside `zundd` (and inside the CLI for `zund secret set/get`).
   Both processes need `SOPS_AGE_KEY_FILE` pointing at a readable age
   key. The vault module defaults to `~/.zund/age.key`, so most users
   never notice — but under custom HOME setups or when running `zundd`
   as a different user, export it explicitly.

3. **Changing a vault secret without re-applying does nothing.**
   Containers get plaintext injected via `incus config set
   environment.*` at launch time. Updating `fleet/secrets/keys.yaml`
   after the fact does not propagate to running containers. Use
   `zund secret rotate` to recreate every running agent that references
   vault secrets, or `zund secret rotate --keys=FOO,BAR` to limit the
   blast radius to specific keys. Note that the resolver skips
   re-resolution on a no-diff apply, so `zund apply` alone isn't enough
   after `zund secret set`.

---

## 6. Security invariants

- **No plaintext in logs.** The resolver never logs secret values. The
  executor logs `hasKey: !!apiKey` (boolean) not the key itself.
- **No plaintext in rollback backups.** `~/.zund/fleet-state.yaml` persists
  `Resource[]` (the desired YAML shape), which contains secret *names* (e.g.
  `anthropic-key`), not plaintext values. Rollback never re-exposes a secret.
- **Ref beats plaintext.** If a future schema revision allows a literal
  plaintext value alongside a ref, the ref wins. The resolver's coercion rule
  (string → vault ref) enforces this — bare strings are always treated as
  vault names, never as literal secrets.
- **Active-surface only.** Only agents that will be launched in the current
  apply are validated. A disabled or unchanged agent with a broken secret
  mapping does not block an otherwise healthy apply. (Filtering happens in
  the executor before calling the resolver; the resolver trusts its input.)
- **Vault read is once-per-apply.** `readAllSecrets` is called once at the
  top of `executeFleetPlan` (and once in the preview branch of
  `handleApply`). The plaintext map is in-memory for the duration of the
  apply and then garbage-collected. It is never written to disk.

---

## 7. Upgrade path

The age + SOPS backend is swappable without changing the fleet schema or the
resolver. The upgrade path mirrors `zund-plan.md §3`:

1. **Today:** single age key at `~/.zund/age.key`. Works for solo developers
   and small teams where everyone shares the key out-of-band.
2. **Team key rotation:** add additional age recipients to `.sops.yaml`
   (`age` field accepts a comma-separated list). `sops updatekeys` re-encrypts
   for all recipients. No fleet YAML changes needed.
3. **KMS (AWS/GCP/Azure):** swap the `creation_rules[].age` entry in
   `.sops.yaml` for a `kms`/`gcp_kms`/`azure_kv` entry. SOPS handles the
   rest. The `vault.ts` wrapper calls `sops -d` regardless of backend.
4. **HashiCorp Vault / external secret stores:** add the `exec` source
   provider (see §5) so agents can reference secrets by path rather than
   requiring them to be in the local SOPS file at all.

---

## 8. Operational procedures

### 8.1 Backing up the age key

`~/.zund/age.key` is the only thing standing between an operator and the
encrypted vault. Losing it loses every secret — SOPS cannot decrypt
`fleet/secrets/keys.yaml` without at least one recipient's private key.

Minimum backup discipline:

- **Copy `~/.zund/age.key` to a password manager** (1Password, Bitwarden,
  a hardware token's file slot) the moment `zund init` finishes. The file
  is ~80 bytes of ASCII — it fits in any secure note.
- **Back up with the pubkey intact.** The key file starts with a
  `# public key: age1…` comment. That comment is what lets you rotate
  recipients later without guessing; keep it in the backup.
- **Never commit the key** — `fleet/.sops.yaml` only lists the *public*
  recipient; the private key never goes to git. `.gitignore` patterns
  shipped by `zund init` cover `.zund/age.key` but double-check before
  your first push.

If the key *is* compromised (leaked, exfiltrated, laptop stolen):

1. Generate a fresh key on a clean machine: `age-keygen -o
   ~/.zund/age.key.new`.
2. Add the new pubkey to `fleet/.sops.yaml` *alongside* the old one:
   `creation_rules: [{ age: "<old>,<new>" }]`.
3. Re-encrypt every file for both recipients: `sops updatekeys
   fleet/secrets/keys.yaml`. SOPS rewrites the file in place; diff to
   confirm only envelope data changed.
4. Swap `age.key.new` into `~/.zund/age.key` on every operator machine
   (share out of band — do NOT send over the compromised channel).
5. Remove the old pubkey from `.sops.yaml`, run `sops updatekeys` again
   to drop it from the envelope, commit.
6. **Rotate any plaintext that the compromised key could have decrypted.**
   Re-encrypting under a new recipient does not change the underlying
   secret values — anyone with the old ciphertext + old private key still
   has them. Regenerate API keys, tokens, passwords at the issuer.

### 8.2 Bootstrapping a remote server

Running `zundd` on a fresh VM / server (as opposed to the operator's
laptop) needs the age key to reach that host exactly once. The remote
daemon then decrypts locally on every apply.

**Minimum steps:**

1. On the remote host: install zund and create `~/.zund/` (e.g. via
   `zund init --yes`, which will *generate a fresh age key* that you do
   not want). Immediately delete the generated key:
   ```bash
   rm ~/.zund/age.key
   ```
2. From the operator machine, copy the operator's age key over an
   authenticated channel (SSH with strict host checking, not HTTP):
   ```bash
   scp ~/.zund/age.key user@remote:~/.zund/age.key
   ssh user@remote chmod 600 ~/.zund/age.key
   ```
3. Confirm the pubkey matches what `fleet/.sops.yaml` already lists.
   If the remote is a shared server with multiple operators, each
   operator's pubkey must be in the recipients list (see 8.1 for the
   `sops updatekeys` flow).
4. Start `zundd`. The vault read at apply time uses
   `~/.zund/age.key` by default; the `SOPS_AGE_KEY_FILE` env var
   overrides this if you store keys elsewhere (e.g. inside a secrets
   mount).

**Alternatives for production deployments:**

- **Separate per-host key, no shared private keys.** Generate a
  host-specific age key on the remote, commit *only* its pubkey to
  `fleet/.sops.yaml`, run `sops updatekeys`. The operator still holds
  their own key locally; the remote holds only its host key. Rotation
  is per-host without touching operator laptops.
- **KMS-backed recipient.** Swap age for KMS as described in §7.3 —
  the remote VM's instance role gains `kms:Decrypt` on the fleet key
  and no file-based key lives on the host at all. `zundd` picks up
  decryption through SOPS's KMS provider; no daemon code changes.
- **CI builds.** Provide the age private key via an encrypted CI
  secret (GitHub Actions encrypted secret, GitLab CI variable marked
  `masked + protected`). Export as `SOPS_AGE_KEY` (inline) or write to
  a tmpfs file and set `SOPS_AGE_KEY_FILE` before invoking `zund
  apply`.

### 8.3 Rotating SOPS recipients

Use the `sops` CLI directly — Zund doesn't wrap this. Typical flow:

```bash
# Edit fleet/.sops.yaml, change creation_rules[].age
# to the new recipient list (add, remove, or replace).

sops updatekeys fleet/secrets/keys.yaml

git diff fleet/secrets/keys.yaml
# Expect: only the `sops.age` block envelope changed; ENC[...] values unchanged.

git commit -am "sops: rotate recipients for keys.yaml"
```

No `zundd` restart needed — the next `zund apply` (or
`zund secret rotate`) decrypts using whichever recipient has a key at
`SOPS_AGE_KEY_FILE`.
