package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/formatter"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"facet/pkg/render"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: facetc <input.fct> -o <output> [-entry <name>] [-set key=value ...] [-libdir <dir>]\n")
	fmt.Fprintf(os.Stderr, "       facetc <input.fct> -ast [-libdir <dir>]\n")
	fmt.Fprintf(os.Stderr, "       facetc <input.fct> -fmt [-w]\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -entry <name>    Entry point function (default: Main)\n")
	fmt.Fprintf(os.Stderr, "  -set key=value   Override a parameter (repeatable)\n")
	fmt.Fprintf(os.Stderr, "  -libdir <dir>    Library search directory\n")
	fmt.Fprintf(os.Stderr, "  -size <px>       Image size for .png/.jpg output (default: 1024)\n")
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
	var size int
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
		case "-size":
			if i+1 >= len(args) {
				usage()
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "error: -size requires a positive integer, got %q\n", args[i])
				os.Exit(1)
			}
			size = n
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

	// An Animation entry has no static solids; export a single-frame snapshot
	// rather than silently writing an empty mesh.
	if result.Animation != nil {
		fmt.Fprintf(os.Stderr, "note: %s returns an Animation; exporting a single frame at the current time\n", entry)
	}
	solids, err := result.StaticSolids(float64(time.Now().UnixMilli()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval error: %v\n", err)
		os.Exit(1)
	}

	switch strings.ToLower(ext) {
	case ".3mf":
		err = manifold.Export3MFMulti(solids, output)
	case ".stl":
		err = manifold.ExportSTLMulti(solids, output)
	case ".obj":
		err = manifold.ExportOBJMulti(solids, output)
	case ".png", ".jpg", ".jpeg":
		err = renderImage(solids, output, strings.ToLower(ext), size)
	default:
		err = fmt.Errorf("unsupported export format %q (supported: .3mf, .stl, .obj, .png)", ext)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "export error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(output)
}

// renderImage rasterizes the solids to a square PNG or JPEG preview. PNG keeps
// the transparent background; JPEG (no alpha) is composited onto white.
func renderImage(solids []*manifold.Solid, output, ext string, size int) error {
	if size <= 0 {
		size = 1024
	}
	dm := manifold.MergeExtractExpandedMeshes(solids, 40)
	img := render.Mesh(expandedFloats(dm.ExpandedRaw), size, size)

	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	if ext == ".png" {
		return png.Encode(f, img)
	}
	// JPEG has no alpha: flatten onto white so the background isn't black.
	bg := image.NewRGBA(img.Bounds())
	for i := range bg.Pix {
		bg.Pix[i] = 0xff
	}
	bg = compositeOver(bg, img)
	return jpeg.Encode(f, bg, &jpeg.Options{Quality: 90})
}

// expandedFloats decodes the kernel's little-endian float32 position buffer.
func expandedFloats(raw []byte) []float32 {
	out := make([]float32, len(raw)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return out
}

// compositeOver alpha-composites src over dst (premultiplied by src alpha).
func compositeOver(dst, src *image.RGBA) *image.RGBA {
	for i := 0; i < len(src.Pix); i += 4 {
		a := uint32(src.Pix[i+3])
		ia := 255 - a
		for c := 0; c < 3; c++ {
			dst.Pix[i+c] = uint8((uint32(src.Pix[i+c])*a + uint32(dst.Pix[i+c])*ia) / 255)
		}
		dst.Pix[i+3] = 0xff
	}
	return dst
}
