---
name: onboard
description: Bootstrap CTX + ADRs from an existing codebase. Scans top-level layout, runs a short goal interview, and drafts CTX-001 plus 1-3 ADRs from load-bearing decisions visible in the code. Use after `docops init` on a brownfield repo or when the user says "onboard" / "bootstrap from existing code".
---

# Cookbook: onboard

## Context
Brownfield on-ramp. The repo already has code, history, and implicit
decisions; DocOps is new. Goal: produce a credible **CTX-001**
(vision + goals) and **1–3 ADRs** for decisions the code already
encodes (framework, auth, datastore, deploy target). Drafts only —
the user reviews and edits before publishing.

Hard rules:
- Code-evidence only. Don't invent ADRs from external knowledge.
- Always confirm before writing. Two confirms: one after the inferred
  summary, one after the drafts.
- Keep ADRs `status: draft`. The user flips to `accepted` later.

## Input
None required. Optional intent signals from the user:
- `--auto` / "skip the interview" — go straight from scan to drafts.
- A specific decision area to focus on ("just auth", "skip frontend").

## Steps

1. **Scan the repo top-level only.** No recursive walks.

   ```
   ls -la
   cat README.md 2>/dev/null | head -80
   git log --oneline -20
   ```

   Read whichever package manifests exist (`package.json`, `go.mod`,
   `Cargo.toml`, `pyproject.toml`, `Gemfile`, `pom.xml`,
   `composer.json`, `requirements.txt`). Note primary directories
   (`src/`, `app/`, `cmd/`, `internal/`, `services/`).

2. **Show the inferred summary.** One short paragraph back to the
   user, e.g.:

   > Looks like a Next.js 14 app on Vercel, ~340 commits, TypeScript +
   > Tailwind, auth via Clerk, primary surface is `/app/dashboard`.
   > Tests in Playwright. No backend service in this repo.

   **Stop and confirm** that the read is roughly right before
   continuing. The user may correct ("we're moving off Clerk", "the
   backend is in another repo").

3. **Goal interview** — at most 3–5 questions, asked in one batch so
   the user answers all at once. Skip entirely if `--auto` was passed
   or the user said skip.

   Use exactly these prompts (cut to fit; never ask all five if a
   short answer would cover several):

   - Who uses this and what do they actually do with it?
   - What's the most painful problem in the codebase right now?
   - What's the next 3-month goal — feature, scale, migration, other?
   - Any hard constraints (compliance, perf budget, team size,
     budget, deadline)?
   - Anything load-bearing that's *not* visible in the code I should
     know about?

4. **Draft CTX-001** from the answers + scan. Sections: Audience,
   Goal, Constraints, Out of scope. Keep it under ~250 words. Use
   `--type brief`.

5. **Draft 1–3 ADRs** — only for decisions the code clearly already
   encodes. One ADR per call (framework, auth, deploy target,
   datastore). Every ADR has Context / Decision / Rationale /
   Consequences and stays `status: draft`. Skip if the evidence is
   weak; better fewer ADRs than speculative ones.

6. **Show drafts inline** before any write. Ask: ship / iterate /
   abort. On iterate, return to step 4 or 5 with the user's edits.

7. **Write via heredocs** so drafts land populated:

   ```
   docops new ctx "Project brief" --type brief --body - <<'EOF'
   ## Audience
   ...
   ## Goal
   ...
   ## Constraints
   ...
   ## Out of scope
   ...
   EOF

   docops new adr "Framework: Next.js 14 App Router" --body - <<'EOF'
   ## Context
   The codebase uses Next.js 14 App Router with React Server Components.
   ## Decision
   We commit to Next.js App Router as the rendering substrate.
   ## Rationale
   ...
   ## Consequences
   ...
   EOF
   ```

8. **Refresh and report:**

   ```
   docops refresh
   ```

   The CLI prints a `→ Next:` block; surface it back to the user
   verbatim. If they want to keep going, point at:

   - `docops new task "..." --requires <ADR-NNNN>` to capture today's
     actual work.
   - `docops audit` to see structural gaps that just appeared.

## Confirm
Report back: CTX ID written, list of ADR IDs (with `status: draft`),
the validator was clean, and the two or three concrete commands the
user is most likely to want next.
