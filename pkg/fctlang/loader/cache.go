package loader

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// Cache bundles the storage primitives the loader uses to maintain a
// per-origin cache of bare git clones. Three pieces:
//
//   FS          — the loader's own bookkeeping (READMEs, the mkdir/mktemp/
//                 rename dance that protects in-flight clones from being seen
//                 mid-write). See fs.go.
//   StorerFor   — go-git object store factory, keyed by path. Native: an
//                 osfs-backed storage. Wasm: in-memory go-git storage.
//   WorktreeFor — billy.Filesystem factory for the working tree. Returns nil
//                 for bare clones (which is what the lib cache always uses).
//
// Path arguments to the factories are opaque keys — whatever ensureSharedRepo
// computed for a given (host, user, repo). The native impl interprets them as
// real disk paths; the wasm impl uses them as map keys.
type Cache struct {
	FS          FS
	StorerFor   func(path string) storage.Storer
	WorktreeFor func(path string) billy.Filesystem
}

// NativeCache returns a Cache backed by the local filesystem.
func NativeCache() *Cache {
	return &Cache{
		FS: diskFS{},
		StorerFor: func(p string) storage.Storer {
			return filesystem.NewStorage(osfs.New(p), cache.NewObjectLRUDefault())
		},
		WorktreeFor: func(p string) billy.Filesystem {
			// The lib cache always uses bare clones — no worktree needed.
			return nil
		},
	}
}

// MemoryCache returns a Cache backed entirely by in-process state — a
// path-keyed map for the loader's bookkeeping plus go-git's memory.Storage
// for the bare-clone object stores. Each call returns a fresh, independent
// cache.
//
// Used by the wasm build (no disk to persist between sessions) and by tests
// that want to exercise the loader without touching disk.
func MemoryCache() *Cache {
	m := newMemFS()
	return &Cache{
		FS:        m,
		StorerFor: m.storerFor,
		WorktreeFor: func(_ string) billy.Filesystem {
			return nil
		},
	}
}
