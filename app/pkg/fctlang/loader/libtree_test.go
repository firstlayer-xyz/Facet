package loader

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

// makeInMemoryRepo builds a fully in-memory bare repository with a single
// commit whose tree contains the provided files. Returns the repo and the
// commit SHA. Used to exercise virtual LibTree reads without touching disk
// or the network.
func makeInMemoryRepo(t *testing.T, files map[string]string) (*git.Repository, plumbing.Hash) {
	t.Helper()
	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	if err != nil {
		t.Fatalf("git.Init: %v", err)
	}

	// Stage blobs and build a tree object manually so we don't need a
	// working directory. go-git's high-level API requires a worktree for
	// commit; the low-level plumbing here creates the same objects.
	entries := []object.TreeEntry{}
	for p, content := range files {
		obj := store.NewEncodedObject()
		obj.SetType(plumbing.BlobObject)
		w, err := obj.Writer()
		if err != nil {
			t.Fatalf("blob writer: %v", err)
		}
		if _, err := io.Copy(w, strings.NewReader(content)); err != nil {
			t.Fatalf("write blob: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("close blob: %v", err)
		}
		h, err := store.SetEncodedObject(obj)
		if err != nil {
			t.Fatalf("store blob: %v", err)
		}
		entries = append(entries, object.TreeEntry{
			Name: p,
			Mode: 0o100644,
			Hash: h,
		})
	}

	// Group entries by directory by recursively building sub-trees.
	rootHash := writeTree(t, store, "", entries)

	// Build the commit pointing at the root tree.
	commit := &object.Commit{
		Author:    object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		Committer: object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		Message:   "test",
		TreeHash:  rootHash,
	}
	cobj := store.NewEncodedObject()
	if err := commit.Encode(cobj); err != nil {
		t.Fatalf("encode commit: %v", err)
	}
	sha, err := store.SetEncodedObject(cobj)
	if err != nil {
		t.Fatalf("store commit: %v", err)
	}
	return repo, sha
}

// writeTree takes a flat list of TreeEntry whose Name may be a nested path,
// groups by first segment, recurses for subdirectories, and writes the
// resulting tree object. Returns the tree's hash.
func writeTree(t *testing.T, store *memory.Storage, _ string, entries []object.TreeEntry) plumbing.Hash {
	t.Helper()
	type group struct {
		subEntries []object.TreeEntry
		isDir      bool
	}
	groups := map[string]*group{}
	for _, e := range entries {
		head, rest, hasSlash := strings.Cut(e.Name, "/")
		g, ok := groups[head]
		if !ok {
			g = &group{}
			groups[head] = g
		}
		if hasSlash {
			g.isDir = true
			g.subEntries = append(g.subEntries, object.TreeEntry{
				Name: rest,
				Mode: e.Mode,
				Hash: e.Hash,
			})
		} else {
			g.subEntries = append(g.subEntries, e) // file at this level
		}
	}

	var out []object.TreeEntry
	for name, g := range groups {
		if g.isDir {
			h := writeTree(t, store, name, g.subEntries)
			out = append(out, object.TreeEntry{Name: name, Mode: 0o040000, Hash: h})
		} else {
			// Leaf: exactly one entry for this name (the file itself).
			e := g.subEntries[0]
			e.Name = name
			out = append(out, e)
		}
	}
	// Git tree ordering: directories sort as if they had a trailing "/".
	// That makes "foo.fct" sort before "foo" (the dir), because "." < "/".
	sortKey := func(e object.TreeEntry) string {
		if e.Mode == 0o040000 {
			return e.Name + "/"
		}
		return e.Name
	}
	sort.Slice(out, func(i, j int) bool { return sortKey(out[i]) < sortKey(out[j]) })
	tree := &object.Tree{Entries: out}
	obj := store.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		t.Fatalf("encode tree: %v", err)
	}
	h, err := store.SetEncodedObject(obj)
	if err != nil {
		t.Fatalf("store tree: %v", err)
	}
	return h
}

// TestVirtualLibTreeReadFile exercises reading .fct source content directly
// from a bare-clone object store — the load path used for cached remote libs.
func TestVirtualLibTreeReadFile(t *testing.T) {
	repo, sha := makeInMemoryRepo(t, map[string]string{
		"fasteners.fct":                "var V = 1\n",
		"fasteners/knurling/knurling.fct": "var K = 2\n",
	})
	tree := &LibTree{repo: repo, sha: sha, origin: "github.com/t/r"}

	got, err := tree.ReadFile("fasteners.fct")
	if err != nil {
		t.Fatalf("ReadFile root: %v", err)
	}
	if string(got) != "var V = 1\n" {
		t.Errorf("ReadFile root content: %q", string(got))
	}

	got, err = tree.ReadFile("fasteners/knurling/knurling.fct")
	if err != nil {
		t.Fatalf("ReadFile nested: %v", err)
	}
	if string(got) != "var K = 2\n" {
		t.Errorf("ReadFile nested content: %q", string(got))
	}

	if _, err := tree.ReadFile("does/not/exist.fct"); err == nil {
		t.Error("ReadFile missing path should error")
	}
}

