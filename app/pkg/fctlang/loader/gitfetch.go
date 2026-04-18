package loader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// LibPath holds the parsed components of a library import path.
//
// Exactly one of IsLocal, IsRelative, or the remote form (Host/User/Repo set)
// applies. Relative paths (./sibling, ../sibling) resolve against the
// importing source's directory; the resolver — not ParseLibPath — turns them
// into concrete disk paths.
type LibPath struct {
	Host       string // e.g. "github.com"
	User       string // e.g. "firstlayer-xyz"
	Repo       string // e.g. "facet-fasteners"
	SubPath    string // e.g. "gears" (empty for root)
	Ref        string // e.g. "v1.0" or "main"
	Raw        string // original path before parsing
	IsLocal    bool   // true for built-in paths like "facet/gears"
	IsRelative bool   // true for "./x" or "../x" — resolves against importer's dir
}

// CloneURL returns the HTTPS clone URL for a remote library.
func (lp *LibPath) CloneURL() string {
	return fmt.Sprintf("https://%s/%s/%s.git", lp.Host, lp.User, lp.Repo)
}

// RepoID returns the stable identifier for the repo (no ref, no subpath).
func (lp *LibPath) RepoID() string {
	return fmt.Sprintf("%s/%s/%s", lp.Host, lp.User, lp.Repo)
}

// ParseLibPath parses a raw library path into its components. Remote paths
// have a hostname (first segment contains '.') and require an @ref pinning
// them to a specific branch, tag, or commit hash — this is the mechanism for
// reproducible builds. Local/built-in paths are plain filesystem-relative
// paths and may optionally carry an @ref (advisory — built-ins ship with the
// binary at a single version).
func ParseLibPath(raw string) (*LibPath, error) {
	lp := &LibPath{Raw: raw}

	// Relative imports ("./x", "../x", "../../x/y") resolve against the
	// importing source's directory. They pin to whatever the importer's
	// parent is pinned to — no @ref, no host/user/repo. The concrete disk
	// path is computed by the resolver; ParseLibPath only flags the shape
	// and stores the normalised path-to-join-with-parent-dir in SubPath.
	//
	// Import paths are always slash-separated (URL-style), so we use the
	// "path" package — not "path/filepath" — to clean them: each ".."
	// collapses one segment, "./foo/../bar" becomes "bar", etc. The disk
	// resolution done later by the resolver uses filepath and produces an
	// OS-correct absolute path.
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		if strings.Contains(raw, "@") {
			return nil, fmt.Errorf("library path %q: relative imports cannot carry @ref — they inherit the importer's pin", lp.Raw)
		}
		subPath := path.Clean(raw)
		// path.Clean("./") = "." and path.Clean("../") = ".." — neither
		// points at a library. path.Clean also collapses ".." so a "./..",
		// "../..", etc. come out as "." or "..".
		if subPath == "." || subPath == ".." {
			return nil, fmt.Errorf("library path %q: relative imports need a name after ./ or ../", lp.Raw)
		}
		lp.IsRelative = true
		lp.SubPath = subPath
		return lp, nil
	}

	// Split off @ref if present
	if idx := strings.LastIndex(raw, "@"); idx >= 0 {
		lp.Ref = raw[idx+1:]
		raw = raw[:idx]
		if lp.Ref == "" {
			return nil, fmt.Errorf("library path %q: empty ref after @", lp.Raw)
		}
	}

	segments := strings.Split(raw, "/")
	if len(segments) < 2 {
		return nil, fmt.Errorf("library path %q: must have at least 2 segments", lp.Raw)
	}

	// Remote detection: first segment contains '.'
	if strings.Contains(segments[0], ".") {
		if lp.Ref == "" {
			return nil, fmt.Errorf("library path %q: remote imports require @ref — a branch, tag, or commit hash (e.g. @main, @v1.0, or @abc1234)", lp.Raw)
		}
		if len(segments) < 3 {
			return nil, fmt.Errorf("library path %q: remote paths need at least host/user/repo", lp.Raw)
		}
		lp.Host = segments[0]
		lp.User = segments[1]
		lp.Repo = segments[2]
		if len(segments) > 3 {
			lp.SubPath = strings.Join(segments[3:], "/")
		}
		return lp, nil
	}

	// Local path — @ref optional
	lp.IsLocal = true
	return lp, nil
}

// DefaultGitCacheDir returns the OS-specific git cache directory.
func DefaultGitCacheDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "Facet", "libcache")
}

