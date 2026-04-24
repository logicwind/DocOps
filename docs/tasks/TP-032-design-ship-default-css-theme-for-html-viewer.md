---
title: Design + ship default CSS theme for HTML viewer
status: done
priority: p2
assignee: unassigned
requires: [ADR-0027]
depends_on: [TP-030]
---

## Status: obsolete — closed by the ADR-0027 pivot

This task is retained for audit trail and closed without implementation.

When ADR-0027 pivoted from Go-side rendering (goldmark + `embed.FS` CSS) to a client-side SPA (Tailwind via jsDelivr, one embedded `index.html`), the design problem this task scoped out evaporated:

- No per-page server-rendered HTML to style — there is one SPA file.
- No CSS file to compile and embed — Tailwind generates utility classes from the SPA markup on the fly.
- No theme tokens to freeze at compile time — edits to one HTML file ship with the next binary.

The visual design intent (font stack, kind accent colors — CTX blue / ADR amber / TP green, status badges, frontmatter tables, graph sidebar, WCAG AA contrast) carries forward into the SPA authored under TP-030. Nothing here is lost; only the "design + embed a CSS file" delivery is obsolete.

## Follow-up (if ever needed)

- Dark mode — Tailwind's `dark:` variants + a toggle button in the SPA header.
- Offline / `--vendor` flag — swap the Tailwind play CDN for a pre-built `tailwind.css` artifact bundled into the embed.

Neither is blocking; both can be opened as fresh tasks when demand appears.
