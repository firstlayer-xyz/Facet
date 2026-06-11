package main

import "testing"

// TestComingSoonCLIsAreDisabledKnownProviders verifies comingSoonCLIs returns
// exactly the known providers absent from enabledCLIs, each with ID and Name
// populated for the settings picker's greyed-out entries.
func TestComingSoonCLIsAreDisabledKnownProviders(t *testing.T) {
	soon := comingSoonCLIs()
	if len(soon) == 0 {
		t.Fatal("expected coming-soon providers while only Claude is enabled")
	}

	names := map[string]string{}
	for _, c := range knownCLIs {
		names[c.ID] = c.Name
	}
	for _, c := range soon {
		if enabledCLIs[c.ID] {
			t.Errorf("coming-soon list includes enabled provider %q", c.ID)
		}
		name, ok := names[c.ID]
		if !ok {
			t.Errorf("coming-soon provider %q is not in knownCLIs", c.ID)
			continue
		}
		if c.Name != name {
			t.Errorf("coming-soon provider %q name = %q, want %q", c.ID, c.Name, name)
		}
	}
}

// TestEnabledAndComingSoonPartitionKnownCLIs verifies the enabled set and the
// coming-soon list together cover every known provider exactly once, and that
// every enabled ID actually exists in knownCLIs — so enabling a provider can't
// silently reference a typo, and adding one can't be forgotten in either list.
func TestEnabledAndComingSoonPartitionKnownCLIs(t *testing.T) {
	inSoon := map[string]bool{}
	for _, c := range comingSoonCLIs() {
		inSoon[c.ID] = true
	}

	knownIDs := map[string]bool{}
	for _, c := range knownCLIs {
		knownIDs[c.ID] = true
		if inSoon[c.ID] == enabledCLIs[c.ID] {
			t.Errorf("provider %q: enabled=%v comingSoon=%v — must be exactly one",
				c.ID, enabledCLIs[c.ID], inSoon[c.ID])
		}
	}
	for id := range enabledCLIs {
		if !knownIDs[id] {
			t.Errorf("enabledCLIs references unknown provider %q", id)
		}
	}
}

// TestDetectAssistantCLIsFiltersDisabledProviders pins the
// enabledCLIs filter on the detection path: with a fake probe that "finds"
// every known binary, only the enabled providers come back. Drops a future
// refactor that loses the filter on the floor before it ships.
func TestDetectAssistantCLIsFiltersDisabledProviders(t *testing.T) {
	probeFindsAll := func(string) string { return "/fake/bin" }
	noModels := func(string, string) []string { return nil }

	got := detectAssistantCLIs(probeFindsAll, noModels)
	if len(got) == 0 {
		t.Fatal("expected at least one enabled CLI to be detected")
	}
	for _, c := range got {
		if !enabledCLIs[c.ID] {
			t.Errorf("detected disabled provider %q (filter regressed)", c.ID)
		}
	}
	// Every enabled provider whose binary "exists" must be in the result.
	for _, cli := range knownCLIs {
		if !enabledCLIs[cli.ID] {
			continue
		}
		found := false
		for _, c := range got {
			if c.ID == cli.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("enabled provider %q not detected despite available binary", cli.ID)
		}
	}
}

// TestDetectAssistantCLIsSkipsMissingBinaries confirms the probe gate: a
// provider whose binary isn't found stays out of the result even when
// enabled.
func TestDetectAssistantCLIsSkipsMissingBinaries(t *testing.T) {
	probeFindsNothing := func(string) string { return "" }
	noModels := func(string, string) []string { return nil }

	got := detectAssistantCLIs(probeFindsNothing, noModels)
	if len(got) != 0 {
		t.Errorf("expected no CLIs when probe finds none, got %d: %+v", len(got), got)
	}
}

// TestIsKnownCLI confirms the helper distinguishes known providers (in
// knownCLIs) from arbitrary strings. Used by Send() to decide whether a
// non-enabled cliID is a stale-but-known selection (coerce to claude) or a
// typo/unknown id (surface an explicit error).
func TestIsKnownCLI(t *testing.T) {
	if !isKnownCLI("claude") {
		t.Error("claude should be a known CLI")
	}
	if !isKnownCLI("ollama") {
		t.Error("ollama should be a known CLI (disabled but defined)")
	}
	if isKnownCLI("not-a-real-cli") {
		t.Error("unknown id should not match")
	}
	if isKnownCLI("") {
		t.Error("empty id should not match")
	}
}

// TestParseEffortLevels parses the --effort levels out of the claude --help
// text, including the common case where the (a, b, c) list wraps onto the line
// after the flag.
func TestParseEffortLevels(t *testing.T) {
	help := "Options:\n" +
		"  --model <model>                       Model for the current session\n" +
		"  --effort <level>                      Effort level for the current session\n" +
		"                                        (low, medium, high, xhigh, max)\n" +
		"  --exclude-dynamic-system-prompt-sections\n"
	got := parseEffortLevels(help)
	want := []string{"low", "medium", "high", "xhigh", "max"}
	if len(got) != len(want) {
		t.Fatalf("parseEffortLevels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("level %d = %q, want %q", i, got[i], want[i])
		}
	}

	if lv := parseEffortLevels("no effort flag here"); lv != nil {
		t.Errorf("expected nil when --effort absent, got %v", lv)
	}
	if lv := parseEffortLevels("  --effort <level>   Effort level (no list yet"); lv != nil {
		t.Errorf("expected nil when the levels group is unterminated, got %v", lv)
	}
}

// TestParseAnthropicModels verifies model-ID extraction from a /v1/models
// response, and that malformed or empty payloads yield nil so the caller falls
// back to the hardcoded aliases.
func TestParseAnthropicModels(t *testing.T) {
	body := []byte(`{
		"data": [
			{"type": "model", "id": "claude-opus-4-8", "display_name": "Claude Opus 4.8"},
			{"type": "model", "id": "claude-sonnet-4-6", "display_name": "Claude Sonnet 4.6"}
		],
		"has_more": false
	}`)
	got := parseAnthropicModels(body)
	want := []string{"claude-opus-4-8", "claude-sonnet-4-6"}
	if len(got) != len(want) {
		t.Fatalf("parseAnthropicModels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("model[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if r := parseAnthropicModels([]byte("not json")); r != nil {
		t.Errorf("malformed JSON: got %v, want nil", r)
	}
	if r := parseAnthropicModels([]byte(`{"data":[]}`)); r != nil {
		t.Errorf("empty data: got %v, want nil", r)
	}
}
