---
name: git-incremental-commits
description: Create small, incremental Git commits from uncommitted changes using Conventional Commits. Use when asked to review git status and split changes into small commits (by file or chunk), especially when dependency changes require lockfiles, and to finish with a clean working tree.
---

# Git Incremental Commits

## Goal

Turn all uncommitted changes into a sequence of small, clear Conventional Commits, stopping when `git status` is clean.

## Workflow

1. Run `git status -sb` to see whether there is anything to commit. Stop if clean.
2. Run `git branch --show-current` (or `git rev-parse --abbrev-ref HEAD`) to confirm the current branch.
3. **Branch safety gate**:
   - If the current branch is `main` or `master` and there are new changes to commit, **do not commit**.
   - Ask the user to create a new branch first (example: `git switch -c <branch-name>`), then re-run `git status -sb` and continue.
   - If in a detached HEAD state (no branch), ask the user to create a branch before committing.
4. Run `git diff --stat` (and `git diff` as needed) to understand scope.
5. Identify logical groups: dependencies, config, feature code, tests, docs, refactors, fixes.
6. Commit in smallest safe slices:
   - Prefer single-purpose commits.
   - Use `git add -p` to stage hunks when files mix concerns.
   - Use whole-file staging when the file is cohesive.
7. After each commit, re-check `git status -sb` and repeat.
8. Stop only when the working tree is clean and no untracked files remain (unless explicitly told to leave them).

## Grouping Heuristics

- **Dependencies**: If package manifests change (e.g., `package.json`), commit them with their lockfiles in the same commit.
- **Config/Build**: Commit build or CI config separately (e.g., `tsconfig`, `eslint`, CI files).
- **Feature work**: Prefer one commit per feature slice.
- **Fixes**: Keep bug fixes isolated from refactors when possible.
- **Tests**: Pair tests with the change they validate, unless tests are a separate logical unit.
- **Docs**: Keep docs-only changes separate.

## Conventional Commit Rules

- Use `type(scope): subject` or `type: subject`.
- Keep subject short, imperative, and specific.
- Suggested types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `build`, `ci`.
- Dependency changes: prefer `build(deps): add <pkg>` or `chore(deps): bump <pkg>`.

## Safety Checks

- Do not include unrelated changes in the same commit.
- If a file mixes unrelated edits and hunk-splitting is ambiguous, ask before proceeding.
- If there are generated files or large diffs, confirm intent before committing.
- Avoid committing directly to `main`/`master`; create a working branch first.

## Command Pattern

- Inspect: `git status -sb`, `git diff --stat`, `git diff`.
- Stage: `git add -p` or `git add <file>`.
- Commit: `git commit -m "type(scope): subject"`.
- Repeat until clean.
