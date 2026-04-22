# DocOps: LLM-Instructions Module

This file serves as the "System Prompt" for any AI Agent (Claude Code, Cursor, etc.) operating within a DocOps-enabled repository. It defines the protocols for discovery, learning, and execution.

## 1. Core Mission
Your mission is to maintain the integrity of the DocOps ecosystem. You must treat the `docs/` folder as the authoritative source of truth for project state, architecture, and requirements.

## 2. Discovery Protocol (How to Learn)

When entering this repository, follow these steps to build your context:

1.  **Initialize Context:** Always start by reading `docs/index.md`. This file contains the high-level project map and links to all active components.
2.  **Requirement Ingestion:** To understand *what* needs to be built, scan `docs/requirements/`. 
    *   Check the `frontmatter` of files in this folder.
    *   Identify the `type` (e.g., `prd`, `user-story`) and `scope` to prioritize which documents to read.
3.  **Technical Grounding:** To understand *how* to build, scan `docs/technical/`.
    *   Analyze `ADRs` (Architecture Decision Records) to understand past constraints and chosen patterns.
    *   Inspect `Mermaid` or `Excalidraw` diagrams to understand component relationships.
4.  **Task Identification:** Scan `docs/tasks/` to identify active tasks (`status: in-progress`) and the backlog (`status: backlog`).

## 3. Execution Protocol (How to Act)

### A. Creating New Work
When you are tasked with a new feature, follow this lifecycle:
1.  **Generate Task ID:** Use the `getNextTaskId` utility to ensure a unique, collision-free ID (e.g., `TP-102`).
2.  **Create Task File:** Create a new `.md` file in `docs/tasks/` with correct YAML frontmatter (`id`, `subject`, `status: backlog`, `priority`).
3.  **Update Requirements/Technical Docs:** If the task changes the system architecture, you **must** create or update an `ADR` in `docs/technical/` or update the relevant specification.
4.  **Commit & Validate:** Ensure your changes pass `docops-lint` before finalizing.

### B. Completing Tasks
1.  **Update Status:** Change the task status in the frontmatter to `done`.
2.  **Verify Implementation:** Ensure all "Acceptance Criteria" listed in the task file are met by the code changes in your PR.
3.  **Clean Up:** If the task is large, archive the task description in `docs/tasks/archived/` if configured via `docops.yaml`.

## 4. Available Commands & Skills

You have access to (or can invoke via shell) the following specialized utilities:

| Command | Purpose |
| :--- | :--- |
| `/setup` | Initializes the DocOps structure in a new directory. |
| `/resync` | Re-indexes the `docs/` folder and updates `docs/index.md`. **Run this after adding new docs.** |
| `getNextTaskId` | Returns the next available `TP-XXX` identifier. |
| `docops-lint` | Validates the structural integrity of your documentation changes. |

## 5. Constraints & Rules
*   **Never** modify the `docops.yaml` without verifying the impact on the `allowed_statuses` list.
*   **Never** move files out of the `docs/` subdirectories; use the `status` field for transitions.
*   **Always** maintain the `docs/index.md` after any major structural change.
*   **Always** ensure every new task includes an `assignee` (even if it is `ai-agent`).
