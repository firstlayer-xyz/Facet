package manifold

import "testing"

func TestSolidExternalMemSize(t *testing.T) {
	cubeFuture := CreateCube(10, 10, 10)
	cube, err := cubeFuture.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	n := cube.ExternalMemSize()
	if n <= 0 {
		t.Fatalf("expected positive memory size for cube, got %d", n)
	}
	t.Logf("cube ExternalMemSize = %d bytes", n)
}

func TestSketchExternalMemSize(t *testing.T) {
	sqFuture := CreateSquare(10, 10)
	sq, err := sqFuture.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	n := sq.ExternalMemSize()
	if n <= 0 {
		t.Fatalf("expected positive memory size for square sketch, got %d", n)
	}
	t.Logf("square ExternalMemSize = %d bytes", n)
}

// Verify that Solid and Sketch implement the ExternalMemory interface.
var _ ExternalMemory = (*Solid)(nil)
var _ ExternalMemory = (*Sketch)(nil)
