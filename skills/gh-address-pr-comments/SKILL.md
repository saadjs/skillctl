---
name: gh-address-pr-comments
description: Address GitHub Pull Request feedback with the gh CLI. Use when a user provides a PR number and asks to fetch comments, verify which comments are meaningful, add regression tests, implement fixes, run all tests, and push the branch.
---

# GH Address PR Comments

## Goal

Resolve actionable PR feedback end-to-end with evidence:

1. Fetch all PR comments with `gh`.
2. Prove meaningful comments with failing regression tests.
3. Apply minimal fixes.
4. Run the full test suite.
5. Commit and push updates.
6. Post reply comments describing what was fixed in the latest commit(s).

## Required Input

- `PR_NUMBER` (required)
- `REPO` (optional; format: `owner/name`)

If `REPO` is missing, infer it with:

```bash
REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
```

## Preconditions

1. Verify GitHub auth:
   - `gh auth status`
2. Ensure a clean working tree before checkout:
   - `git status -sb`
3. Check out the PR branch:
   - `gh pr checkout "$PR_NUMBER" ${REPO:+--repo "$REPO"}`

## Workflow

1. Fetch comment sources with `gh`.
   - PR metadata and review summaries:
     - `gh pr view "$PR_NUMBER" ${REPO:+--repo "$REPO"} --json number,title,url,headRefName,baseRefName,comments,reviews`
   - Inline code review comments:
     - `gh api "repos/$REPO/pulls/$PR_NUMBER/comments?per_page=100" --paginate`
   - Issue-level PR comments:
     - `gh api "repos/$REPO/issues/$PR_NUMBER/comments?per_page=100" --paginate`

2. Build a comment checklist.
   - Normalize each comment into:
     - `id`
     - `author`
     - `location` (file/line if present)
     - `request` (one-sentence requested change)
     - `classification` (`meaningful` or `not-meaningful`)
   - Classify as `meaningful` only when the comment identifies a verifiable defect, regression risk, missing test, or concrete requirement mismatch.
   - Classify as `not-meaningful` when the comment is subjective, unclear, duplicate, or contradicted by code/tests.

3. Add regression tests before code fixes for meaningful comments.
   - Convert each meaningful comment into an expected behavior statement.
   - Add or update the smallest possible test that should fail before the fix.
   - Run a narrow test command to confirm failure.
   - If no failing test can be produced, re-check classification:
     - Downgrade to `not-meaningful`, or
     - Keep as meaningful only when the change is non-testable by nature (for example docs wording), and record why.

4. Implement fixes.
   - Apply minimal, targeted code changes tied to checklist items.
   - Avoid unrelated refactors.
   - Re-run the narrow tests after each fix until they pass.

5. Run the full test suite.
   - Use the repository standard test command(s).
   - If multiple standard suites exist (for example backend + frontend), run all of them.
   - Do not continue to commit/push with failing tests.

6. Commit and push.
   - Review final diff for comment-to-change traceability.
   - Commit with focused message(s), for example:
     - `fix(pr-123): address review comments with regression coverage`
   - Push branch updates:
     - `git push` (or `git push -u origin HEAD` when no upstream exists)

7. Post GitHub reply comments after push.
   - Capture latest pushed commits for reference links, for example:
     - `gh pr view "$PR_NUMBER" ${REPO:+--repo "$REPO"} --json commits -q '.commits[].oid'`
   - For each **meaningful** addressed comment, post a reply on the same thread when possible:
     - Inline review comment reply:
       - `gh api -X POST "repos/$REPO/pulls/$PR_NUMBER/comments/<comment_id>/replies" -f body='<reply>'`
       - If this returns `404`, verify the endpoint includes `pulls/$PR_NUMBER/comments/...` and that `REPO` is `owner/name`.
   - For issue-level PR comments (no inline thread), post a new issue comment that references the original comment URL:
     - `gh api -X POST "repos/$REPO/issues/$PR_NUMBER/comments" -f body='<reply>'`
   - Reply body requirements:
     - Start with what changed to address the concern.
     - Include the latest commit SHA(s) that contain the fix.
     - Mention test evidence when applicable.
   - Example reply:
     - `Fixed by validating empty payloads in request parsing and adding regression coverage in api/request_parser_test.go. Included in commits abc1234 and def5678; full test suite now passes.`

8. Report completion.
   - Summarize:
     - meaningful comments addressed
     - comments rejected as non-meaningful (with brief rationale)
     - regression tests added
     - full-suite test result
     - pushed branch name
     - reply comments posted (count and any skipped with reason)

## Guardrails

- Do not apply comment requests blindly.
- Do not mark a meaningful code comment as resolved without either:
  - a regression test that failed before the fix and passes after, or
  - a short explicit reason why the comment is non-testable.
- Do not push if the full suite fails.
- Do not finish after push until comment replies are posted (or explicitly skipped with reason).
- Keep changes scoped to comment resolution.
