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
skillctl add saadjs/agent-skills --tool codex --scope global
skillctl add saadjs/agent-skills --tool codex --scope project
skillctl add saadjs/agent-skills --tool cursor --scope project
skillctl add saadjs/agent-skills --tool claude --scope project
skillctl add ./path/to/skills-repo --dest /tmp/skills
skillctl list --tool cursor --scope project
skillctl remove --tool copilot --scope project --skill de-dupe
```

## Structure

- `skills/<skill-name>/SKILL.md` defines a skill, its triggers, and workflow.

## Setup (dev)

```sh
go build ./cmd/skillctl
```
