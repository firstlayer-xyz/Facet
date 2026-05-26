#!/usr/bin/env bash
# Tests for check-workflow-diff.sh
# Each test sets up a temp git repo, stages a diff, invokes the script,
# and asserts on the exit code and output.

set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$SCRIPT_DIR/check-workflow-diff.sh"

PASS=0
FAIL=0

# Run the script with a controlled diff. Args:
#   $1 = test name
#   $2 = expected exit code
#   $3 = labels (newline-separated)
#   stdin = a heredoc of the diff to apply on top of a baseline
run_case() {
  local name="$1"
  local expected="$2"
  local labels="${3:-}"

  local tmp
  tmp=$(mktemp -d)
  (
    cd "$tmp"
    git init -q -b main
    git config user.email "t@t"
    git config user.name "t"
    mkdir -p .github/workflows
    # Baseline: an empty ci.yml on main.
    cat > .github/workflows/ci.yml <<'EOF'
name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
EOF
    git add -A && git commit -q -m "baseline"
    git checkout -q -b feature
    # Apply the test's change.
    cat > .github/workflows/ci.yml
    # --allow-empty: the "no changes" test case writes identical content.
    git add -A && git commit -q --allow-empty -m "change"

    PR_LABELS="$labels" BASE_REF="main" HEAD_REF="HEAD" "$SCRIPT" > out 2>&1
    local actual=$?
    if [ "$actual" -eq "$expected" ]; then
      echo "PASS: $name"
    else
      echo "FAIL: $name  (expected exit $expected, got $actual)"
      echo "--- output ---"; cat out; echo "--- end ---"
    fi
  )
  rm -rf "$tmp"
}

# --- Test cases ---

run_case "no .github changes passes" 0 <<'EOF'
name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
EOF

run_case "pull_request_target added fails" 1 <<'EOF'
name: CI
on:
  push:
  pull_request_target:
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
EOF

run_case "secrets reference added to ci.yml fails" 1 <<'EOF'
name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo ${{ secrets.APPLE_ID }}
EOF

run_case "permissions write-all fails" 1 <<'EOF'
name: CI
on: [push]
permissions: write-all
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
EOF

run_case "override label bypasses risky change" 0 "workflow-change-approved" <<'EOF'
name: CI
on:
  push:
  pull_request_target:
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
EOF

echo
echo "Test run complete. (Counters not aggregated across subshells — inspect PASS/FAIL lines above.)"
