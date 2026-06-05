package main

import (
	"math"

	"facet/pkg/manifold"
)

// solidBounds returns the axis-aligned bounding box enclosing every solid, with
// non-finite components sanitized to 0. An empty input yields a zero box.
func solidBounds(solids []*manifold.Solid) (globalMin, globalMax [3]float64) {
	if len(solids) > 0 {
		globalMin = [3]float64{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
		globalMax = [3]float64{-math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	}
	for _, s := range solids {
		mnX, mnY, mnZ, mxX, mxY, mxZ := s.BoundingBox()
		globalMin[0] = math.Min(globalMin[0], mnX)
		globalMin[1] = math.Min(globalMin[1], mnY)
		globalMin[2] = math.Min(globalMin[2], mnZ)
		globalMax[0] = math.Max(globalMax[0], mxX)
		globalMax[1] = math.Max(globalMax[1], mxY)
		globalMax[2] = math.Max(globalMax[2], mxZ)
	}
	globalMin[0] = sanitizeBBox(globalMin[0])
	globalMin[1] = sanitizeBBox(globalMin[1])
	globalMin[2] = sanitizeBBox(globalMin[2])
	globalMax[0] = sanitizeBBox(globalMax[0])
	globalMax[1] = sanitizeBBox(globalMax[1])
	globalMax[2] = sanitizeBBox(globalMax[2])
	return
}

func sanitizeBBox(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
