package evaluator

import (
	"context"
	"facet/app/pkg/fctlang/loader"
	"os"
	"strings"
	"testing"
)

func TestEvalLibraryCall(t *testing.T) {
	// Create a temp library directory with a test library
	libDir := t.TempDir()
	libPath := libDir + "/test/mylib"
	if err := os.MkdirAll(libPath, 0755); err != nil {
		t.Fatal(err)
	}
	libSrc := `
fn MakeBox(size Length) {
    return Cube(size: Vec3{x: size, y: size, z: size});
}
`
	if err := os.WriteFile(libPath+"/mylib.fct", []byte(libSrc), 0644); err != nil {
		t.Fatal(err)
	}

	src := `
var MyLib = lib "test/mylib";

fn Main() {
    return MyLib.MakeBox(size: 10 mm);
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

func TestEvalLibraryInvalidPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"single segment", "foo", "must have at least 2 segments"},
		{"dot-dot", "foo/../bar", "'..' is not allowed"},
		{"absolute path", "/etc/passwd", "absolute paths are not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `var L = lib "` + tt.path + `";
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`
			prog := parseTestProg(t, src)
			err := loader.ResolveLibraries(context.Background(), prog, testMainKey, t.TempDir(), nil)
			if err == nil {
				t.Fatal("expected error for invalid library path")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q should contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestEvalLibraryNotFound(t *testing.T) {
	src := `
var MyLib = lib "nonexistent/lib";

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	err := loader.ResolveLibraries(context.Background(), prog, testMainKey, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for nonexistent library")
	}
	if !strings.Contains(err.Error(), "nonexistent/lib") {
		t.Errorf("error should mention library path: %v", err)
	}
}
