package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"facet/app/pkg/fctlang/loader"
)

// readDirSkipMissing returns nil on IsNotExist (the directory just hasn't been
// created yet — expected during first launch) and propagates every other
// error. Library listing walks use this to distinguish "no libraries
// installed" (fine) from "permission denied" (a real problem the user needs
// to see rather than a silently empty library menu).
func readDirSkipMissing(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	return entries, nil
}

// LibraryInfo describes an installed/cached library for the settings UI.
// Cached (git) entries are one per repo — refs aren't exposed in the UI. Ref
// is only populated for local-library listings where it names the folder
// segment the library lives under.
type LibraryInfo struct {
	ID   string `json:"id"`   // "github.com/user/repo" (full path for operations)
	Name string `json:"name"` // "user/repo" (display name)
	Path string `json:"path"` // local filesystem path
}

// LibraryManager owns cached/local library operations (install, list, update,
// fork, pull, clear cache, create). It needs the app context for git
// operations and a reference to the assistant so it can refresh the cached
// system prompt when the library catalog changes.
type LibraryManager struct {
	ctx       context.Context
	assistant *AssistantService
}

// NewLibraryManager creates a new library manager. The context is set later
// (via SetContext) once the Wails runtime has started.
func NewLibraryManager(assistant *AssistantService) *LibraryManager {
	return &LibraryManager{assistant: assistant}
}

// SetContext wires the Wails startup context into the manager. Git
// operations (clone, pull) use it for cancellation.
func (m *LibraryManager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

// InstallLibrary clones a remote repo into the git cache. Takes just a URL —
// the cache is a bare clone with every ref, so there's nothing to "pick" at
// install time. Individual refs are selected later by `lib ... "ref"`
// statements in .fct source.
func (m *LibraryManager) InstallLibrary(url string) error {
	lp, err := parseRepoURL(url)
	if err != nil {
		return err
	}
	if err := loader.EnsureRepoClone(m.ctx, loader.DefaultGitCacheDir(), lp); err != nil {
		return err
	}
	m.assistant.RebuildSystemPrompt()
	return nil
}

// UpdateLibrary fetches every ref from origin for the repo named by id
// ("host/user/repo"). The bare clone is a single shared object store, so one
// fetch updates every tag, branch, and SHA the clone knows about.
func (m *LibraryManager) UpdateLibrary(id string) error {
	lp, err := parseRepoID(id)
	if err != nil {
		return err
	}
	if err := loader.RefreshRepoClone(m.ctx, loader.DefaultGitCacheDir(), lp); err != nil {
		fmt.Fprintf(os.Stderr, "UpdateLibrary %s: %v\n", id, err)
		return err
	}
	return nil
}

// ForkLibrary materializes the repo's default branch (origin/HEAD) into the
// local libraries directory as an editable copy. Hidden files/dirs are
// skipped by LibTree.ExtractTo.
func (m *LibraryManager) ForkLibrary(id string) error {
	lp, err := parseRepoID(id)
	if err != nil {
		return err
	}
	tree, err := loader.OpenRepoHeadTree(m.ctx, loader.DefaultGitCacheDir(), lp)
	if err != nil {
		return err
	}

	// Strip host from id (github.com/user/repo -> user/repo) for the local
	// destination — only strip when the first segment looks like a hostname.
	localID := id
	if parts := strings.SplitN(id, "/", 2); len(parts) > 1 && strings.Contains(parts[0], ".") {
		localID = parts[1]
	}
	libDirPath, err := libraryDir()
	if err != nil {
		return err
	}
	dst := filepath.Join(libDirPath, localID)

	if err := tree.ExtractTo(dst); err != nil {
		return err
	}
	m.assistant.RebuildSystemPrompt()
	return nil
}

// RemoveLibrary deletes the cache for a single repo. Identifies the repo by
// id ("host/user/repo"). The bare clone and any enclosing README are removed;
// empty parent dirs (host/, host/user/) are pruned so a subsequent ListLibraries
// doesn't surface ghost empty entries.
func (m *LibraryManager) RemoveLibrary(id string) error {
	lp, err := parseRepoID(id)
	if err != nil {
		return err
	}
	cacheDir := loader.DefaultGitCacheDir()
	repoDir := filepath.Join(cacheDir, lp.Host, lp.User, lp.Repo)
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("remove %s: %w", repoDir, err)
	}
	// Best-effort prune of empty parent dirs — harmless if they still hold
	// sibling repos.
	_ = os.Remove(filepath.Join(cacheDir, lp.Host, lp.User))
	_ = os.Remove(filepath.Join(cacheDir, lp.Host))
	m.assistant.RebuildSystemPrompt()
	return nil
}

