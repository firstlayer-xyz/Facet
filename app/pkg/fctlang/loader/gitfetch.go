package loader

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// LibPath holds the parsed components of a library import path.
type LibPath struct {
	Host    string // e.g. "github.com"
	User    string // e.g. "firstlayer-xyz"
	Repo    string // e.g. "facet-fasteners"
	SubPath string // e.g. "gears" (empty for root)
	Ref     string // e.g. "v1.0" or "main"
	Raw     string // original path before parsing
	IsLocal bool   // true for built-in paths like "facet/gears"
}

// CloneURL returns the HTTPS clone URL for a remote library.
func (lp *LibPath) CloneURL() string {
	return fmt.Sprintf("https://%s/%s/%s.git", lp.Host, lp.User, lp.Repo)
}

// ParseLibPath parses a raw library path into its components. Remote paths
// have a hostname (first segment contains '.') and require an @ref pinning
// them to a specific branch, tag, or commit hash — this is the mechanism for
// reproducible builds. Local/built-in paths are plain filesystem-relative
// paths and may optionally carry an @ref (advisory — built-ins ship with the
// binary at a single version).
func ParseLibPath(raw string) (*LibPath, error) {
	lp := &LibPath{Raw: raw}

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

// resolveLibPath resolves a raw library path to a local directory.
// Resolution order:
//  1. Built-in paths (IsLocal) → libDir/<path>
//  2. Settings-installed working copies (installedLibs overrides)
//  3. Git cache (previously cloned)
//  4. Remote clone (git clone --depth 1)
func resolveLibPath(ctx context.Context, libDir, gitCacheDir string, installedLibs map[string]string, rawPath string) (string, error) {
	lp, err := ParseLibPath(rawPath)
	if err != nil {
		return "", err
	}

	// Default to the standard cache directory if none provided
	if gitCacheDir == "" {
		gitCacheDir = DefaultGitCacheDir()
	}

	// Local/built-in path → validate and resolve against libDir. When an @ref
	// is present it is part of the identity (for cache keying) but does not
	// participate in the on-disk lookup — built-in libraries ship at a single
	// version tied to the Facet binary, so the ref is advisory here.
	if lp.IsLocal {
		localPath := rawPath
		if lp.Ref != "" {
			localPath = strings.TrimSuffix(rawPath, "@"+lp.Ref)
		}
		if err := validateLibPath(localPath); err != nil {
			return "", err
		}
		return filepath.Join(libDir, localPath), nil
	}

	// Check settings-installed working copies first (library authors)
	libID := fmt.Sprintf("%s/%s/%s", lp.Host, lp.User, lp.Repo)
	if installedLibs != nil {
		if dir, ok := installedLibs[libID]; ok {
			if lp.SubPath != "" {
				return filepath.Join(dir, lp.SubPath), nil
			}
			return dir, nil
		}
	}

	// Check/ensure git cache
	dir, err := ensureCloned(ctx, gitCacheDir, lp)
	if err != nil {
		return "", err
	}

	if lp.SubPath != "" {
		return filepath.Join(dir, lp.SubPath), nil
	}
	return dir, nil
}

// ensureCloned checks the git cache and clones if needed.
// Returns the path to the cached repo directory.
func ensureCloned(ctx context.Context, gitCacheDir string, lp *LibPath) (string, error) {
	cacheDir := filepath.Join(gitCacheDir, lp.Host, lp.User, lp.Repo, lp.Ref)

	// Already cached?
	if info, err := os.Stat(cacheDir); err == nil && info.IsDir() {
		return cacheDir, nil
	}

	// Clone to temp dir, then atomic rename
	if err := CloneRepo(ctx, lp.CloneURL(), lp.Ref, cacheDir); err != nil {
		return "", fmt.Errorf("library %q: %w", lp.Raw, err)
	}

	return cacheDir, nil
}

// PullRepo opens an existing git repo at dir and pulls the current branch.
// If the pull fails (e.g. shallow clone issues with go-git), the cache dir
// is deleted and re-cloned from scratch.
//
// If HEAD is detached (the cache was cloned at a specific commit SHA), the
// content is immutable and this is a no-op.
func PullRepo(ctx context.Context, dir string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", dir, err)
	}
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("head %s: %w", dir, err)
	}
	if head.Name() == plumbing.HEAD || !head.Name().IsBranch() {
		// Detached HEAD — pinned to a commit SHA, nothing to pull.
		return nil
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree %s: %w", dir, err)
	}
	// Reset any local changes (cache is read-only, safe to discard).
	if err := w.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
		log.Printf("gitfetch: reset %s: %v", dir, err)
	}
	err = w.Pull(&git.PullOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		// Shallow single-branch clones can fail with go-git; nuke and re-clone.
		log.Printf("gitfetch: pull failed for %s: %v — attempting re-clone", dir, err)
		return recloneCache(ctx, dir)
	}
	// Best-effort: also fetch all tags (ignore errors).
	repo.Fetch(&git.FetchOptions{
		Tags: git.AllTags,
	})
	return nil
}

