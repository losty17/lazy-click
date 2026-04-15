# lazy-click

lazy-click is a high-performance Go TUI for project management systems, starting with ClickUp.

## Current Status

Scaffolded foundation:

- Provider abstraction contracts.
- Unified domain models.
- Bubble Tea app shell.
- SQLite cache wiring scaffold.
- Living roadmap in `ROADMAP.md`.

## Run

```bash
cp .env.example .env
# fill in CLICKUP_API_TOKEN as needed
go run ./cmd/lazy-click
```
