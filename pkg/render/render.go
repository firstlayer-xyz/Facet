// Package render rasterizes a triangle mesh to an image with a small software
// renderer (flat shading, z-buffer, supersampled). It uses no GPU and no cgo, so
// it runs headless anywhere — CI, Linux, Windows — and backs `facetc -o *.png`.
package render

import (
	"image"
	"image/color"
	"math"
)

type vec3 struct{ x, y, z float64 }

func (a vec3) sub(b vec3) vec3      { return vec3{a.x - b.x, a.y - b.y, a.z - b.z} }
func (a vec3) add(b vec3) vec3      { return vec3{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a vec3) scale(s float64) vec3 { return vec3{a.x * s, a.y * s, a.z * s} }
func (a vec3) dot(b vec3) float64   { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec3) cross(b vec3) vec3 {
	return vec3{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}
func (a vec3) normalized() vec3 {
	l := math.Sqrt(a.dot(a))
	if l == 0 {
		return a
	}
	return a.scale(1 / l)
}

// Mesh renders expanded (non-indexed) triangle positions — 9 floats per triangle
// (three xyz verts) — to a width×height RGBA image with a transparent
// background, viewed from a fixed 3/4 angle with Z up. Returns a blank
// transparent image when there are no triangles.
func Mesh(positions []float32, width, height int) *image.RGBA {
	const ss = 3 // supersampling factor for anti-aliasing
	w, h := width*ss, height*ss
	hi := image.NewRGBA(image.Rect(0, 0, w, h))

	nTri := len(positions) / 9
	if nTri == 0 || width <= 0 || height <= 0 {
		return downsample(hi, ss)
	}

	// Triangles + bounding box.
	type tri struct{ a, b, c vec3 }
	tris := make([]tri, nTri)
	lo := vec3{math.Inf(1), math.Inf(1), math.Inf(1)}
	hiB := vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for i := 0; i < nTri; i++ {
		o := i * 9
		t := tri{
			vec3{float64(positions[o]), float64(positions[o+1]), float64(positions[o+2])},
			vec3{float64(positions[o+3]), float64(positions[o+4]), float64(positions[o+5])},
			vec3{float64(positions[o+6]), float64(positions[o+7]), float64(positions[o+8])},
		}
		tris[i] = t
		for _, v := range [3]vec3{t.a, t.b, t.c} {
			lo = vec3{math.Min(lo.x, v.x), math.Min(lo.y, v.y), math.Min(lo.z, v.z)}
			hiB = vec3{math.Max(hiB.x, v.x), math.Max(hiB.y, v.y), math.Max(hiB.z, v.z)}
		}
	}
	center := lo.add(hiB).scale(0.5)
	radius := hiB.sub(lo).scale(0.5)
	r := math.Sqrt(radius.dot(radius))
	if r == 0 {
		r = 1
	}

	// Camera basis: look from a 3/4 angle, Z up. forward points into the scene.
	up := vec3{0, 0, 1}
	eye := center.add(vec3{1.1, -1.6, 0.95}.normalized().scale(r * 3))
	fwd := center.sub(eye).normalized()
	right := fwd.cross(up).normalized()
	camUp := right.cross(fwd)

	// Orthographic fit.
	scale := float64(min(w, h)) / (2 * r * 1.15)
	cx, cy := float64(w)/2, float64(h)/2

	// Fixed world-space light from the upper-front-right; flat Lambert + ambient.
	lightTo := vec3{-0.4, 0.6, -0.8}.normalized() // direction light travels
	const ambient = 0.30
	base := vec3{0.82, 0.84, 0.88}

	zbuf := make([]float64, w*h)
	for i := range zbuf {
		zbuf[i] = math.Inf(1)
	}

	type sv struct {
		x, y, depth float64
	}
	toScreen := func(p vec3) sv {
		d := p.sub(eye)
		return sv{cx + d.dot(right)*scale, cy - d.dot(camUp)*scale, d.dot(fwd)}
	}

	for _, t := range tris {
		n := t.b.sub(t.a).cross(t.c.sub(t.a)).normalized()
		// Back-face cull: skip triangles facing away from the camera.
		if n.dot(fwd) >= 0 {
			continue
		}
		shade := ambient + 0.80*math.Max(0, -n.dot(lightTo))
		if shade > 1 {
			shade = 1
		}
		col := color.RGBA{
			R: uint8(math.Min(255, base.x*shade*255+0.5)),
			G: uint8(math.Min(255, base.y*shade*255+0.5)),
			B: uint8(math.Min(255, base.z*shade*255+0.5)),
			A: 255,
		}
		a, b, c := toScreen(t.a), toScreen(t.b), toScreen(t.c)
		area := edge(a.x, a.y, b.x, b.y, c.x, c.y)
		if area == 0 {
			continue
		}
		minX := max(0, int(math.Floor(math.Min(a.x, math.Min(b.x, c.x)))))
		maxX := min(w-1, int(math.Ceil(math.Max(a.x, math.Max(b.x, c.x)))))
		minY := max(0, int(math.Floor(math.Min(a.y, math.Min(b.y, c.y)))))
		maxY := min(h-1, int(math.Ceil(math.Max(a.y, math.Max(b.y, c.y)))))
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				px, py := float64(x)+0.5, float64(y)+0.5
				w0 := edge(b.x, b.y, c.x, c.y, px, py) / area
				w1 := edge(c.x, c.y, a.x, a.y, px, py) / area
				w2 := edge(a.x, a.y, b.x, b.y, px, py) / area
				if w0 < 0 || w1 < 0 || w2 < 0 {
					continue
				}
				depth := w0*a.depth + w1*b.depth + w2*c.depth
				idx := y*w + x
				if depth < zbuf[idx] {
					zbuf[idx] = depth
					hi.SetRGBA(x, y, col)
				}
			}
		}
	}
	return downsample(hi, ss)
}

// edge is the signed area of the parallelogram spanned by ab and ap (×2).
func edge(ax, ay, bx, by, px, py float64) float64 {
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

// downsample box-filters the supersampled image down to its final size, which
// also resolves coverage to alpha so edges anti-alias against the transparent
// background.
func downsample(src *image.RGBA, ss int) *image.RGBA {
	w, h := src.Bounds().Dx()/ss, src.Bounds().Dy()/ss
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	n := float64(ss * ss)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var rr, gg, bb, aa float64
			for dy := 0; dy < ss; dy++ {
				for dx := 0; dx < ss; dx++ {
					c := src.RGBAAt(x*ss+dx, y*ss+dy)
					rr += float64(c.R)
					gg += float64(c.G)
					bb += float64(c.B)
					aa += float64(c.A)
				}
			}
			out.SetRGBA(x, y, color.RGBA{
				R: uint8(rr/n + 0.5), G: uint8(gg/n + 0.5), B: uint8(bb/n + 0.5), A: uint8(aa/n + 0.5),
			})
		}
	}
	return out
}
