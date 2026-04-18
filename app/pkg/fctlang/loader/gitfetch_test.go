package loader

import (
	"context"
	"os"
	"path/filepath"
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

func Test_resolveLibPathLocalNoRef(t *testing.T) {
	libDir := t.TempDir()
	dir, err := resolveLibPath(context.Background(), libDir, "", nil, "facet/gears")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(libDir, "facet/gears")
	if dir != want {
		t.Errorf("expected %q, got %q", want, dir)
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

func Test_resolveLibPathLocal(t *testing.T) {
	libDir := t.TempDir()
	dir, err := resolveLibPath(context.Background(), libDir, "", nil, "facet/gears@v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(libDir, "facet/gears")
	if dir != want {
		t.Errorf("expected %q, got %q", want, dir)
	}
}

func Test_resolveLibPathLocalValidation(t *testing.T) {
	// Ensure path traversal is rejected
	_, err := resolveLibPath(context.Background(), "/tmp", "", nil, "foo/../bar@v1")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func Test_resolveLibPathInstalledOverride(t *testing.T) {
	installed := map[string]string{
		"github.com/user/repo": "/custom/path",
	}
	dir, err := resolveLibPath(context.Background(), "/lib", "/cache", installed, "github.com/user/repo@v1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/custom/path" {
		t.Errorf("expected /custom/path, got %q", dir)
	}
}

func Test_resolveLibPathInstalledOverrideSubpath(t *testing.T) {
	installed := map[string]string{
		"gitlab.com/user/monorepo": "/custom/mono",
	}
	dir, err := resolveLibPath(context.Background(), "/lib", "/cache", installed, "gitlab.com/user/monorepo/gears@main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/custom/mono/gears" {
		t.Errorf("expected /custom/mono/gears, got %q", dir)
	}
}

func Test_resolveLibPathCached(t *testing.T) {
	cacheDir := t.TempDir()
	// Pre-create a cached directory
	cachedRepo := filepath.Join(cacheDir, "github.com", "user", "repo", "v1.0")
	if err := os.MkdirAll(cachedRepo, 0755); err != nil {
		t.Fatal(err)
	}
	// Write a dummy .fct file so it looks like a valid library
	if err := os.WriteFile(filepath.Join(cachedRepo, "v1.0.fct"), []byte("Dummy() { return Cube({1 mm, 1 mm, 1 mm}); }"), 0644); err != nil {
		t.Fatal(err)
	}

	dir, err := resolveLibPath(context.Background(), "/lib", cacheDir, nil, "github.com/user/repo@v1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != cachedRepo {
		t.Errorf("expected %q, got %q", cachedRepo, dir)
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
