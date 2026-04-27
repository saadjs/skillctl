# skillctl

Go CLI for installing, syncing, listing, removing, and validating agent skills.

> If you asked your agent to perform the same task twice, it should probably be a skill.

The personal skills that previously lived in this repository now live in
[`saadjs/agent-stuff`](https://github.com/saadjs/agent-stuff) under `skills/`.

## Install (Homebrew)

```sh
brew tap saadjs/homebrew-tap
brew install skillctl
```

## Install (manual)

Download the latest release for your platform from GitHub Releases, then:

```sh
tar -xzf skillctl_<version>_<os>_<arch>.tar.gz
sudo mv skillctl /usr/local/bin/skillctl
```

## Usage

```sh
# Install to the new standard ~/.agents/skills directory (recommended)
skillctl add saadjs/agent-stuff --tool agents --scope global
skillctl add saadjs/agent-stuff --tool agents --scope project

# Or install to tool-specific directories
skillctl add saadjs/agent-stuff --tool codex --scope global
skillctl add saadjs/agent-stuff --tool cursor --scope project
skillctl add saadjs/agent-stuff --tool claude --scope project
skillctl add ./path/to/skills-repo --dest /tmp/skills
skillctl add ./path/to/skills-repo --dest /tmp/skills --force
skillctl list --tool agents --scope global
skillctl remove --tool agents --scope project --skill de-dupe

# Sync changed skills only (default)
skillctl sync

# Force re-sync of every selected skill, even if unchanged
skillctl sync --all
```

### Sync mode: `--all`

`skillctl sync` is checksum-aware by default: it only copies skills that changed since the last sync for each configured tool.

Use `skillctl sync --all` when you need a full refresh. It ignores stored checksums and re-copies every selected skill, which helps when local skill folders were manually edited, partially deleted, or drifted out of sync without source changes.

### Security Scan During Install

`skillctl add` performs a built-in security scan before installing skills. The scan checks the configured skills subtree (`--path`, default `skills`) in the source for suspicious commands, potential exfiltration patterns, and malicious agent instructions.

- If findings are detected, install is blocked by default.
- In interactive mode, you can confirm and continue.
- In non-interactive mode (`--yes`), rerun with `--force` to bypass.
- `--dry-run` on local sources executes the security scan, but does not install files.
- `--dry-run` on remote sources does not clone or scan, and requires at least one `--skill` value.

Example flows:

```sh
# Default behavior: blocked if findings are detected
skillctl add owner/repo --tool agents --scope global --yes

# Explicit bypass for automation/non-interactive environments
skillctl add owner/repo --tool agents --scope global --yes --force

# Local dry run still scans, but performs no install writes
skillctl add owner/repo --tool agents --scope global --dry-run --yes --force
```

### Supported Tools & Paths

| Tool     | Global Path                  | Project Path       |
| -------- | ---------------------------- | ------------------ |
| agents   | `~/.agents/skills`           | `.agents/skills`   |
| codex    | `~/.codex/skills`            | `.codex/skills`    |
| claude   | `~/.claude/skills`           | `.claude/skills`   |
| cursor   | `~/.cursor/skills`           | `.cursor/skills`   |
| windsurf | `~/.codeium/windsurf/skills` | `.windsurf/skills` |
| copilot  | `~/.copilot/skills`          | `.github/skills`   |

## Skill Repo Structure

- `skills/<skill-name>/SKILL.md` defines a skill, its triggers, and workflow.

## Setup (dev)

```sh
go build ./cmd/skillctl
```
