package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/firstlayer-xyz/meshio"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/formatter"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"facet/pkg/meshpreview"
	"facet/pkg/render"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: facetc <input.fct> -o <output> [-entry <name>] [-set key=value ...] [-libdir <dir>]\n")
	fmt.Fprintf(os.Stderr, "       facetc <input.fct> -ast [-libdir <dir>]\n")
	fmt.Fprintf(os.Stderr, "       facetc <input.fct> -fmt [-w]\n")
	fmt.Fprintf(os.Stderr, "       facetc <input.stl|.obj|.3mf> -o <output.png>\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -entry <name>    Entry point function (default: Main)\n")
	fmt.Fprintf(os.Stderr, "  -set key=value   Override a parameter (repeatable)\n")
	fmt.Fprintf(os.Stderr, "  -libdir <dir>    Library search directory\n")
	fmt.Fprintf(os.Stderr, "  -size <px>       Image size for .png/.jpg output (default: 1024)\n")
	fmt.Fprintf(os.Stderr, "  -format <ext>    Output format, overriding the output file's extension\n")
	fmt.Fprintf(os.Stderr, "  -ast             Dump parsed AST as JSON\n")
	fmt.Fprintf(os.Stderr, "  -fmt             Format source code\n")
	fmt.Fprintf(os.Stderr, "  -w               Write formatted output back to file (with -fmt)\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  facetc model.fct -o model.3mf\n")
	fmt.Fprintf(os.Stderr, "  facetc model.fct -o model.stl -entry Bracket -set radius=12 -set height=30\n")
	fmt.Fprintf(os.Stderr, "  facetc model.3mf -o preview.png\n")
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
	var format string
	overrides := map[string]interface{}{}

	args := os.Args[1:]
	i := 0
	next := func() string {
		i++
		if i >= len(args) {
			usage()
		}
		return args[i]
	}
	for ; i < len(args); i++ {
		switch args[i] {
		case "-o":
			output = next()
		case "-entry":
			entry = next()
		case "-set":
			k, v, ok := strings.Cut(next(), "=")
			if !ok {
				fmt.Fprintf(os.Stderr, "error: -set requires key=value, got %q\n", args[i])
				os.Exit(1)
			}
			overrides[k] = parseValue(v)
		case "-libdir":
			libDir = next()
		case "-size":
			n, err := strconv.Atoi(next())
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "error: -size requires a positive integer, got %q\n", args[i])
				os.Exit(1)
			}
			size = n
		case "-format":
			format = next()
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

	// Mesh inputs (.stl/.obj/.3mf) are loaded as raw triangles and rendered to
	// an image — no Facet evaluation, image output only.
	if meshio.CanRead(input) {
		if doFmt || dumpAST {
			fmt.Fprintf(os.Stderr, "error: -fmt/-ast cannot be used with a mesh file\n")
			os.Exit(1)
		}
		ext := resolveExt(output, format)
		if ext == "" {
			fmt.Fprintf(os.Stderr, "error: output needs an extension or -format (e.g. .png, .jpg)\n")
			os.Exit(1)
		}
		switch ext {
		case ".png", ".jpg", ".jpeg":
			if err := renderMeshFile(input, output, ext, size); err != nil {
				fmt.Fprintf(os.Stderr, "render error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(output)
			return
		default:
			fmt.Fprintf(os.Stderr, "error: mesh inputs render to images only (.png/.jpg); got %q\n", ext)
			os.Exit(1)
		}
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

	// The output format comes from -format when given (so callers like the Linux
	// thumbnailer can write to a path without a .png extension), else the output
	// file's extension.
	ext := resolveExt(output, format)
	if ext == "" {
		fmt.Fprintf(os.Stderr, "error: output needs an extension or -format (e.g. .stl, .obj, .3mf, .png)\n")
		os.Exit(1)
	}

	if entry == "" {
		entry = "Main"
	}

	result, err := evaluator.Eval(ctx, prog, input, overrides, entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval error: %v\n", err)
		os.Exit(1)
	}

	// An Animation entry has no static solids; export a single-frame snapshot
	// rather than silently writing an empty mesh.
	if result.Animation != nil {
		fmt.Fprintf(os.Stderr, "note: %s returns an Animation; exporting a single frame at the current time\n", entry)
	}
	solids, err := result.StaticSolids(context.Background(), float64(time.Now().UnixMilli()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval error: %v\n", err)
		os.Exit(1)
	}

	switch ext {
	case ".3mf":
		err = manifold.Export3MFMulti(solids, output, nil)
	case ".stl":
		err = manifold.ExportSTLMulti(solids, output)
	case ".obj":
		err = manifold.ExportOBJMulti(solids, output)
	case ".png", ".jpg", ".jpeg":
		dm := manifold.MergeExtractExpandedMeshes(solids, manifold.DefaultDisplayEdgeThresholdDeg)
		err = renderImage(dm.ExpandedPositions(), dm.ExpandedColors(), output, ext, size)
	default:
		err = fmt.Errorf("unsupported export format %q (supported: .3mf, .stl, .obj, .png)", ext)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "export error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(output)
}

// resolveExt returns the lowercased output extension (with leading dot) from
// -format when given, else from the output file's extension. Returns "" when
// neither yields an extension.
func resolveExt(output, format string) string {
	if format != "" {
		return "." + strings.TrimPrefix(strings.ToLower(format), ".")
	}
	return strings.ToLower(filepath.Ext(output))
}

// renderMeshFile loads a mesh file (.stl/.obj/.3mf) and rasterizes it to a
// square PNG/JPEG preview. Mesh inputs only support image output.
func renderMeshFile(input, output, ext string, size int) error {
	positions, colors, err := meshpreview.LoadColored(input)
	if err != nil {
		return err
	}
	return renderImage(positions, colors, output, ext, size)
}

// renderImage rasterizes expanded positions (+ optional per-vertex colors) to a
// square PNG or JPEG. PNG keeps the transparent background; JPEG (no alpha) is
// composited onto white.
func renderImage(positions []float32, colors []byte, output, ext string, size int) error {
	if size <= 0 {
		size = 1024
	}
	img := render.Mesh(positions, colors, size, size)

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

// compositeOver alpha-composites src over an opaque dst. src.Pix is
// alpha-premultiplied (render.Mesh's box-filter downsample averages opaque
// covered subpixels with transparent black), so the over operator is
// out = src + dst*(1-a), not src*a + dst*(1-a).
func compositeOver(dst, src *image.RGBA) *image.RGBA {
	for i := 0; i < len(src.Pix); i += 4 {
		ia := 255 - uint32(src.Pix[i+3])
		for c := 0; c < 3; c++ {
			dst.Pix[i+c] = uint8(uint32(src.Pix[i+c]) + uint32(dst.Pix[i+c])*ia/255)
		}
		dst.Pix[i+3] = 0xff
	}
	return dst
}
