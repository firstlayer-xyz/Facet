package manifold

import (
	"math"
	"testing"
)

// checkEuler verifies V-E+F=2 for a convex polyhedron.
func checkEuler(t *testing.T, name string, pm *PolyMesh) {
	t.Helper()
	V := pm.numVerts()
	F := len(pm.Faces)
	E := len(pm.edges())
	euler := V - E + F
	if euler != 2 {
		t.Errorf("%s: Euler V-E+F = %d-%d+%d = %d, want 2", name, V, E, F, euler)
	}
}

func TestPlatonicSolids(t *testing.T) {
	tests := []struct {
		name      string
		mesh      *PolyMesh
		verts     int
		edges     int
		faces     int
		unitRadius bool // true if circumradius should be 1.0
	}{
		{"Tetrahedron", newTetrahedron(), 4, 6, 4, true},
		{"Octahedron", newOctahedron(), 6, 12, 8, true},
		{"Cube", newPlatoCube(), 8, 12, 6, true},
		{"Icosahedron", newIcosahedron(), 12, 30, 20, true},
		{"Dodecahedron", newDodecahedron(), 20, 30, 12, false}, // dual doesn't preserve circumradius
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mesh.numVerts(); got != tt.verts {
				t.Errorf("vertices: got %d, want %d", got, tt.verts)
			}
			if got := len(tt.mesh.edges()); got != tt.edges {
				t.Errorf("edges: got %d, want %d", got, tt.edges)
			}
			if got := len(tt.mesh.Faces); got != tt.faces {
				t.Errorf("faces: got %d, want %d", got, tt.faces)
			}
			checkEuler(t, tt.name, tt.mesh)

			if tt.unitRadius {
				cr := tt.mesh.circumradius()
				if math.Abs(cr-1.0) > 1e-10 {
					t.Errorf("circumradius: got %f, want 1.0", cr)
				}
			}
		})
	}
}

func TestConwayDual(t *testing.T) {
	// Dual of cube = octahedron (8 faces → 8 verts, 6 faces)
	cube := newPlatoCube()
	dual := cube.Dual()
	if got := dual.numVerts(); got != 6 {
		t.Errorf("dual(cube) verts: got %d, want 6", got)
	}
	if got := len(dual.Faces); got != 8 {
		t.Errorf("dual(cube) faces: got %d, want 8", got)
	}
	checkEuler(t, "Dual(Cube)", dual)
}

func TestConwayAmbo(t *testing.T) {
	// Ambo of cube = cuboctahedron (12V, 24E, 14F)
	cube := newPlatoCube()
	ambo := cube.Ambo()
	checkEuler(t, "Ambo(Cube)", ambo)
	if got := ambo.numVerts(); got != 12 {
		t.Errorf("ambo(cube) verts: got %d, want 12", got)
	}
}

func TestConwayKis(t *testing.T) {
	// Kis of cube: 6 quads → 24 tris, 8+6=14 verts
	cube := newPlatoCube()
	kis := cube.Kis()
	checkEuler(t, "Kis(Cube)", kis)
	if got := len(kis.Faces); got != 24 {
		t.Errorf("kis(cube) faces: got %d, want 24", got)
	}
}

func TestConwayTruncate(t *testing.T) {
	// Truncated cube: 14 faces (6 octagonal + 8 triangular)
	cube := newPlatoCube()
	trunc := cube.Truncate()
	checkEuler(t, "Truncate(Cube)", trunc)
	if got := len(trunc.Faces); got != 14 {
		t.Errorf("truncate(cube) faces: got %d, want 14", got)
	}
}

func TestConwayExpand(t *testing.T) {
	cube := newPlatoCube()
	exp := cube.Expand()
	checkEuler(t, "Expand(Cube)", exp)
}

func TestConwaySnub(t *testing.T) {
	cube := newPlatoCube()
	snub := cube.Snub()
	checkEuler(t, "Snub(Cube)", snub)
}

func TestCanonicalize(t *testing.T) {
	cube := newPlatoCube()
	canon := cube.Canonicalize(200, 1e-6)
	checkEuler(t, "Canonicalize(Cube)", canon)

	// After canonicalization, all edge midpoints should be at similar
	// distances from origin (tangent to a sphere)
	ef := canon.edgeFaces()
	var sumR float64
	for e := range ef {
		mx, my, mz := canon.edgeMidpoint(e)
		sumR += math.Sqrt(mx*mx + my*my + mz*mz)
	}
	avgR := sumR / float64(len(ef))
	for e := range ef {
		mx, my, mz := canon.edgeMidpoint(e)
		r := math.Sqrt(mx*mx + my*my + mz*mz)
		if math.Abs(r-avgR)/avgR > 0.01 {
			t.Errorf("edge midpoint distance %f differs from avg %f by >1%%", r, avgR)
		}
	}
}

func TestScaleToRadius(t *testing.T) {
	ico := newIcosahedron()
	scaled := ico.ScaleToRadius(20.0)
	cr := scaled.circumradius()
	if math.Abs(cr-20.0) > 1e-10 {
		t.Errorf("ScaleToRadius(20): got circumradius %f", cr)
	}
}

func TestToSolidRoundTrip(t *testing.T) {
	// Create a PolyMesh, convert to Solid, extract back, verify structure
	cube := newPlatoCube().ScaleToRadius(10.0)
	sf, err := cube.ToSolid()
	if err != nil {
		t.Fatalf("ToSolid: %v", err)
	}
	solid, err := sf.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Volume should be positive
	vol := solid.Volume()
	if vol <= 0 {
		t.Errorf("volume: got %f, want positive", vol)
	}

	// Extract back to PolyMesh
	extracted := ExtractPolyMesh(solid)
	if extracted.numVerts() < 8 {
		t.Errorf("extracted verts: got %d, want >= 8", extracted.numVerts())
	}
	if len(extracted.Faces) < 6 {
		t.Errorf("extracted faces: got %d, want >= 6", len(extracted.Faces))
	}
}

func TestToDisplayMesh(t *testing.T) {
	cube := newPlatoCube().ScaleToRadius(10.0)
	dm := cube.ToDisplayMesh()
	if dm.VertexCount == 0 {
		t.Error("ToDisplayMesh: no vertices")
	}
	if dm.IndexCount == 0 {
		t.Error("ToDisplayMesh: no indices")
	}
	if dm.FaceGroupCount == 0 {
		t.Error("ToDisplayMesh: no face groups")
	}
}
