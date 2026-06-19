package evaluator

import (
	"context"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"testing"
)

// warpBenchSrc warps a high-segment sphere with a callback modeled on the
// "Fuzzy Bear" example: the lambda references globals (FuzzyHash, Sqrt, Number)
// and captures locals (rNum, amt), so it exercises the full callFunctionVal
// path — scope construction, captured copy, arg coercion — once per vertex.
const warpBenchSrc = `
fn FuzzyHash(x, y, z Number) Number {
    var v = Sin(a: (x * 127.1 + y * 311.7 + z * 74.7) deg) * 43758.5453
    return v - Floor(n: v)
}

fn Main() Solid {
    var r = 30 mm;
    var rNum = Number(from: r);
    var amt = 0.5;
    return Sphere(r: r, segments: 160).Warp(f: fn(p Vec3) Vec3 {
        var x = Number(from: p.x)
        var y = Number(from: p.y)
        var z = Number(from: p.z)
        var ox = x - rNum
        var oy = y - rNum
        var oz = z - rNum
        var len = Sqrt(n: ox * ox + oy * oy + oz * oz)
        var ux = ox / len
        var uy = oy / len
        var uz = oz / len
        var h = FuzzyHash(x: x * 10, y: y * 10, z: z * 10)
        var disp = h * amt
        return Vec3{
            x: (x + ux * disp) mm,
            y: (y + uy * disp) mm,
            z: (z + uz * disp) mm,
        }
    });
}
`

func parseBenchProg(b *testing.B, src string) loader.Program {
	b.Helper()
	s, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	prog := testStdlibLibs()
	prog.Sources[testMainKey] = s
	return prog
}

func BenchmarkWarp(b *testing.B) {
	prog := parseBenchProg(b, warpBenchSrc)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Eval(ctx, prog, testMainKey, nil, "Main"); err != nil {
			b.Fatalf("eval: %v", err)
		}
	}
}