// ---------------------------------------------------------------------------
// Cache layout
// ---------------------------------------------------------------------------
//
// One shared bare clone per repo — nothing else is materialized. Library files
// are read directly from the bare clone's object store by (commit-SHA,
// tree-path) via go-git.
//
//   <cacheDir>/<host>/<user>/<repo>/
//       .repo/   shared bare clone (object store, all refs)

const (
	sharedRepoName = ".repo"

	// LibSourceScheme prefixes the canonical source key for a file read
	// out of a virtualized (bare-clone) library. Example:
	//   git+github.com/foo/bar@<sha>/subpath/file.fct
	// The prefix makes virtual keys unambiguously distinguishable from
	// absolute filesystem paths at a glance.
	LibSourceScheme = "git+"
)

// repoLocks serializes fetch operations within one process. Inter-process
// races fall back to the atomic rename in ensureSharedRepo.
var repoLocks sync.Map // sharedDir → *sync.Mutex

func repoLock(sharedDir string) *sync.Mutex {
	v, _ := repoLocks.LoadOrStore(sharedDir, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// LibTree is a read-only view of a library's source files, backed by either a
// go-git bare clone (virtual) or a real on-disk directory (override/local).
// The resolver treats both uniformly through ReadFile / Walk / SourceKey.
type LibTree struct {
	// Virtual backing — set when sourced from a bare clone.
	repo   *git.Repository
	sha    plumbing.Hash
	origin string // "host/user/repo" — used for building source keys

	// Physical backing — set when sourced from a real directory.
	diskDir string
}

// IsVirtual reports whether this tree reads from a go-git object store (true)
// or from the filesystem (false).
func (t *LibTree) IsVirtual() bool { return t.repo != nil }

// DiskDir returns the on-disk directory for physical trees, or "" for virtual.
func (t *LibTree) DiskDir() string { return t.diskDir }

// Origin returns the "host/user/repo" identifier for virtual trees, or "" for
// physical.
func (t *LibTree) Origin() string { return t.origin }

// SHA returns the resolved commit SHA for virtual trees, or the zero hash for
// physical.
func (t *LibTree) SHA() plumbing.Hash { return t.sha }

// SourceKey returns the canonical identifier for the file at subPath. For
// virtual trees this is a URI (git+host/user/repo@<sha>/subpath); for
// physical trees it is an absolute filesystem path.
func (t *LibTree) SourceKey(subPath string) string {
	if t.IsVirtual() {
		if subPath == "" {
			return fmt.Sprintf("%s%s@%s", LibSourceScheme, t.origin, t.sha.String())
		}
		return fmt.Sprintf("%s%s@%s/%s", LibSourceScheme, t.origin, t.sha.String(), subPath)
	}
	if subPath == "" {
		return t.diskDir
	}
	return filepath.Join(t.diskDir, filepath.FromSlash(subPath))
}

// IsVirtualSourceKey reports whether key is a virtualized library URI (as
// produced by LibTree.SourceKey on a virtual tree). Non-virtual keys are real
// filesystem paths or well-known pseudo-paths like StdlibPath.
func IsVirtualSourceKey(key string) bool {
	return strings.HasPrefix(key, LibSourceScheme)
}

// ReadFile returns the contents of the file at subPath within the tree.
// subPath is forward-slash-separated and relative to the tree root.
func (t *LibTree) ReadFile(subPath string) ([]byte, error) {
	if subPath == "" {
		return nil, fmt.Errorf("ReadFile: empty subPath")
	}
	if t.IsVirtual() {
		tree, err := t.gitTree()
		if err != nil {
			return nil, err
		}
		f, err := tree.File(subPath)
		if err != nil {
			return nil, fmt.Errorf("git tree %s: %s: %w", t.sha.String()[:8], subPath, err)
		}
		// Contents() slurps the blob — fine for .fct source files; they're
		// small and we already pay the full cost in the parser.
		s, err := f.Contents()
		if err != nil {
			return nil, fmt.Errorf("read blob %s: %w", subPath, err)
		}
		return []byte(s), nil
	}
	return os.ReadFile(filepath.Join(t.diskDir, filepath.FromSlash(subPath)))
}

// HasFile reports whether subPath exists in the tree.
func (t *LibTree) HasFile(subPath string) bool {
	if t.IsVirtual() {
		tree, err := t.gitTree()
		if err != nil {
			return false
		}
		_, err = tree.File(subPath)
		return err == nil
	}
	_, err := os.Stat(filepath.Join(t.diskDir, filepath.FromSlash(subPath)))
	return err == nil
}

// Walk invokes visit for every regular file in the tree. Paths are slash-
// separated and tree-relative.
func (t *LibTree) Walk(visit func(subPath string, r io.Reader) error) error {
	if t.IsVirtual() {
		tree, err := t.gitTree()
		if err != nil {
			return err
		}
		return tree.Files().ForEach(func(f *object.File) error {
			if f.Mode == filemode.Submodule {
				return nil
			}
			r, err := f.Reader()
			if err != nil {
				return err
			}
			defer r.Close()
			return visit(f.Name, r)
		})
	}
	return filepath.Walk(t.diskDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(t.diskDir, p)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		return visit(filepath.ToSlash(rel), f)
	})
}

// gitTree returns the object.Tree for this virtual tree's commit.
func (t *LibTree) gitTree() (*object.Tree, error) {
	commit, err := t.repo.CommitObject(t.sha)
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", t.sha, err)
	}
	return commit.Tree()
}

