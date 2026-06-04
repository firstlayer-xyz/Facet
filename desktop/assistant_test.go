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
