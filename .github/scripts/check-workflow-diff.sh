#!/usr/bin/env bash
# check-workflow-diff.sh
#
# Inspects the unified diff between $BASE_REF and $HEAD_REF for risky
# changes to .github/** paths. Exits 0 on pass, 1 on detected risk.
#
# The PR_LABELS env var is a newline-separated list of labels on the
# PR; a label of "workflow-change-approved" disables the risk check.
#
# This script is a tripwire, not a wall — see the spec for limits.

set -uo pipefail

BASE_REF="${BASE_REF:-origin/main}"
HEAD_REF="${HEAD_REF:-HEAD}"
PR_LABELS="${PR_LABELS:-}"
WORKFLOW_PATHS="${WORKFLOW_PATHS:-.github/}"

# Resolve a merge-base. Fall back to BASE_REF directly if no shared
# history exists (e.g. in test harnesses that create orphan branches).
if MERGE_BASE=$(git merge-base "$BASE_REF" "$HEAD_REF" 2>/dev/null); then
  :
else
  MERGE_BASE="$BASE_REF"
fi

# Split colon-separated paths into git pathspec args.
IFS=':' read -r -a PATHSPEC <<< "$WORKFLOW_PATHS"

DIFF=$(git diff "$MERGE_BASE" "$HEAD_REF" -- "${PATHSPEC[@]}" 2>/dev/null || true)

if [ -z "$DIFF" ]; then
  echo "::notice::No changes under ${WORKFLOW_PATHS} — workflow-diff-check passes trivially."
  exit 0
fi

# Override label short-circuit.
if printf '%s\n' "$PR_LABELS" | grep -qx "workflow-change-approved"; then
  echo "::notice::PR carries the workflow-change-approved label — risk inspection bypassed."
  echo "::notice::Reviewer is responsible for confirming the workflow change is safe."
  exit 0
fi

FAIL=0
check() {
  local pattern="$1" description="$2"
  if printf '%s\n' "$DIFF" | grep -qE "$pattern"; then
    echo "::error::Risky workflow change detected: ${description}"
    echo "::error::Pattern matched: ${pattern}"
    FAIL=1
  fi
}

check '^\+.*pull_request_target' \
      "Added pull_request_target trigger (gives PRs access to secrets + write context)"
check '^\+.*permissions:[[:space:]]*write-all' \
      "Escalated workflow permissions to write-all"
check '^-[[:space:]]*if:.*pull_request' \
      "Removed a pull_request gate (may expose a secret-using step to PRs)"
check '^\+.*ref:[[:space:]]*\$\{\{[[:space:]]*github\.event\.pull_request\.head\.sha' \
      "Checking out PR head SHA in elevated context (pwn-request pattern)"

# Patterns scoped to specific files (ci.yml and guards.yml must never reference secrets).
DIFF_CI=$(git diff "$MERGE_BASE" "$HEAD_REF" -- .github/workflows/ci.yml .github/workflows/guards.yml 2>/dev/null || true)
if printf '%s\n' "$DIFF_CI" | grep -qE '^\+.*\$\{\{[[:space:]]*secrets\.'; then
  echo "::error::Risky workflow change detected: secrets reference added to ci.yml or guards.yml (these files must remain secret-free)"
  FAIL=1
fi

# CODEOWNERS changes.
DIFF_CO=$(git diff "$MERGE_BASE" "$HEAD_REF" -- .github/CODEOWNERS 2>/dev/null || true)
if [ -n "$DIFF_CO" ]; then
  echo "::error::Risky workflow change detected: .github/CODEOWNERS modified"
  FAIL=1
fi

# Environment-block changes in release.yml.
DIFF_REL=$(git diff "$MERGE_BASE" "$HEAD_REF" -- .github/workflows/release.yml 2>/dev/null || true)
if printf '%s\n' "$DIFF_REL" | grep -qE '^[+-][[:space:]]*(name:[[:space:]]*signing-|environment:|deployment_branch_policy)'; then
  echo "::error::Risky workflow change detected: environment binding modified in release.yml"
  FAIL=1
fi

if [ "$FAIL" -eq 1 ]; then
  echo "::error::workflow-diff-check failed. If this change is intentional, add the label 'workflow-change-approved' to the PR (requires owner approval per CODEOWNERS)."
  exit 1
fi

echo "::notice::No risky patterns detected."
exit 0
