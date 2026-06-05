package manifold

import "math"

// SolidsBounds returns the axis-aligned bounding box enclosing every solid, with
// non-finite components sanitized to 0. An empty input yields a zero box. Shared
// by the desktop backend and the wasm preview so a single solid's frame stats
// and the whole-model bounds are computed the same way on both.
func SolidsBounds(solids []*Solid) (boxMin, boxMax [3]float64) {
	if len(solids) > 0 {
		boxMin = [3]float64{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
		boxMax = [3]float64{-math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	}
	for _, s := range solids {
		mnX, mnY, mnZ, mxX, mxY, mxZ := s.BoundingBox()
		boxMin[0] = math.Min(boxMin[0], mnX)
		boxMin[1] = math.Min(boxMin[1], mnY)
		boxMin[2] = math.Min(boxMin[2], mnZ)
		boxMax[0] = math.Max(boxMax[0], mxX)
		boxMax[1] = math.Max(boxMax[1], mxY)
		boxMax[2] = math.Max(boxMax[2], mxZ)
	}
	for i := range boxMin {
		boxMin[i] = sanitizeBound(boxMin[i])
		boxMax[i] = sanitizeBound(boxMax[i])
	}
	return
}

func sanitizeBound(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
