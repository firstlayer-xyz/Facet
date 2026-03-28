package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/formatter"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: facetc <input.fct> -o <output> [-entry <name>] [-set key=value ...] [-libdir <dir>]\n")
	fmt.Fprintf(os.Stderr, "       facetc <input.fct> -ast [-libdir <dir>]\n")
	fmt.Fprintf(os.Stderr, "       facetc <input.fct> -fmt [-w]\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -entry <name>    Entry point function (default: Main)\n")
	fmt.Fprintf(os.Stderr, "  -set key=value   Override a parameter (repeatable)\n")
	fmt.Fprintf(os.Stderr, "  -libdir <dir>    Library search directory\n")
	fmt.Fprintf(os.Stderr, "  -ast             Dump parsed AST as JSON\n")
	fmt.Fprintf(os.Stderr, "  -fmt             Format source code\n")
	fmt.Fprintf(os.Stderr, "  -w               Write formatted output back to file (with -fmt)\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  facetc model.fct -o model.3mf\n")
	fmt.Fprintf(os.Stderr, "  facetc model.fct -o model.stl -entry Bracket -set radius=12 -set height=30\n")
	os.Exit(1)
}

// parseValue converts a string to a typed value: number, bool, or string.
func parseValue(s string) interface{} {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return s
}

func main() {
	var input, output, libDir, entry string
	var dumpAST, doFmt, fmtWrite bool
	overrides := map[string]interface{}{}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) {
				usage()
			}
			i++
			output = args[i]
		case "-entry":
			if i+1 >= len(args) {
				usage()
			}
			i++
			entry = args[i]
		case "-set":
			if i+1 >= len(args) {
				usage()
			}
			i++
			k, v, ok := strings.Cut(args[i], "=")
			if !ok {
				fmt.Fprintf(os.Stderr, "error: -set requires key=value, got %q\n", args[i])
				os.Exit(1)
			}
			overrides[k] = parseValue(v)
		case "-libdir":
			if i+1 >= len(args) {
				usage()
			}
			i++
			libDir = args[i]
		case "-ast":
			dumpAST = true
		case "-fmt":
			doFmt = true
		case "-w":
			fmtWrite = true
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

	if input == "" || (!dumpAST && !doFmt && output == "") {
		usage()
	}

	source, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Format mode: format and print (or write back with -w)
	if doFmt {
		src, err := parser.Parse(string(source), "", parser.SourceUser)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}
		formatted := formatter.Format(src)
		if fmtWrite {
			if err := os.WriteFile(input, []byte(formatted), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Print(formatted)
		}
		return
	}

	ctx := context.Background()

	prog, err := loader.Load(ctx, string(source), input, parser.SourceUser, libDir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if errs := checker.Check(prog).Errors; len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "type error: %v\n", errs[0])
		os.Exit(1)
	}

	if dumpAST {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(prog); err != nil {
			fmt.Fprintf(os.Stderr, "json error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	ext := filepath.Ext(output)
	if ext == "" {
		fmt.Fprintf(os.Stderr, "error: output file must have an extension (e.g. .stl, .obj, .3mf, .glb)\n")
		os.Exit(1)
	}

	if entry == "" {
		entry = "Main"
	}

	var overridesMap map[string]interface{}
	if len(overrides) > 0 {
		overridesMap = overrides
	}

	result, err := evaluator.Eval(ctx, prog, input, overridesMap, entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval error: %v\n", err)
		os.Exit(1)
	}

	// Use Go-native writers for 3MF/STL/OBJ; assimp for other formats.
	switch strings.ToLower(ext) {
	case ".3mf":
		err = manifold.Export3MFMulti(result.Solids, output)
	case ".stl":
		err = manifold.ExportSTLMulti(result.Solids, output)
	case ".obj":
		err = manifold.ExportOBJMulti(result.Solids, output)
	default:
		err = manifold.ExportMeshes(result.Solids, output)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "export error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(output)
}
