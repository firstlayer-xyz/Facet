package manifold

import "testing"

func TestSolidExternalMemSize(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	n := cube.ExternalMemSize()
	if n <= 0 {
		t.Fatalf("expected positive memory size for cube, got %d", n)
	}
	t.Logf("cube ExternalMemSize = %d bytes", n)
}

func TestSketchExternalMemSize(t *testing.T) {
	sq := CreateSquare(10, 10)
	n := sq.ExternalMemSize()
	if n <= 0 {
		t.Fatalf("expected positive memory size for square sketch, got %d", n)
	}
	t.Logf("square ExternalMemSize = %d bytes", n)
}

// Verify that Solid and Sketch implement the ExternalMemory interface.
var _ ExternalMemory = (*Solid)(nil)
var _ ExternalMemory = (*Sketch)(nil)