// ExtractTo writes every file in the tree to destDir atomically, skipping
// hidden entries (dotfiles, .git, etc.). Used by ForkLibrary to materialize
// a local working copy of a virtualized library.
func (t *LibTree) ExtractTo(destDir string) error {
	parent := filepath.Dir(destDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(parent, ".fork-*")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(tmpDir)
		}
	}()

	err = t.Walk(func(subPath string, r io.Reader) error {
		// Skip hidden files/dirs anywhere along the path — forks carry
		// only the source a user would edit, not git bookkeeping.
		for _, seg := range strings.Split(subPath, "/") {
			if strings.HasPrefix(seg, ".") {
				return nil
			}
		}
		dst := filepath.Join(tmpDir, filepath.FromSlash(subPath))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		w, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
			w.Close()
			return err
		}
		return w.Close()
	})
	if err != nil {
		return err
	}

	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("remove existing fork dest: %w", err)
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return fmt.Errorf("rename fork dest: %w", err)
	}
	cleanup = false
	return nil
}

// EnsureLib guarantees the bare clone exists, resolves lp.Ref to a SHA, and
// returns a virtual tree handle. The bare clone is the only on-disk footprint.
func EnsureLib(ctx context.Context, gitCacheDir string, lp *LibPath) (*LibTree, error) {
	return ensureLib(ctx, gitCacheDir, lp, false /*forceFetch*/)
}

// RefreshLib is EnsureLib but forces a fetch from origin even for immutable
// refs. Use for explicit user-triggered updates.
func RefreshLib(ctx context.Context, gitCacheDir string, lp *LibPath) error {
	_, err := ensureLib(ctx, gitCacheDir, lp, true /*forceFetch*/)
	return err
}

// EnsureRepoClone guarantees the bare clone for the repo lp points at exists
// on disk, without resolving any ref. Used by the settings UI's "Clone"
// action where the user wants the repo cached but hasn't picked a ref.
func EnsureRepoClone(ctx context.Context, gitCacheDir string, lp *LibPath) error {
	if lp.IsLocal || lp.Host == "" {
		return fmt.Errorf("EnsureRepoClone: not a remote repo: %q", lp.Raw)
	}
	if gitCacheDir == "" {
		gitCacheDir = DefaultGitCacheDir()
	}
	repoDir := filepath.Join(gitCacheDir, lp.Host, lp.User, lp.Repo)
	sharedDir := filepath.Join(repoDir, sharedRepoName)

	lock := repoLock(sharedDir)
	lock.Lock()
	defer lock.Unlock()

	writeCacheReadme(gitCacheDir)
	writeRepoReadme(repoDir, lp)

	_, err := ensureSharedRepo(ctx, sharedDir, lp.CloneURL())
	if err != nil {
		return fmt.Errorf("shared clone %s: %w", lp.CloneURL(), err)
	}
	return nil
}

// RefreshRepoClone is EnsureRepoClone plus a git fetch to pull every new ref
// from origin. Used by "Update" and "Pull All" in the settings UI.
func RefreshRepoClone(ctx context.Context, gitCacheDir string, lp *LibPath) error {
	if err := EnsureRepoClone(ctx, gitCacheDir, lp); err != nil {
		return err
	}
	sharedDir := filepath.Join(gitCacheDir, lp.Host, lp.User, lp.Repo, sharedRepoName)
	repo, err := git.PlainOpen(sharedDir)
	if err != nil {
		return fmt.Errorf("open bare clone: %w", err)
	}
	return fetchAll(ctx, repo)
}

