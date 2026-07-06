package loader

import (
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

// TestResolveRefPrefersFetchedHeadOverStaleLocalBranch reproduces the @main
// staleness bug: on a bare clone go-git freezes refs/heads/<default> at clone
// time while fetchAll advances only refs/remotes/origin/*. resolveRef must
// resolve a branch through its fetched remote-tracking ref, and resolveRepoHead
// (the Fork entry point) must agree — otherwise a pinned @main permanently
// serves the clone-time commit.
func TestResolveRefPrefersFetchedHeadOverStaleLocalBranch(t *testing.T) {
	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	if err != nil {
		t.Fatalf("git.Init: %v", err)
	}
	stale := writeCommit(t, store, "clone-time", map[string]string{"lib.fct": "// v1"})
	fresh := writeCommit(t, store, "post-fetch", map[string]string{"lib.fct": "// v2"})
	if stale == fresh {
		t.Fatal("commits must differ for the test to distinguish them")
	}

	setHash := func(name string, target plumbing.Hash) {
		ref := plumbing.NewHashReference(plumbing.ReferenceName(name), target)
		if err := store.SetReference(ref); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	setHash("refs/heads/main", stale)          // frozen local branch
	setHash("refs/remotes/origin/main", fresh) // fetched, up-to-date
	head := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName("refs/heads/main"))
	if err := store.SetReference(head); err != nil {
		t.Fatalf("set HEAD: %v", err)
	}

	got, err := resolveRef(repo, "main")
	if err != nil {
		t.Fatalf("resolveRef(main): %v", err)
	}
	if *got != fresh {
		t.Errorf("resolveRef(main) = %s, want fresh remote head %s (served stale local branch %s)",
			got.String()[:7], fresh.String()[:7], stale.String()[:7])
	}

	repoHead, err := resolveRepoHead(repo)
	if err != nil {
		t.Fatalf("resolveRepoHead: %v", err)
	}
	if repoHead != fresh {
		t.Errorf("resolveRepoHead = %s, want fresh %s (served stale %s)",
			repoHead.String()[:7], fresh.String()[:7], stale.String()[:7])
	}
}

// TestLibPathToNamespace pins the canonical namespace shape that both
// the checker (for `var T = lib "..."` typing) and the desktop /eval
// handler (for scoping the docIndex) rely on. The ref must be dropped;
// the subpath must be preserved. Local libraries pass through as-is.
func TestLibPathToNamespace(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"github.com/firstlayer-xyz/facetlibs@main", "github.com/firstlayer-xyz/facetlibs"},
		{"github.com/firstlayer-xyz/facetlibs/fasteners@main", "github.com/firstlayer-xyz/facetlibs/fasteners"},
		{"github.com/firstlayer-xyz/facetlibs/fasteners@3af7741", "github.com/firstlayer-xyz/facetlibs/fasteners"},
		{"github.com/firstlayer-xyz/facetlibs/fasteners@v1.2.3", "github.com/firstlayer-xyz/facetlibs/fasteners"},
		// No ref — still drop nothing because there's nothing to drop.
		{"github.com/firstlayer-xyz/facetlibs/fasteners", "github.com/firstlayer-xyz/facetlibs/fasteners"},
		// Local libraries pass through unchanged (no @ref or host structure).
		{"mylib", "mylib"},
		{"facet/gears@v1", "facet/gears@v1"}, // looks remote but lacks a host segment → local
	}
	for _, c := range cases {
		got := LibPathToNamespace(c.raw)
		if got != c.want {
			t.Errorf("LibPathToNamespace(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestParseLibPathLocal(t *testing.T) {
	lp, err := ParseLibPath("facet/gears@v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !lp.IsLocal {
		t.Error("expected IsLocal=true")
	}
	if lp.Raw != "facet/gears@v1" {
		t.Errorf("expected Raw='facet/gears@v1', got %q", lp.Raw)
	}
	if lp.Ref != "v1" {
		t.Errorf("expected Ref='v1', got %q", lp.Ref)
	}
}

func TestParseLibPathLocalNoRef(t *testing.T) {
	lp, err := ParseLibPath("facet/gears")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !lp.IsLocal {
		t.Error("expected IsLocal=true")
	}
	if lp.Ref != "" {
		t.Errorf("expected empty Ref, got %q", lp.Ref)
	}
	if lp.Raw != "facet/gears" {
		t.Errorf("expected Raw='facet/gears', got %q", lp.Raw)
	}
}

func TestParseLibPathRemote(t *testing.T) {
	lp, err := ParseLibPath("github.com/user/repo@v1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lp.IsLocal {
		t.Error("expected IsLocal=false")
	}
	if lp.Host != "github.com" {
		t.Errorf("expected Host='github.com', got %q", lp.Host)
	}
	if lp.User != "user" {
		t.Errorf("expected User='user', got %q", lp.User)
	}
	if lp.Repo != "repo" {
		t.Errorf("expected Repo='repo', got %q", lp.Repo)
	}
	if lp.Ref != "v1.0" {
		t.Errorf("expected Ref='v1.0', got %q", lp.Ref)
	}
	if lp.SubPath != "" {
		t.Errorf("expected SubPath='', got %q", lp.SubPath)
	}
	if lp.CloneURL() != "https://github.com/user/repo.git" {
		t.Errorf("unexpected CloneURL: %q", lp.CloneURL())
	}
}

func TestParseLibPathRemoteSubpath(t *testing.T) {
	lp, err := ParseLibPath("gitlab.com/user/monorepo/gears@main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lp.IsLocal {
		t.Error("expected IsLocal=false")
	}
	if lp.Host != "gitlab.com" {
		t.Errorf("expected Host='gitlab.com', got %q", lp.Host)
	}
	if lp.User != "user" {
		t.Errorf("expected User='user', got %q", lp.User)
	}
	if lp.Repo != "monorepo" {
		t.Errorf("expected Repo='monorepo', got %q", lp.Repo)
	}
	if lp.SubPath != "gears" {
		t.Errorf("expected SubPath='gears', got %q", lp.SubPath)
	}
	if lp.Ref != "main" {
		t.Errorf("expected Ref='main', got %q", lp.Ref)
	}
}

func TestParseLibPathRelative(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		subPath string
	}{
		{"sibling", "./knurling", "knurling"},
		{"uncle", "../knurling", "../knurling"},
		{"uncle with subdir", "../sub/knurling", "../sub/knurling"},
		{"cousin", "../../familyB/cousin", "../../familyB/cousin"},
		{"deeper sibling", "./sub/nested", "sub/nested"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp, err := ParseLibPath(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !lp.IsRelative {
				t.Error("expected IsRelative=true")
			}
			if lp.IsLocal {
				t.Error("expected IsLocal=false")
			}
			if lp.Host != "" || lp.User != "" || lp.Repo != "" || lp.Ref != "" {
				t.Errorf("expected empty remote fields, got host=%q user=%q repo=%q ref=%q", lp.Host, lp.User, lp.Repo, lp.Ref)
			}
			if lp.SubPath != tt.subPath {
				t.Errorf("expected SubPath=%q, got %q", tt.subPath, lp.SubPath)
			}
		})
	}
}

func TestParseLibPathErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"remote no ref", "github.com/user/repo", "remote imports require @ref"},
		{"single segment", "foo@v1", "at least 2 segments"},
		{"single segment no ref", "foo", "at least 2 segments"},
		{"remote too few segments", "github.com/user@v1", "at least host/user/repo"},
		{"empty ref", "github.com/user/repo@", "empty ref"},
		{"relative with ref", "./knurling@v1", "relative imports cannot carry @ref"},
		{"relative parent with ref", "../knurling@main", "relative imports cannot carry @ref"},
		{"relative empty name", "./", "need a name after"},
		{"relative parent empty", "../", "need a name after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseLibPath(tt.input)
			if err == nil {
				t.Fatalf("expected error for %q", tt.input)
			}
			if got := err.Error(); !contains(got, tt.want) {
				t.Errorf("error %q should contain %q", got, tt.want)
			}
		})
	}
}

