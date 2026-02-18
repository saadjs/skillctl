---
name: git-atomic-commits
description: Organize uncommitted changes into atomic Git commits using Conventional Commits. Use before opening a PR or merging to produce a clean, reviewable history where each commit represents one logical change.
---

# Git Atomic Commits

## Goal

Transform all uncommitted changes into **atomic** Conventional Commits.

Stop only when `git status` is clean (and no untracked files remain unless explicitly told to leave them).

## Definition: Atomic Commit

A commit should answer:

> “What single logical change does this introduce?”

An atomic commit is easy to:

- review
- revert
- bisect/debug

Avoid mixing concerns (feature + refactor + formatting) in one commit.

## Workflow

1. Check status:
   - `git status -sb`
   - Stop if clean.

2. Confirm branch:
   - `git branch --show-current` (or `git rev-parse --abbrev-ref HEAD`)
   - If detached HEAD, ask the user to create a branch before committing.

3. **Branch safety gate**
   - If the current branch is `main` or `master` and there are changes to commit, **do not commit**.
   - Ask the user to create a working branch first:
     ```bash
     git switch -c <branch-name>
     ```
   - Then re-run `git status -sb` and continue.

4. Inspect scope:
   - `git diff --stat`
   - `git diff` (as needed)

5. Identify atomic units (one purpose each):
   - dependencies
   - config/build/CI
   - refactor (behavior-preserving)
   - bug fix
   - feature slice
   - tests
   - docs
   - formatting-only

6. Stage ONLY what belongs to one atomic unit:
   - Prefer `git add -p` for mixed files.
   - Use whole-file staging when the file is cohesive.

7. Commit with Conventional Commits message.

8. Repeat:
   - After each commit, run `git status -sb`
   - Continue staging/committing atomic units until clean.

## Atomic Grouping Rules

- **Dependencies**
  - If manifests change (e.g., `package.json`, `pyproject.toml`), commit them with lockfiles in the same commit.
  - Prefer `build(deps): ...` or `chore(deps): ...`.

- **Config / CI / Build**
  - Keep build, lint, and CI config changes in their own commit.

- **Refactors**
  - Refactors must not change behavior.
  - Never mix refactors with feature work or bug fixes.

- **Bug fixes**
  - Keep the fix isolated from refactors and formatting when possible.

- **Feature work**
  - Prefer one commit per “feature slice” (a coherent, reviewable increment), not “the whole feature” if large.

- **Tests**
  - Include tests with the change they validate when feasible.
  - If tests are a separate logical unit, commit them separately as `test:`.

- **Docs**
  - Docs-only changes should be a `docs:` commit.

- **Formatting**
  - Formatting-only changes should be separate (`style:`), never bundled with logic changes.

## Conventional Commit Rules

- Use `type(scope): subject` or `type: subject`
- Keep subject short, imperative, and specific
- Suggested types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `build`, `ci`, `style`

Examples:

- `feat(auth): add OAuth login`
- `fix(api): handle timeout errors`
- `refactor(cache): simplify invalidation logic`
- `build(deps): bump react to 19.0.0`
- `style: format codebase`

## Safety Checks

- Do not include unrelated changes in the same commit.
- If a file mixes unrelated edits and hunk-splitting is ambiguous, ask before proceeding.
- Confirm intent before committing generated files or large diffs.
- Avoid committing build artifacts (`dist/`, `build/`, `coverage/`, compiled outputs) unless the repo intentionally tracks them.
- Ensure deletions and renames are included when relevant.

## History Quality Check

Before finishing, verify:

✔ Each commit has one purpose  
✔ Commit messages explain intent  
✔ Commits can be reviewed independently  
✔ Commits can be reverted safely

## Optional: History Cleanup

If the current commit sequence contains WIP/noisy commits (or the user wants a cleaner story), offer an interactive rebase:

```bash
git rebase -i <base-branch>
```

Squash/reword/reorder so each remaining commit is atomic and clearly described.

## Command Pattern

Inspect:

- `git status -sb`
- `git diff --stat`
- `git diff`

Stage:

- `git add -p`
- `git add <file>`

Commit:

- `git commit -m "type(scope): subject"`

Repeat until clean.