// parseRepoURL parses a user-supplied URL into a LibPath with only host/user/repo
// populated (no ref). Accepts https://, http://, or bare host/user/repo, with
// optional trailing .git or slash.
func parseRepoURL(url string) (*loader.LibPath, error) {
	raw := strings.TrimPrefix(url, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimSuffix(raw, ".git")
	raw = strings.TrimSuffix(raw, "/")
	return parseRepoID(raw)
}

// parseRepoID parses a bare "host/user/repo" identifier, rejecting local
// paths and any trailing @ref.
func parseRepoID(id string) (*loader.LibPath, error) {
	if strings.Contains(id, "@") {
		return nil, fmt.Errorf("library id must not carry @ref: %q", id)
	}
	segments := strings.Split(id, "/")
	if len(segments) < 3 || !strings.Contains(segments[0], ".") {
		return nil, fmt.Errorf("library id must be host/user/repo: %q", id)
	}
	return &loader.LibPath{
		Host: segments[0],
		User: segments[1],
		Repo: segments[2],
		Raw:  id,
	}, nil
}

// ListLibraries returns one entry per cached repo. Refs aren't surfaced —
// the bare clone holds every ref and .fct `lib` statements pick the one they
// need at resolve time.
func (m *LibraryManager) ListLibraries() ([]LibraryInfo, error) {
	cacheDir := loader.DefaultGitCacheDir()
	var libs []LibraryInfo

	hosts, err := readDirSkipMissing(cacheDir)
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		if !h.IsDir() || strings.HasPrefix(h.Name(), ".") {
			continue
		}
		hostDir := filepath.Join(cacheDir, h.Name())
		users, err := readDirSkipMissing(hostDir)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			if !u.IsDir() {
				continue
			}
			userDir := filepath.Join(hostDir, u.Name())
			repos, err := readDirSkipMissing(userDir)
			if err != nil {
				return nil, err
			}
			for _, r := range repos {
				if !r.IsDir() {
					continue
				}
				libs = append(libs, LibraryInfo{
					ID:   fmt.Sprintf("%s/%s/%s", h.Name(), u.Name(), r.Name()),
					Name: fmt.Sprintf("%s/%s", u.Name(), r.Name()),
					Path: filepath.Join(userDir, r.Name()),
				})
			}
		}
	}
	return libs, nil
}

// ListLocalLibraries scans the local libraries directory recursively for libraries.
// A library is any directory containing a <dirname>.fct file.
func (m *LibraryManager) ListLocalLibraries() ([]LibraryInfo, error) {
	libDir, err := libraryDir()
	if err != nil {
		return nil, err
	}
	var libs []LibraryInfo
	var walkErr error
	var walk func(dir, rel string)
	walk = func(dir, rel string) {
		if walkErr != nil {
			return
		}
		entries, err := readDirSkipMissing(dir)
		if err != nil {
			walkErr = err
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			// Skip the embedded "facet" namespace at top level
			if rel == "" && e.Name() == "facet" {
				continue
			}
			childDir := filepath.Join(dir, e.Name())
			childRel := e.Name()
			if rel != "" {
				childRel = rel + "/" + e.Name()
			}
			// Check if this directory is a library (contains <name>.fct)
			fctFile := filepath.Join(childDir, e.Name()+".fct")
			if _, err := os.Stat(fctFile); err == nil {
				libs = append(libs, LibraryInfo{
					ID:   childRel,
					Path: childDir,
				})
			}
			// Recurse deeper
			walk(childDir, childRel)
		}
	}
	walk(libDir, "")
	if walkErr != nil {
		return nil, walkErr
	}
	return libs, nil
}

