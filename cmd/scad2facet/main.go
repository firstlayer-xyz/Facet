// Command scad2facet transpiles OpenSCAD source into idiomatic Facet source.
//
// The emitted Facet is round-tripped through Facet's own parser and formatter,
// so the output is always canonically formatted (or the command fails loudly).
// If any OpenSCAD construct cannot be faithfully translated, the command exits
// with a non-zero status and prints every untranslatable construct with its
// source location — no partial or placeholder output is produced.
package main

import (
	"fmt"
	"os"
	"strings"

	"facet/pkg/scad"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: scad2facet <input.scad> [-o <output.fct>]\n")
	fmt.Fprintf(os.Stderr, "\nTranspile OpenSCAD source into idiomatic Facet source.\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -o <file>   Write Facet output to file (default: stdout)\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  scad2facet part.scad -o part.fct\n")
	fmt.Fprintf(os.Stderr, "  scad2facet part.scad > part.fct\n")
	os.Exit(1)
}

func main() {
	var input, output string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) {
				usage()
			}
			i++
			output = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				usage()
			}
			if input != "" {
				usage()
			}
			input = args[i]
		}
	}

	if input == "" {
		usage()
	}

	source, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	result, err := scad.Transpile(string(source), input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if output == "" {
		fmt.Print(result.Facet)
		return
	}
	if err := os.WriteFile(output, []byte(result.Facet), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(output)
}