// TestVirtualLibTreeWalk verifies Walk iterates every regular file in the
// tree with slash-separated tree-relative paths.
func TestVirtualLibTreeWalk(t *testing.T) {
	repo, sha := makeInMemoryRepo(t, map[string]string{
		"a.fct":     "A",
		"sub/b.fct": "B",
	})
	tree := &LibTree{repo: repo, sha: sha, origin: "github.com/t/r"}

	seen := map[string]string{}
	err := tree.Walk(func(subPath string, r io.Reader) error {
		buf, _ := io.ReadAll(r)
		seen[subPath] = string(buf)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if seen["a.fct"] != "A" {
		t.Errorf("walk a.fct: got %q", seen["a.fct"])
	}
	if seen["sub/b.fct"] != "B" {
		t.Errorf("walk sub/b.fct: got %q", seen["sub/b.fct"])
	}
	if len(seen) != 2 {
		t.Errorf("expected 2 entries, got %d: %v", len(seen), seen)
	}
}

// TestVirtualLibTreeExtractTo confirms the fork path writes the tree to
// disk and skips hidden entries.
func TestVirtualLibTreeExtractTo(t *testing.T) {
	repo, sha := makeInMemoryRepo(t, map[string]string{
		"visible.fct":     "keep\n",
		".hidden":         "skip\n",
		"sub/.dotdir/x":   "skip nested\n",
		"sub/kept.fct":    "nested keep\n",
	})
	tree := &LibTree{repo: repo, sha: sha, origin: "github.com/t/r"}

	dest := filepath.Join(t.TempDir(), "fork")
	if err := tree.ExtractTo(dest); err != nil {
		t.Fatalf("ExtractTo: %v", err)
	}

	// Present
	if b, err := os.ReadFile(filepath.Join(dest, "visible.fct")); err != nil || !bytes.Equal(b, []byte("keep\n")) {
		t.Errorf("visible.fct missing or wrong: err=%v, content=%q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "sub", "kept.fct")); err != nil || !bytes.Equal(b, []byte("nested keep\n")) {
		t.Errorf("sub/kept.fct missing or wrong: err=%v, content=%q", err, b)
	}
	// Hidden — must be absent
	if _, err := os.Stat(filepath.Join(dest, ".hidden")); err == nil {
		t.Error(".hidden should have been skipped")
	}
	if _, err := os.Stat(filepath.Join(dest, "sub", ".dotdir")); err == nil {
		t.Error(".dotdir should have been skipped")
	}
}

// TestSourceKeyVirtual confirms the URI shape for virtual-tree keys and
// that IsVirtualSourceKey recognises them.
func TestSourceKeyVirtual(t *testing.T) {
	tree := &LibTree{
		repo:   nil, // unused for SourceKey
		sha:    plumbing.NewHash("abcdef1234567890abcdef1234567890abcdef12"),
		origin: "github.com/x/y",
	}
	// Force IsVirtual by giving it a non-nil repo pointer. SourceKey only
	// consults repo!=nil; tree.sha is already set above.
	tree.repo = &git.Repository{}

	got := tree.SourceKey("sub/foo.fct")
	want := "git+github.com/x/y@abcdef1234567890abcdef1234567890abcdef12/sub/foo.fct"
	if got != want {
		t.Errorf("SourceKey got %q, want %q", got, want)
	}
	if !IsVirtualSourceKey(got) {
		t.Error("IsVirtualSourceKey returned false for virtual URI")
	}
	if IsVirtualSourceKey("/abs/path/foo.fct") {
		t.Error("IsVirtualSourceKey true for abs path")
	}
}

// TestPhysicalTreeSourceKey confirms physical-tree SourceKey is a plain
// filesystem path.
func TestPhysicalTreeSourceKey(t *testing.T) {
	dir := t.TempDir()
	tree := PhysicalTree(dir)
	got := tree.SourceKey("sub/foo.fct")
	want := filepath.Join(dir, "sub", "foo.fct")
	if got != want {
		t.Errorf("SourceKey got %q, want %q", got, want)
	}
	if IsVirtualSourceKey(got) {
		t.Error("IsVirtualSourceKey true for physical key")
	}
}
