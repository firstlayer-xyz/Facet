package evaluator

import (
	"context"
	"encoding/binary"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/manifold"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalCube(t *testing.T) {
	src := `fn Main() Solid { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
	if len(mesh.Indices) == 0 {
		t.Error("expected non-empty indices")
	}
}

func TestEvalCubeWithUnits(t *testing.T) {
	// 1 ft = 304.8 mm, so Cube(size: {1 ft, 1 ft, 1 ft}) should produce a mesh
	src := `fn Main() Solid { return Cube(size: Vec3{x: 1 ft, y: 1 ft, z: 1 ft}); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMultiFunction(t *testing.T) {
	src := `
fn MyShape() Solid {
    return Sphere(radius: 5 mm);
}

fn Main() Solid {
    return MyShape();
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalCustomEntryPoint(t *testing.T) {
	src := `
fn MyCube() Solid {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)

	// Should fail with default entry point (no Main function)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error when no Main() exists")
	}

	// Should succeed with custom entry point
	result, err := Eval(context.Background(), prog, testMainKey, nil, "MyCube")
	if err != nil {
		t.Fatalf("eval error with custom entry point: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Fatal("expected at least one solid")
	}
}

func TestEvalCustomEntryPointWithParams(t *testing.T) {
	src := `
fn MyBox(w Length = 20 where [10:100], h Length = 30 where [10:100]) Solid {
    return Cube(size: Vec3{x: w, y: w, z: h});
}
`
	prog := parseTestProg(t, src)

	// With defaults
	result, err := Eval(context.Background(), prog, testMainKey, nil, "MyBox")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Fatal("expected at least one solid")
	}

	// With overrides
	overrides := map[string]interface{}{"w": 50.0, "h": 80.0}
	result2, err := Eval(context.Background(), prog, testMainKey, overrides, "MyBox")
	if err != nil {
		t.Fatalf("eval error with overrides: %v", err)
	}
	if len(result2.Solids) == 0 {
		t.Fatal("expected at least one solid with overrides")
	}
}

func TestEvalVariables(t *testing.T) {
	src := `
fn Main() Solid {
    var size = 10 mm;
    return Cube(size: Vec3{x: size, y: size, z: size});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalVariableFromCall(t *testing.T) {
	src := `
fn Main() Solid {
    var box = Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    return box;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalLengthReturnType(t *testing.T) {
	src := `
fn HalfInch() Length {
    return 1/2 in;
}

fn Main() Solid {
    return Cube(size: Vec3{x: HalfInch(), y: HalfInch(), z: HalfInch()});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalLengthParam(t *testing.T) {
	src := `
fn Box(size Length) Solid {
    return Cube(size: Vec3{x: size, y: size, z: size});
}

fn Main() Solid {
    return Box(size: 10 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}


func TestEvalCancelledContext(t *testing.T) {
	src := `fn Main() Solid { return Cube(size: Vec3{x: 10, y: 10, z: 10}); }`
	prog := parseTestProg(t, src)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := Eval(ctx, prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestEvalInferredReturnType(t *testing.T) {
	src := `fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalInferredReturnTypeUserFunc(t *testing.T) {
	src := `
fn HalfInch() {
    return 1/2 in;
}

fn Main() {
    return Cube(size: Vec3{x: HalfInch(), y: HalfInch(), z: HalfInch()});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalGlobalVar(t *testing.T) {
	src := `
var size = 10 mm;

fn Main() {
    return Cube(size: Vec3{x: size, y: size, z: size});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalGlobalVarInUserFunc(t *testing.T) {
	src := `
var size = 5 mm;

fn Box() {
    return Cube(size: Vec3{x: size, y: size, z: size});
}

fn Main() {
    return Box();
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalBareNumberAsLength(t *testing.T) {
	// Bare numbers default to mm, so Cube(size: {10, 10, 10}) should work
	src := `fn Main() Solid { return Cube(size: Vec3{x: 10, y: 10, z: 10}); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalVec2(t *testing.T) {
	src := `
fn Main() {
    var p = Vec2{x: 5 mm, y: 10 mm};
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalVec3(t *testing.T) {
	src := `
fn Main() {
    var p = Vec3{x: 5 mm, y: 10 mm, z: 15 mm};
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalHullVec3(t *testing.T) {
	src := `
fn Main() {
    var pts = []Vec3[
        {x: 0 mm, y: 0 mm, z: 0 mm},
        {x: 10 mm, y: 0 mm, z: 0 mm},
        {x: 0 mm, y: 10 mm, z: 0 mm},
        {x: 0 mm, y: 0 mm, z: 10 mm}
    ];
    return Hull(arr: pts);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices from hull")
	}
}

func TestEvalThreadedBolt(t *testing.T) {
	src := `
fn Main() {
    var head = Cylinder(bottom: 2.75 mm, top: 2.75 mm, height: 3 mm);
    var sr = 1.44 mm;
    var socket = Polygon(points: for i[0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * sr, y: Sin(a: i * 60 deg) * sr};
    }).Extrude(height: 2 mm);

    var shaft_len = 4 mm;
    var pitch = 0.5 mm;
    var major_r = 1.5 mm;
    var minor_r = 1.23 mm;

    var hp = pitch / 4;
    var tooth = Polygon(points: [
        Vec2{x: minor_r, y: -hp},
        Vec2{x: major_r, y: 0 mm},
        Vec2{x: minor_r, y: hp}
    ]);

    var thread = tooth.Extrude(height: shaft_len, slices: 80, twist: 2880 deg, taperX: 1, taperY: 1)
        .Translate(v: Vec3 { x: 0 mm, y: 0 mm, z: -shaft_len / 2 });
    var core = Cylinder(bottom: minor_r, top: minor_r, height: shaft_len);
    var shaft = (core + thread).Translate(v: Vec3 { x: 0 mm, y: 0 mm, z: -3.5 mm });

    return (head + shaft) - socket;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalOptionalParam(t *testing.T) {
	src := `
fn Box(size Length, height Length = 5 mm) Solid {
    return Cube(size: Vec3{x: size, y: size, z: height});
}

fn Main() {
    return Box(size: 10 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalOptionalParamOverride(t *testing.T) {
	src := `
fn Box(size Length, height Length = 5 mm) Solid {
    return Cube(size: Vec3{x: size, y: size, z: height});
}

fn Main() {
    return Box(size: 10 mm, height: 20 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalSphereDefaultSegments(t *testing.T) {
	src := `fn Main() { return Sphere(radius: 10 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalCircleDefaultSegments(t *testing.T) {
	src := `fn Main() { return Circle(radius: 5 mm).Extrude(height: 10 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalCylinderDefaultSegments(t *testing.T) {
	src := `fn Main() { return Cylinder(bottom: 5 mm, top: 5 mm, height: 10 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalRevolveDefaultSegments(t *testing.T) {
	src := `fn Main() { return Circle(radius: 3 mm).Translate(v: Vec2 { x: 10 mm, y: 0 mm }).Revolve(); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func threadResolveOpts() *loader.Options {
	localFacetlibs, _ := filepath.Abs(filepath.Join("..", "..", "..", "..", "facetlibs"))
	return &loader.Options{
		InstalledLibs: map[string]string{
			"github.com/firstlayer-xyz/facetlibs": localFacetlibs,
		},
	}
}

func TestEvalThreadStructAPI(t *testing.T) {
	src := `
var T = lib "github.com/firstlayer-xyz/facetlibs/threads@main";

fn Main() {
    return T.Thread(size: "m3").Outside(length: 2 mm);
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSAEThread(t *testing.T) {
	src := `
var T = lib "github.com/firstlayer-xyz/facetlibs/threads@main";

fn Main() {
    return T.Thread(size: "1/4-20").Outside(length: 5 mm);
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSAEFineThread(t *testing.T) {
	src := `
var T = lib "github.com/firstlayer-xyz/facetlibs/threads@main";

fn Main() {
    return T.Thread(size: "1/4-28").Outside(length: 5 mm);
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalNPTThread(t *testing.T) {
	src := `
var T = lib "github.com/firstlayer-xyz/facetlibs/threads@main";

fn Main() {
    return T.Thread(size: "1/4-npt").Outside(length: 10 mm);
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalStructBasic(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
    d Length;
}

fn Main() {
    var box = Dims { w: 10 mm, h: 20 mm, d: 5 mm };
    return Cube(size: Vec3{x: box.w, y: box.h, z: box.d});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalStructMethod(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
    d Length;
}

fn Dims.ToCube() Solid {
    return Cube(size: Vec3{x: self.w, y: self.h, z: self.d});
}

fn Main() {
    var box = Dims { w: 10 mm, h: 20 mm, d: 5 mm };
    return box.ToCube();
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalStructTypeCheck(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
}

fn MakeBox(dims Dims) Solid {
    return Cube(size: Vec3{x: dims.w, y: dims.h, z: 10 mm});
}

fn Main() {
    var d = Dims { w: 5 mm, h: 8 mm };
    return MakeBox(dims: d);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalStructFieldError(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
}

fn Main() {
    var d = Dims { w: 5 mm, h: 8 mm };
    return Cube(size: Vec3{x: d.z, y: d.h, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for accessing nonexistent field")
	}
	if !strings.Contains(err.Error(), "no field") {
		t.Errorf("error should mention 'no field': %v", err)
	}
}

func TestEvalLoadMeshSTL(t *testing.T) {
	// Write a minimal binary STL cube to a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "cube.stl")

	type vec3 struct{ x, y, z float32 }
	type tri struct {
		normal         vec3
		v1, v2, v3     vec3
	}

	triangles := []tri{
		// Front (z=1, normal +z)
		{vec3{0, 0, 1}, vec3{0, 0, 1}, vec3{1, 0, 1}, vec3{1, 1, 1}},
		{vec3{0, 0, 1}, vec3{0, 0, 1}, vec3{1, 1, 1}, vec3{0, 1, 1}},
		// Back (z=0, normal -z)
		{vec3{0, 0, -1}, vec3{1, 0, 0}, vec3{0, 0, 0}, vec3{0, 1, 0}},
		{vec3{0, 0, -1}, vec3{1, 0, 0}, vec3{0, 1, 0}, vec3{1, 1, 0}},
		// Top (y=1, normal +y)
		{vec3{0, 1, 0}, vec3{0, 1, 0}, vec3{0, 1, 1}, vec3{1, 1, 1}},
		{vec3{0, 1, 0}, vec3{0, 1, 0}, vec3{1, 1, 1}, vec3{1, 1, 0}},
		// Bottom (y=0, normal -y)
		{vec3{0, -1, 0}, vec3{0, 0, 0}, vec3{1, 0, 0}, vec3{1, 0, 1}},
		{vec3{0, -1, 0}, vec3{0, 0, 0}, vec3{1, 0, 1}, vec3{0, 0, 1}},
		// Right (x=1, normal +x)
		{vec3{1, 0, 0}, vec3{1, 0, 0}, vec3{1, 1, 0}, vec3{1, 1, 1}},
		{vec3{1, 0, 0}, vec3{1, 0, 0}, vec3{1, 1, 1}, vec3{1, 0, 1}},
		// Left (x=0, normal -x)
		{vec3{-1, 0, 0}, vec3{0, 0, 0}, vec3{0, 0, 1}, vec3{0, 1, 1}},
		{vec3{-1, 0, 0}, vec3{0, 0, 0}, vec3{0, 1, 1}, vec3{0, 1, 0}},
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 80)
	f.Write(header)
	binary.Write(f, binary.LittleEndian, uint32(len(triangles)))
	for _, tri := range triangles {
		for _, v := range []vec3{tri.normal, tri.v1, tri.v2, tri.v3} {
			binary.Write(f, binary.LittleEndian, v.x)
			binary.Write(f, binary.LittleEndian, v.y)
			binary.Write(f, binary.LittleEndian, v.z)
		}
		binary.Write(f, binary.LittleEndian, uint16(0))
	}
	f.Close()

	src := fmt.Sprintf(`fn Main() Solid { return LoadMesh(path: "%s"); }`, path)
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalLoadMeshOBJ(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tetra.obj")

	content := `v 0 0 0
v 10 0 0
v 5 10 0
v 5 5 10
f 1 3 2
f 1 2 4
f 2 3 4
f 1 4 3
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf(`fn Main() Solid { return LoadMesh(path: "%s"); }`, path)
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalLoadMeshFileNotFound(t *testing.T) {
	src := `fn Main() Solid { return LoadMesh(path: "/tmp/nonexistent_model.stl"); }`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("error should mention file not found: %v", err)
	}
}

func TestEvalRangeInclusive(t *testing.T) {
	// [0:6] → inclusive → [0, 1, 2, 3, 4, 5, 6] (7 elements)
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i[0:6] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
    }).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeStep(t *testing.T) {
	// [0:10:2] → inclusive step 2 → [0, 2, 4, 6, 8, 10] (6 elements)
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i[0:10:2] {
        yield Vec2{x: Cos(a: i * 36 deg) * r, y: Sin(a: i * 36 deg) * r};
    }).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeExclusiveStep(t *testing.T) {
	// [0:<10:2] → exclusive step 2 → [0, 2, 4, 6, 8] (5 elements)
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i[0:<10:2] {
        yield Vec2{x: Cos(a: i * 72 deg) * r, y: Sin(a: i * 72 deg) * r};
    }).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeNegativeStep(t *testing.T) {
	// [6:0:-1] → inclusive countdown → [6, 5, 4, 3, 2, 1, 0] (7 elements)
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i[6:0:-1] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
    }).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeNegativeExclusive(t *testing.T) {
	// [6:>0:-1] → exclusive countdown → [6, 5, 4, 3, 2, 1] (6 elements)
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i[6:>0:-1] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
    }).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeAutoCountdown(t *testing.T) {
	// [6:0] → auto-infers step=-1 since start > end → [6, 5, 4, 3, 2, 1, 0]
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i[6:0] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
    }).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeFractionalStep(t *testing.T) {
	// [0:4:1/2] → 0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4 (9 elements)
	src := `
fn Main() {
    var pts = for i[0:4:1/2] {
        yield Vec2{x: i * 5 mm, y: 0 mm};
    };
    return Polygon(points: [
        Vec2{x: 0 mm, y: 0 mm},
        Vec2{x: 20 mm, y: 0 mm},
        Vec2{x: 10 mm, y: 10 mm}
    ]).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeWithLengths(t *testing.T) {
	// [0 in : < 1 ft : 50 mm] — length range with mixed units
	src := `
fn Main() {
    var pts = for i[0 in : < 1 ft : 50 mm] {
        yield Vec2{x: i, y: i};
    };
    return Polygon(points: [
        Vec2{x: 0 mm, y: 0 mm},
        Vec2{x: 100 mm, y: 0 mm},
        Vec2{x: 100 mm, y: 100 mm}
    ]).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeLengthStep(t *testing.T) {
	// [0 mm : < 10 mm : 2 mm] — 5 elements: 0, 2, 4, 6, 8
	src := `
fn Main() {
    var pts = for i[0 mm : < 10 mm : 2 mm] {
        yield Vec2{x: i, y: 0 mm};
    };
    return Polygon(points: [
        Vec2{x: 0 mm, y: 0 mm},
        Vec2{x: 10 mm, y: 0 mm},
        Vec2{x: 5 mm, y: 10 mm}
    ]).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeMixedUnits(t *testing.T) {
	// [0 in : 1 cm : 1 mm] — inclusive range with mixed units
	src := `
fn Main() {
    var pts = for i[0 in : 1 cm : 1 mm] {
        yield Vec2{x: i, y: 0 mm};
    };
    return Polygon(points: [
        Vec2{x: 0 mm, y: 0 mm},
        Vec2{x: 10 mm, y: 0 mm},
        Vec2{x: 5 mm, y: 10 mm}
    ]).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalRangeLengthNoSpace(t *testing.T) {
	// [0mm:<10mm:2mm] — no-space variant
	src := `
fn Main() {
    var pts = for i[0mm:<10mm:2mm] {
        yield Vec2{x: i, y: 0 mm};
    };
    return Polygon(points: [
        Vec2{x: 0 mm, y: 0 mm},
        Vec2{x: 10 mm, y: 0 mm},
        Vec2{x: 5 mm, y: 10 mm}
    ]).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalFunUnits(t *testing.T) {
	// A cube that's 1 smoot x 1 cubit x 1 hand
	src := `fn Main() { return Cube(size: Vec3{x: 1 smoot, y: 1 cubit, z: 1 hand}); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalFunUnitConversions(t *testing.T) {
	// 8 furlongs = 1 mile
	src := `
fn Main() {
    if 8 furlong == 1 mi {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("8 furlongs should equal 1 mile: %v", err)
	}
}

func TestEvalFunUnitFathom(t *testing.T) {
	// 1 fathom = 6 ft
	src := `
fn Main() {
    if 1 fathom == 6 ft {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("1 fathom should equal 6 ft: %v", err)
	}
}

func TestEvalFunUnitLeague(t *testing.T) {
	// 1 league = 3 mi
	src := `
fn Main() {
    if 1 league == 3 mi {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("1 league should equal 3 miles: %v", err)
	}
}

func TestEvalConstantPI(t *testing.T) {
	src := `
fn Main() {
    if PI * 2 == TAU {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("PI * 2 should equal TAU: %v", err)
	}
}

func TestEvalConstantUnicode(t *testing.T) {
	src := `
fn Main() {
    if π == PI {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("π should equal PI: %v", err)
	}
}

func TestEvalConstantE(t *testing.T) {
	src := `
fn Main() {
    if E > 2.718 && E < 2.719 {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("E should be approximately 2.718: %v", err)
	}
}

func TestEvalSketchFillet(t *testing.T) {
	src := `
fn Main() {
    var sq = Square(x: 20 mm, y: 20 mm);
    var filleted = sq.Fillet(radius: 2 mm);
    var originalArea = sq.Area();
    var filletedArea = filleted.Area();
    if filletedArea < originalArea {
        return filleted.Extrude(height: 5 mm);
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh (filleted area should be less than original)")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSketchChamfer(t *testing.T) {
	src := `
fn Main() {
    var sq = Square(x: 20 mm, y: 20 mm);
    var chamfered = sq.Chamfer(distance: 2 mm);
    var originalArea = sq.Area();
    var chamferedArea = chamfered.Area();
    if chamferedArea < originalArea {
        return chamfered.Extrude(height: 5 mm);
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh (chamfered area should be less than original)")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSolidLinearPattern(t *testing.T) {
	src := `
fn Main() {
    return Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).LinearPattern(count: 3, spacingX: 10 mm, spacingY: 0 mm, spacingZ: 0 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSolidCircularPattern(t *testing.T) {
	src := `
fn Main() {
    return Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Translate(v: Vec3 { x: 10 mm, y: 0 mm, z: 0 mm }).CircularPattern(count: 6);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSolidCircularPatternPartial(t *testing.T) {
	src := `
fn Main() {
    return Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Translate(v: Vec3 { x: 10 mm, y: 0 mm, z: 0 mm }).CircularPattern(count: 3, span: 180 deg);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSketchLinearPattern(t *testing.T) {
	src := `
fn Main() {
    return Square(x: 5 mm, y: 5 mm).LinearPattern(count: 3, spacingX: 10 mm, spacingY: 0 mm).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSketchCircularPattern(t *testing.T) {
	src := `
fn Main() {
    return Square(x: 5 mm, y: 5 mm).Translate(v: Vec2 { x: 10 mm, y: 0 mm }).CircularPattern(count: 4).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSketchFilletCustomSegments(t *testing.T) {
	src := `
fn Main() {
    var sq = Square(x: 20 mm, y: 20 mm);
    var filleted = sq.Fillet(radius: 2 mm);
    return filleted.Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalAsin(t *testing.T) {
	src := `
fn Main() {
    if Sin(a: Asin(n: 1)) == 1 {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("Sin(a: Asin(n: 1)) should equal 1: %v", err)
	}
}

func TestEvalAcos(t *testing.T) {
	src := `
fn Main() {
    # Cos(Acos(0)) should equal 0 (tests near-zero epsilon)
    if Cos(a: Acos(n: 0)) == 0 {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("Cos(Acos(0)) should equal 0: %v", err)
	}
}

func TestEvalAtan2(t *testing.T) {
	src := `
fn Main() {
    if Atan2(y: 1, x: 1) == 45 deg {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("Atan2(1, 1) should equal 45 deg: %v", err)
	}
}

func TestEvalAsinDomainError(t *testing.T) {
	src := `fn Main() { var a = Asin(n: 2); return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for Asin(2) out of domain")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error should mention out of range: %v", err)
	}
}

func TestEvalAngleDivAngle(t *testing.T) {
	src := `
fn Main() {
    if 90 deg / 45 deg == 2 {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("90 deg / 45 deg should equal 2: %v", err)
	}
}

func TestEvalImplicitReturn(t *testing.T) {
	src := `fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}


func TestEvalImplicitReturnIfElse(t *testing.T) {
	src := `
fn Main() {
    if true {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Sphere(radius: 5 mm);
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalImplicitYieldForYield(t *testing.T) {
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i [0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
    }).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalImplicitReturnFold(t *testing.T) {
	src := `
fn Main() {
    var cubes = for i [0:<4] {
        yield Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Translate(v: Vec3 { x: i * 10 mm, y: 0 mm, z: 0 mm });
    };
    return fold a, b cubes {
        yield a + b;
    };
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalImplicitReturnMethodChain(t *testing.T) {
	src := `
fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
        .Translate(v: Vec3 { x: 5 mm, y: 0 mm, z: 0 mm })
        .Rotate(rx: 0 deg, ry: 0 deg, rz: 45 deg, pivot: WorldOrigin);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalBoundingBox(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(size: Vec3{x: 10 mm, y: 20 mm, z: 30 mm}).Bounds();
    # Cube spans from origin to (x, y, z)
    assert box.min.x == 0 mm, "min.x should be 0";
    assert box.max.x == 10 mm, "max.x should be 10";
    assert box.min.y == 0 mm, "min.y should be 0";
    assert box.max.y == 20 mm, "max.y should be 20";
    assert box.min.z == 0 mm, "min.z should be 0";
    assert box.max.z == 30 mm, "max.z should be 30";
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalBoxMethods(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(size: Vec3{x: 10 mm, y: 20 mm, z: 30 mm}).Bounds();
    assert box.Width() == 10 mm, "width should be 10";
    assert box.Height() == 30 mm, "height should be 30 (Z extent)";
    assert box.Depth() == 20 mm, "depth should be 20 (Y extent)";
    var c = box.Center();
    assert c.x == 5 mm, "center x should be 5";
    assert c.y == 10 mm, "center y should be 10";
    assert c.z == 15 mm, "center z should be 15";
    assert box.ContainsPoint(p: Vec3{x: 5 mm, y: 10 mm, z: 15 mm}), "center should be inside";
    assert !(box.ContainsPoint(p: Vec3{x: 100 mm, y: 0 mm, z: 0 mm})), "far point should be outside";
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalBoxOverlapsAndUnion(t *testing.T) {
	src := `
fn Main() {
    var a = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Bounds();
    var b = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Translate(v: Vec3 { x: 3 mm, y: 0 mm, z: 0 mm }).Bounds();
    assert a.Overlaps(other: b), "boxes should overlap";
    var u = a.Union(other: b);
    assert u.min.x == 0 mm, "union min.x should be 0";
    assert u.max.x == 13 mm, "union max.x should be 13";
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalVec3FieldAccess(t *testing.T) {
	src := `
fn Main() {
    var p = Vec3{x: 1 mm, y: 2 mm, z: 3 mm};
    assert p.x == 1 mm, "x should be 1";
    assert p.y == 2 mm, "y should be 2";
    assert p.z == 3 mm, "z should be 3";
    return Cube(size: Vec3{x: p.x, y: p.y, z: p.z});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalVec2FieldAccess(t *testing.T) {
	src := `
fn Main() {
    var p = Vec2{x: 5 mm, y: 10 mm};
    assert p.x == 5 mm, "x should be 5";
    assert p.y == 10 mm, "y should be 10";
    return Cube(size: Vec3{x: p.x, y: p.y, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalKnurlCylinder(t *testing.T) {
	src := `
var K = lib "github.com/firstlayer-xyz/facetlibs/knurling@main";

fn Main() {
    var knurl = K.Knurl(count: 20, depth: 0.5 mm, angle: 30 deg);
    var cyl = Cylinder(bottom: 5 mm, top: 5 mm, height: 20 mm);
    return knurl.Apply(target: cyl);
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalKnurlSphere(t *testing.T) {
	src := `
var K = lib "github.com/firstlayer-xyz/facetlibs/knurling@main";

fn Main() {
    var knurl = K.Knurl(count: 16, depth: 0.5 mm, angle: 30 deg);
    var sph = Sphere(radius: 5 mm);
    return knurl.Apply(target: sph);
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMainReturnsArray(t *testing.T) {
	src := `
fn Main() {
    return []Solid[Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}), Sphere(radius: 8 mm), Cylinder(bottom: 5 mm, top: 15 mm, height: 5 mm)];
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMainReturnsArrayWithReturnType(t *testing.T) {
	src := `
fn Main() []Solid {
    return []Solid[Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}), Sphere(radius: 5 mm)];
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMainReturnsEmptyArray(t *testing.T) {
	src := `
fn Main() {
    return []Solid[];
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for empty array")
	}
	if !strings.Contains(err.Error(), "empty array") {
		t.Errorf("error should mention empty array: %v", err)
	}
}

func TestEvalMainReturnsArrayWithNonSolid(t *testing.T) {
	src := `
fn Main() {
    return []Solid[Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}), 42];
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for non-Solid element in array")
	}
	if !strings.Contains(err.Error(), "must be Solid") {
		t.Errorf("error should mention must be Solid: %v", err)
	}
}

func TestEvalMainReturnsSingleElementArray(t *testing.T) {
	src := `
fn Main() {
    return []Solid[Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}),];
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

// --- Struct field defaults ---

func TestEvalStructFieldDefaults(t *testing.T) {
	src := `
type Config {
    count Number = 10;
    depth Length = 5 mm;
    angle Angle = 45 deg;
}

fn Main() {
    var c = Config {};
    assert c.count == 10, "default count";
    assert c.depth == 5 mm, "default depth";
    assert c.angle == 45 deg, "default angle";
    return Cube(size: Vec3{x: c.depth, y: c.depth, z: c.depth});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStructFieldConstraintValid(t *testing.T) {
	src := `
type Config {
    count Number = 5 where [1:10]
    size Length = 10 mm where [1:100] mm
}

fn Main() {
    var c = Config { count: 3, size: 50 mm };
    return Cube(size: Vec3{x: c.size, y: c.size, z: c.size});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalStructFieldConstraintViolation(t *testing.T) {
	src := `
type Config {
    count Number = 5 where [1:10]
}

fn Main() {
    var c = Config { count: 20 };
    return Cube(size: 10 mm);
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected constraint violation error")
	}
	if !strings.Contains(err.Error(), "out of range") && !strings.Contains(err.Error(), "constraint") {
		t.Fatalf("expected constraint error, got: %v", err)
	}
}

func TestEvalStructFieldConstraintOnAssignment(t *testing.T) {
	src := `
type Config {
    count Number = 5 where [1:10]
}

fn Main() {
    var c = Config {};
    c.count = 20;
    return Cube(size: 10 mm);
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected constraint violation error on field assignment")
	}
}

func TestEvalStructFieldDefaultPartialOverride(t *testing.T) {
	src := `
type Config {
    count Number = 10;
    depth Length = 5 mm;
}

fn Main() {
    var c = Config { count: 99 };
    assert c.count == 99, "overridden count";
    assert c.depth == 5 mm, "default depth";
    return Cube(size: Vec3{x: c.depth, y: c.depth, z: c.depth});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStructFieldZeroValue(t *testing.T) {
	// Anonymous struct coerced to named type: missing fields get zero values
	src := `
type Config {
    count Number;
    depth Length;
    enabled Bool;
}

fn MakeBox(c Config) Solid {
    var size = 10 mm;
    if c.count > 0 { size = c.depth; }
    return Cube(size: Vec3{x: size, y: size, z: size});
}

fn Main() {
    return MakeBox(c: { count: 5, depth: 8 mm });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

// --- Anonymous struct literals ---

func TestEvalAnonymousStructBasic(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
    d Length;
}

fn MakeBox(dims Dims) Solid {
    return Cube(size: Vec3{x: dims.w, y: dims.h, z: dims.d});
}

fn Main() {
    return MakeBox(dims: { w: 10 mm, h: 20 mm, d: 5 mm });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalAnonymousStructWrongField(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
}

fn MakeBox(dims Dims) Solid {
    return Cube(size: Vec3{x: dims.w, y: dims.h, z: 10 mm});
}

fn Main() {
    return MakeBox(dims: { w: 10 mm, z: 5 mm });
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for unknown field in anonymous struct")
	}
	if !strings.Contains(err.Error(), "not in Dims") {
		t.Errorf("error should mention field not in struct: %v", err)
	}
}

func TestEvalAnonymousStructWrongType(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
}

fn MakeBox(dims Dims) Solid {
    return Cube(size: Vec3{x: dims.w, y: dims.h, z: 10 mm});
}

fn Main() {
    return MakeBox(dims: { w: true, h: 5 mm });
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for wrong field type in anonymous struct")
	}
	if !strings.Contains(err.Error(), "must be Length") {
		t.Errorf("error should mention type mismatch: %v", err)
	}
}

func TestEvalAnonymousStructMissingFieldUsesDefault(t *testing.T) {
	src := `
type Config {
    count Number = 40;
    depth Length;
    pitch_angle Angle = 30 deg;
}

fn MakeShape(c Config) Solid {
    assert c.count == 40, "default count";
    assert c.pitch_angle == 30 deg, "default pitch_angle";
    return Cube(size: Vec3{x: c.depth, y: c.depth, z: c.depth});
}

fn Main() {
    return MakeShape(c: { depth: 10 mm });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalAnonymousStructMethodCall(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
    d Length;
}

fn Dims.ToCube() Solid {
    return Cube(size: Vec3{x: self.w, y: self.h, z: self.d});
}

fn MakeBox(dims Dims) Solid {
    return dims.ToCube();
}

fn Main() {
    return MakeBox(dims: { w: 10 mm, h: 20 mm, d: 5 mm });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalAnonymousStructLibraryCall(t *testing.T) {
	// Test anonymous struct passed to a library function
	libDir := t.TempDir()
	libPath := libDir + "/test/mylib"
	if err := os.MkdirAll(libPath, 0755); err != nil {
		t.Fatal(err)
	}
	libSrc := `
type Opts {
    size Length;
    count Number = 3;
}

fn MakeThing(opts Opts) Solid {
    return Cube(size: Vec3{x: opts.size, y: opts.size, z: opts.size});
}
`
	if err := os.WriteFile(libPath+"/mylib.fct", []byte(libSrc), 0644); err != nil {
		t.Fatal(err)
	}

	src := `
var L = lib "test/mylib";

fn Main() {
    return L.MakeThing(opts: { size: 15 mm });
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, libDir, &loader.Options{})
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalAnonymousStructMethodParam(t *testing.T) {
	// Anonymous struct passed as a parameter to a user-defined struct method
	src := `
type Config {
    size Length;
    count Number;
}

type Builder {
    base Length;
}

fn Builder.Build(cfg Config) Solid {
    return Cube(size: Vec3{x: self.base + cfg.size, y: self.base, z: self.base});
}

fn Main() {
    var b = Builder { base: 5 mm };
    return b.Build(cfg: { size: 10 mm, count: 3 });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMeshRoundtrip(t *testing.T) {
	src := `
fn Main() {
    var cube = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var vol1 = cube.Volume();
    var m = cube.Mesh();
    var rebuilt = m.Solid();
    var vol2 = rebuilt.Volume();
    assert vol1 == vol2, "volumes should match after roundtrip";
    return rebuilt;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMeshFaceNormals(t *testing.T) {
	src := `
fn Main() {
    var cube = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var m = cube.Mesh();
    var normals = m.FaceNormals();
    # A cube has 12 triangles (2 per face)
    assert Size(of: normals) == Size(of: m.indices), "one normal per face";
    return cube;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalMeshVertexNormals(t *testing.T) {
	src := `
fn Main() {
    var cube = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var m = cube.Mesh();
    var vn = m.VertexNormals();
    assert Size(of: vn) == Size(of: m.vertices), "one normal per vertex";
    return cube;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalAnonymousStructTransitiveLibrary(t *testing.T) {
	// Anonymous struct duck-typed to a type defined in a transitive library.
	// lib "test/inner" defines struct Opts; lib "test/outer" loads inner and
	// exposes a function that takes Opts. User passes an anonymous struct.
	libDir := t.TempDir()

	// Inner library defines the struct and a method on it
	innerPath := libDir + "/test/inner"
	if err := os.MkdirAll(innerPath, 0755); err != nil {
		t.Fatal(err)
	}
	innerSrc := `
type Opts {
    width Length;
    height Length;
    count Number = 4;
}

fn Opts.Build() Solid {
    return Cube(size: Vec3{x: self.width, y: self.height, z: self.width});
}
`
	if err := os.WriteFile(innerPath+"/inner.fct", []byte(innerSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Outer library loads inner and uses Opts as a parameter type
	outerPath := libDir + "/test/outer"
	if err := os.MkdirAll(outerPath, 0755); err != nil {
		t.Fatal(err)
	}
	outerSrc := `
var I = lib "test/inner";

fn MakeFromOpts(opts I.Opts) Solid {
    return opts.Build();
}
`
	if err := os.WriteFile(outerPath+"/outer.fct", []byte(outerSrc), 0644); err != nil {
		t.Fatal(err)
	}

	src := `
var O = lib "test/outer";

fn Main() {
    return O.MakeFromOpts(opts: { width: 10 mm, height: 5 mm });
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, libDir, &loader.Options{})
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalStructNamespaceCollision(t *testing.T) {
	// Two libraries define a struct with the same bare name "Config".
	// Functions use qualified types (A.Config, B.Config) to disambiguate.
	// The evaluator should resolve the correct declaration for each.
	libDir := t.TempDir()

	// Library A defines Config with a "width" field
	libAPath := libDir + "/test/liba"
	if err := os.MkdirAll(libAPath, 0755); err != nil {
		t.Fatal(err)
	}
	libASrc := `
type Config {
    width Length;
}

fn Build(c Config) Solid {
    return Cube(size: Vec3{x: c.width, y: c.width, z: c.width});
}
`
	if err := os.WriteFile(libAPath+"/liba.fct", []byte(libASrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Library B defines Config with a "height" field
	libBPath := libDir + "/test/libb"
	if err := os.MkdirAll(libBPath, 0755); err != nil {
		t.Fatal(err)
	}
	libBSrc := `
type Config {
    height Length;
}

fn Build(c Config) Solid {
    return Cube(size: Vec3{x: c.height, y: c.height, z: c.height});
}
`
	if err := os.WriteFile(libBPath+"/libb.fct", []byte(libBSrc), 0644); err != nil {
		t.Fatal(err)
	}

	src := `
var A = lib "test/liba";
var B = lib "test/libb";

fn Main() Solid {
    var a = A.Build(c: { width: 10 mm });
    var b = B.Build(c: { height: 20 mm });
    return a + b;
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, libDir, &loader.Options{})
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalLibStructFactoryThenMethod(t *testing.T) {
	// Mirrors threads library pattern: factory function returns named struct,
	// user calls a method on the returned struct.
	// Tests both simple factory and nested if/else factory (like Thread()).
	libDir := t.TempDir()
	libPath := libDir + "/test/mylib"
	if err := os.MkdirAll(libPath, 0755); err != nil {
		t.Fatal(err)
	}
	libSrc := `
type Widget {
    size Length;
    count Number;
}

fn Widget(spec String) Widget {
    if spec == "big" {
        return Widget { size: 20 mm, count: 8 };
    } else {
        return Widget { size: 10 mm, count: 4 };
    }
}

fn Widget.Build() Solid {
    return Cube(size: Vec3{x: self.size, y: self.size, z: self.size});
}
`
	if err := os.WriteFile(libPath+"/mylib.fct", []byte(libSrc), 0644); err != nil {
		t.Fatal(err)
	}

	src := `
var T = lib "test/mylib";

fn Main() Solid {
    var w = T.Widget(spec: "big");
    return w.Build();
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, libDir, &loader.Options{})
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalFieldAssignBasic(t *testing.T) {
	src := `
type Dims {
    w Length = 10 mm;
    h Length = 10 mm;
}

fn Main() {
    var d = Dims {};
    d.h = 2 mm;
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-nil mesh with vertices")
	}
}

func TestEvalFieldAssignCompound(t *testing.T) {
	src := `
type Dims {
    w Length = 10 mm;
    h Length = 10 mm;
}

fn Main() {
    var d = Dims {};
    d.w += 5 mm;
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-nil mesh with vertices")
	}
}

func TestEvalFieldAssignNumberToLength(t *testing.T) {
	// Bare number assigned to Length field should be coerced to mm
	src := `
type Dims {
    w Length = 10 mm;
    h Length = 10 mm;
}

fn Main() {
    var d = Dims {};
    d.h = 2;
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-nil mesh with vertices")
	}
}

func TestEvalFieldAssignTypeMismatch(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{
			name: "bool to length",
			src: `
type Dims { w Length = 10 mm; }
fn Main() { var d = Dims {}; d.w = true; return Cube(size: Vec3{x: d.w, y: d.w, z: d.w}); }
`,
			wantMsg: "cannot assign Bool to field \"w\" of type Length",
		},
		{
			name: "angle to length",
			src: `
type Dims { w Length = 10 mm; }
fn Main() { var d = Dims {}; d.w = 45 deg; return Cube(size: Vec3{x: d.w, y: d.w, z: d.w}); }
`,
			wantMsg: "cannot assign Angle to field \"w\" of type Length",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := parseTestProg(t, tc.src)
			_, err := evalMerged(context.Background(), prog, nil)
			if err == nil {
				t.Fatal("expected error for type mismatch on field assign")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("expected %q, got: %v", tc.wantMsg, err)
			}
		})
	}
}

func TestEvalFieldAssignWrongField(t *testing.T) {
	src := `
type Dims {
    w Length = 10 mm;
}

fn Main() {
    var d = Dims {};
    d.nope = 5 mm;
    return Cube(size: Vec3{x: d.w, y: d.w, z: d.w});
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected error for assignment to nonexistent field")
	}
}

func TestEvalCoercionNumberToLength(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "struct literal field",
			src: `
type Box { w Length; h Length; d Length; }
fn Main() { var b = Box { w: 10, h: 20, d: 30 }; return Cube(size: Vec3{x: b.w, y: b.h, z: b.d}); }
`,
		},
		{
			name: "field assignment",
			src: `
type Box { w Length = 1 mm; h Length = 1 mm; d Length = 1 mm; }
fn Main() { var b = Box {}; b.w = 10; b.h = 20; b.d = 30; return Cube(size: Vec3{x: b.w, y: b.h, z: b.d}); }
`,
		},
		{
			name: "function argument",
			src: `
fn MakeBox(w, h, d Length) Solid { return Cube(size: Vec3{x: w, y: h, z: d}); }
fn Main() { return MakeBox(w: 10, h: 20, d: 30); }
`,
		},
		{
			name: "return value",
			src: `
fn GetLen() Length { return 10; }
fn Main() { return Cube(size: Vec3{x: GetLen(), y: GetLen(), z: GetLen()}); }
`,
		},
		{
			name: "anonymous struct to named",
			src: `
type Dims { w Length; h Length; }
fn MakeBox(d Dims) Solid { return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm}); }
fn Main() { return MakeBox(d: { w: 10, h: 20 }); }
`,
		},
		{
			name: "library struct field assignment",
			src: `
var F = lib "github.com/firstlayer-xyz/facetlibs/fasteners@main";
fn Main() {
    var n = F.HexNut(size: "m8");
    n.nut_h = 2;
    return n.Solid();
}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := parseTestProg(t, tc.src)
			resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if mesh == nil || len(mesh.Vertices) == 0 {
				t.Fatal("expected non-nil mesh with vertices")
			}
		})
	}
}

func TestEvalCoercionNumberToAngle(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "struct literal field",
			src: `
type Rot { a Angle; }
fn Main() { var r = Rot { a: 45 }; return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: 0 deg, ry: 0 deg, rz: r.a, pivot: WorldOrigin); }
`,
		},
		{
			name: "field assignment",
			src: `
type Rot { a Angle = 0 deg; }
fn Main() { var r = Rot {}; r.a = 45; return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: 0 deg, ry: 0 deg, rz: r.a, pivot: WorldOrigin); }
`,
		},
		{
			name: "function argument",
			src: `
fn Spin(a Angle) Solid { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: 0 deg, ry: 0 deg, rz: a, pivot: WorldOrigin); }
fn Main() { return Spin(a: 45); }
`,
		},
		{
			name: "return value",
			src: `
fn GetAngle() Angle { return 45; }
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: 0 deg, ry: 0 deg, rz: GetAngle(), pivot: WorldOrigin); }
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := parseTestProg(t, tc.src)
			mesh, err := evalMerged(context.Background(), prog, nil)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if mesh == nil || len(mesh.Vertices) == 0 {
				t.Fatal("expected non-nil mesh with vertices")
			}
		})
	}
}

func TestEvalCoercionRejectsWrongType(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "bool to length field",
			src:  `type S { w Length; } fn Main() { var s = S { w: true }; return Cube(size: Vec3{x: s.w, y: s.w, z: s.w}); }`,
		},
		{
			name: "string to angle field",
			src:  `type S { a Angle; } fn Main() { var s = S { a: "hello" }; return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`,
		},
		{
			name: "bool to length arg",
			src:  `fn F(x Length) Solid { return Cube(size: Vec3{x: x, y: x, z: x}); } fn Main() { return F(x: true); }`,
		},
		{
			name: "bool to length return",
			src:  `fn F() Length { return true; } fn Main() { return Cube(size: Vec3{x: F(), y: F(), z: F()}); }`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := parseTestProg(t, tc.src)
			_, err := evalMerged(context.Background(), prog, nil)
			if err == nil {
				t.Fatal("expected type error")
			}
		})
	}
}

func TestEvalStructCopyOnAssignment(t *testing.T) {
	// Struct assignment copies — changes to b should NOT affect a
	src := `
type Box { w Length = 10 mm; h Length = 10 mm; d Length = 10 mm; }
fn Main() {
    var a = Box {};
    var b = a;
    b.w = 99 mm;
    assert a.w == 10 mm, "copy-on-assign: a.w must remain 10mm";
    assert b.w == 99 mm, "copy-on-assign: b.w must be 99mm";
    return Cube(size: Vec3{x: a.w, y: a.h, z: a.d});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-nil mesh with vertices")
	}
}

func TestEvalStructLiteralCoercion(t *testing.T) {
	// Struct literal with bare numbers for Length and Angle fields
	src := `
type Config {
    w Length;
    h Length;
    rotation Angle;
}
fn Main() {
    var c = Config { w: 10, h: 20, rotation: 45 };
    return Cube(size: Vec3{x: c.w, y: c.h, z: 10 mm}).Rotate(rx: 0 deg, ry: 0 deg, rz: c.rotation, pivot: WorldOrigin);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-nil mesh with vertices")
	}
}

func TestEvalFastenerTypes(t *testing.T) {
	cases := []struct {
		name string
		expr string
	}{
		{"HexNut", `F.HexNut(size: "m8").Solid()`},
		{"HexBolt", `F.HexBolt(size: "m8", length: 20 mm).Solid()`},
		{"SocketHeadCapScrew", `F.SocketHeadCapScrew(size: "m8", length: 20 mm).Solid()`},
		{"ButtonHeadCapScrew", `F.ButtonHeadCapScrew(size: "m8", length: 20 mm).Solid()`},
		{"CountersunkScrew", `F.CountersunkScrew(size: "m8", length: 20 mm).Solid()`},
		{"SetScrew", `F.SetScrew(size: "m8", length: 20 mm).Solid()`},
		{"Thumbscrew", `F.Thumbscrew(size: "m8", length: 20 mm).Solid()`},
		{"ThumbscrewCustom", `F.Thumbscrew(size: "m8", length: 20 mm, head_h: 6 mm, head_r: 10 mm, knurl: F.Knurl(count: 48, depth: 0.3 mm, angle: 30 deg)).Solid()`},
		{"FlatHeadScrew", `F.FlatHeadScrew(size: "m8", length: 20 mm).Solid()`},
		{"PhillipsScrew", `F.PhillipsScrew(size: "m8", length: 20 mm).Solid()`},
		{"Standoff", `F.Standoff(size: "m8", length: 20 mm).Solid()`},
		{"MaleStandoff", `F.MaleStandoff(size: "m8", length: 20 mm, thread_len: 10 mm).Solid()`},
		{"Showcase", `F.Showcase(size: "m8", shaft_len: 30 mm, standoff_len: 20 mm)[0]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
var F = lib "github.com/firstlayer-xyz/facetlibs/fasteners@main";
fn Main() { return %s; }
`, tc.expr)
			prog := parseTestProg(t, src)
			resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if mesh == nil {
				t.Fatal("expected non-nil mesh")
			}
			if len(mesh.Vertices) == 0 {
				t.Error("expected non-empty vertices")
			}
		})
	}
}

func TestEvalArrayConcat(t *testing.T) {
	// [1,2] + [3,4] → [1,2,3,4]; use Size to verify via cube dimension
	src := `
fn Main() {
    var a = []Number[1,2] + []Number[3,4];
    return Cube(size: Vec3{x: Size(of: a) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	// 4mm wide cube
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalArrayAppendElement(t *testing.T) {
	// [1,2] + 3 → [1,2,3]; use Size to verify
	src := `
fn Main() {
    var a = []Number[1,2] + 3;
    return Cube(size: Vec3{x: Size(of: a) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalArrayCompoundAssign(t *testing.T) {
	// var a = []; a += Vec2; → array with 1 element
	src := `
fn Main() {
    var a = []Vec2[];
    a += Vec2{x: 1 mm, y: 2 mm};
    a += Vec2{x: 3 mm, y: 4 mm};
    return Cube(size: Vec3{x: Size(of: a) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMetricFineThread(t *testing.T) {
	src := `
var T = lib "github.com/firstlayer-xyz/facetlibs/threads@main";
fn Main() { return T.Thread(size: "m8x1").Outside(length: 5 mm); }
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalPosMap(t *testing.T) {
	src := `fn Main() Solid { return Cube(size: 10 mm); }`
	prog := parseTestProg(t, src)
	result, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Fatal("expected solids")
	}
	if len(result.PosMap) == 0 {
		t.Fatal("expected PosMap to be populated")
	}
	for _, entry := range result.PosMap {
		t.Logf("PosMap entry: line=%d col=%d faceIDs=%v", entry.Line, entry.Col, entry.FaceIDs)
		if len(entry.FaceIDs) == 0 {
			t.Error("expected non-empty faceIDs")
		}
	}
}

func TestEvalInferredArrayNumber(t *testing.T) {
	src := `
fn Main() {
    var a = [1, 2, 3];
    return Cube(size: Vec3{x: Size(of: a) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalInferredArrayConcat(t *testing.T) {
	src := `
fn Main() {
    var a = [1, 2] + [3, 4];
    return Cube(size: Vec3{x: Size(of: a) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalInferredArrayForYield(t *testing.T) {
	src := `
fn Main() {
    var cubes = for i [0:<3] {
        yield Cube(size: Vec3{x: (i + 1) mm, y: 1 mm, z: 1 mm});
    };
    return fold a, b cubes { yield a + b; };
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalNegativeIndex(t *testing.T) {
	src := `
fn Main() {
    var a = [1, 2, 3, 4, 5];
    return Cube(size: Vec3{x: a[-1] * 1 mm, y: a[-2] * 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalArraySlice(t *testing.T) {
	src := `
fn Main() {
    var a = [10, 20, 30, 40, 50];
    var sub = a[1:3];
    return Cube(size: Vec3{x: Size(of: sub) * 1 mm, y: sub[0] * 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalArraySliceOpen(t *testing.T) {
	src := `
fn Main() {
    var a = [10, 20, 30, 40, 50];
    var first3 = a[:3];
    var last2 = a[3:];
    return Cube(size: Vec3{x: Size(of: first3) * 1 mm, y: Size(of: last2) * 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalArraySliceNegative(t *testing.T) {
	src := `
fn Main() {
    var a = [10, 20, 30, 40, 50];
    var last3 = a[-3:];
    return Cube(size: Vec3{x: Size(of: last3) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalPosMapTransitiveLibrary(t *testing.T) {
	// Fasteners library imports threads internally.
	// PosMap should contain entries from both the user file AND the fastener library.
	src := `
var F = lib "github.com/firstlayer-xyz/facetlibs/fasteners@main";

fn Main() {
    return F.HexBolt(size: "m8", length: 30 mm).Solid();
}
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", threadResolveOpts())

	result, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	// Collect unique files from posMap with their face counts
	type fileInfo struct {
		entries  int
		faceIDs int
	}
	files := make(map[string]*fileInfo)
	for _, entry := range result.PosMap {
		fi, ok := files[entry.File]
		if !ok {
			fi = &fileInfo{}
			files[entry.File] = fi
		}
		fi.entries++
		fi.faceIDs += len(entry.FaceIDs)
	}

	t.Logf("PosMap has %d entries across %d files:", len(result.PosMap), len(files))
	for f, fi := range files {
		t.Logf("  file: %s — %d entries, %d faceIDs", f, fi.entries, fi.faceIDs)
	}

	// Log detailed entries for threads file
	for _, entry := range result.PosMap {
		if strings.Contains(entry.File, "threads") {
			t.Logf("  threads entry: line=%d col=%d faceIDs=%v", entry.Line, entry.Col, entry.FaceIDs)
		}
	}

	// Should have entries from the main file
	if files[testMainKey] == nil {
		t.Errorf("posMap missing main file %q", testMainKey)
	}

	// Should have entries from threads (transitive via fasteners)
	hasThreads := false
	for f := range files {
		if strings.Contains(f, "threads") {
			hasThreads = true
			break
		}
	}
	if !hasThreads {
		t.Error("posMap has no threads file entries — transitive tracking not working for thread geometry")
	}

	// Check that threads entries actually have face IDs
	for f, fi := range files {
		if strings.Contains(f, "threads") && fi.faceIDs == 0 {
			t.Errorf("threads file has %d entries but 0 faceIDs — geometry not tracked", fi.entries)
		}
	}

	// Check posMap faceIDs exist in the display mesh's face groups
	if len(result.Solids) > 0 {
		dm := manifold.MergeExtractDisplayMeshes(result.Solids)
		t.Logf("Display mesh: %d verts, %d indices, %d faceGroups", dm.VertexCount, dm.IndexCount, dm.FaceGroupCount)
		if dm.FaceGroupCount == 0 {
			t.Error("Display mesh has NO face groups — face-click can't work")
		}
	}

	// Check posMap faceIDs exist in the final solid's FaceMap
	if len(result.Solids) > 0 {
		// Collect all faceIDs from the final solid(s)
		solidFaceIDs := make(map[uint32]bool)
		for _, s := range result.Solids {
			for id := range s.FaceMap {
				solidFaceIDs[id] = true
			}
		}
		t.Logf("Final solid(s) have %d unique face IDs in FaceMap", len(solidFaceIDs))

		// Check which posMap faceIDs are in the final solid
		matched := 0
		missing := 0
		for _, entry := range result.PosMap {
			for _, fid := range entry.FaceIDs {
				if solidFaceIDs[fid] {
					matched++
				} else {
					missing++
				}
			}
		}
		t.Logf("PosMap faceIDs: %d matched final solid, %d missing", matched, missing)
		if missing > 0 {
			t.Logf("WARNING: %d posMap faceIDs don't match the final solid — these faces won't be clickable", missing)
		}
		if matched == 0 {
			t.Errorf("NO posMap faceIDs match the final solid — face IDs are from different ID spaces")
		}
	}
}
