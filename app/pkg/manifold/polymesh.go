package manifold

import (
	"math"
	"sort"
)

// PolyMesh is a polygonal mesh with arbitrary N-gon faces.
// Vertices are stored as flat xyz triples; each face is a list of vertex indices.
type PolyMesh struct {
	Vertices []float64 // flat xyz (len = numVerts * 3)
	Faces    [][]int   // each face = ordered list of vertex indices
}

// edge is a canonical undirected edge (lo < hi).
type edge struct{ a, b int }

func makeEdge(i, j int) edge {
	if i < j {
		return edge{i, j}
	}
	return edge{j, i}
}

// numVerts returns the number of vertices.
func (pm *PolyMesh) numVerts() int { return len(pm.Vertices) / 3 }

// vert returns the xyz coordinates of vertex i.
func (pm *PolyMesh) vert(i int) (float64, float64, float64) {
	return pm.Vertices[i*3], pm.Vertices[i*3+1], pm.Vertices[i*3+2]
}

// setVert sets the xyz coordinates of vertex i.
func (pm *PolyMesh) setVert(i int, x, y, z float64) {
	pm.Vertices[i*3] = x
	pm.Vertices[i*3+1] = y
	pm.Vertices[i*3+2] = z
}

// edgeFaces builds a map from undirected edge → up to 2 adjacent face indices.
func (pm *PolyMesh) edgeFaces() map[edge][2]int {
	ef := make(map[edge][2]int, len(pm.Faces)*3)
	count := make(map[edge]int, len(pm.Faces)*3)
	for fi, face := range pm.Faces {
		n := len(face)
		for i := 0; i < n; i++ {
			e := makeEdge(face[i], face[(i+1)%n])
			c := count[e]
			if c < 2 {
				pair := ef[e]
				pair[c] = fi
				ef[e] = pair
			}
			count[e]++
		}
	}
	return ef
}

// vertFaces builds a map from vertex index → ordered ring of adjacent face indices.
func (pm *PolyMesh) vertFaces() map[int][]int {
	vf := make(map[int][]int, pm.numVerts())
	for fi, face := range pm.Faces {
		for _, vi := range face {
			vf[vi] = append(vf[vi], fi)
		}
	}
	return vf
}

// faceCentroid returns the centroid of face fi.
func (pm *PolyMesh) faceCentroid(fi int) (float64, float64, float64) {
	face := pm.Faces[fi]
	var cx, cy, cz float64
	for _, vi := range face {
		x, y, z := pm.vert(vi)
		cx += x
		cy += y
		cz += z
	}
	n := float64(len(face))
	if n == 0 {
		return 0, 0, 0
	}
	return cx / n, cy / n, cz / n
}

// edgeMidpoint returns the midpoint of an edge.
func (pm *PolyMesh) edgeMidpoint(e edge) (float64, float64, float64) {
	ax, ay, az := pm.vert(e.a)
	bx, by, bz := pm.vert(e.b)
	return (ax + bx) / 2, (ay + by) / 2, (az + bz) / 2
}

// circumradius returns the maximum distance from origin to any vertex.
func (pm *PolyMesh) circumradius() float64 {
	var maxR float64
	nv := pm.numVerts()
	for i := 0; i < nv; i++ {
		x, y, z := pm.vert(i)
		r := math.Sqrt(x*x + y*y + z*z)
		if r > maxR {
			maxR = r
		}
	}
	return maxR
}

// ---------- Platonic solid constructors (unit circumradius, test-only) ----------

// newTetrahedron returns a regular tetrahedron with unit circumradius.
func newTetrahedron() *PolyMesh {
	// Vertices of a regular tetrahedron inscribed in unit sphere
	s := 1.0 / math.Sqrt(3.0)
	return &PolyMesh{
		Vertices: []float64{
			s, s, s,
			s, -s, -s,
			-s, s, -s,
			-s, -s, s,
		},
		Faces: [][]int{
			{0, 1, 2},
			{0, 3, 1},
			{0, 2, 3},
			{1, 3, 2},
		},
	}
}

