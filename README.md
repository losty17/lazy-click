# lazy-click

lazy-click is a high-performance Go TUI for project management systems, starting with ClickUp.

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
# fill in CLICKUP_API_TOKEN as needed
go run ./cmd/lazy-click
```

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
