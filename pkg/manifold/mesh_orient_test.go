package manifold

import "testing"

// A Solid is a bounded, outward-facing region. Like extrude and the kernel
// primitives, a hand-built Mesh must orient itself outward (positive volume)
// regardless of the input triangle winding — it must not be possible to produce
// an inside-out solid. Both windings of the same tetrahedron must yield the same
// positive volume.
func TestCreateSolidFromMeshOrientsOutward(t *testing.T) {
	// Tetra A=(0,0,0) B=(10,0,0) C=(0,10,0) D=(0,0,10); |volume| = 166.67.
	verts := []float32{0, 0, 0, 10, 0, 0, 0, 10, 0, 0, 0, 10}
	ccw := []uint32{0, 2, 1, 0, 1, 3, 0, 3, 2, 1, 2, 3} // outward (CCW-from-outside)
	cw := []uint32{0, 1, 2, 0, 3, 1, 0, 2, 3, 1, 3, 2}  // inside-out (each triangle reversed)

	for _, tc := range []struct {
		name string
		idx  []uint32
	}{{"ccw", ccw}, {"cw", cw}} {
		s, err := CreateSolidFromMesh(verts, tc.idx)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if v := s.Volume(); v < 166 || v > 167 {
			t.Errorf("%s: volume = %v, want ~+166.67 (oriented outward regardless of input winding)", tc.name, v)
		}
	}
}
