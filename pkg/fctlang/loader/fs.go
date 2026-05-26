package loader

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
)

// FS is the filesystem the loader uses to manage its on-disk bookkeeping —
// per-repo directory tree, README breadcrumbs, the mkdir/mktemp/rename dance
// that protects in-flight clones from being seen by other readers.
//
// The native implementation wraps os.*. The wasm build supplies an in-memory
// implementation so the same code path doesn't fail with "not implemented on
// js" the moment it tries to mkdir.
//
// Paths are absolute and as the underlying implementation requires (slash on
// wasm/Linux/macOS, mixed on Windows); FS treats them as opaque keys.
type FS interface {
	// Stat returns FileInfo for the entry at path. Missing entries return an
	// error satisfying os.IsNotExist.
	Stat(path string) (fs.FileInfo, error)

	// MkdirAll creates path and all missing parents. Idempotent.
	MkdirAll(path string, perm fs.FileMode) error

	// MkdirTemp creates a new temporary directory under parent whose name
	// matches pattern (with the single '*' in pattern replaced by a random
	// suffix). Returns the absolute path of the new directory.
	MkdirTemp(parent, pattern string) (string, error)

	// ReadFile returns the contents of the file at path.
	ReadFile(path string) ([]byte, error)

	// WriteFile atomically writes data to path, creating parent directories
	// as needed. Implementations are responsible for atomicity (e.g. tempfile
	// + rename on disk).
	WriteFile(path string, data []byte, perm fs.FileMode) error

	// Rename moves a file or directory from old to new.
	Rename(old, new string) error

	// RemoveAll removes path and any children. Missing entries are not an error.
	RemoveAll(path string) error
}

// ── diskFS — native os.* implementation ───────────────────────────────────────

type diskFS struct{}

func (diskFS) Stat(p string) (fs.FileInfo, error)        { return os.Stat(p) }
func (diskFS) MkdirAll(p string, perm fs.FileMode) error { return os.MkdirAll(p, perm) }
func (diskFS) MkdirTemp(parent, pattern string) (string, error) {
	return os.MkdirTemp(parent, pattern)
}
func (diskFS) ReadFile(p string) ([]byte, error) { return os.ReadFile(p) }
func (diskFS) Rename(old, new string) error      { return os.Rename(old, new) }
func (diskFS) RemoveAll(p string) error          { return os.RemoveAll(p) }

// WriteFile is the atomic-replace pattern: write to a sibling tempfile, set
// permissions, then rename over the destination. Made part of the FS contract
// so the in-memory impl satisfies it trivially (no observers).
func (diskFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, p); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// ── memFS — path-keyed in-memory implementation ───────────────────────────────
//
// Used by the wasm build (no disk to persist between sessions) and by tests
// that want to exercise the loader without touching disk.
//
// memFS shares state with the per-path go-git memory.Storage map (used by
// MemoryCache.StorerFor). Rename moves both layers in lockstep so a
// subsequent open at the new path finds the storage that was created under
// the old path. Without that re-keying the post-rename open would see an
// empty storage and report ErrRepositoryNotExists.

type memEntry struct {
	isDir   bool
	data    []byte
	mode    fs.FileMode
	modTime time.Time
}

// memFS holds the path-keyed map plus the path → memory.Storage map used by
// the storer factory.
type memFS struct {
	mu      sync.RWMutex
	entries map[string]*memEntry
	storers sync.Map // path → *memory.Storage
}

func newMemFS() *memFS {
	return &memFS{entries: make(map[string]*memEntry)}
}

func (f *memFS) storerFor(p string) storage.Storer {
	if v, ok := f.storers.Load(p); ok {
		return v.(*memory.Storage)
	}
	s := memory.NewStorage()
	actual, _ := f.storers.LoadOrStore(p, s)
	return actual.(*memory.Storage)
}

func (f *memFS) Stat(p string) (fs.FileInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.entries[p]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}
	return memFileInfo{name: path.Base(p), entry: e}, nil
}

func (f *memFS) MkdirAll(p string, perm fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ensureDirsLocked(p, perm)
}

