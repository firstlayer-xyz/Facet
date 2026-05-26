#!/usr/bin/env bash
# Tests for check-workflow-diff.sh
# Each test sets up a temp git repo, stages a diff, invokes the script,
# and asserts on the exit code and output.

set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$SCRIPT_DIR/check-workflow-diff.sh"

FAIL_FILE=$(mktemp)
rm -f "$FAIL_FILE"   # subshells touch it to signal failure; absence means all-pass

# Run the script with a controlled diff. Args:
#   $1 = test name
#   $2 = expected exit code
#   $3 = labels (newline-separated, optional)
#   $4 = target path to write stdin to (optional, default .github/workflows/ci.yml)
#   stdin = the new content to write to the target path
run_case() {
  local name="$1"
  local expected="$2"
  local labels="${3:-}"
  local target="${4:-.github/workflows/ci.yml}"

  local tmp
  tmp=$(mktemp -d)
  (
    cd "$tmp"
    git init -q -b main
    git config user.email "t@t"
    git config user.name "t"
    mkdir -p .github/workflows

    # Baseline files. Each test case modifies one of them via $target.
    cat > .github/workflows/ci.yml <<'EOF'
name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
EOF
    cat > .github/workflows/guards.yml <<'EOF'
name: Guards
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: echo guard
EOF
    cat > .github/workflows/release.yml <<'EOF'
name: Release
on:
  push:
    branches: [main]
jobs:
  sign:
    runs-on: ubuntu-latest
    if: github.event_name != 'pull_request'
    environment:
      name: signing-main
    steps:
      - run: echo sign
EOF
    cat > .github/CODEOWNERS <<'EOF'
.github/ @owner
EOF
    git add -A && git commit -q -m "baseline"
    git checkout -q -b feature

    # Apply the test's change to the specified target.
    cat > "$target"
    # --allow-empty: the "no changes" test case writes identical content.
    git add -A && git commit -q --allow-empty -m "change"

    PR_LABELS="$labels" BASE_REF="main" HEAD_REF="HEAD" "$SCRIPT" > out 2>&1
    local actual=$?
    if [ "$actual" -eq "$expected" ]; then
      echo "PASS: $name"
    else
      echo "FAIL: $name  (expected exit $expected, got $actual)"
      echo "--- output ---"; cat out; echo "--- end ---"
      touch "$FAIL_FILE"
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

run_case "removed pull_request gate fails" 1 "" .github/workflows/release.yml <<'EOF'
name: Release
on:
  push:
    branches: [main]
jobs:
  sign:
    runs-on: ubuntu-latest
    environment:
      name: signing-main
    steps:
      - run: echo sign
EOF
# Note: baseline has "if: github.event_name != 'pull_request'" at job level;
# this version omits it, triggering the removed-gate check.

run_case "pwn-request head SHA checkout fails" 1 "" .github/workflows/ci.yml <<'EOF'
name: CI
on:
  pull_request_target:
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
EOF

run_case "CODEOWNERS modification fails" 1 "" .github/CODEOWNERS <<'EOF'
.github/ @other-owner
EOF

run_case "release.yml environment binding modified fails" 1 "" .github/workflows/release.yml <<'EOF'
name: Release
on:
  push:
    branches: [main]
jobs:
  sign:
    runs-on: ubuntu-latest
    if: github.event_name != 'pull_request'
    environment:
      name: signing-release
    steps:
      - run: echo sign
EOF

run_case "secrets in guards.yml fails" 1 "" .github/workflows/guards.yml <<'EOF'
name: Guards
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: echo ${{ secrets.SOMETHING }}
EOF

echo
echo "Test run complete."
if [ -f "$FAIL_FILE" ]; then
  rm -f "$FAIL_FILE"
  echo "One or more tests failed."
  exit 1
fi
exit 0
