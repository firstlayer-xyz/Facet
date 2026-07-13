//go:build !js

package manifold

import "testing"

func TestSolidMemSize(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if cube.memSize == 0 {
		t.Fatal("expected positive memory size for cube")
	}
	t.Logf("cube memSize = %d bytes", cube.memSize)
}

func TestSketchMemSize(t *testing.T) {
	sq, err := CreateSquare(10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if sq.memSize == 0 {
		t.Fatal("expected positive memory size for square sketch")
	}
	t.Logf("square memSize = %d bytes", sq.memSize)
}