func (f *memFS) MkdirTemp(parent, pattern string) (string, error) {
	for i := 0; i < 100; i++ {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		suffix := hex.EncodeToString(b[:])
		var name string
		if strings.Contains(pattern, "*") {
			name = strings.Replace(pattern, "*", suffix, 1)
		} else {
			name = pattern + suffix
		}
		full := path.Join(parent, name)
		f.mu.Lock()
		if _, taken := f.entries[full]; taken {
			f.mu.Unlock()
			continue
		}
		if err := f.ensureDirsLocked(parent, 0o755); err != nil {
			f.mu.Unlock()
			return "", err
		}
		f.entries[full] = &memEntry{isDir: true, mode: 0o700 | fs.ModeDir, modTime: time.Now()}
		f.mu.Unlock()
		return full, nil
	}
	return "", errors.New("MkdirTemp: too many collisions")
}

func (f *memFS) ReadFile(p string) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.entries[p]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	if e.isDir {
		return nil, &fs.PathError{Op: "open", Path: p, Err: errors.New("is a directory")}
	}
	out := make([]byte, len(e.data))
	copy(out, e.data)
	return out, nil
}

func (f *memFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if dir := path.Dir(p); dir != "" && dir != "." && dir != "/" {
		if err := f.ensureDirsLocked(dir, 0o755); err != nil {
			return err
		}
	}
	f.entries[p] = &memEntry{
		data:    append([]byte(nil), data...),
		mode:    perm,
		modTime: time.Now(),
	}
	return nil
}

func (f *memFS) Rename(oldPath, newPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.entries[oldPath]; !ok {
		return &fs.PathError{Op: "rename", Path: oldPath, Err: fs.ErrNotExist}
	}
	if _, ok := f.entries[newPath]; ok {
		return &fs.PathError{Op: "rename", Path: newPath, Err: fs.ErrExist}
	}
	oldPrefix := oldPath + "/"
	for k, v := range f.entries {
		if k == oldPath {
			f.entries[newPath] = v
			delete(f.entries, k)
		} else if strings.HasPrefix(k, oldPrefix) {
			f.entries[newPath+"/"+k[len(oldPrefix):]] = v
			delete(f.entries, k)
		}
	}
	// Re-key any cached storer at oldPath or beneath → newPath.
	type move struct {
		k string
		v any
	}
	var moves []move
	f.storers.Range(func(k, v any) bool {
		ks := k.(string)
		switch {
		case ks == oldPath:
			moves = append(moves, move{newPath, v})
			f.storers.Delete(k)
		case strings.HasPrefix(ks, oldPrefix):
			moves = append(moves, move{newPath + "/" + ks[len(oldPrefix):], v})
			f.storers.Delete(k)
		}
		return true
	})
	for _, m := range moves {
		f.storers.Store(m.k, m.v)
	}
	return nil
}

func (f *memFS) RemoveAll(p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := p + "/"
	for k := range f.entries {
		if k == p || strings.HasPrefix(k, prefix) {
			delete(f.entries, k)
		}
	}
	f.storers.Range(func(k, _ any) bool {
		ks := k.(string)
		if ks == p || strings.HasPrefix(ks, prefix) {
			f.storers.Delete(k)
		}
		return true
	})
	return nil
}

// ensureDirsLocked walks every component of p, creating dir entries as
// needed. Caller must hold f.mu.Lock().
func (f *memFS) ensureDirsLocked(p string, perm fs.FileMode) error {
	cleaned := path.Clean(p)
	if cleaned == "" || cleaned == "." || cleaned == "/" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	cur := ""
	if strings.HasPrefix(cleaned, "/") {
		cur = "/"
	}
	now := time.Now()
	for _, part := range parts {
		switch cur {
		case "":
			cur = part
		case "/":
			cur += part
		default:
			cur += "/" + part
		}
		if e, ok := f.entries[cur]; ok {
			if !e.isDir {
				return &fs.PathError{Op: "mkdir", Path: cur, Err: errors.New("not a directory")}
			}
			continue
		}
		f.entries[cur] = &memEntry{isDir: true, mode: perm | fs.ModeDir, modTime: now}
	}
	return nil
}

type memFileInfo struct {
	name  string
	entry *memEntry
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return int64(len(i.entry.data)) }
func (i memFileInfo) Mode() fs.FileMode  { return i.entry.mode }
func (i memFileInfo) ModTime() time.Time { return i.entry.modTime }
func (i memFileInfo) IsDir() bool        { return i.entry.isDir }
func (i memFileInfo) Sys() any           { return nil }
