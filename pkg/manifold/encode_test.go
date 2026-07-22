package manifold

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/firstlayer-xyz/meshio"
)

// a single triangle (3 verts, 1 face) is the minimal valid mesh.
var triVerts = []float32{0, 0, 0, 1, 0, 0, 0, 1, 0}
var triIndices = []uint32{0, 1, 2}

func TestEncodeSolidMeshSTLIsBinary(t *testing.T) {
	got, err := EncodeSolidMesh(triVerts, triIndices, nil, "stl", nil)
	if err != nil {
		t.Fatalf("EncodeSolidMesh stl: %v", err)
	}
	// binary STL = 80-byte header + uint32 triangle count + 50 bytes/triangle.
	const want = 80 + 4 + 50
	if len(got) != want {
		t.Fatalf("stl length = %d, want %d", len(got), want)
	}
	count := binary.LittleEndian.Uint32(got[80:84])
	if count != 1 {
		t.Fatalf("stl triangle count = %d, want 1", count)
	}
}

func TestEncodeSolidMesh3MFIsZip(t *testing.T) {
	got, err := EncodeSolidMesh(triVerts, triIndices, []string{"#FF0000"}, "3mf", nil)
	if err != nil {
		t.Fatalf("EncodeSolidMesh 3mf: %v", err)
	}
	// 3MF is a zip container; zip files start with the "PK\x03\x04" local-file magic.
	if !bytes.HasPrefix(got, []byte("PK\x03\x04")) {
		t.Fatalf("3mf does not start with zip magic: % x", got[:4])
	}
}

func TestEncodeSolidMeshEmptyErrors(t *testing.T) {
	if _, err := EncodeSolidMesh(nil, nil, nil, "stl", nil); err == nil {
		t.Fatal("expected error for empty mesh, got nil")
	}
}

func TestEncodeSolidMeshUnknownFormatErrors(t *testing.T) {
	if _, err := EncodeSolidMesh(triVerts, triIndices, nil, "gcode", nil); err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}

func TestEncodeSolidMesh_3MFCarriesAttachment(t *testing.T) {
	verts := []float32{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0}
	indices := []uint32{0, 1, 2, 0, 2, 3}
	atts := []meshio.Attachment{{
		Path: "Metadata/Facet/project.json", ContentType: "application/vnd.facet.project+json", Data: []byte(`{"version":1}`),
	}}
	data, err := EncodeSolidMesh(verts, indices, nil, "3mf", atts)
	if err != nil {
		t.Fatalf("EncodeSolidMesh: %v", err)
	}
	m, err := meshio.Decode(bytes.NewReader(data), "3mf")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Path != "Metadata/Facet/project.json" {
		t.Fatalf("attachment not carried: %+v", m.Attachments)
	}
}

func TestEncodeSolidMesh_AttachmentsRejectedForSTL(t *testing.T) {
	verts := []float32{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0}
	indices := []uint32{0, 1, 2, 0, 2, 3}
	atts := []meshio.Attachment{{Path: "x", ContentType: "y", Data: []byte("z")}}
	if _, err := EncodeSolidMesh(verts, indices, nil, "stl", atts); err == nil {
		t.Fatal("expected error embedding attachments in STL")
	}
}
