# lazy-click

lazy-click is a high-performance Go TUI for project management systems, starting with ClickUp.

It supports multiple providers (including a built-in Local provider) and lets you switch providers from the Control Center.

License: GPL-3.0-only

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
# optional: set CLICKUP_CLIENT_ID and LAZY_CLICK_OAUTH_BACKEND_URL for ClickUp OAuth via backend
go run ./cmd/lazy-click
```

Default database location is `$HOME/.local/share/lazy-click/lazy-click.db`.
You can override it with `COMMAND_TASK_DB_PATH`.

## Providers

- `local` provider is enabled by default and stores tasks in your SQLite database.
- ClickUp can be connected via Control Center: `Connect ClickUp (OAuth)`.
- Switch providers in Control Center using `Switch provider (next)` or `Use provider: ...` commands.

### ClickUp OAuth notes

- OAuth token exchange is handled by a separate FastAPI backend under `backend/`.
- The Go TUI only needs `CLICKUP_CLIENT_ID` and `LAZY_CLICK_OAUTH_BACKEND_URL`.
- Keep `CLICKUP_CLIENT_SECRET` only in backend env, never in shipped client binaries.

## OAuth backend

- The backend lives in `backend/` and is deployed separately.
- See `backend/README.md` for setup and deployment details.
- A ready-to-use `docker-compose.yml` is included at the repo root for backend deployment.

## Shipping defaults in builds

You can bake default OAuth client settings into release binaries via linker flags (still overridable by env vars at runtime):

```bash
BUILD_DEFAULT_CLICKUP_CLIENT_ID=your_client_id \
BUILD_DEFAULT_OAUTH_BACKEND_URL=https://oauth.yourdomain.com \
goreleaser release --clean
```

At runtime, these env vars still take precedence if set:

- `CLICKUP_CLIENT_ID`
- `LAZY_CLICK_OAUTH_BACKEND_URL`

## Install

Prebuilt binaries and packages are published on every tagged release.

### macOS (Homebrew)

```bash
brew tap losty17/tap
brew install lazy-click
```

### Linux (Debian/Ubuntu)

Download the latest `.deb` from the GitHub Releases page, then install it:

```bash
sudo dpkg -i lazy-click_<version>_linux_x86_64.deb
```

### Linux (Fedora/RHEL)

Download the latest `.rpm` from the GitHub Releases page, then install it:

```bash
sudo rpm -i lazy-click_<version>_linux_x86_64.rpm
```

### Manual binary install (any platform)

Download the archive for your platform from Releases, extract it, and move `lazy-click` into your `PATH`.

## Release process

Creating and pushing a semver tag (for example `v0.1.0`) triggers the release workflow, which builds and publishes:

- macOS binaries (`darwin/amd64`, `darwin/arm64`)
- Linux binaries (`linux/amd64`, `linux/arm64`)
- Linux packages (`.deb`, `.rpm`)
- Checksums

For Homebrew formula publishing, add `HOMEBREW_TAP_GITHUB_TOKEN` as an Actions secret in this repository.
