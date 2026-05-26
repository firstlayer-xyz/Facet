// Package docgen generates HTML documentation from markdown guides and facet API docs.
package docgen

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"facet/pkg/fctlang/doc"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
)

// GuideSection is a navigable h2 section within a guide page.
type GuideSection struct {
	Name string
	Slug string
}

// Guide holds a parsed markdown guide.
type Guide struct {
	Title    string // extracted from first # heading
	Slug     string // filename without extension
	Markdown string // raw markdown source
	HTML     string // rendered HTML
	Sections []GuideSection
}

// APISection holds doc entries for one source-level section within a library.
type APISection struct {
	Name    string         // "3D Constructors", etc.; empty for ungrouped entries
	Slug    string         // URL-safe anchor id derived from Name
	Entries []doc.DocEntry
}

// APIGroup holds all sections for a library or category.
type APIGroup struct {
	Name     string       // "Standard Library", "facet/gears", etc.
	Sections []APISection
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

var guideH2Re = regexp.MustCompile(`<h2 id="([^"]+)">([^<]*)</h2>`)

// renderMarkdown converts markdown text to HTML using goldmark.
// WithAutoHeadingID injects id attributes on headings for in-page navigation.
func renderMarkdown(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID()))
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// extractGuideSections returns the h2 sections from rendered guide HTML.
func extractGuideSections(html string) []GuideSection {
	var sections []GuideSection
	for _, m := range guideH2Re.FindAllStringSubmatch(html, -1) {
		sections = append(sections, GuideSection{Slug: m[1], Name: m[2]})
	}
	return sections
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
			Sections: extractGuideSections(html),
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

// groupByLibrary groups doc entries into APIGroups, preserving source order of
// both libraries and sections within each library.
func groupByLibrary(entries []doc.DocEntry) []APIGroup {
	type sectionKey struct{ library, section string }

	sectionEntries := make(map[sectionKey][]doc.DocEntry)
	var libOrder []string
	libSeen := make(map[string]bool)
	sectionOrder := make(map[string][]string) // library → ordered section names
	sectionSeen := make(map[sectionKey]bool)

	for _, e := range entries {
		lib := e.Library
		if lib == "" {
			lib = "Standard Library"
		}
		if !libSeen[lib] {
			libOrder = append(libOrder, lib)
			libSeen[lib] = true
		}
		key := sectionKey{lib, e.Section}
		if !sectionSeen[key] {
			sectionOrder[lib] = append(sectionOrder[lib], e.Section)
			sectionSeen[key] = true
		}
		sectionEntries[key] = append(sectionEntries[key], e)
	}

	// Standard Library first, then alphabetical.
	sort.SliceStable(libOrder, func(i, j int) bool {
		if libOrder[i] == "Standard Library" {
			return true
		}
		if libOrder[j] == "Standard Library" {
			return false
		}
		return libOrder[i] < libOrder[j]
	})

	var result []APIGroup
	for _, lib := range libOrder {
		var sections []APISection
		for _, sec := range sectionOrder[lib] {
			key := sectionKey{lib, sec}
			sections = append(sections, APISection{
				Name:    sec,
				Slug:    slugify(sec),
				Entries: sectionEntries[key],
			})
		}
		result = append(result, APIGroup{Name: lib, Sections: sections})
	}
	return result
}
