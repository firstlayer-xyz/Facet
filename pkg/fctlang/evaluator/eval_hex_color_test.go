package evaluator

import "testing"

// TestParseHexColorRejectsGarbage guards the fix for Sscanf's prefix matching:
// "%02x" happily consumed a single digit and ignored trailing non-hex bytes, so
// "abcdeg" parsed as a valid color. Every rune must now be a hex digit.
func TestParseHexColorRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"abcdeg", "1234567g", "gg0000", "#12x456", "12345"} {
		if _, _, _, _, err := parseHexColorRGBA(bad); err == nil {
			t.Errorf("parseHexColorRGBA(%q) should error", bad)
		}
	}
	for _, good := range []string{"ff0000", "#00ff00", "abc", "ff0000ff", "AABBCC"} {
		if _, _, _, _, err := parseHexColorRGBA(good); err != nil {
			t.Errorf("parseHexColorRGBA(%q) should succeed, got %v", good, err)
		}
	}
}
