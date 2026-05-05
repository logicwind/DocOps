package nextsteps

// Outcome is the post-mutation context a command passes to a Suggest
// helper. Only the fields a given verb cares about are populated; the
// helpers ignore the rest.
type Outcome struct {
	// Init.
	Brownfield bool
	// New / Amend / Supersede / Refresh.
	ID string
	// Audit.
	HasGaps bool
}

// ForInit returns the closing block printed by `docops init`.
// Greenfield routes the user toward `docops new ctx` and /docops:plan;
// brownfield foregrounds /docops:onboard so the agent can bootstrap
// CTX/ADRs from the existing code.
func ForInit(o Outcome) []Step {
	if o.Brownfield {
		return []Step{
			{Label: "bootstrap CTX + ADRs from this codebase", Command: "/docops:onboard"},
			{Label: "or start fresh from a blank brief", Command: `docops new ctx --type brief "..."`},
			{Label: "see what the validator + audit see now", Command: "docops refresh && docops audit"},
		}
	}
	return []Step{
		{Label: "capture the project brief", Command: `docops new ctx --type brief "..."`},
		{Label: "drive an ADR + tasks from the brief", Command: "/docops:plan"},
	}
}

// ForNewCTX returns suggestions after `docops new ctx`.
func ForNewCTX(o Outcome) []Step {
	return []Step{
		{Label: "drive an ADR + tasks from this CTX", Command: "/docops:plan " + o.ID},
		{Label: "or write the first decision directly", Command: `docops new adr "..." --related ` + o.ID},
		{Label: "validate + index + state", Command: "docops refresh"},
	}
}

// ForNewADR returns suggestions after `docops new adr`.
func ForNewADR(o Outcome) []Step {
	return []Step{
		{Label: "create the first follow-up task", Command: `docops new task "..." --requires ` + o.ID},
		{Label: "validate + index + state", Command: "docops refresh"},
		{Label: "see what's open across the project", Command: "docops audit"},
	}
}

// ForNewTask returns suggestions after `docops new task`.
func ForNewTask(o Outcome) []Step {
	return []Step{
		{Label: "validate + index + state", Command: "docops refresh"},
		{Label: "ship it with the standard skill flow", Command: `/docops:do "ship ` + o.ID + `"`},
		{Label: "or pick the next task to work on", Command: "docops next"},
	}
}

// ForRefresh returns suggestions after `docops refresh`.
func ForRefresh(o Outcome) []Step {
	return []Step{
		{Label: "see structural gaps", Command: "docops audit"},
		{Label: "pick the next task", Command: "docops next"},
	}
}

// ForAmend returns suggestions after `docops amend`.
func ForAmend(o Outcome) []Step {
	return []Step{
		{Label: "validate + index + state", Command: "docops refresh"},
		{Label: "review the ADR with its amendment chain", Command: "docops get " + o.ID},
	}
}

// ForAudit returns suggestions after `docops audit`.
func ForAudit(o Outcome) []Step {
	if o.HasGaps {
		return []Step{
			{Label: "open a task for one of the gaps above", Command: `docops new task "..." --requires <ADR-ID>`},
			{Label: "or see the next task already queued", Command: "docops next"},
		}
	}
	return []Step{
		{Label: "no structural gaps — pick the next task", Command: "docops next"},
		{Label: "or capture a new decision", Command: `docops new adr "..."`},
	}
}

// ForState returns suggestions after `docops state`.
func ForState(o Outcome) []Step {
	return []Step{
		{Label: "see structural gaps", Command: "docops audit"},
		{Label: "pick the next task", Command: "docops next"},
	}
}

// ForUpgrade returns suggestions after `docops upgrade`.
func ForUpgrade(o Outcome) []Step {
	return []Step{
		{Label: "validate + index + state", Command: "docops refresh"},
	}
}
