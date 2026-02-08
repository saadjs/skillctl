---
name: gh-pr-create
description: Create a GitHub Pull Request using the gh CLI after git-incremental-commits has produced a clean working tree. Use when asked to open a PR, create a PR after committing, or "make a PR". Generates a PR body with Summary, Major changes, optional Screenshots, optional Tests, and optional Additional info.
---

# GitHub PR Create (gh)

## Goal

After `$git-incremental-commits` finishes (working tree clean), push the branch and open a PR with a consistent body format.

## Preconditions

- Repo is on a non-`main`/`master` branch.
- `git status -sb` is clean.
- `gh` is installed and authenticated (`gh auth status`).

## Workflow

1. Verify working tree is clean.
   - `git status -sb`
   - Stop if there are uncommitted changes.

2. Confirm you are on a branch suitable for a PR.
   - `git branch --show-current`
   - If on `main`/`master` or detached HEAD: stop and create/switch to a branch.

3. Ensure the branch is pushed.
   - If no upstream is set: `git push -u origin HEAD`
   - Otherwise: `git push`

4. Determine PR base branch.
   - Prefer the repo default branch: `gh repo view --json defaultBranchRef -q .defaultBranchRef.name`

5. Create a PR body in this format.

- Include these headings exactly:
  1. `## Summary`
  2. `## Major changes`

- Conditionally include these headings:
  - `## Screenshots`: include only if relevant (UI changes or image diffs).
  - `## Tests`: include only if the repo has a configured test command/CI expectation.
  - `## Additional info`: include only if there is something non-obvious to call out (follow-ups, rollout notes, risks).

6. Open the PR with `gh pr create`.

- Title:
  - Default to a summarized Conventional Commit style title derived from the branch commits (e.g. `Feat: ...`, `Fix: ...`, `Docs: ...`).
  - Override with `PR_TITLE="..."` if needed.

- Body:
  - Use the helper script:
    - `./skills/gh-pr-create/scripts/gh-pr-create.sh`
  - Or inline:
    - `gh pr create --title "..." --body "..."`

## Notes

- Keep the PR body concise but specific.
- If there are multiple major changes, list them as bullets under `## Major changes`.
- If tests are not configured, omit the entire `## Tests` section (do not include an empty heading).
- The helper script auto-detects when to include `## Tests` and `## Screenshots`.
  - Overrides:
    - `PR_TESTS=1` force include, `PR_TESTS=0` force omit
    - `PR_SCREENSHOTS=1` force include, `PR_SCREENSHOTS=0` force omit
