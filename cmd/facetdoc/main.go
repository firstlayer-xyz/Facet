// Command facetdoc generates HTML documentation from markdown guides and facet API sources.
//
// Usage:
//
//	facetdoc generate --docs docs/ --libs <libdir> --out site/
package main

import (
	"flag"
	"fmt"
	"os"

	"facet/app/pkg/docgen"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: facetdoc <command>\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  generate    Generate HTML documentation site\n\n")
	fmt.Fprintf(os.Stderr, "Run 'facetdoc generate -help' for details.\n")
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	docsDir := fs.String("docs", "docs", "Directory containing markdown guide files")
	libDir := fs.String("libs", "", "Library directory (default: platform library dir)")
	outDir := fs.String("out", "site", "Output directory for generated HTML")
	fs.Parse(args)

	// libDir defaults to "" — libraries resolve from embedded FS

	fmt.Printf("Building documentation site...\n")
	fmt.Printf("  Guides:    %s\n", *docsDir)
	fmt.Printf("  Libraries: %s\n", *libDir)
	fmt.Printf("  Output:    %s\n", *outDir)

	site, err := docgen.BuildSite(*docsDir, *libDir)
	if err != nil {
		return fmt.Errorf("building site: %w", err)
	}

	fmt.Printf("  Found %d guides, %d API groups\n", len(site.Guides), len(site.APIGroups))

	if err := docgen.GenerateSite(site, *outDir); err != nil {
		return fmt.Errorf("generating site: %w", err)
	}

	fmt.Printf("Done. Open %s/index.html in a browser.\n", *outDir)
	return nil
}