// CreateLocalLibrary creates a new library inside a folder with a starter template.
// The library is created at <libDir>/<folder>/<name>/<name>.fct.
// Import path: lib "<folder>/<name>"
func (m *LibraryManager) CreateLocalLibrary(folder, name string) (string, error) {
	if err := validateLibName(folder); err != nil {
		return "", fmt.Errorf("folder: %w", err)
	}
	if err := validateLibName(name); err != nil {
		return "", fmt.Errorf("library: %w", err)
	}

	libDir, err := libraryDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(libDir, folder, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	filePath := filepath.Join(dir, name+".fct")
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil // already exists — just open it
	}

	template := fmt.Sprintf("# %s/%s library\n\n", folder, name)
	if err := os.WriteFile(filePath, []byte(template), 0o644); err != nil {
		return "", err
	}
	return filePath, nil
}

// CreateLibraryFolder creates a new top-level library folder.
// Import paths for libraries inside will be "<folder>/<name>".
func (m *LibraryManager) CreateLibraryFolder(folder string) error {
	if err := validateLibName(folder); err != nil {
		return err
	}
	libDir, err := libraryDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(libDir, folder), 0o755)
}

// ListLibraryFolders returns the top-level library folder names.
func (m *LibraryManager) ListLibraryFolders() ([]string, error) {
	libDir, err := libraryDir()
	if err != nil {
		return nil, err
	}
	entries, err := readDirSkipMissing(libDir)
	if err != nil {
		return nil, err
	}
	var folders []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "facet" {
			folders = append(folders, e.Name())
		}
	}
	return folders, nil
}

// PullAllLibraries fetches every cached repo's bare clone from origin.
func (m *LibraryManager) PullAllLibraries() error {
	libs, err := m.ListLibraries()
	if err != nil {
		return err
	}
	cacheDir := loader.DefaultGitCacheDir()
	var errs []string
	for _, lib := range libs {
		lp, err := parseRepoID(lib.ID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", lib.Name, err))
			continue
		}
		if err := loader.RefreshRepoClone(m.ctx, cacheDir, lp); err != nil {
			msg := fmt.Sprintf("%s: %v", lib.Name, err)
			fmt.Fprintf(os.Stderr, "PullAllLibraries: %s\n", msg)
			errs = append(errs, msg)
		}
	}
	m.assistant.RebuildSystemPrompt()
	if len(errs) > 0 {
		return fmt.Errorf("pull errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// OpenLibraryDir reads the .fct file from a library directory and returns its path and source.
func (m *LibraryManager) OpenLibraryDir(dir string) (map[string]string, error) {
	name := filepath.Base(dir)
	fctPath := filepath.Join(dir, name+".fct")
	data, err := os.ReadFile(fctPath)
	if err != nil {
		return nil, err
	}
	return map[string]string{"path": fctPath, "source": string(data)}, nil
}

// GetLibraryDir returns the path to the local libraries root directory.
func (m *LibraryManager) GetLibraryDir() (string, error) {
	return libraryDir()
}

// validateLibName checks that a name contains only safe characters for library paths.
func validateLibName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("name may only contain letters, digits, hyphens, and underscores")
		}
	}
	return nil
}

// ClearLibCache removes all cached (git-cloned) libraries.
func (m *LibraryManager) ClearLibCache() error {
	cacheDir := loader.DefaultGitCacheDir()
	entries, err := readDirSkipMissing(cacheDir)
	if err != nil {
		return err
	}
	var errs []string
	for _, e := range entries {
		path := filepath.Join(cacheDir, e.Name())
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", path, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("clear cache: %s", strings.Join(errs, "; "))
	}
	return nil
}
