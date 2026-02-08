#!/usr/bin/env bash
set -euo pipefail

# Creates a GitHub PR using `gh` with a standardized body.
# Intended to run after `git-incremental-commits` has left a clean working tree.

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "error: missing required command: $1" >&2
    exit 1
  }
}

require git
require gh

# Safety: require clean working tree.
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "error: working tree is not clean. Commit/stash changes before creating a PR." >&2
  git status -sb >&2 || true
  exit 1
fi

branch="$(git branch --show-current)"
if [[ -z "${branch}" ]]; then
  echo "error: detached HEAD. Create a branch before creating a PR." >&2
  exit 1
fi

if [[ "${branch}" == "main" || "${branch}" == "master" ]]; then
  echo "error: refusing to create a PR from ${branch}. Create a feature branch first." >&2
  exit 1
fi

# Ensure auth works early.
if ! gh auth status >/dev/null 2>&1; then
  echo "error: gh is not authenticated. Run: gh auth login" >&2
  exit 1
fi

# Push branch if needed.
upstream_ref=""
if upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null)"; then
  :
else
  upstream_ref=""
fi

if [[ -z "${upstream_ref}" ]]; then
  git push -u origin HEAD
else
  git push
fi

base_branch="$(gh repo view --json defaultBranchRef -q .defaultBranchRef.name 2>/dev/null || true)"
if [[ -z "${base_branch}" ]]; then
  # Fallback if `gh repo view` fails (e.g. unusual permissions).
  base_branch="main"
fi

# Build a short summary from commits + diffstat.
# Use triple-dot to compare against merge base.
diffstat="$(git diff --stat "${base_branch}...HEAD" 2>/dev/null || true)"
commit_list="$(git log --oneline "${base_branch}...HEAD" 2>/dev/null || true)"

# PR title: summarize changes and choose an appropriate Conventional Commit label.
# You can override with PR_TITLE="...".
type_rank() {
  # Higher number = higher priority.
  case "$1" in
    feat) echo 90 ;;
    fix) echo 80 ;;
    perf) echo 70 ;;
    refactor) echo 60 ;;
    docs) echo 50 ;;
    test) echo 45 ;;
    build) echo 40 ;;
    ci) echo 35 ;;
    chore) echo 30 ;;
    style) echo 20 ;;
    revert) echo 10 ;;
    *) echo 0 ;;
  esac
}

type_label() {
  case "$1" in
    feat) echo "Feat" ;;
    fix) echo "Fix" ;;
    perf) echo "Perf" ;;
    refactor) echo "Refactor" ;;
    docs) echo "Docs" ;;
    test) echo "Test" ;;
    build) echo "Build" ;;
    ci) echo "CI" ;;
    chore) echo "Chore" ;;
    style) echo "Style" ;;
    revert) echo "Revert" ;;
    *) echo "Chore" ;;
  esac
}

parse_type_from_subject() {
  # Extract "type" from Conventional Commit subjects:
  # - type(scope): subject
  # - type: subject
  # Returns empty if not matching.
  local s="$1"
  local re
  re='^([[:alpha:]]+)(\([^)]*\))?:[[:space:]]+.+$'
  if [[ "$s" =~ $re ]]; then
    printf "%s" "${BASH_REMATCH[1],,}"
  fi
}

strip_conventional_prefix() {
  # Convert "type(scope): subject" -> "subject", "type: subject" -> "subject"
  local s="$1"
  local re
  re='^[[:alpha:]]+(\([^)]*\))?:[[:space:]]+(.+)$'
  if [[ "$s" =~ $re ]]; then
    printf "%s" "${BASH_REMATCH[2]}"
  else
    printf "%s" "$s"
  fi
}

pick_title() {
  local base="$1"
  local subjects raw line t best_type best_rank cleaned first_cleaned count
  best_type=""
  best_rank=-1
  count=0
  first_cleaned=""

  raw="$(git log --pretty=%s "${base}...HEAD" 2>/dev/null || true)"
  while IFS= read -r line; do
    [[ -z "${line}" ]] && continue
    count=$((count + 1))
    t="$(parse_type_from_subject "${line}" || true)"
    if [[ -n "${t}" ]]; then
      local r
      r="$(type_rank "${t}")"
      if [[ "${r}" -gt "${best_rank}" ]]; then
        best_rank="${r}"
        best_type="${t}"
      fi
    fi
    cleaned="$(strip_conventional_prefix "${line}")"
    if [[ -z "${first_cleaned}" && -n "${cleaned}" ]]; then
      first_cleaned="${cleaned}"
    fi
  done <<<"${raw}"

  if [[ -z "${first_cleaned}" ]]; then
    first_cleaned="$(git log -1 --pretty=%s 2>/dev/null || echo "Update")"
  fi

  if [[ -z "${best_type}" ]]; then
    # If commit subjects aren't Conventional Commits, default to Chore.
    best_type="chore"
  fi

  if [[ "${count}" -gt 1 ]]; then
    printf "%s: %s (+%d)" "$(type_label "${best_type}")" "${first_cleaned}" "$((count - 1))"
  else
    printf "%s: %s" "$(type_label "${best_type}")" "${first_cleaned}"
  fi
}

title="${PR_TITLE:-"$(pick_title "${base_branch}")"}"

# Optional knobs:
# - PR_DRAFT=1 to open as draft
# - PR_TITLE="..." to override the PR title
# - PR_SCREENSHOTS=1 to force include Screenshots section
# - PR_SCREENSHOTS=0 to force omit Screenshots section
# - PR_TESTS=1 to force include Tests section
# - PR_TESTS=0 to force omit Tests section
# - PR_ADDITIONAL_INFO=... to include Additional info section text
# - PR_MAJOR_CHANGES=... to prefill Major changes bullet list text

