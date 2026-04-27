# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Go CLI (`skillctl`) for installing, syncing, and validating markdown-based skills. The personal skills that previously lived here have moved to `../agent-stuff/skills`.

- `cmd/skillctl/` — CLI entry point
- `internal/` — cli, config, git, paths, prompts, security, skills, utils

## Commands

- Test: `make test` — wraps `go test ./...` with isolated `GOCACHE`, `GOPATH`, `GOMODCACHE` under `/tmp`. Do NOT run `go test ./...` directly; it will pollute the user's Go caches and can fail on sandboxed environments.
- Build: `go build ./cmd/skillctl`
- Format: `npm run format` (Prettier for md/json/yaml; gofmt for Go). There is no linter configured.

## Code style

- Prettier config: semi-colons, single quotes, ES5 trailing commas. Applies to non-Go files.
- Go: standard `gofmt`.

## Commits

- Use Conventional Commits (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`…).
- Keep commits atomic — don't mix feature + refactor + formatting.
- Never add `Co-Authored-by` trailers for AI agents.

## Bug fixes

- When fixing a user-reported bug, add 1–2 regression tests and re-run `make test` to confirm they pass.
- Remove tests that are just noise.

## Skills

- A skill lives at `skills/<name>/SKILL.md` with YAML frontmatter (`name`, `description`, optional `disable-model-invocation`).
- The installer runs security scans on skills; avoid shell patterns that look suspicious (obfuscated curl|bash, writes outside the skill dir, etc.) unless genuinely needed.
