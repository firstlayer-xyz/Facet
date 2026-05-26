package loader

import (
	"errors"
	"io/fs"
	"testing"
)

func TestMemoryCacheFS_WriteRead(t *testing.T) {
	c := MemoryCache()
	if err := c.FS.WriteFile("/a/b/file.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := c.FS.ReadFile("/a/b/file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	// Parent dir auto-created.
	info, err := c.FS.Stat("/a/b")
	if err != nil {
		t.Fatalf("Stat /a/b: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("/a/b is not a dir")
	}
}

func TestMemoryCacheFS_StatMissing(t *testing.T) {
	c := MemoryCache()
	_, err := c.FS.Stat("/nope")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat missing: want fs.ErrNotExist, got %v", err)
	}
}

func TestMemoryCacheFS_MkdirTempFreshAndUnique(t *testing.T) {
	c := MemoryCache()
	if err := c.FS.MkdirAll("/cache", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	a, err := c.FS.MkdirTemp("/cache", ".clone-*")
	if err != nil {
		t.Fatalf("MkdirTemp a: %v", err)
	}
	b, err := c.FS.MkdirTemp("/cache", ".clone-*")
	if err != nil {
		t.Fatalf("MkdirTemp b: %v", err)
	}
	if a == b {
		t.Errorf("MkdirTemp returned same path twice: %q", a)
	}
	for _, p := range []string{a, b} {
		info, err := c.FS.Stat(p)
		if err != nil {
			t.Errorf("Stat %s: %v", p, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s should be a dir", p)
		}
	}
}

// Rename must move both the FS entry tree AND the storer keyed at the old
// path, so a subsequent StorerFor at the new path returns the same storage
// that was created under the old path. This is the property that makes the
// mkdir+temp+rename pattern actually work for the in-memory cache.
func TestMemoryCacheFS_RenameRekeysStorer(t *testing.T) {
	c := MemoryCache()
	if err := c.FS.MkdirAll("/cache/parent", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	tmp, err := c.FS.MkdirTemp("/cache/parent", ".clone-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	storerBefore := c.StorerFor(tmp)
	if storerBefore == nil {
		t.Fatal("StorerFor returned nil")
	}
	final := "/cache/parent/.repo"
	if err := c.FS.Rename(tmp, final); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	storerAfter := c.StorerFor(final)
	if storerAfter != storerBefore {
		t.Errorf("storer was not re-keyed by Rename — got fresh storage at new path")
	}
	// Old path is gone.
	if _, err := c.FS.Stat(tmp); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("tmp dir still present after rename: stat err=%v", err)
	}
	if _, err := c.FS.Stat(final); err != nil {
		t.Errorf("final dir not present after rename: %v", err)
	}
}

func TestMemoryCacheFS_RenameNested(t *testing.T) {
	c := MemoryCache()
	if err := c.FS.WriteFile("/src/a/file.txt", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := c.FS.Rename("/src", "/dst"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	got, err := c.FS.ReadFile("/dst/a/file.txt")
	if err != nil {
		t.Fatalf("ReadFile after rename: %v", err)
	}
	if string(got) != "hi" {
		t.Errorf("got %q, want %q", got, "hi")
	}
}

func TestMemoryCacheFS_RemoveAllNested(t *testing.T) {
	c := MemoryCache()
	if err := c.FS.WriteFile("/x/y/file.txt", []byte("z"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := c.FS.RemoveAll("/x"); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if _, err := c.FS.Stat("/x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("/x still present")
	}
	if _, err := c.FS.Stat("/x/y/file.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("nested file still present")
	}
}
