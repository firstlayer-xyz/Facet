package docgen

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"facet/app/pkg/fctlang/doc"
)

const cssStyle = `
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: #e0e0e0; background: #1a1a2e; line-height: 1.6; }
.layout { display: flex; min-height: 100vh; }
nav { width: 260px; background: #16162a; border-right: 1px solid #2a2a4a; padding: 20px; position: fixed; top: 0; bottom: 0; overflow-y: auto; }
nav h1 { font-size: 18px; margin-bottom: 16px; color: #fff; }
nav a { display: block; padding: 4px 8px; color: #aaa; text-decoration: none; font-size: 14px; border-radius: 4px; }
nav a:hover { color: #fff; background: #2a2a4a; }
nav a.active { color: #fff; background: #3a3a5a; }
nav .section-label { font-size: 11px; text-transform: uppercase; color: #666; margin-top: 16px; margin-bottom: 4px; letter-spacing: 0.5px; }
main { flex: 1; margin-left: 260px; padding: 40px; max-width: 900px; }
h1 { font-size: 28px; margin-bottom: 16px; color: #fff; }
h2 { font-size: 22px; margin-top: 32px; margin-bottom: 12px; color: #fff; border-bottom: 1px solid #2a2a4a; padding-bottom: 8px; }
h3 { font-size: 18px; margin-top: 24px; margin-bottom: 8px; color: #ddd; }
p { margin-bottom: 12px; }
code { font-family: "JetBrains Mono", "Fira Code", monospace; background: #2a2a4a; padding: 2px 6px; border-radius: 3px; font-size: 13px; }
pre { background: #12122a; border: 1px solid #2a2a4a; border-radius: 6px; padding: 16px; margin-bottom: 16px; overflow-x: auto; }
pre code { background: none; padding: 0; }
table { width: 100%; border-collapse: collapse; margin-bottom: 16px; }
th, td { padding: 8px 12px; border: 1px solid #2a2a4a; text-align: left; font-size: 14px; }
th { background: #12122a; color: #fff; }
ul, ol { margin-bottom: 12px; padding-left: 24px; }
li { margin-bottom: 4px; }
.doc-entry { margin-bottom: 16px; padding: 12px; background: #12122a; border: 1px solid #2a2a4a; border-radius: 6px; }
.doc-entry .signature { font-family: "JetBrains Mono", "Fira Code", monospace; font-size: 13px; color: #7ec8e3; margin-bottom: 4px; }
.doc-entry .doc { font-size: 14px; color: #bbb; }
.doc-entry .meta { font-size: 12px; color: #666; margin-top: 4px; }
a { color: #7ec8e3; }
`

// GenerateSite writes a complete HTML documentation site to outDir.
func GenerateSite(site *DocSite, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	// Build nav links
	navHTML := buildNav(site)

	// Generate index page
	if err := writeHTMLPage(filepath.Join(outDir, "index.html"), "Facet Documentation", navHTML, buildIndexContent(site)); err != nil {
		return err
	}

	// Generate guide pages
	for _, g := range site.Guides {
		if err := writeHTMLPage(filepath.Join(outDir, g.Slug+".html"), g.Title+" - Facet", navHTML, g.HTML); err != nil {
			return err
		}
	}

	// Generate API index page
	if err := os.MkdirAll(filepath.Join(outDir, "api"), 0755); err != nil {
		return err
	}
	if err := writeHTMLPage(filepath.Join(outDir, "api", "index.html"), "API Reference - Facet", navHTML, buildAPIContent(site)); err != nil {
		return err
	}

	return nil
}

func buildNav(site *DocSite) string {
	var b strings.Builder
	b.WriteString(`<h1>Facet Docs</h1>`)
	b.WriteString(`<a href="index.html">Home</a>`)

	if len(site.Guides) > 0 {
		b.WriteString(`<div class="section-label">Guides</div>`)
		for _, g := range site.Guides {
			fmt.Fprintf(&b, `<a href="%s.html">%s</a>`, g.Slug, html.EscapeString(g.Title))
		}
	}

	b.WriteString(`<div class="section-label">API Reference</div>`)
	b.WriteString(`<a href="api/index.html">All APIs</a>`)
	for _, g := range site.APIGroups {
		fmt.Fprintf(&b, `<a href="api/index.html#%s">%s</a>`, slugify(g.Name), html.EscapeString(g.Name))
	}

	return b.String()
}

func buildIndexContent(site *DocSite) string {
	var b strings.Builder
	b.WriteString(`<h1>Facet Documentation</h1>`)
	b.WriteString(`<p>Welcome to the Facet documentation. Facet is a desktop CAD application where you write code to describe 3D models.</p>`)

	if len(site.Guides) > 0 {
		b.WriteString(`<h2>Guides</h2><ul>`)
		for _, g := range site.Guides {
			fmt.Fprintf(&b, `<li><a href="%s.html">%s</a></li>`, g.Slug, html.EscapeString(g.Title))
		}
		b.WriteString(`</ul>`)
	}

	b.WriteString(`<h2>API Reference</h2>`)
	b.WriteString(`<p><a href="api/index.html">Browse the complete API reference</a> for all built-in functions, methods, types, and keywords.</p>`)

	return b.String()
}

func buildAPIContent(site *DocSite) string {
	var b strings.Builder
	b.WriteString(`<h1>API Reference</h1>`)

	for _, group := range site.APIGroups {
		fmt.Fprintf(&b, `<h2 id="%s">%s</h2>`, slugify(group.Name), html.EscapeString(group.Name))
		for _, e := range group.Entries {
			renderDocEntry(&b, e)
		}
	}

	return b.String()
}

func renderDocEntry(b *strings.Builder, e doc.DocEntry) {
	b.WriteString(`<div class="doc-entry">`)
	if e.Signature != "" {
		fmt.Fprintf(b, `<div class="signature">%s</div>`, html.EscapeString(e.Signature))
	} else {
		fmt.Fprintf(b, `<div class="signature">%s</div>`, html.EscapeString(e.Name))
	}
	if e.Doc != "" {
		fmt.Fprintf(b, `<div class="doc">%s</div>`, html.EscapeString(e.Doc))
	}
	meta := e.Kind
	if e.Library != "" {
		meta += " (" + e.Library + ")"
	}
	fmt.Fprintf(b, `<div class="meta">%s</div>`, html.EscapeString(meta))
	b.WriteString(`</div>`)
}

func writeHTMLPage(path, title, navHTML, bodyHTML string) error {
	var b strings.Builder
	fmt.Fprintf(&b, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>%s</style>
</head>
<body>
<div class="layout">
<nav>%s</nav>
<main>%s</main>
</div>
</body>
</html>`, html.EscapeString(title), cssStyle, navHTML, bodyHTML)

	return os.WriteFile(path, []byte(b.String()), 0644)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "<", "")
	s = strings.ReplaceAll(s, ">", "")
	return s
}
