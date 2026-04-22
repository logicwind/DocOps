Here's a full manual walkthrough, grouped by what works out of the box vs. what needs extra setup. Run top-to-bottom in a fresh terminal.

Setup (no daemon needed yet)

cd /Users/nix/dev/lw/zund

# 1. Smoke the binary

bun apps/cli/src/index.ts --version # → 0.3.0
bun apps/cli/src/index.ts --help # lists all top-level commands
bun apps/cli/src/index.ts agent --help # nested help works

# 2. Alias the binary for convenience

alias zund="bun /Users/nix/dev/lw/zund/apps/cli/src/index.ts"
zund --version

Init flow

# 3. Scripted init into a scratch project

cd /tmp && rm -rf zund-demo
zund init zund-demo --yes
ls zund-demo # zund.config.yaml + fleet/
cat zund-demo/zund.config.yaml # verify fleet + daemon block
cat zund-demo/fleet/fleet.yaml # sample kind:fleet
cat zund-demo/fleet/assistant.yaml # sample agent

# 4. Config walk-up: run from a subdir, see project config pick up

cd zund-demo
mkdir -p deep/nested/dir && cd deep/nested/dir
zund config show # daemon.source should be "project"
zund config show -o json | jq # json mode drops transport hint
cd /tmp/zund-demo

# 5. Interactive wizard (quit with Ctrl+C after browsing)

cd /tmp && rm -rf zund-wizard-demo
zund init zund-wizard-demo # arrow through prompts

Daemon reachability

# 6. Friendly unreachable error

zund daemon status --socket /tmp/nope.sock # exit 1, install hint
zund daemon url --socket /tmp/nope.sock # prints target without trying to connect

# 7. Live daemon (in a second terminal, run: bun apps/daemon/src/index.ts)

zund daemon status # → ● unix://... uptime N
zund daemon status -o json | jq

Read commands (empty fleet)

cd /tmp/zund-demo # walk-up picks up zund.config.yaml
zund status # top-level alias
zund fleet status -o table
zund fleet export # "# No resources in fleet" until apply
zund agent list # empty table
zund agent list -o json # → []
zund agent get ghost # exit 1, 404 error
zund agent get ghost -o yaml # same, -o yaml works

Mutating commands (daemon only — no Incus required)

# 8. Memory round-trip

zund memory save writer "user wants terse replies"
zund memory save writer "prefers markdown output"
zund memory list writer # table view
zund memory search writer terse -o json
zund memory save writer "gets removed" --scope agent:writer
zund memory list writer -o json | jq '.[].id' # grab an id
zund memory delete writer <ID>
zund memory prune writer --scope agent:writer # clears the remaining two

# 9. Working memory

echo "## Preferences

- terse replies
- markdown

## Context

just started the project" | zund wm set writer
zund wm get writer # prints the markdown
zund wm scopes writer # lists which scopes have entries

# Slot patch (the Wave-1 feature): update one ## section in place

zund wm patch writer --slot Preferences --op replace --content "- loves haiku"
zund wm get writer # Preferences swapped, Context intact

zund wm patch writer --slot Context --op append --content "- now testing the CLI"
zund wm get writer # Context has both lines

Apply + events (requires Incus for real agents; preview works without it)

# 10. Preview against the sample fleet the wizard wrote

cd /tmp/zund-demo
zund apply --preview # diff plan, no execution
zund apply --preview -o yaml

# 11. Real apply (only if you have Incus + zund/base image set up)

zund apply # creates containers
zund fleet status
zund agent list
zund agent get assistant

# 12. Message + chat streaming

zund agent message assistant "write a haiku about rain" # streams to stdout
zund agent chat assistant # REPL; /exit to quit

SSE events (separate terminal)

# 13. Watch live events while you apply / message in another terminal

zund fleet events # streams forever, Ctrl+C to stop
zund fleet events -o json # one JSON line per event

Secrets (needs sops + age — Wave 5 deferred bundling)

# 14. These will show friendly errors until you install sops+age

# Install temporarily: brew install sops age

zund secret list # [] works without binaries (early return)
zund secret set ANTHROPIC_API_KEY sk-ant-test # expects age key at ~/.zund/age.key # missing → friendly error (no stack)

# After `age-keygen -o ~/.zund/age.key && brew install sops`:

zund secret set ANTHROPIC_API_KEY sk-ant-test
zund secret list # shows ANTHROPIC_API_KEY
zund secret get ANTHROPIC_API_KEY # masked
zund secret get ANTHROPIC_API_KEY --show # plaintext
zund secret remove ANTHROPIC_API_KEY

Teardown

# 15. Clean up everything the daemon is holding

zund fleet delete --yes
cd / && rm -rf /tmp/zund-demo /tmp/zund-wizard-demo

Things to check while you go

- Every command prints → unix://... (from project|env|default) in dim on stderr in table mode, nothing in -o json|yaml mode
- NO_COLOR=1 zund fleet status strips ANSI
- zund --host http://remote:4000 fleet status overrides whatever's in zund.config.yaml
- Exit codes: 0 on success, 1 on validation/HTTP errors or missing arguments
- zund fleet delete alone refuses; --yes is required

If anything surprises you, tell me what command + what you saw and I'll fix it. The parts I haven't exercised beyond the test suite are: the interactive init wizard (ran the flags path
only), agent chat over a real streaming session, and SSE events against actual fleet changes — those three are where new bugs would most likely hide.
