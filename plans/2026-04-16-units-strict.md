# Units-Strict Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Length/Number type semantics strict and consistent between checker and evaluator. Length→Number conversion requires the existing explicit `Number()` stdlib function; no silent stripping anywhere.

**Architecture:** The bug has three parallel symptoms — an `asNumber`-based arithmetic fallthrough in `eval_ops.go`, a `requireNumber` helper that accepts Length, and a permissive checker op table in `check_ops.go`. Fix all three to agree on one rule set. Migrate builtin call sites and stdlib to the new rules.

**Tech Stack:** Go (evaluator + checker), Facet language (`.fct` stdlib), `make test` for verification (never raw `go test`).

## Rule Set (source of truth for this plan)

| Expression | Result |
|---|---|
| `Number + Number`, `-`, `*`, `/`, `%` | Number |
| `Length + Length`, `-` | Length |
| `Length / Length` | Number (dimensionless ratio) |
| `Length * Length` | **Error** (no Area type) |
| `Length % Length` | **Error** (modulo of lengths is rarely what's meant; use `Number(a) % Number(b)` explicitly) |
| `Length * Number`, `Number * Length` | **Length** (scale) |
| `Length / Number` | **Length** (scale down) |
| `Length + Number`, `Number + Length`, `-`, `%` | **Error**; user writes `5 mm` or `Number(len) + n` |
| `Angle + Angle`, `-` | Angle |
| `Angle / Angle` | Number |
| `Angle * Number`, `Number * Angle` | Angle |
| `Angle / Number` | Angle |
| Comparisons `Length vs Number` / vice versa | Allowed; Number→Length at comparison boundary |

**Boundary coercion:** `Number → Length` and `Number → Angle` still auto-coerce at function-argument, variable-declaration, and return boundaries (checker's existing `typeCompatible`). `Length → Number` does **not** coerce anywhere — users must call `Number(x)`.

**Helpers:**
- `requireLength(v)` — accepts `length` OR bare `float64` (boundary-coerced case); returns mm.
- `requireNumber(v)` — accepts **only** `float64`; errors on `length`.
- `asNumber(v)` — unchanged for for-loop range bounds (out of scope); NOT used in `evalBinary` arithmetic anymore.

**Non-goals for this plan:**
- `Area` type (deferred).
- `asNumber` semantics in for-loops / ranges (`eval_constraints.go`, `eval_expr.go` range eval). Keep as-is.
- Facetlibs migration beyond stdlib. The stdlib under `app/stdlib/libraries/facet/std/std.fct` is in scope; external facetlibs at `/facetlibs/` are not.

---

## File Structure

**Evaluator (`app/pkg/fctlang/evaluator/`):**
- `eval_ops.go` — rewrite the arithmetic fallthrough (delete `asNumber` use at lines 259-285); add explicit `Length×Number = Length`, `Number×Length = Length`, `Length/Number = Length`, `Length+Length = Length` cases.
- `eval_helpers.go` — change `requireNumber` to reject `length`.
- `eval_builtins_solid.go` — swap `requireNumber` → `requireLength` for x/y/z coordinate args in Translate, TranslateXY, ScaleAt-style positions, etc.
- `eval_builtins_sketch.go` — same for any positional Length args (e.g. `scaleX`, `scaleY` may actually be Number — audit case-by-case).
- `eval_operators_test.go` — add new tests for strict rules; update any existing test that relied on old lenient behavior.
- `eval_errors_test.go` — add tests for expected errors (`Length + Number`, `Length * Length`, `requireNumber(5 mm)`).

**Checker (`app/pkg/fctlang/checker/`):**
- `check_ops.go` — rewrite `inferBinaryOp`:
  - `Length × Length` (op `*`): error instead of Number.
  - `Length × Number` / `Number × Length`: only `*` and `/` return Length; `+` `-` `%` error.
- `checker_test.go` — add tests for new errors.

**Stdlib (`app/stdlib/libraries/facet/std/std.fct`):**
- Audit for patterns that break under new rules. Primary suspects: `Length * Length` (unlikely in a geometry stdlib), mixed `Length + Number` (likely in scaled positions), `requireNumber`-fed args that receive Length.

**Docs (`app/pkg/fctlang/README.md`):**
- Add a short "Type System" section documenting the rule set above.

---

## Task 1: Failing evaluator tests for strict arithmetic

**Files:**
- Modify: `app/pkg/fctlang/evaluator/eval_operators_test.go`
- Modify: `app/pkg/fctlang/evaluator/eval_errors_test.go`

- [ ] **Step 1: Add positive tests to `eval_operators_test.go`**

Append these tests to the existing file (after the last test function):

```go
// TestEvalLengthTimesNumber verifies Length*Number = Length (scale).
func TestEvalLengthTimesNumber(t *testing.T) {
	src := `fn Main() Solid {
    var w = 5 mm * 2;
    return Cube(size: Vec3{x: w, y: w, z: w});
}`
	// w must evaluate to 10 mm. If it becomes dimensionless Number, Cube size rejects it.
	result := mustEval(t, src, "Main")
	if len(result.Solids) != 1 {
		t.Fatalf("expected 1 solid, got %d", len(result.Solids))
	}
}

// TestEvalNumberTimesLength verifies Number*Length = Length (commutative).
func TestEvalNumberTimesLength(t *testing.T) {
	src := `fn Main() Solid {
    var w = 2 * 5 mm;
    return Cube(size: Vec3{x: w, y: w, z: w});
}`
	result := mustEval(t, src, "Main")
	if len(result.Solids) != 1 {
		t.Fatalf("expected 1 solid, got %d", len(result.Solids))
	}
}

// TestEvalLengthDivNumber verifies Length/Number = Length.
func TestEvalLengthDivNumber(t *testing.T) {
	src := `fn Main() Solid {
    var w = 10 mm / 2;
    return Cube(size: Vec3{x: w, y: w, z: w});
}`
	result := mustEval(t, src, "Main")
	if len(result.Solids) != 1 {
		t.Fatalf("expected 1 solid, got %d", len(result.Solids))
	}
}
```

**Note on `mustEval`:** Verify this helper exists in `helpers_test.go`. If the name differs, grep for the existing pattern in `eval_operators_test.go` (look at `TestEvalComparisonLength` at line 342 to see the idiom in use) and match that.

- [ ] **Step 2: Add error-expecting tests to `eval_errors_test.go`**

Append:

```go
// TestErrorLengthPlusNumber: bare Number added to Length is an error.
func TestErrorLengthPlusNumber(t *testing.T) {
	src := `fn Main() Solid {
    var w = 5 mm + 3;
    return Cube(size: Vec3{x: w, y: w, z: w});
}`
	_, err := tryEval(src, "Main")
	if err == nil {
		t.Fatal("expected error for Length + Number, got nil")
	}
	if !strings.Contains(err.Error(), "incompatible") && !strings.Contains(err.Error(), "Length") {
		t.Errorf("expected dimensional error, got: %v", err)
	}
}

// TestErrorLengthTimesLength: Length * Length has no Area type — error.
func TestErrorLengthTimesLength(t *testing.T) {
	src := `fn Main() Solid {
    var a = 5 mm * 3 mm;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}`
	_, err := tryEval(src, "Main")
	if err == nil {
		t.Fatal("expected error for Length * Length, got nil")
	}
}

// TestErrorRequireNumberRejectsLength: builtins typed Number must reject Length.
// Lerp(from Number, to Number, t Number) — passing a Length as t is a type error.
func TestErrorRequireNumberRejectsLength(t *testing.T) {
	src := `fn Main() Solid {
    var x = Lerp(from: 0, to: 10, t: 5 mm);
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}`
	_, err := tryEval(src, "Main")
	if err == nil {
		t.Fatal("expected error when passing Length to Number param, got nil")
	}
}
```

**Note on `tryEval`:** If a helper that returns `(result, error)` without `t.Fatal`-ing on error does not exist, inline the eval call using the pattern already in `eval_errors_test.go`. Read the first existing test in that file to see the idiom and copy it.

- [ ] **Step 3: Run the new tests and verify they fail**

```bash
make test 2>&1 | grep -E '(FAIL|PASS).*(LengthTimes|LengthDiv|NumberTimes|ErrorLength|ErrorRequire)'
```

Expected: the three `TestEval*` tests fail (current code returns Number for `Length * Number`). The three error tests may fail differently — either they PASS (current code errors for other reasons) or FAIL (current code silently accepts). Either way, record the output; correct behavior after the fix is all six passing.

- [ ] **Step 4: Commit the failing tests**

```bash
git add app/pkg/fctlang/evaluator/eval_operators_test.go app/pkg/fctlang/evaluator/eval_errors_test.go
git commit -m "test(fctlang): add failing tests for strict Length/Number arithmetic"
```

---

## Task 2: Fix evaluator arithmetic in `eval_ops.go`

**Files:**
- Modify: `app/pkg/fctlang/evaluator/eval_ops.go:237-285`

- [ ] **Step 1: Replace the Length/Length + Length/Number fallthrough blocks**

Replace the code block from line 237 (`// Length / Length → Number (dimensionless ratio)`) through line 285 (end of `evalBinary`) with:

```go
	// Length arithmetic
	ll, lIsLength := lv.(length)
	rl, rIsLength := rv.(length)
	lf, lIsNum := lv.(float64)
	rf, rIsNum := rv.(float64)

	// Length op Length
	if lIsLength && rIsLength {
		switch ex.Op {
		case "+":
			return length{mm: ll.mm + rl.mm}, nil
		case "-":
			return length{mm: ll.mm - rl.mm}, nil
		case "%":
			if rl.mm == 0 {
				return nil, e.errAt(ex.Pos, "modulo by zero")
			}
			return length{mm: math.Mod(ll.mm, rl.mm)}, nil
		case "/":
			if rl.mm == 0 {
				return nil, e.errAt(ex.Pos, "division by zero")
			}
			return ll.mm / rl.mm, nil
		case "*":
			return nil, e.errAt(ex.Pos, "operator *: Length * Length has no Area type; use Number(a) * Number(b) if you want a dimensionless product")
		default:
			return nil, e.errAt(ex.Pos, "operator %s: incompatible types Length and Length", ex.Op)
		}
	}

	// Length op Number (scale)
	if lIsLength && rIsNum {
		switch ex.Op {
		case "*":
			return length{mm: ll.mm * rf}, nil
		case "/":
			if rf == 0 {
				return nil, e.errAt(ex.Pos, "division by zero")
			}
			return length{mm: ll.mm / rf}, nil
		default:
			return nil, e.errAt(ex.Pos, "operator %s: incompatible types Length and Number; use Number(len) %s n or n mm", ex.Op, ex.Op)
		}
	}

	// Number op Length (commutative multiply only)
	if lIsNum && rIsLength {
		switch ex.Op {
		case "*":
			return length{mm: lf * rl.mm}, nil
		default:
			return nil, e.errAt(ex.Pos, "operator %s: incompatible types Number and Length; use n mm %s len or n %s Number(len)", ex.Op, ex.Op, ex.Op)
		}
	}

	return nil, e.errAt(ex.Pos, "operator %s: incompatible types %s and %s", ex.Op, typeName(lv), typeName(rv))
}
```

Key points:
- Removed the `asNumber` path entirely — no silent stripping in arithmetic.
- `Length / Length` is now inside the `lIsLength && rIsLength` block (was a separate case at line 238-248).
- Error messages hint at the user-facing fix (`use Number(len)` / `use n mm`).
- Preserve the existing `import "math"` (already imported at line 6).

- [ ] **Step 2: Run operator tests**

```bash
make test 2>&1 | grep -E 'FAIL|ok' | head -20
```

Expected: `TestEvalLengthTimesNumber`, `TestEvalNumberTimesLength`, `TestEvalLengthDivNumber` now PASS. `TestErrorLengthPlusNumber`, `TestErrorLengthTimesLength` now PASS. Some pre-existing tests may fail — that's Task 6 territory; record them and move on.

- [ ] **Step 3: Commit**

```bash
git add app/pkg/fctlang/evaluator/eval_ops.go
git commit -m "fix(fctlang): strict Length/Number arithmetic in evaluator"
```

---

## Task 3: Fix `requireNumber` to reject Length

**Files:**
- Modify: `app/pkg/fctlang/evaluator/eval_helpers.go:21-33`

- [ ] **Step 1: Replace `requireNumber`**

Replace lines 21-33 with:

```go
// requireNumber extracts a plain numeric value from an argument.
// Accepts only float64 (Number). Length values are rejected — use Number(x)
// to convert explicitly.
func requireNumber(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	n, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("%s() argument %d must be a Number, got %s (use Number(x) to convert a Length)", funcName, argNum, typeName(v))
	}
	return n, nil
}
```

Leave `requireLength` unchanged — it continues to accept both Length and bare Number (matching the checker's boundary coercion rule).

- [ ] **Step 2: Run tests**

```bash
make test 2>&1 | grep -E 'FAIL|ok' | head -40
```

Expected: `TestErrorRequireNumberRejectsLength` PASSes. Many builtin-using tests likely fail — specifically any test that exercises Solid transform builtins with Length coordinates (Translate, Rotate, etc.). That's Task 4.

- [ ] **Step 3: Commit**

```bash
git add app/pkg/fctlang/evaluator/eval_helpers.go
git commit -m "fix(fctlang): requireNumber rejects Length values"
```

---

## Task 4: Switch Length-typed builtin args to `requireLength`

**Files:**
- Modify: `app/pkg/fctlang/evaluator/eval_builtins_solid.go` (many sites)
- Modify: `app/pkg/fctlang/evaluator/eval_builtins_sketch.go` (audit; likely none)
- Modify: `app/pkg/fctlang/evaluator/eval_builtins_struct.go` (audit)

- [ ] **Step 1: Identify sites that receive Length per the stdlib signatures**

Run:

```bash
grep -n 'requireNumber' app/pkg/fctlang/evaluator/eval_builtins_solid.go
```

For each hit, find the corresponding stdlib signature in `app/stdlib/libraries/facet/std/std.fct`. Example mapping (verify by reading both files):

| Line in eval_builtins_solid.go | Builtin | Arg semantics | Change to |
|---|---|---|---|
| 107, 111, 115 | `_translate` x/y/z | Length | `requireLength` |
| 136, 140 | `_translate_xy` x/y | Length | `requireLength` |
| 297, 301, 305 | (confirm site) | Length if x/y/z | `requireLength` |
| 457, 461, 465 | Length if x/y/z | `requireLength` |
| 575, 579, 583 | rgb 0..1 | **Number** | leave as `requireNumber` |
| 329 | minSmoothness | Number | leave |
| 345 | count / n | Number | leave |

Coordinates (x/y/z, nx/ny/nz, ax/ay translate/rotate origin) → Length. Counts, factors, ratios, angles-as-numbers → stay Number.

- [ ] **Step 2: Apply edits**

For each coordinate site, change `requireNumber(...)` to `requireLength(...)`. Keep the same variable names and error handling. Do NOT change sites for non-coordinate args.

- [ ] **Step 3: Audit `eval_builtins_sketch.go`**

```bash
grep -n 'requireNumber' app/pkg/fctlang/evaluator/eval_builtins_sketch.go
```

For each, cross-reference the stdlib signature. `slices` (segment count) is Number. `scaleX`, `scaleY` in `_extrude` are likely scale factors (Number). If any turn out to be Length, switch.

- [ ] **Step 4: Audit `eval_builtins_struct.go` and others**

```bash
grep -rn 'requireNumber' app/pkg/fctlang/evaluator/
```

Cross-check each remaining site against its stdlib signature. Specifically verify `eval_builtins_struct.go:60,67` (looks like Vec/Pt construction — likely Length) and `:99` (factor — Number).

- [ ] **Step 5: Run full test suite**

```bash
make test 2>&1 | tail -30
```

Expected: pre-existing tests that exercise transforms (Translate, etc.) now pass. If new failures remain, they're likely stdlib `.fct` usages of the old lenient rules — those get fixed in Task 6.

- [ ] **Step 6: Commit**

```bash
git add app/pkg/fctlang/evaluator/
git commit -m "refactor(fctlang): use requireLength for coordinate builtin args"
```

---

## Task 5: Failing checker tests for strict op rules

**Files:**
- Modify: `app/pkg/fctlang/checker/checker_test.go`

- [ ] **Step 1: Add checker tests**

Append to `checker_test.go`. Use the existing `checkSource(t, src)` / `expectErrors(t, src, ...)` idiom — read the first 50 lines of the file to see the exact helper names and copy the pattern.

```go
// TestCheckerLengthPlusNumber: Length + Number is a type error.
func TestCheckerLengthPlusNumber(t *testing.T) {
	src := `fn F() Length { return 5 mm + 3 }`
	errs := checkErrors(t, src)
	if len(errs) == 0 {
		t.Fatal("expected checker error for Length + Number, got none")
	}
}

// TestCheckerLengthTimesLength: Length * Length is a type error.
func TestCheckerLengthTimesLength(t *testing.T) {
	src := `fn F() Number { return 5 mm * 3 mm }`
	errs := checkErrors(t, src)
	if len(errs) == 0 {
		t.Fatal("expected checker error for Length * Length, got none")
	}
}

// TestCheckerLengthTimesNumber: Length * Number is a Length, legal.
func TestCheckerLengthTimesNumber(t *testing.T) {
	src := `fn F() Length { return 5 mm * 3 }`
	errs := checkErrors(t, src)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

// TestCheckerLengthDivNumber: Length / Number is a Length.
func TestCheckerLengthDivNumber(t *testing.T) {
	src := `fn F() Length { return 10 mm / 2 }`
	errs := checkErrors(t, src)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}
```

If the helper is not named `checkErrors`, adjust. (Look near the top of `checker_test.go` for patterns like `func TestCheckerX(t *testing.T) { ... }` and copy the setup verbatim.)

- [ ] **Step 2: Run and verify the negative tests fail**

```bash
make test 2>&1 | grep -E 'FAIL.*Checker(Length)'
```

Expected: `TestCheckerLengthPlusNumber` and `TestCheckerLengthTimesLength` FAIL (current checker accepts these). `TestCheckerLengthTimesNumber` and `TestCheckerLengthDivNumber` may PASS already since the current checker returns Length for all mixed ops.

- [ ] **Step 3: Commit**

```bash
git add app/pkg/fctlang/checker/checker_test.go
git commit -m "test(fctlang): add failing checker tests for strict op rules"
```

---

## Task 6: Fix checker `inferBinaryOp`

**Files:**
- Modify: `app/pkg/fctlang/checker/check_ops.go:118-145`

- [ ] **Step 1: Replace the Length/Length and Length/Number blocks**

Replace lines 118-145 with:

```go
	// Length op Length
	if left.ft == typeLength && right.ft == typeLength {
		switch op {
		case "+", "-", "%":
			return simple(typeLength)
		case "/":
			return simple(typeNumber)
		case "*":
			c.addError(ex.Pos, "operator *: Length * Length has no Area type; use Number(a) * Number(b) if you want a dimensionless product")
			return unknown()
		default:
			c.addError(ex.Pos, fmt.Sprintf("unknown operator %q", op))
			return unknown()
		}
	}

	// Length * Number, Length / Number → Length (scale)
	if left.ft == typeLength && right.ft == typeNumber {
		switch op {
		case "*", "/":
			return simple(typeLength)
		default:
			c.addError(ex.Pos, fmt.Sprintf("operator %s: incompatible types Length and Number; use Number(len) %s n or n mm", op, op))
			return unknown()
		}
	}

	// Number * Length → Length (commutative multiply only)
	if left.ft == typeNumber && right.ft == typeLength {
		if op == "*" {
			return simple(typeLength)
		}
		c.addError(ex.Pos, fmt.Sprintf("operator %s: incompatible types Number and Length; use n mm %s len or n %s Number(len)", op, op, op))
		return unknown()
	}
```

- [ ] **Step 2: Run checker tests**

```bash
make test 2>&1 | grep -E 'FAIL|ok.*checker' | head -20
```

Expected: all four new checker tests pass. Pre-existing tests may fail if they exercise the old lenient behavior — those are stdlib or test-data issues. Record the failures for Task 7.

- [ ] **Step 3: Commit**

```bash
git add app/pkg/fctlang/checker/check_ops.go
git commit -m "fix(fctlang): strict Length/Number rules in checker"
```

---

## Task 7: Audit and migrate stdlib

**Files:**
- Modify: `app/stdlib/libraries/facet/std/std.fct`

- [ ] **Step 1: Run the full test suite and capture stdlib-related failures**

```bash
make test 2>&1 | tee /tmp/units-test.log | grep -E 'FAIL' | head -40
```

Read through failures. Many stdlib functions will be parse/check-validated against the new rules because the stdlib loads every time.

- [ ] **Step 2: Grep for suspect patterns in the stdlib**

```bash
grep -nE '(mm|Length)' app/stdlib/libraries/facet/std/std.fct | grep -E '[+\-%].*[0-9]' | head -40
```

Look for:
- `x + 1` where `x` is Length → rewrite as `x + 1 mm`
- `len * len` where both are Length → rewrite using `Number(a) * Number(b)` or reshape the function
- Functions declared returning `Number` but whose body returns a Length expression

- [ ] **Step 3: Fix each affected function**

For each failure, edit the stdlib:
- If the intent was "add n mm to a length", rewrite `x + n` → `x + n mm`.
- If the intent was a dimensionless ratio, rewrite `lenA * lenB` → `Number(lenA) * Number(lenB)`.
- If a function declared `Length` return but the body computes a Number, fix the return type.

Show the diff to yourself before saving each edit — don't change intent, just notation.

- [ ] **Step 4: Run tests until clean**

```bash
make test 2>&1 | tail -20
```

Iterate until all tests pass.

- [ ] **Step 5: Commit**

```bash
git add app/stdlib/libraries/facet/std/std.fct
git commit -m "refactor(stdlib): migrate to strict Length/Number rules"
```

---

## Task 8: Document the type system

**Files:**
- Modify: `app/pkg/fctlang/README.md`

- [ ] **Step 1: Add a "Type System" section**

After the existing "4. evaluator" section and before "Supporting packages", insert:

```markdown
## Type System

Facet has four primitive types — `Number`, `Length`, `Angle`, `Bool` — plus `String`, arrays, structs, and function types.

### Dimensional rules

- `Length + Length → Length`, `Length - Length → Length`, `Length % Length → Length`
- `Length / Length → Number` (dimensionless ratio)
- `Length * Number → Length`, `Number * Length → Length`, `Length / Number → Length` (scale)
- `Length * Length` — **error** (no Area type)
- `Length + Number` — **error**; write `x + 5 mm` or `Number(x) + n`

### Boundary coercion

At function-argument, variable-declaration, and return boundaries, bare `Number` coerces to `Length` or `Angle`:

```fct
fn F(x Length) Length { return x }
F(10)       // OK — 10 becomes 10 mm
var y Length = 5   // OK — y is 5 mm
```

The reverse coercion (`Length → Number`) never happens silently. To extract the raw mm value, call `Number(x)`:

```fct
fn ToNumber(x Length) Number { return Number(x) }   // explicit
```

### Angles

`Angle + Angle → Angle`, `Angle / Angle → Number`, `Angle * Number → Angle` (`/` also). Mixed `Angle + Number` is an error; write `a + 15 deg`.
```

- [ ] **Step 2: Commit**

```bash
git add app/pkg/fctlang/README.md
git commit -m "docs(fctlang): document strict Length/Number type system"
```

---

## Task 9: Full verification pass

**Files:** (none modified)

- [ ] **Step 1: Clean full test run**

```bash
make test
```

Expected: all green. If any failures remain, triage — they're either real regressions (fix them) or tests that were themselves wrong under the old rules (update them).

- [ ] **Step 2: Build the app**

```bash
make build
```

Expected: clean build. This catches any cascading API changes (e.g., if `asNumber` or a helper was wrongly deleted).

- [ ] **Step 3: Smoke test the dev server**

```bash
make dev &
# Wait a few seconds, open app, load an example, verify it renders
```

Manually run a couple of `.fct` examples that mix Length and Number (Grid, Chain, any arithmetic-heavy example). Confirm nothing obvious broke.

Kill `make dev` when done.

- [ ] **Step 4: Summary commit message / PR body draft**

Draft a PR description (in `/tmp/pr-body.md` or similar). Include:
- Rule table (from top of this plan).
- List of breaking changes for user code (`Length + Number` now errors, etc.).
- Migration note: `Number(x)` is the explicit conversion.

Do NOT push or open a PR — leave that to the user.

---

## Self-Review Checklist

- [x] **Spec coverage:** Every rule in the top table has a task that implements or tests it.
- [x] **Placeholder scan:** No "TBD", "similar to above", "handle edge cases" — all code blocks are concrete.
- [x] **Type consistency:** `requireLength` / `requireNumber` / `length` / `typeLength` names used consistently across tasks.
- [x] **Ordering:** Tests → evaluator fix → checker fix → stdlib migration → docs is correct (runtime truth first, then static check, then callers).
- [x] **Non-goals called out:** `Area` type, `asNumber` in for-loops, facetlibs — explicitly deferred.

## Known risks

- **Stdlib migration (Task 7) is open-ended.** Could touch 10-50 lines. Budget time for iteration.
- **Checker currently returns `typeLength` for `Length * Number`** — this already happens to be correct under the new rules. But it also returned `Length` for `Length + Number`, which is now wrong. Task 6's block replacement handles both.
- **Comparisons (`<`, `>`, `==`) are left alone.** The existing `inferComparison` at `check_ops.go:152-183` allows `Length vs Number` mixed comparison via Number→Length coercion — consistent with the boundary-coercion rule.
- **User-written operator functions.** `op` declarations in user code (e.g. `op Foo(Length, Length) Number`) bypass the hardcoded rules. They still work; users who defined a `*` op on Length,Length can keep doing so, which is consistent with the error message recommending `Number(a) * Number(b)` — user can also provide their own op.
