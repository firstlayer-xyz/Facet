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
type LibraryInfo struct {
	ID   string `json:"id"`   // "github.com/user/repo" (full path for operations)
	Name string `json:"name"` // "user/repo" (display name)
	Ref  string `json:"ref"`  // "v1.0" or "main"
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

// InstallLibrary clones a remote library into the git cache.
func (m *LibraryManager) InstallLibrary(url, ref string) error {
	if ref == "" {
		ref = "main"
	}
	// Parse the URL to determine cache path
	// Expected format: github.com/user/repo or https://github.com/user/repo.git
	rawPath := strings.TrimPrefix(url, "https://")
	rawPath = strings.TrimPrefix(rawPath, "http://")
	rawPath = strings.TrimSuffix(rawPath, ".git")
	rawPath = strings.TrimSuffix(rawPath, "/")

	segments := strings.Split(rawPath, "/")
	if len(segments) < 3 {
		return fmt.Errorf("invalid library URL: need host/user/repo")
	}

	cacheDir := loader.DefaultGitCacheDir()
	dest := filepath.Join(cacheDir, segments[0], segments[1], segments[2], ref)

	cloneURL := fmt.Sprintf("https://%s/%s/%s.git", segments[0], segments[1], segments[2])
	if err := loader.CloneRepo(m.ctx, cloneURL, ref, dest); err != nil {
		return err
	}

	m.assistant.RebuildSystemPrompt()
	return nil
}

// UpdateLibrary runs git pull in a cached library directory to fetch updates.
func (m *LibraryManager) UpdateLibrary(id, ref string) error {
	cacheDir := loader.DefaultGitCacheDir()
	dir := filepath.Join(cacheDir, id, ref)
	if err := loader.PullRepo(m.ctx, dir); err != nil {
		fmt.Fprintf(os.Stderr, "UpdateLibrary %s@%s: %v\n", id, ref, err)
		return err
	}

	return nil
}

// ForkLibrary copies a cached library to the local libraries directory,
// skipping hidden files/directories (like .git).
func (m *LibraryManager) ForkLibrary(id, ref string) error {
	cacheDir := loader.DefaultGitCacheDir()
	src := filepath.Join(cacheDir, id, ref)
	// Strip host from id (github.com/user/repo -> user/repo) for local path.
	// Only strip the first segment if it looks like a hostname (contains a dot).
	localID := id
	if parts := strings.SplitN(id, "/", 2); len(parts) > 1 && strings.Contains(parts[0], ".") {
		localID = parts[1]
	}
	libDirPath, err := libraryDir()
	if err != nil {
		return err
	}
	dst := filepath.Join(libDirPath, localID)

	if err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden files/directories
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	}); err != nil {
		return err
	}
	m.assistant.RebuildSystemPrompt()
	return nil
}

// ListLibraries scans the git cache directory for cloned libraries.
func (m *LibraryManager) ListLibraries() ([]LibraryInfo, error) {
	cacheDir := loader.DefaultGitCacheDir()
	var libs []LibraryInfo

	// Walk <cacheDir>/<host>/<user>/<repo>/<ref>/
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
				repoDir := filepath.Join(userDir, r.Name())
				refs, err := readDirSkipMissing(repoDir)
				if err != nil {
					return nil, err
				}
				for _, ref := range refs {
					if !ref.IsDir() {
						continue
					}
					libs = append(libs, LibraryInfo{
						ID:   fmt.Sprintf("%s/%s/%s", h.Name(), u.Name(), r.Name()),
						Name: fmt.Sprintf("%s/%s", u.Name(), r.Name()),
						Ref:  ref.Name(),
						Path: filepath.Join(repoDir, ref.Name()),
					})
				}
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

// PullAllLibraries pulls all cached libraries, fetching updates and tags.
func (m *LibraryManager) PullAllLibraries() error {
	libs, err := m.ListLibraries()
	if err != nil {
		return err
	}
	var errs []string
	for _, lib := range libs {
		if err := loader.PullRepo(m.ctx, lib.Path); err != nil {
			msg := fmt.Sprintf("%s@%s: %v", lib.Name, lib.Ref, err)
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
