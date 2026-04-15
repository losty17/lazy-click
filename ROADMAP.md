# Command-Task Roadmap (Living Document)

This roadmap is actively maintained during development. It defines the current architecture, milestones, and evolving decisions.

## Vision

Build a high-performance Go TUI that unifies project/task workflows across providers (starting with ClickUp), with instant local reads backed by SQLite and reliable background sync.

## Core Architecture

### 1) Provider Abstraction

Define a provider contract in `internal/provider`:

- `GetSpaces(ctx)`
- `GetLists(ctx, spaceID)`
- `GetTasks(ctx, listID, filter)`
- `UpdateTask(ctx, taskID, data)`
- `AddComment(ctx, taskID, text)`

Use unified models (`Task`, `User`, `Priority`, `Tag`, etc.) and map provider payloads into these models.

### 2) Cache-First Runtime

- SQLite as local source of truth for all TUI reads.
- Background sync engine:
  - Pull remote data into SQLite.
  - Push local mutations to remote provider APIs.
  - Track conflicts and sync errors explicitly.

### 3) TUI Architecture (Bubble Tea)

- Sidebar: Workspace > Space > List
- Task Table: Title, Priority, Due Date, Tags
- Detail View:
  - Markdown task description
  - Comment thread
  - Metadata editor (status/dates)
- Keyboard-first controls:
  - `j/k` navigation
  - `/` fuzzy find
  - `i` edit
  - `c` comment

## Phases

## Phase 1 — Foundation (Completed)

- [x] Initialize Go module and project structure.
- [x] Define provider interface and unified domain models.
- [x] Add app bootstrap/runtime skeleton.
- [x] Add initial Bubble Tea app shell.
- [x] Add SQLite cache wiring scaffold.
- [x] Add migrations and repository query contracts.

## Phase 2 — Sync Engine

- [x] Implement ClickUp client/auth skeleton (API token path completed; OAuth2/keyring pending).
- [x] Implement ClickUp provider methods for spaces/lists/tasks.
- [x] Implement pull sync (remote -> cache) scaffold.
- [x] Implement push queue (cache -> remote) scaffold.
- [x] Add conflict/sync-error model + handling policy scaffold.

## Phase 3 — Interface

- [x] Build hierarchical sidebar navigation (scaffold).
- [x] Implement task table (scaffold, now cache-bound to selected list).
- [ ] Implement detail panel with markdown renderer.
- [ ] Implement comment thread panel.
- [ ] Implement status filter UX (including ClickUp statuses).

## Phase 4 — Interaction

- [x] Implement task editing flow (`UpdateTask` pipeline, title toggle MVP via `i`).
- [ ] Implement comment creation flow (`AddComment` pipeline).
- [ ] Implement fuzzy find (`/`) across task fields.
- [x] Implement optimistic updates + reconcile with provider (queue + periodic push cycle).
- [ ] Add robust error surfacing in TUI.

## Cross-Cutting Quality Gates

- [ ] Unit tests for provider mappings and cache repository.
- [ ] Integration tests for sync pull/push paths.
- [ ] Structured logging around sync and mutation flows.
- [ ] Basic performance budget checks (table rendering, sync throughput).

## Improvement Log

### 2026-04-15

- Added initial project scaffolding and provider/cache/TUI contracts.
- Established this roadmap as the source of truth for implementation flow.
- Implemented cache schema, queue model, and repository query contracts.
- Added initial ClickUp API client + provider mappings for spaces/lists/tasks/update/comment.
- Added initial auth abstraction and environment-based token retrieval scaffold.
- Added initial sync engine (cycle, pull, queue push, conflict policy baseline).
- Added pane-based TUI navigation wiring with sidebar/task/detail component scaffolds.
- Wired startup ClickUp sync (`CLICKUP_API_TOKEN`) and periodic background sync loop.
- Removed startup blocking sync to eliminate slow TUI launch (sync now background-only).
- Bound sidebar/task/detail views to SQLite cache data (selected list drives table).
- Added MVP task edit action (`i`) that updates local cache and queues remote push.
- Eliminated empty-queue log noise by treating no pending sync items as normal.
- Added periodic cache polling + explicit manual sync key (`s`) to expose sync issues in-app.
- Fixed ClickUp list discovery to include folder lists (`/folder/{id}/list`) in addition to space root lists.
- Added bounded panel layout with per-panel internal scrolling and line truncation to prevent screen overflow.
- Reworked to a two-column layout (left sidebar, right table+detail stack) with strict max-height fitting.
- Tightened terminal-fit logic (footer truncation + safety height margin) and made pull sync resilient to per-list failures.
- Next improvement targets: full inline editor, comment compose flow, markdown rendering, and status filtering.
