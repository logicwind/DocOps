# DocOps: AI-Native GitOps Documentation & Project Management

## 1. Vision
DocOps is a specialized documentation and project management framework designed for **AI-First development workflows**. It transforms the `docs/` folder of a repository into a structured, machine-readable "Context Engine." 

While traditional tools (Jira, Linear) exist in external silos, DocOps lives within the Git tree. This allows AI coding agents (Claude Code, Cursor, etc.) to have a perfect, up-to-date understanding of:
*   **Tasks & Progress:** What is being worked on and what is finished.
*   **Architecture:** How the system is designed (via Mermaid/Excalidraw).
*   **Decision History:** Why certain paths were taken (via ADRs).
*   **Project Map:** An indexed overview of all documentation.

## 2. Core Components

### A. The Context Engine (The "Source of Truth")
The `docs/` folder is structured into specific, machine-parsable domains:
*   `docs/tasks/`: All task-related `.md` files. Uses `docops.yaml` to define valid statuses (e.g., `backlog`, `in-progress`).
*   `docs/requirements/`: A collection of high-level documents (PRDs, User Stories, etc.). We do not control the content, only the frontmatter (e.g., `type: prd`, `scope: core`), allowing the LLM to identify which document to ingest for a specific task.
*   `docs/technical/`: Contains technical specifications, architecture diagrams (Mermaid/Excalidraw), and RFCs. Frontmatter allows the LLM to distinguish between "high-level architecture" and "low-level component details."
*   `docs/index.md`: The "Project Entry Point"—an auto-generated manifest that provides a high-level summary of the entire repo.

### B. The TypeScript Core (The "Logic")
A developer-installable dependency (`@docops/core`) providing:
*   **Linter/Validator:** A `pre-commit` hook that validates YAML frontmatter, status transitions, and structural integrity.
*   **The Indexer:** A script that crawls the `docs/` directory to rebuild `docs/index.md` and the project summary.
*   **The Config (`docops.yaml`):** A project-specific configuration file defining allowed statuses, priorities, and validation rules.

### C. The Web Interface (The "Human/Agent Gateway")
A web-based dashboard (Next.js) for non-technical stakeholders and human managers:
*   **Kanban/List Views:** Visualizing tasks from `..docs/tasks/`.
*   **Direct Editing:** An interface to edit Markdown/YAML and trigger a `git commit` via a server-side agent.
*   **The "Context Viewer":** A specialized view optimized for humans to read ADRs and Diagrams.

## 3. Technical Architecture

### Data Flow
1.  **Developer/AI Action:** A developer (or AI Agent) creates a task file or modifies an ADR.
2.  **Validation (Pre-commit):** The `docops-lint` hook runs. If the `docops.yaml` rules are violated (e.g., invalid status), the commit is rejected.
3.  **Indexing:** Upon successful commit, the `docops-index` command (triggered via CI or local) updates `docs/index.config.md`.
4.  **Visualization:** The Web UI syncs with the Git repo, rendering the new state for human review.

### Tech Stack
*   **Language:** TypeScript (End-to-end type safety).
*   **Linter/CLI:** Node.js (distributed via NPM as a dev-dependency).
*   **Web UI:** Next.js, Tailwind CSS, Lucide Icons.
*   **Diagrams:** Mermaid.js, Excalidraw integration.
*   **Deployment:** Local/Server-side Git-orchestration agent.

## 4. AI-Agent Integration (LLM-First)

The core of DocOps is the `llm-instructions.md` file. This file serves as the "System Prompt" for any agent working in the repo, instructing it on:
*   **Discovery:** How to use the `docs/index.md` to navigate the repo.
*   **Learning:** How to parse the `docs/requirements/` and `docs/technical/` folders by reading frontmatter.
*   **Execution:** The protocol for creating new tasks, ADRs, or technical docs.
*   **Command Usage:** How to utilize available CLI tools and skills (e.g., `/setup`, `/resync`).

### Agentic Capabilities (Scripts & Skills)
The ecosystem provides specialized utilities for both humans and LLMs:
*   **`/setup` (Skill/Command):** Initializes a new DocOps-annotable project (scaffolds folders, `docops.yaml`, and `llm-instructions.md`).
*   **`/resync` (Skill/Command):** Regenerates the `docs/index.md` and updates the project manifest after new files are added.
*   **`getNextTaskId` (Utility):** A script to deterministically generate the next valid `TP-XXX` ID to prevent collisions.
*   **`docops-lint` (Hook):** Ensuring all manual or agentic edits adhere to the project's structural rules.

## 5. Success Metric
The "Context Window Efficiency" test: Can an LLM agent, given *only* the `docs/index.md` and the relevant `.md` files, successfully execute a complex feature task without asking for clarification on the architecture or project status?
