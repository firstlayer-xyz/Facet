package evaluator

import "testing"

// String.IndexOf must return a rune index, consistent with SubStr/Length. In
// "héllo" the substring "llo" starts at rune 2 but byte 3 (é is two bytes); a
// byte offset would mis-index the string.
func TestEvalStringIndexOfReturnsRuneIndex(t *testing.T) {
	stdlibIfThenCube(t, `("héllo".IndexOf(substr: "llo") ?? -1) == 2`)
}
