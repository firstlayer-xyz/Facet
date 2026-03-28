// Package docgen generates HTML documentation from markdown guides and facet API docs.
package docgen

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"facet/app/pkg/fctlang/doc"

	"github.com/yuin/goldmark"
)

// Guide holds a parsed markdown guide.
type Guide struct {
	Title    string // extracted from first # heading
	Slug     string // filename without extension
	Markdown string // raw markdown source
	HTML     string // rendered HTML
}

// APIGroup holds doc entries for a library or category.
type APIGroup struct {
	Name    string             // "Standard Library", "facet/gears", etc.
	Entries []doc.DocEntry // doc entries in this group
}

// DocSite holds all documentation data.
type DocSite struct {
	Guides    []Guide
	APIGroups []APIGroup
}

// BuildSite reads markdown guides from guidesDir, extracts API docs from
// the embedded stdlib and any libraries in libDir, and returns a DocSite.
func BuildSite(guidesDir, libDir string) (*DocSite, error) {
	site := &DocSite{}

	// 1. Parse markdown guides
	if guidesDir != "" {
		guides, err := loadGuides(guidesDir)
		if err != nil {
			return nil, err
		}
		site.Guides = guides
	}

	// 2. Build API doc entries (stdlib + embedded libraries + filesystem libraries)
	entries := doc.BuildDocIndex("", nil)

	// Also include filesystem library entries if libDir is provided
	if libDir != "" {
		libEntries := doc.BuildLibDocEntries(libDir)
		// Deduplicate by name+library (embedded libs overlap with filesystem)
		seen := make(map[string]bool)
		for _, e := range entries {
			seen[e.Name+"|"+e.Library] = true
		}
		for _, e := range libEntries {
			key := e.Name + "|" + e.Library
			if !seen[key] {
				entries = append(entries, e)
				seen[key] = true
			}
		}
	}

	// 3. Group entries by library
	site.APIGroups = groupByLibrary(entries)

	return site, nil
}

// renderMarkdown converts markdown text to HTML using goldmark.
func renderMarkdown(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New()
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// loadGuides reads all .md files from the given directory.
func loadGuides(dir string) ([]Guide, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var guides []Guide
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		src := string(data)
		html, err := renderMarkdown(src)
		if err != nil {
			return nil, err
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		title := extractTitle(src)
		if title == "" {
			title = slug
		}
		guides = append(guides, Guide{
			Title:    title,
			Slug:     slug,
			Markdown: src,
			HTML:     html,
		})
	}
	return guides, nil
}

// extractTitle finds the first # heading in markdown.
func extractTitle(md string) string {
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}
	return ""
}

// groupByLibrary groups doc entries into APIGroups.
func groupByLibrary(entries []doc.DocEntry) []APIGroup {
	groups := make(map[string][]doc.DocEntry)
	for _, e := range entries {
		key := e.Library
		if key == "" {
			key = "Standard Library"
		}
		groups[key] = append(groups[key], e)
	}

	var result []APIGroup
	for name, ents := range groups {
		result = append(result, APIGroup{Name: name, Entries: ents})
	}
	sort.Slice(result, func(i, j int) bool {
		// Standard Library first, then alphabetical
		if result[i].Name == "Standard Library" {
			return true
		}
		if result[j].Name == "Standard Library" {
			return false
		}
		return result[i].Name < result[j].Name
	})
	return result
}
