#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <cmath>
#include <cstdlib>
#include <vector>

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, as_cpp_cs, wrap, wrap_cs, facetClear

extern "C" {

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

void facet_delete_solid(ManifoldPtr* m) try { delete as_cpp(m); } catch (...) {}
void facet_delete_sketch(ManifoldCrossSection* cs) try { delete as_cpp_cs(cs); } catch (...) {}

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

void facet_cube(double x, double y, double z, FacetSolidRet* out) try {
  wrap(new Manifold(Manifold::Cube({x, y, z}, false).AsOriginal()), out);
} catch (...) { facetClear(out); }

void facet_sphere(double radius, int segments, FacetSolidRet* out) try {
  // Manifold::Sphere is centered at origin; translate so bbox starts at (0,0,0).
  auto s = Manifold::Sphere(radius, segments).Translate({radius, radius, radius});
  wrap(new Manifold(s.AsOriginal()), out);
} catch (...) { facetClear(out); }

void facet_cylinder(double height, double radius_low, double radius_high, int segments, FacetSolidRet* out) try {
  // Manifold::Cylinder is XY-centered with base at z=0; translate XY so bbox
  // starts at (0,0,0) on all axes, matching cube/sphere/square/circle.
  //
  // Workaround: Manifold's Cylinder returns an empty mesh when radius_low=0
  // and radius_high>0 (the bottom-tip cone is unsupported; the top-tip cone
  // works). Build the symmetric (top-tip) cone instead and reflect Z about
  // height/2 so the apex ends up at z=0 with the base at z=height. The Z
  // reflection is a Scale({1,1,-1}) followed by a Translate({0,0,height}),
  // which composes onto the same XY translate we use for the regular path.
  double r = std::fmax(radius_low, radius_high);
  manifold::Manifold s;
  if (radius_low == 0 && radius_high > 0) {
    s = Manifold::Cylinder(height, radius_high, 0, segments)
            .Scale({1, 1, -1})
            .Translate({r, r, height});
  } else {
    s = Manifold::Cylinder(height, radius_low, radius_high, segments)
            .Translate({r, r, 0});
  }
  wrap(new Manifold(s.AsOriginal()), out);
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

void facet_square(double x, double y, FacetSketchRet* out) try {
  // Not centered — bbox min at (0,0), matching cube/sphere/cylinder convention.
  wrap_cs(new CrossSection(CrossSection::Square({x, y}, false)), out);
} catch (...) { facetClear(out); }

void facet_circle(double radius, int segments, FacetSketchRet* out) try {
  // CrossSection::Circle is origin-centered; translate so bbox min is at (0,0),
  // matching square / cube / sphere / cylinder convention.
  auto c = CrossSection::Circle(radius, segments).Translate({radius, radius});
  wrap_cs(new CrossSection(std::move(c)), out);
} catch (...) { facetClear(out); }

void facet_polygon(
  const double* outer_xy_pairs, size_t outer_n,
  const double* holes_xy_pairs, const size_t* hole_sizes, size_t n_holes,
  FacetSketchRet* out) try {
  Polygons polygons;
  polygons.reserve(1 + n_holes);

  SimplePolygon outer(outer_n);
  for (size_t i = 0; i < outer_n; i++) {
    outer[i] = {outer_xy_pairs[i * 2], outer_xy_pairs[i * 2 + 1]};
  }
  polygons.push_back(std::move(outer));

  size_t off = 0;
  for (size_t h = 0; h < n_holes; h++) {
    size_t hn = hole_sizes[h];
    SimplePolygon hole(hn);
    for (size_t i = 0; i < hn; i++) {
      hole[i] = {holes_xy_pairs[(off + i) * 2], holes_xy_pairs[(off + i) * 2 + 1]};
    }
    polygons.push_back(std::move(hole));
    off += hn;
  }
  // EvenOdd: nested rings alternate fill regardless of winding direction.
  // Lets callers pass rings in any orientation; works the same for the
  // n_holes=0 plain-polygon case.
  wrap_cs(new CrossSection(CrossSection(std::move(polygons), CrossSection::FillRule::EvenOdd)), out);
} catch (...) { facetClear(out); }

void facet_cs_empty(FacetSketchRet* out) try {
  wrap_cs(new CrossSection(), out);
} catch (...) { facetClear(out); }

}  // extern "C"
