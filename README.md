# agent-skills

Collection of Agent Skills for various tasks.

> If you asked your agent to perform the same task twice, it should probably be a skill.

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
skillctl add saadjs/agent-skills --tool agents --scope global
skillctl add saadjs/agent-skills --tool agents --scope project

# Or install to tool-specific directories
skillctl add saadjs/agent-skills --tool codex --scope global
skillctl add saadjs/agent-skills --tool cursor --scope project
skillctl add saadjs/agent-skills --tool claude --scope project
skillctl add ./path/to/skills-repo --dest /tmp/skills
skillctl add ./path/to/skills-repo --dest /tmp/skills --force
skillctl list --tool agents --scope global
skillctl remove --tool agents --scope project --skill de-dupe
```

### Security Scan During Install

`skillctl add` performs a built-in security scan before installing skills. The scan checks the full source repository (local path or cloned GitHub repo) for suspicious commands, potential exfiltration patterns, and malicious agent instructions.

- If findings are detected, install is blocked by default.
- In interactive mode, you can confirm and continue.
- In non-interactive mode (`--yes`), rerun with `--force` to bypass.
- `--dry-run` also executes the security scan, but does not install files.

Example flows:

```sh
# Default behavior: blocked if findings are detected
skillctl add owner/repo --tool agents --scope global --yes

# Explicit bypass for automation/non-interactive environments
skillctl add owner/repo --tool agents --scope global --yes --force

# Dry run still scans, but performs no install writes
skillctl add owner/repo --tool agents --scope global --dry-run --yes --force
```

### Supported Tools & Paths

| Tool     | Global Path                  | Project Path       |
| -------- | ---------------------------- | ------------------ |
| agents   | `~/.agents/skills`           | `.agents/skills`   |
| codex    | `~/.codex/skills`            | `.codex/skills`    |
| claude   | `~/.claude/skill`            | `.claude/skills`   |
| cursor   | `~/.cursor/skills`           | `.cursor/skills`   |
| windsurf | `~/.codeium/windsurf/skills` | `.windsurf/skills` |
| copilot  | `~/.copilot/skills`          | `.github/skills`   |

## Structure

- `skills/<skill-name>/SKILL.md` defines a skill, its triggers, and workflow.

## Setup (dev)

```sh
go build ./cmd/skillctl
```