// newOctahedron returns a regular octahedron with unit circumradius.
func newOctahedron() *PolyMesh {
	return &PolyMesh{
		Vertices: []float64{
			1, 0, 0,
			-1, 0, 0,
			0, 1, 0,
			0, -1, 0,
			0, 0, 1,
			0, 0, -1,
		},
		Faces: [][]int{
			{0, 2, 4},
			{2, 1, 4},
			{1, 3, 4},
			{3, 0, 4},
			{0, 3, 5},
			{3, 1, 5},
			{1, 2, 5},
			{2, 0, 5},
		},
	}
}

// newPlatoCube returns a regular cube with unit circumradius.
func newPlatoCube() *PolyMesh {
	s := 1.0 / math.Sqrt(3.0)
	return &PolyMesh{
		Vertices: []float64{
			-s, -s, -s,
			s, -s, -s,
			s, s, -s,
			-s, s, -s,
			-s, -s, s,
			s, -s, s,
			s, s, s,
			-s, s, s,
		},
		Faces: [][]int{
			{0, 3, 2, 1}, // bottom
			{4, 5, 6, 7}, // top
			{0, 1, 5, 4}, // front
			{2, 3, 7, 6}, // back
			{0, 4, 7, 3}, // left
			{1, 2, 6, 5}, // right
		},
	}
}

// newIcosahedron returns a regular icosahedron with unit circumradius.
func newIcosahedron() *PolyMesh {
	phi := (1.0 + math.Sqrt(5.0)) / 2.0
	r := math.Sqrt(1.0 + phi*phi)
	a := 1.0 / r
	b := phi / r

	return &PolyMesh{
		Vertices: []float64{
			-a, b, 0,
			a, b, 0,
			-a, -b, 0,
			a, -b, 0,
			0, -a, b,
			0, a, b,
			0, -a, -b,
			0, a, -b,
			b, 0, -a,
			b, 0, a,
			-b, 0, -a,
			-b, 0, a,
		},
		Faces: [][]int{
			{0, 11, 5},
			{0, 5, 1},
			{0, 1, 7},
			{0, 7, 10},
			{0, 10, 11},
			{1, 5, 9},
			{5, 11, 4},
			{11, 10, 2},
			{10, 7, 6},
			{7, 1, 8},
			{3, 9, 4},
			{3, 4, 2},
			{3, 2, 6},
			{3, 6, 8},
			{3, 8, 9},
			{4, 9, 5},
			{2, 4, 11},
			{6, 2, 10},
			{8, 6, 7},
			{9, 8, 1},
		},
	}
}

// newDodecahedron returns a regular dodecahedron with unit circumradius.
// Computed as the dual of an icosahedron.
func newDodecahedron() *PolyMesh {
	return newIcosahedron().Dual()
}

// ---------- Conway operators ----------

// Dual returns the dual polyhedron: face centroids become vertices,
// vertex face-rings become faces.
func (pm *PolyMesh) Dual() *PolyMesh {
	nf := len(pm.Faces)
	vf := pm.vertFaces()
	ef := pm.edgeFaces()

	// New vertices = face centroids
	verts := make([]float64, nf*3)
	for fi := 0; fi < nf; fi++ {
		cx, cy, cz := pm.faceCentroid(fi)
		verts[fi*3] = cx
		verts[fi*3+1] = cy
		verts[fi*3+2] = cz
	}

	// New faces = one per original vertex, ordered ring of adjacent faces
	faces := make([][]int, 0, pm.numVerts())
	for vi := 0; vi < pm.numVerts(); vi++ {
		adjFaces := vf[vi]
		if len(adjFaces) < 3 {
			continue
		}
		// Order faces around vertex using edge adjacency
		ordered := orderFaceRing(vi, adjFaces, pm.Faces, ef)
		faces = append(faces, ordered)
	}

	return &PolyMesh{Vertices: verts, Faces: faces}
}

