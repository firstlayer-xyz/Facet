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

// ParseLibPath parses a raw library path into its components.
// Remote paths have a hostname (first segment contains '.') and require @ref.
// Local paths (e.g. "facet/gears") are returned with IsLocal=true.
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
		if len(segments) < 3 {
			return nil, fmt.Errorf("library path %q: remote paths need at least host/user/repo", lp.Raw)
		}
		if lp.Ref == "" {
			return nil, fmt.Errorf("library path %q: remote paths require @ref (e.g. @v1.0 or @main)", lp.Raw)
		}
		lp.Host = segments[0]
		lp.User = segments[1]
		lp.Repo = segments[2]
		if len(segments) > 3 {
			lp.SubPath = strings.Join(segments[3:], "/")
		}
		return lp, nil
	}

	// Local path
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

// ResolveLibPath resolves a raw library path to a local directory.
// Resolution order:
//  1. Built-in paths (IsLocal) → libDir/<path>
//  2. Settings-installed working copies (installedLibs overrides)
//  3. Git cache (previously cloned)
//  4. Remote clone (git clone --depth 1)
func ResolveLibPath(ctx context.Context, libDir, gitCacheDir string, installedLibs map[string]string, rawPath string) (string, error) {
	lp, err := ParseLibPath(rawPath)
	if err != nil {
		return "", err
	}

	// Default to the standard cache directory if none provided
	if gitCacheDir == "" {
		gitCacheDir = DefaultGitCacheDir()
	}

	// Local/built-in path → validate and resolve against libDir
	if lp.IsLocal {
		if err := ValidateLibPath(rawPath); err != nil {
			return "", err
		}
		return filepath.Join(libDir, rawPath), nil
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
func PullRepo(ctx context.Context, dir string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", dir, err)
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

// CloneRepo performs a shallow git clone into dest using an atomic temp+rename.
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

	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.NewBranchReferenceName(ref),
		Depth:         1,
		SingleBranch:  true,
	})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Branch ref failed — try as a tag
		_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
			URL:           url,
			ReferenceName: plumbing.NewTagReferenceName(ref),
			Depth:         1,
			SingleBranch:  true,
		})
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("git clone %s@%s: %w", url, ref, err)
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
