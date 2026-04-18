package main

// Wails-bound delegates for library operations. All logic lives in
// LibraryManager; this file exists only so the App struct (which Wails
// introspects) keeps the same method surface.

// InstallLibrary clones a remote repo into the git cache.
func (a *App) InstallLibrary(url string) error {
	return a.libraries.InstallLibrary(url)
}

// UpdateLibrary fetches the latest refs for a cached repo.
func (a *App) UpdateLibrary(id string) error {
	return a.libraries.UpdateLibrary(id)
}

// ForkLibrary materializes the repo's default branch into the local
// libraries directory as an editable copy.
func (a *App) ForkLibrary(id string) error {
	return a.libraries.ForkLibrary(id)
}

// RemoveLibrary deletes a single repo from the cache.
func (a *App) RemoveLibrary(id string) error {
	return a.libraries.RemoveLibrary(id)
}

// ListLibraries scans the git cache directory for cloned libraries.
func (a *App) ListLibraries() ([]LibraryInfo, error) {
	return a.libraries.ListLibraries()
}

// ListLocalLibraries scans the local libraries directory recursively.
func (a *App) ListLocalLibraries() ([]LibraryInfo, error) {
	return a.libraries.ListLocalLibraries()
}

// CreateLocalLibrary creates a new library inside a folder with a starter template.
func (a *App) CreateLocalLibrary(folder, name string) (string, error) {
	return a.libraries.CreateLocalLibrary(folder, name)
}

// CreateLibraryFolder creates a new top-level library folder.
func (a *App) CreateLibraryFolder(folder string) error {
	return a.libraries.CreateLibraryFolder(folder)
}

// ListLibraryFolders returns the top-level library folder names.
func (a *App) ListLibraryFolders() ([]string, error) {
	return a.libraries.ListLibraryFolders()
}

// PullAllLibraries pulls all cached libraries, fetching updates and tags.
func (a *App) PullAllLibraries() error {
	return a.libraries.PullAllLibraries()
}

// OpenLibraryDir reads the .fct file from a library directory.
func (a *App) OpenLibraryDir(dir string) (map[string]string, error) {
	return a.libraries.OpenLibraryDir(dir)
}

// GetLibraryDir returns the path to the local libraries root directory.
func (a *App) GetLibraryDir() (string, error) {
	return a.libraries.GetLibraryDir()
}

// ClearLibCache removes all cached (git-cloned) libraries.
func (a *App) ClearLibCache() error {
	return a.libraries.ClearLibCache()
}

// RevealInFileManager opens the given path in the OS file manager and brings it to front.
func (a *App) RevealInFileManager(path string) error {
	return revealInFileManager(path)
}