func TestValidateLibPathTraversal(t *testing.T) {
	if err := validateLibPath("foo/../bar"); err == nil {
		t.Fatal("expected error for '..' traversal")
	}
	if err := validateLibPath("/abs/path"); err == nil {
		t.Fatal("expected error for absolute path")
	}
	if err := validateLibPath("single"); err == nil {
		t.Fatal("expected error for single-segment path")
	}
	if err := validateLibPath("vendor/name"); err != nil {
		t.Fatalf("unexpected error for vendor/name: %v", err)
	}
	// A trailing `..@ref` must not evade the traversal check (the final segment
	// would otherwise read as "..@main", not "..").
	if err := validateLibPath("github.com/u/r/sub/..@main"); err == nil {
		t.Fatal("expected error for '..' traversal hidden behind an @ref")
	}
	// A backslash-separated segment is a real traversal on Windows and must not
	// slip past the check by hiding inside a single forward-slash segment.
	if err := validateLibPath("vendor\\..\\secret"); err == nil {
		t.Fatal("expected error for '..' traversal using backslash separators")
	}
	if err := validateLibPath("vendor/sub\\..\\..\\etc"); err == nil {
		t.Fatal("expected error for mixed-separator '..' traversal")
	}
}

// TestParseLibPathRefVariants pins parsing across the three @ref shapes
// the user is allowed to write — branch, semver tag, and commit SHA —
// to guard against future regressions when changing the parser.
func TestParseLibPathRefVariants(t *testing.T) {
	cases := []struct {
		raw     string
		wantRef string
	}{
		{"github.com/firstlayer-xyz/facetlibs@main", "main"},
		{"github.com/firstlayer-xyz/facetlibs@v1.0", "v1.0"},
		{"github.com/firstlayer-xyz/facetlibs@v1.2.3", "v1.2.3"},
		{"github.com/firstlayer-xyz/facetlibs@release/1.0", "release/1.0"},
		{"github.com/firstlayer-xyz/facetlibs@abc1234", "abc1234"},
		// 40-char full SHA
		{"github.com/firstlayer-xyz/facetlibs@a1b2c3d4e5f6789012345678901234567890abcd", "a1b2c3d4e5f6789012345678901234567890abcd"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			lp, err := ParseLibPath(tc.raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if lp.Ref != tc.wantRef {
				t.Errorf("Ref = %q, want %q", lp.Ref, tc.wantRef)
			}
			if lp.RepoID() != "github.com/firstlayer-xyz/facetlibs" {
				t.Errorf("RepoID = %q, want stable cross-ref value", lp.RepoID())
			}
		})
	}
}

