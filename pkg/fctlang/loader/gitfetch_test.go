package loader

import (
	"testing"
)

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
