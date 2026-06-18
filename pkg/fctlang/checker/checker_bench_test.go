package checker

import "testing"

// Check runs on every debounced keystroke, and the stdlib — the bulk of the
// program by far — is immutable and ships pre-validated. Re-checking its bodies
// each time is pure waste. This benchmark runs Check over a tiny user program
// plus the full stdlib; the validateFunctions stdlib-skip is what keeps it cheap.
func BenchmarkCheckWithStdlib(b *testing.B) {
	s, err := parse("fn Main() Solid {\n    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})\n}\n")
	if err != nil {
		b.Fatal(err)
	}
	prog := testStdlibLibs()
	prog.Sources[testMainKey] = s
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Check(prog)
	}
}