// TestParseLibPathRefIdentityAcrossSameRepo pins the contract that
// different refs of the same repo share a stable RepoID — that's what
// the loader uses to share an on-disk clone — while keeping distinct
// Ref values so each pin resolves to its own commit.
func TestParseLibPathRefIdentityAcrossSameRepo(t *testing.T) {
	refs := []string{"main", "v1.0", "v2.3.4", "abc1234", "release/2.0"}
	var lps []*LibPath
	for _, r := range refs {
		lp, err := ParseLibPath("github.com/firstlayer-xyz/facetlibs@" + r)
		if err != nil {
			t.Fatalf("parse @%s: %v", r, err)
		}
		lps = append(lps, lp)
	}
	// All share the same RepoID (so the loader can de-dup the on-disk clone)
	for i := 1; i < len(lps); i++ {
		if lps[i].RepoID() != lps[0].RepoID() {
			t.Errorf("RepoID drift at @%s: got %q, want %q", refs[i], lps[i].RepoID(), lps[0].RepoID())
		}
	}
	// But each carries its own Ref distinct from all others.
	seen := map[string]bool{}
	for i, lp := range lps {
		if seen[lp.Ref] {
			t.Errorf("@%s: Ref %q collides with an earlier entry", refs[i], lp.Ref)
		}
		seen[lp.Ref] = true
	}
}

// TestParseLibPathRefMutabilityClassification verifies each ref shape
// flows into the right isImmutableRef bucket. Branches and semver tags
// are mutable (can move); only commit SHAs are immutable. Wrong
// classification would let stale caches serve the wrong code.
func TestParseLibPathRefMutabilityClassification(t *testing.T) {
	cases := []struct {
		raw           string
		wantImmutable bool
	}{
		{"github.com/x/lib@main", false},
		{"github.com/x/lib@v1.0", false},     // semver tags are mutable: a maintainer can re-point them
		{"github.com/x/lib@release/1.0", false},
		{"github.com/x/lib@abc1234", true},   // 7-char SHA prefix is immutable
		{"github.com/x/lib@a1b2c3d4e5f6789012345678901234567890abcd", true}, // full SHA
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			lp, err := ParseLibPath(tc.raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := isImmutableRef(lp.Ref)
			if got != tc.wantImmutable {
				t.Errorf("isImmutableRef(%q) = %v, want %v", lp.Ref, got, tc.wantImmutable)
			}
		})
	}
}

func TestIsImmutableRef(t *testing.T) {
	immutable := []string{
		"abc1234",
		"0123abcd",
		"f277ee7",
		"F277EE7A1BCD",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 40-char full SHA
	}
	mutable := []string{
		"",
		"main",
		"v1.0",
		"v1",          // too short for SHA
		"1234",        // too short
		"release/1.0", // contains slash
		"xyz1234",     // non-hex char
		"abcdefghi",   // non-hex letters
		"0123456789abcdef0123456789abcdef012345678", // 41 chars
	}
	for _, r := range immutable {
		if !isImmutableRef(r) {
			t.Errorf("expected %q to be immutable", r)
		}
	}
	for _, r := range mutable {
		if isImmutableRef(r) {
			t.Errorf("expected %q to be mutable", r)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
