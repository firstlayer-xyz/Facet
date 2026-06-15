package main

import (
	"path/filepath"
	"strings"
	"testing"

	"facet/pkg/facet3mf"

	"github.com/firstlayer-xyz/meshio"
)

func write3MF(t *testing.T, atts []meshio.Attachment) string {
	t.Helper()
	m := &meshio.Mesh{
		Vertices:    []float32{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0},
		Indices:     []uint32{0, 1, 2, 0, 2, 3},
		Attachments: atts,
	}
	path := filepath.Join(t.TempDir(), "m.3mf")
	if err := m.Write3MF(path); err != nil {
		t.Fatalf("Write3MF: %v", err)
	}
	return path
}

func TestReadOpenedFile_FacetProject(t *testing.T) {
	att, err := facet3mf.Marshal(facet3mf.Project{
		Version: facet3mf.Version, Entry: "Main",
		Overrides: map[string]interface{}{"width": 40.0},
		Source:    "fn Main() Solid {\n\treturn Cube(size: 10)\n}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	of, err := readOpenedFile(write3MF(t, []meshio.Attachment{att}))
	if err != nil {
		t.Fatalf("readOpenedFile: %v", err)
	}
	if !of.Imported {
		t.Fatal("recovered project should open as a new unsaved (imported) tab")
	}
	if of.Entry != "Main" || of.Overrides["width"] != 40.0 ||
		!strings.Contains(of.Source, "Cube(size: 10)") {
		t.Fatalf("project not recovered: %+v", of)
	}
}

func TestReadOpenedFile_PlainMeshGeneratesWrapper(t *testing.T) {
	of, err := readOpenedFile(write3MF(t, nil))
	if err != nil {
		t.Fatalf("readOpenedFile: %v", err)
	}
	if !of.Imported || !strings.Contains(of.Source, "LoadMesh(") {
		t.Fatalf("plain 3mf should import via LoadMesh wrapper: %+v", of)
	}
}

func TestReadOpenedFile_STLGeneratesWrapper(t *testing.T) {
	of, err := readOpenedFile("/tmp/does-not-exist.stl")
	if err != nil {
		t.Fatalf("readOpenedFile: %v", err)
	}
	if !of.Imported || !strings.Contains(of.Source, `LoadMesh(path: "/tmp/does-not-exist.stl")`) {
		t.Fatalf("stl wrapper wrong: %+v", of)
	}
}

func TestReadOpenedFile_CorruptFacetPartErrors(t *testing.T) {
	bad := meshio.Attachment{Path: facet3mf.PartPath, ContentType: facet3mf.ContentType, Data: []byte("{bad")}
	if _, err := readOpenedFile(write3MF(t, []meshio.Attachment{bad})); err == nil {
		t.Fatal("expected error on corrupt Facet part")
	}
}