// recloneCache deletes a cached repo and re-clones it by inferring the
// clone URL and ref from the cache directory structure:
// .../libcache/<host>/<user>/<repo>/<ref>[/<subpath>]
func recloneCache(ctx context.Context, dir string) error {
	// dir must be the git root itself (caller is responsible for this).
	// Do NOT walk up the directory tree — that risks finding an unrelated
	// parent repo and deleting it.

	// Safety: never RemoveAll outside the libcache directory tree.
	cacheDir := DefaultGitCacheDir()
	if !strings.HasPrefix(dir, cacheDir+string(filepath.Separator)) {
		return fmt.Errorf("reclone: refusing to delete %s (not inside cache %s)", dir, cacheDir)
	}

	// Parse cache path: .../libcache/<host>/<user>/<repo>/<ref>
	ref := filepath.Base(dir)
	repoDir := filepath.Dir(dir)
	repo := filepath.Base(repoDir)
	userDir := filepath.Dir(repoDir)
	user := filepath.Base(userDir)
	hostDir := filepath.Dir(userDir)
	host := filepath.Base(hostDir)

	url := fmt.Sprintf("https://%s/%s/%s.git", host, user, repo)
	log.Printf("gitfetch: re-cloning %s@%s into %s", url, ref, dir)

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("reclone: remove %s: %w", dir, err)
	}
	return CloneRepo(ctx, url, ref, dir)
}

// CloneRepo performs a full git clone into dest using an atomic temp+rename,
// then resolves ref via git's standard DWIM rules (commit object → tag →
// branch → remote-tracking branch) and checks out the resulting commit in a
// detached HEAD. The ref may be a full or partial commit SHA, a tag, a branch
// name, or anything else `git rev-parse` would accept.
//
// A full clone is used (rather than shallow + SingleBranch) so any ref the
// server advertises can be resolved, including historical commits.
func CloneRepo(ctx context.Context, url, ref, dest string) error {
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp(parent, ".clone-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) // clean up on failure

	repo, err := git.PlainCloneContext(ctx, tmpDir, false, &git.CloneOptions{
		URL:        url,
		NoCheckout: true,
	})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("git clone %s: %w", url, err)
	}

	hash, err := resolveRef(repo, ref)
	if err != nil {
		return fmt.Errorf("git resolve %s@%s: %w", url, ref, err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git worktree %s: %w", url, err)
	}
	if err := w.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return fmt.Errorf("git checkout %s@%s: %w", url, ref, err)
	}

	// Atomic rename
	if err := os.Rename(tmpDir, dest); err != nil {
		// Another process may have beat us — check if dest exists now
		if info, statErr := os.Stat(dest); statErr == nil && info.IsDir() {
			return nil
		}
		return fmt.Errorf("renaming clone to cache: %w", err)
	}

	return nil
}

// resolveRef applies git's DWIM rules for rev-parsing a simple ref. Order
// matches gitrevisions(7): exact object (full or abbreviated SHA), tag,
// local branch, remote-tracking branch. A freshly cloned repo has only the
// default branch as a local head, so most branch names resolve via the
// remote-tracking fallback.
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
