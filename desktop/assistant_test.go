package main

import "testing"

// TestComingSoonCLIsAreDisabledKnownProviders verifies ComingSoonCLIs returns
// exactly the known providers absent from enabledCLIs, each with ID and Name
// populated for the settings picker's greyed-out entries.
func TestComingSoonCLIsAreDisabledKnownProviders(t *testing.T) {
	soon := (&App{}).ComingSoonCLIs()
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
	for _, c := range (&App{}).ComingSoonCLIs() {
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