// orderFaceRing orders the faces around a vertex into a consistent ring.
func orderFaceRing(vi int, adjFaces []int, allFaces [][]int, ef map[edge][2]int) []int {
	if len(adjFaces) <= 1 {
		return adjFaces
	}

	// Build adjacency between faces sharing an edge through vi
	faceSet := make(map[int]bool, len(adjFaces))
	for _, fi := range adjFaces {
		faceSet[fi] = true
	}

	// For each face, find which other faces share an edge through vi
	type faceAdj struct {
		neighbors [2]int
		count     int
	}
	adj := make(map[int]*faceAdj, len(adjFaces))
	for _, fi := range adjFaces {
		adj[fi] = &faceAdj{}
	}

	for _, fi := range adjFaces {
		face := allFaces[fi]
		n := len(face)
		for i := 0; i < n; i++ {
			if face[i] != vi {
				continue
			}
			// Edges from vi in this face
			prev := face[(i-1+n)%n]
			next := face[(i+1)%n]
			for _, e := range []edge{makeEdge(vi, prev), makeEdge(vi, next)} {
				pair := ef[e]
				other := pair[0]
				if other == fi {
					other = pair[1]
				}
				if faceSet[other] {
					a := adj[fi]
					if a.count < 2 {
						a.neighbors[a.count] = other
						a.count++
					}
				}
			}
			break
		}
	}

	// Walk the ring
	ordered := make([]int, 0, len(adjFaces))
	ordered = append(ordered, adjFaces[0])
	visited := make(map[int]bool, len(adjFaces))
	visited[adjFaces[0]] = true

	for len(ordered) < len(adjFaces) {
		cur := ordered[len(ordered)-1]
		a := adj[cur]
		found := false
		for i := 0; i < a.count; i++ {
			if !visited[a.neighbors[i]] {
				ordered = append(ordered, a.neighbors[i])
				visited[a.neighbors[i]] = true
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	return ordered
}

// Ambo returns the rectified (ambo) polyhedron: edge midpoints become vertices.
func (pm *PolyMesh) Ambo() *PolyMesh {
	ef := pm.edgeFaces()

	// Assign an index to each edge
	edgeIdx := make(map[edge]int, len(ef))
	edgeList := make([]edge, 0, len(ef))
	for e := range ef {
		edgeIdx[e] = len(edgeList)
		edgeList = append(edgeList, e)
	}

	// New vertices = edge midpoints
	verts := make([]float64, len(edgeList)*3)
	for i, e := range edgeList {
		mx, my, mz := pm.edgeMidpoint(e)
		verts[i*3] = mx
		verts[i*3+1] = my
		verts[i*3+2] = mz
	}

	// New faces: one per original face + one per original vertex
	faces := make([][]int, 0, len(pm.Faces)+pm.numVerts())

	// One face per original face: edges of original face → new vertices
	for _, face := range pm.Faces {
		n := len(face)
		newFace := make([]int, n)
		for i := 0; i < n; i++ {
			e := makeEdge(face[i], face[(i+1)%n])
			newFace[i] = edgeIdx[e]
		}
		faces = append(faces, newFace)
	}

	// One face per original vertex: ring of edges around vertex
	vf := pm.vertFaces()
	for vi := 0; vi < pm.numVerts(); vi++ {
		adjFaces := vf[vi]
		if len(adjFaces) < 3 {
			continue
		}
		ordered := orderFaceRing(vi, adjFaces, pm.Faces, ef)
		newFace := make([]int, len(ordered))
		for i, fi := range ordered {
			// Find the edge between vi and the next vertex in this face's winding
			face := pm.Faces[fi]
			n := len(face)
			for j := 0; j < n; j++ {
				if face[j] == vi {
					// Edge from vi to next vertex in this face
					next := face[(j+1)%n]
					e := makeEdge(vi, next)
					newFace[i] = edgeIdx[e]
					break
				}
			}
		}
		faces = append(faces, newFace)
	}

	return &PolyMesh{Vertices: verts, Faces: faces}
}

// Kis raises a pyramid on each face from its centroid.
func (pm *PolyMesh) Kis() *PolyMesh {
	nv := pm.numVerts()
	nf := len(pm.Faces)

	// Copy original vertices + add face centroids
	verts := make([]float64, (nv+nf)*3)
	copy(verts, pm.Vertices)
	for fi := 0; fi < nf; fi++ {
		cx, cy, cz := pm.faceCentroid(fi)
		idx := nv + fi
		verts[idx*3] = cx
		verts[idx*3+1] = cy
		verts[idx*3+2] = cz
	}

	// Each face splits into triangles from centroid
	faces := make([][]int, 0, nf*4)
	for fi, face := range pm.Faces {
		ci := nv + fi
		n := len(face)
		for i := 0; i < n; i++ {
			faces = append(faces, []int{face[i], face[(i+1)%n], ci})
		}
	}

	return &PolyMesh{Vertices: verts, Faces: faces}
}

// Truncate cuts each vertex, creating a new face per vertex.
func (pm *PolyMesh) Truncate() *PolyMesh {
	nv := pm.numVerts()
	ef := pm.edgeFaces()
	vf := pm.vertFaces()

	// For each original edge, create two new vertices (one near each endpoint).
	// Each new vertex is at 1/3 along the edge from its parent vertex.
	type splitVert struct {
		parentVert int
		edge       edge
	}
	splitIdx := make(map[splitVert]int)
	verts := make([]float64, 0, nv*6) // rough estimate

	addSplitVert := func(vi int, e edge) int {
		key := splitVert{vi, e}
		if idx, ok := splitIdx[key]; ok {
			return idx
		}
		idx := len(verts) / 3
		splitIdx[key] = idx
		// 1/3 from vi toward the other end
		other := e.b
		if e.b == vi {
			other = e.a
		}
		vx, vy, vz := pm.vert(vi)
		ox, oy, oz := pm.vert(other)
		verts = append(verts, vx+(ox-vx)/3, vy+(oy-vy)/3, vz+(oz-vz)/3)
		return idx
	}

	// Build new faces
	faces := make([][]int, 0, len(pm.Faces)+nv)

	// Truncated original faces: each edge replaced by its two split verts
	for _, face := range pm.Faces {
		n := len(face)
		newFace := make([]int, 0, n*2)
		for i := 0; i < n; i++ {
			vi := face[i]
			vj := face[(i+1)%n]
			e := makeEdge(vi, vj)
			newFace = append(newFace, addSplitVert(vi, e))
			newFace = append(newFace, addSplitVert(vj, e))
		}
		faces = append(faces, newFace)
	}

	// New vertex faces: ring of split verts around each original vertex
	for vi := 0; vi < nv; vi++ {
		adjFaces := vf[vi]
		if len(adjFaces) < 3 {
			continue
		}
		ordered := orderFaceRing(vi, adjFaces, pm.Faces, ef)
		newFace := make([]int, len(ordered))
		for i, fi := range ordered {
			face := pm.Faces[fi]
			n := len(face)
			for j := 0; j < n; j++ {
				if face[j] == vi {
					next := face[(j+1)%n]
					e := makeEdge(vi, next)
					newFace[i] = addSplitVert(vi, e)
					break
				}
			}
		}
		faces = append(faces, newFace)
	}

	return &PolyMesh{Vertices: verts, Faces: faces}
}

// Expand returns the expanded (cantellated) polyhedron: Ambo of Ambo.
func (pm *PolyMesh) Expand() *PolyMesh {
	return pm.Ambo().Ambo()
}

// Snub applies the snub operator: a chiral triangulated expansion.
func (pm *PolyMesh) Snub() *PolyMesh {
	nv := pm.numVerts()
	ef := pm.edgeFaces()
	vf := pm.vertFaces()

	// For each face, create a smaller inset face (vertices moved toward centroid).
	// Then triangulate the gaps.
	nf := len(pm.Faces)
	insetFactor := 1.0 / 3.0

	// Count total new vertices: one per face-vertex pair (inset vertex)
	type faceVert struct{ face, vert int }
	insetIdx := make(map[faceVert]int)
	verts := make([]float64, 0)

	addInsetVert := func(fi, vi int) int {
		key := faceVert{fi, vi}
		if idx, ok := insetIdx[key]; ok {
			return idx
		}
		idx := len(verts) / 3
		insetIdx[key] = idx
		cx, cy, cz := pm.faceCentroid(fi)
		vx, vy, vz := pm.vert(vi)
		verts = append(verts,
			vx+(cx-vx)*insetFactor,
			vy+(cy-vy)*insetFactor,
			vz+(cz-vz)*insetFactor)
		return idx
	}

	faces := make([][]int, 0, nf*5)

	// Inset faces (same topology, new verts)
	for fi, face := range pm.Faces {
		newFace := make([]int, len(face))
		for i, vi := range face {
			newFace[i] = addInsetVert(fi, vi)
		}
		faces = append(faces, newFace)
	}

	// Edge triangles: for each edge, two triangles connecting inset verts
	visited := make(map[edge]bool)
	for fi, face := range pm.Faces {
		n := len(face)
		for i := 0; i < n; i++ {
			vi := face[i]
			vj := face[(i+1)%n]
			e := makeEdge(vi, vj)
			if visited[e] {
				continue
			}
			visited[e] = true
			pair := ef[e]
			fi2 := pair[0]
			if fi2 == fi {
				fi2 = pair[1]
			}
			// 4 inset verts: fi:vi, fi:vj, fi2:vi, fi2:vj
			a := addInsetVert(fi, vi)
			b := addInsetVert(fi, vj)
			c := addInsetVert(fi2, vj)
			d := addInsetVert(fi2, vi)
			faces = append(faces, []int{a, b, c})
			faces = append(faces, []int{a, c, d})
		}
	}

	// Vertex faces: for each vertex, create a polygon connecting the inset verts around it
	for vi := 0; vi < nv; vi++ {
		adjFaces := vf[vi]
		if len(adjFaces) < 3 {
			continue
		}
		ordered := orderFaceRing(vi, adjFaces, pm.Faces, ef)
		vertFace := make([]int, len(ordered))
		for i, fi := range ordered {
			vertFace[i] = addInsetVert(fi, vi)
		}
		faces = append(faces, vertFace)
	}

	return &PolyMesh{Vertices: verts, Faces: faces}
}

// ---------- Canonicalize (Hart's algorithm) ----------

// Canonicalize adjusts vertices so all edges are tangent to the unit sphere.
// Uses Hart's iterative algorithm with the given maximum iterations and tolerance.
func (pm *PolyMesh) Canonicalize(maxIter int, tolerance float64) *PolyMesh {
	if maxIter <= 0 {
		maxIter = 200
	}
	if tolerance <= 0 {
		tolerance = 1e-6
	}

	// Deep copy
	verts := make([]float64, len(pm.Vertices))
	copy(verts, pm.Vertices)
	faces := make([][]int, len(pm.Faces))
	for i, f := range pm.Faces {
		faces[i] = make([]int, len(f))
		copy(faces[i], f)
	}

	result := &PolyMesh{Vertices: verts, Faces: faces}

	for iter := 0; iter < maxIter; iter++ {
		// Phase 1: Adjust edges to be tangent to unit sphere
		maxShift := result.canonicalizeStep()
		if maxShift < tolerance {
			break
		}
		// Phase 2: Center and normalize
		result.centerVertices()
	}

	// Final normalization: scale so average edge midpoint distance = 1
	result.normalizeEdgeTangency()

	return result
}

// canonicalizeStep performs one iteration of Hart's canonicalization.
// Returns the maximum vertex displacement.
func (pm *PolyMesh) canonicalizeStep() float64 {
	nv := pm.numVerts()
	// Accumulate adjustments per vertex
	adj := make([]float64, nv*3)
	counts := make([]float64, nv)

	for fi := range pm.Faces {
		face := pm.Faces[fi]
		n := len(face)
		// Newell method for face normal
		var nx, ny, nz float64
		for i := 0; i < n; i++ {
			x1, y1, z1 := pm.vert(face[i])
			x2, y2, z2 := pm.vert(face[(i+1)%n])
			nx += (y1 - y2) * (z1 + z2)
			ny += (z1 - z2) * (x1 + x2)
			nz += (x1 - x2) * (y1 + y2)
		}
		nl := math.Sqrt(nx*nx + ny*ny + nz*nz)
		if nl < 1e-12 {
			continue
		}
		nx /= nl
		ny /= nl
		nz /= nl

		// Face centroid
		fcx, fcy, fcz := pm.faceCentroid(fi)

		// Project each vertex onto the face plane, then push toward plane
		for _, vi := range face {
			vx, vy, vz := pm.vert(vi)
			dx := vx - fcx
			dy := vy - fcy
			dz := vz - fcz
			dist := dx*nx + dy*ny + dz*nz
			adj[vi*3] -= dist * nx * 0.1
			adj[vi*3+1] -= dist * ny * 0.1
			adj[vi*3+2] -= dist * nz * 0.1
			counts[vi]++
		}
	}

	// Apply tangent adjustment: push edge midpoints toward unit sphere
	ef := pm.edgeFaces()
	for e := range ef {
		mx, my, mz := pm.edgeMidpoint(e)
		r := math.Sqrt(mx*mx + my*my + mz*mz)
		if r < 1e-12 {
			continue
		}
		// Push midpoint to unit sphere distance
		scale := (1.0 - r) / r * 0.3
		for _, vi := range []int{e.a, e.b} {
			adj[vi*3] += mx * scale
			adj[vi*3+1] += my * scale
			adj[vi*3+2] += mz * scale
			counts[vi]++
		}
	}

	// Apply adjustments
	var maxShift float64
	for i := 0; i < nv; i++ {
		if counts[i] == 0 {
			continue
		}
		dx := adj[i*3] / counts[i]
		dy := adj[i*3+1] / counts[i]
		dz := adj[i*3+2] / counts[i]
		pm.Vertices[i*3] += dx
		pm.Vertices[i*3+1] += dy
		pm.Vertices[i*3+2] += dz
		shift := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if shift > maxShift {
			maxShift = shift
		}
	}

	return maxShift
}

// centerVertices moves the centroid of all vertices to the origin.
func (pm *PolyMesh) centerVertices() {
	nv := pm.numVerts()
	if nv == 0 {
		return
	}
	var cx, cy, cz float64
	for i := 0; i < nv; i++ {
		x, y, z := pm.vert(i)
		cx += x
		cy += y
		cz += z
	}
	n := float64(nv)
	cx /= n
	cy /= n
	cz /= n
	for i := 0; i < nv; i++ {
		pm.Vertices[i*3] -= cx
		pm.Vertices[i*3+1] -= cy
		pm.Vertices[i*3+2] -= cz
	}
}

// normalizeEdgeTangency scales the mesh so average edge-midpoint distance ≈ 1.
func (pm *PolyMesh) normalizeEdgeTangency() {
	ef := pm.edgeFaces()
	if len(ef) == 0 {
		return
	}
	var sumR float64
	for e := range ef {
		mx, my, mz := pm.edgeMidpoint(e)
		sumR += math.Sqrt(mx*mx + my*my + mz*mz)
	}
	avgR := sumR / float64(len(ef))
	if avgR < 1e-12 {
		return
	}
	scale := 1.0 / avgR
	for i := range pm.Vertices {
		pm.Vertices[i] *= scale
	}
}

// ---------- Utility methods ----------

// ScaleToRadius returns a new PolyMesh scaled so its circumradius equals the given radius.
func (pm *PolyMesh) ScaleToRadius(radius float64) *PolyMesh {
	cr := pm.circumradius()
	if cr < 1e-12 {
		cr = 1.0
	}
	s := radius / cr
	verts := make([]float64, len(pm.Vertices))
	for i, v := range pm.Vertices {
		verts[i] = v * s
	}
	faces := make([][]int, len(pm.Faces))
	for i, f := range pm.Faces {
		faces[i] = make([]int, len(f))
		copy(faces[i], f)
	}
	return &PolyMesh{Vertices: verts, Faces: faces}
}

// ScaleUniform returns a new PolyMesh scaled uniformly by the given factor.
func (pm *PolyMesh) ScaleUniform(factor float64) *PolyMesh {
	verts := make([]float64, len(pm.Vertices))
	for i, v := range pm.Vertices {
		verts[i] = v * factor
	}
	faces := make([][]int, len(pm.Faces))
	for i, f := range pm.Faces {
		faces[i] = make([]int, len(f))
		copy(faces[i], f)
	}
	return &PolyMesh{Vertices: verts, Faces: faces}
}

// ToSolid converts the PolyMesh to a Manifold Solid by fan-triangulating faces.
// Each polygon face is tagged with a unique faceID so that polygon boundaries
// survive through Manifold's boolean operations and can be reconstructed later.
func (pm *PolyMesh) ToSolid() (*SolidFuture, error) {
	// Convert vertices to float32
	verts := make([]float32, len(pm.Vertices))
	for i, v := range pm.Vertices {
		verts[i] = float32(v)
	}

	// Fan-triangulate with per-triangle faceID
	var indices []uint32
	var faceIDs []uint32
	for fi, face := range pm.Faces {
		n := len(face)
		if n < 3 {
			continue
		}
		for i := 1; i < n-1; i++ {
			indices = append(indices, uint32(face[0]), uint32(face[i]), uint32(face[i+1]))
			faceIDs = append(faceIDs, uint32(fi))
		}
	}

	return createSolidFromMeshWithFaceIDs(verts, indices, faceIDs)
}

// ToDisplayMesh converts the PolyMesh to a DisplayMesh for rendering.
// Includes face group IDs for polygon-boundary wireframe rendering.
func (pm *PolyMesh) ToDisplayMesh() *DisplayMesh {
	verts := make([]float32, len(pm.Vertices))
	for i, v := range pm.Vertices {
		verts[i] = float32(v)
	}

	var indices []uint32
	var faceGroups []uint32
	for fi, face := range pm.Faces {
		n := len(face)
		if n < 3 {
			continue
		}
		for i := 1; i < n-1; i++ {
			indices = append(indices, uint32(face[0]), uint32(face[i]), uint32(face[i+1]))
			faceGroups = append(faceGroups, uint32(fi))
		}
	}

	return buildDisplayMesh(verts, indices, faceGroups)
}

// edges returns all unique edges in the mesh.
func (pm *PolyMesh) edges() []edge {
	seen := make(map[edge]bool)
	var result []edge
	for _, face := range pm.Faces {
		n := len(face)
		for i := 0; i < n; i++ {
			e := makeEdge(face[i], face[(i+1)%n])
			if !seen[e] {
				seen[e] = true
				result = append(result, e)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].a != result[j].a {
			return result[i].a < result[j].a
		}
		return result[i].b < result[j].b
	})
	return result
}