draft_flag=()
if [[ "${PR_DRAFT:-}" == "1" ]]; then
  draft_flag=(--draft)
fi

has_cmd() { command -v "$1" >/dev/null 2>&1; }

should_include_screenshots() {
  if [[ "${PR_SCREENSHOTS:-}" == "1" ]]; then return 0; fi
  if [[ "${PR_SCREENSHOTS:-}" == "0" ]]; then return 1; fi

  # Auto: include if image assets changed on this branch.
  local names
  names="$(git diff --name-only "${base_branch}...HEAD" 2>/dev/null || true)"
  if printf "%s\n" "${names}" | rg -qi '\.(png|jpe?g|gif|webp|svg)$'; then
    return 0
  fi
  return 1
}

detect_test_command() {
  # Print a suggested test command if tests appear configured, else print empty.
  # Heuristic: prefer explicit build tools present in repo.
  if [[ -f package.json ]]; then
    # If python3 exists, parse package.json reliably.
    if has_cmd python3; then
      local test_script
      test_script="$(python3 - <<'PY' 2>/dev/null || true
import json
try:
  with open("package.json","r",encoding="utf-8") as f:
    pkg=json.load(f)
  s=((pkg.get("scripts") or {}).get("test") or "").strip()
  print(s)
except Exception:
  pass
PY
)"
      if [[ -n "${test_script}" ]]; then
        # Common placeholder: "echo \"Error: no test specified\" && exit 1"
        if printf "%s" "${test_script}" | rg -qi 'no test specified'; then
          :
        else
          if [[ -f pnpm-lock.yaml ]]; then echo "pnpm test"; return 0; fi
          if [[ -f yarn.lock ]]; then echo "yarn test"; return 0; fi
          if [[ -f package-lock.json ]]; then echo "npm test"; return 0; fi
          echo "npm test"; return 0
        fi
      fi
    else
      # Fallback: coarse grep for a test script key.
      if rg -n '"scripts"\s*:\s*\{' package.json >/dev/null 2>&1 && rg -n '"test"\s*:\s*"' package.json >/dev/null 2>&1; then
        if [[ -f pnpm-lock.yaml ]]; then echo "pnpm test"; return 0; fi
        if [[ -f yarn.lock ]]; then echo "yarn test"; return 0; fi
        if [[ -f package-lock.json ]]; then echo "npm test"; return 0; fi
        echo "npm test"; return 0
      fi
    fi
  fi

  if [[ -f go.mod ]]; then echo "go test ./..."; return 0; fi
  if [[ -f Cargo.toml ]]; then echo "cargo test"; return 0; fi
  if [[ -f pytest.ini || -f pyproject.toml || -f setup.cfg ]]; then
    if rg -n 'pytest' pyproject.toml setup.cfg 2>/dev/null | head -n 1 >/dev/null 2>&1; then
      echo "pytest"; return 0
    fi
  fi
  if [[ -f Makefile || -f makefile ]]; then
    if rg -n '^\s*test\s*:' Makefile makefile 2>/dev/null | head -n 1 >/dev/null 2>&1; then
      echo "make test"; return 0
    fi
  fi

  echo ""
}

should_include_tests() {
  if [[ "${PR_TESTS:-}" == "1" ]]; then return 0; fi
  if [[ "${PR_TESTS:-}" == "0" ]]; then return 1; fi
  local cmd
  cmd="$(detect_test_command)"
  [[ -n "${cmd}" ]]
}

body_file="$(mktemp -t gh-pr-body.XXXXXX)"
cleanup() { rm -f "${body_file}"; }
trap cleanup EXIT

test_cmd="$(detect_test_command)"

{
  echo "## Summary"
  if [[ -n "${diffstat}" ]]; then
    echo
    echo "\`\`\`"
    echo "${diffstat}"
    echo "\`\`\`"
  fi

  if [[ -n "${commit_list}" ]]; then
    echo
    echo "Commits:"
    echo
    echo "\`\`\`"
    echo "${commit_list}"
    echo "\`\`\`"
  fi

  echo
  echo "## Major changes"
  echo "${PR_MAJOR_CHANGES:-"- "}"

  if should_include_screenshots; then
    echo
    echo "## Screenshots"
    echo "- "
  fi

  if should_include_tests; then
    echo
    echo "## Tests"
    if [[ -n "${test_cmd}" ]]; then
      echo "- \`${test_cmd}\`"
    else
      echo "- "
    fi
  fi

  if [[ -n "${PR_ADDITIONAL_INFO:-}" ]]; then
    echo
    echo "## Additional info"
    echo "${PR_ADDITIONAL_INFO}"
  fi
} >"${body_file}"

# If a PR already exists for this branch, avoid creating duplicates.
if gh pr view --head "${branch}" >/dev/null 2>&1; then
  echo "info: PR already exists for branch '${branch}'." >&2
  gh pr view --head "${branch}" --web
  exit 0
fi

# Create the PR.
# Use --head explicitly for clarity.
# Let gh infer base if possible; still pass base as a best effort.
set +e
create_out="$(gh pr create "${draft_flag[@]}" --title "${title}" --body-file "${body_file}" --base "${base_branch}" --head "${branch}" 2>&1)"
status=$?
set -e

if [[ $status -ne 0 ]]; then
  echo "error: failed to create PR." >&2
  echo "${create_out}" >&2
  exit $status
fi

# Print the URL (gh already prints it, but keep it explicit for logs).
url="$(printf "%s\n" "${create_out}" | tail -n 1)"
echo "${url}"