// OpenRepoHeadTree opens the bare clone for the repo lp points at and returns
// a virtual tree pinned to the remote's default branch (origin/HEAD). Used by
// "Fork" in the settings UI to materialize the latest revision without the
// caller needing to know which branch is the default.
func OpenRepoHeadTree(ctx context.Context, gitCacheDir string, lp *LibPath) (*LibTree, error) {
	if err := EnsureRepoClone(ctx, gitCacheDir, lp); err != nil {
		return nil, err
	}
	sharedDir := filepath.Join(gitCacheDir, lp.Host, lp.User, lp.Repo, sharedRepoName)
	repo, err := git.PlainOpen(sharedDir)
	if err != nil {
		return nil, fmt.Errorf("open bare clone: %w", err)
	}
	// Refresh so HEAD resolves against what's actually upstream — otherwise
	// we'd happily fork a stale default branch.
	if err := fetchAll(ctx, repo); err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	sha, err := resolveRepoHead(repo)
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	return &LibTree{repo: repo, sha: sha, origin: lp.RepoID()}, nil
}

// resolveRepoHead returns the commit SHA of the repo's HEAD. A bare clone
// tracks the remote's default branch via HEAD, so this is what "latest" means
// for a Fork operation.
func resolveRepoHead(repo *git.Repository) (plumbing.Hash, error) {
	ref, err := repo.Head()
	if err == nil {
		return ref.Hash(), nil
	}
	// Bare clones created by go-git sometimes expose the default branch only
	// under refs/remotes/origin/HEAD — try the DWIM candidates before giving up.
	h, err2 := resolveRef(repo, "HEAD")
	if err2 == nil {
		return *h, nil
	}
	return plumbing.ZeroHash, err
}

func ensureLib(ctx context.Context, gitCacheDir string, lp *LibPath, forceFetch bool) (*LibTree, error) {
	if lp.IsLocal || lp.Host == "" {
		return nil, fmt.Errorf("ensureLib: not a remote lib: %q", lp.Raw)
	}
	if lp.Ref == "" {
		return nil, fmt.Errorf("ensureLib: ref required: %q", lp.Raw)
	}
	if gitCacheDir == "" {
		gitCacheDir = DefaultGitCacheDir()
	}

	repoDir := filepath.Join(gitCacheDir, lp.Host, lp.User, lp.Repo)
	sharedDir := filepath.Join(repoDir, sharedRepoName)

	lock := repoLock(sharedDir)
	lock.Lock()
	defer lock.Unlock()

	writeCacheReadme(gitCacheDir)
	writeRepoReadme(repoDir, lp)

	repo, err := ensureSharedRepo(ctx, sharedDir, lp.CloneURL())
	if err != nil {
		return nil, fmt.Errorf("shared clone %s: %w", lp.CloneURL(), err)
	}

	sha, err := resolveToSHA(ctx, repo, lp.Ref, forceFetch)
	if err != nil {
		return nil, fmt.Errorf("resolve %s@%s: %w", lp.CloneURL(), lp.Ref, err)
	}

	return &LibTree{
		repo:   repo,
		sha:    sha,
		origin: lp.RepoID(),
	}, nil
}

// Text written to README.txt at the cache root so a user browsing
// ~/.../Facet/libcache/ in a file manager can tell what's inside before
// deciding an "empty" folder is safe to remove.
const cacheRootReadme = `Facet library cache
===================

This directory stores Git clones of every remote Facet library you've
imported with a "lib" expression. Each repo is laid out like:

    <host>/<user>/<repo>/
        .repo/   bare git clone (all objects — hidden)

Library source files are read directly from the bare clone at the
pinned commit, so each repo folder looks empty at a glance — the real
contents live inside the hidden .repo/ subdir.

Safe to delete this entire directory — Facet will re-clone anything it
needs the next time you open a project that imports a remote library.
`

// Text written to README.txt inside each <host>/<user>/<repo>/ dir. %s is
// substituted with the HTTPS clone URL for the repo.
const repoReadmeFmt = `Facet library cache — %s
========================================================

This folder mirrors a Git repository you've imported via "lib" in Facet.
The real contents live in the hidden .repo/ subdirectory (a bare clone
with every commit, tag, and branch). Facet reads library source files
directly from it at the pinned commit — please don't delete it.

Safe to delete this entire folder — Facet will re-clone on next use.
`

// writeCacheReadme writes the top-level README.txt, but only when it's
// missing. Idempotent; errors are swallowed — the README is cosmetic.
func writeCacheReadme(cacheDir string) {
	writeReadmeIfMissing(filepath.Join(cacheDir, "README.txt"), cacheRootReadme)
}

