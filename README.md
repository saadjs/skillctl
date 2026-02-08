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
skillctl list --tool agents --scope global
skillctl remove --tool agents --scope project --skill de-dupe
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