// writeRepoReadme writes the per-repo README.txt, customized with the repo's
// clone URL. Idempotent; errors are swallowed.
func writeRepoReadme(repoDir string, lp *LibPath) {
	writeReadmeIfMissing(
		filepath.Join(repoDir, "README.txt"),
		fmt.Sprintf(repoReadmeFmt, lp.CloneURL()),
	)
}

// writeReadmeIfMissing writes content to path if nothing is there yet. A
// mildly atomic pattern (temp file + rename) so we never leave a partial
// README visible.
func writeReadmeIfMissing(path, content string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".readme-*")
	if err != nil {
		return
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return
	}
	_ = os.Rename(tmp.Name(), path)
}

// ensureSharedRepo opens the shared bare clone, creating it if missing.
func ensureSharedRepo(ctx context.Context, sharedDir, cloneURL string) (*git.Repository, error) {
	if repo, err := git.PlainOpen(sharedDir); err == nil {
		return repo, nil
	}

	parent := filepath.Dir(sharedDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", parent, err)
	}
	tmpDir, err := os.MkdirTemp(parent, ".clone-*")
	if err != nil {
		return nil, fmt.Errorf("mktemp: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(tmpDir)
		}
	}()

	if _, err := git.PlainCloneContext(ctx, tmpDir, true /*bare*/, &git.CloneOptions{
		URL:  cloneURL,
		Tags: git.AllTags,
	}); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("git clone %s: %w", cloneURL, err)
	}

	if err := os.Rename(tmpDir, sharedDir); err != nil {
		// Another process may have won the race — retry open.
		if repo, openErr := git.PlainOpen(sharedDir); openErr == nil {
			return repo, nil
		}
		return nil, fmt.Errorf("rename shared clone: %w", err)
	}
	cleanup = false
	return git.PlainOpen(sharedDir)
}

// resolveToSHA resolves ref to a commit hash on repo. Immutable refs (full
// or abbreviated SHAs) are content-addressed, so a local hit is authoritative
// and no fetch is needed. Mutable refs (branches, tags) require a successful
// fetch — a failed fetch is a failed resolve, not an excuse to serve a stale
// commit.
func resolveToSHA(ctx context.Context, repo *git.Repository, ref string, forceFetch bool) (plumbing.Hash, error) {
	if isImmutableRef(ref) && !forceFetch {
		if h, err := resolveRef(repo, ref); err == nil {
			return *h, nil
		}
	}
	if err := fetchAll(ctx, repo); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("fetch: %w", err)
	}
	h, err := resolveRef(repo, ref)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return *h, nil
}

// fetchAll fetches all branches and tags from origin. NoErrAlreadyUpToDate is
// not an error — it means we had the latest already.
func fetchAll(ctx context.Context, repo *git.Repository) error {
	err := repo.FetchContext(ctx, &git.FetchOptions{
		Tags:  git.AllTags,
		Force: true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}

// resolveRef applies git's DWIM rules for rev-parsing a simple ref on a bare
// clone. Order matches gitrevisions(7): exact object (full or abbreviated
// SHA), tag, branch.
func resolveRef(repo *git.Repository, ref string) (*plumbing.Hash, error) {
	candidates := []plumbing.Revision{
		plumbing.Revision(ref),
		plumbing.Revision("refs/tags/" + ref),
		plumbing.Revision("refs/heads/" + ref),
		plumbing.Revision("refs/remotes/origin/" + ref),
	}
	var lastErr error
	for _, c := range candidates {
		h, err := repo.ResolveRevision(c)
		if err == nil {
			return h, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// OpenVirtualTree opens a bare clone at repoDir/.repo and returns a virtual
// tree handle at sha. Used by code that already knows the (repo, sha) tuple
// (e.g. ListLibraries walking the pins dir) without re-running resolution.
func OpenVirtualTree(repoDir, origin string, sha plumbing.Hash) (*LibTree, error) {
	repo, err := git.PlainOpen(filepath.Join(repoDir, sharedRepoName))
	if err != nil {
		return nil, err
	}
	return &LibTree{repo: repo, sha: sha, origin: origin}, nil
}

// PhysicalTree wraps an on-disk library directory as a LibTree. Used for
// InstalledLibs overrides and local/built-in libraries.
func PhysicalTree(dir string) *LibTree {
	return &LibTree{diskDir: dir}
}

// isImmutableRef returns true if ref looks like a commit SHA (7-40 hex
// chars). Branches and tags are treated as mutable — tags can be force-moved,
// and we'd rather pay a round-trip than serve a stale SHA.
func isImmutableRef(ref string) bool {
	if len(ref) < 7 || len(ref) > 40 {
		return false
	}
	for _, c := range ref {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
