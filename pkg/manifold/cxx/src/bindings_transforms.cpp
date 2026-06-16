#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <cmath>

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, as_cpp_cs, wrap, wrap_cs, facetClear

extern "C" {

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

void facet_translate(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Translate({x, y, z})), out);
} catch (...) { facetClear(out); }

void facet_rotate(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Rotate(x, y, z)), out);
} catch (...) { facetClear(out); }

void facet_scale(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Scale({x, y, z})), out);
} catch (...) { facetClear(out); }

void facet_mirror(ManifoldPtr* m, double nx, double ny, double nz, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Mirror({nx, ny, nz})), out);
} catch (...) { facetClear(out); }

// Pivot operations — translate-op-translate fused into a single C++ call.

// Scale pivoting at the bounding box min corner (bottom-left-front stays fixed).
void facet_scale_local(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) try {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double mx = bb.min.x, my = bb.min.y, mz = bb.min.z;
  Manifold result = src->Translate({-mx, -my, -mz}).Scale({x, y, z}).Translate({mx, my, mz});
  wrap(new Manifold(std::move(result)), out);
} catch (...) { facetClear(out); }

// Rotate pivoting at the bounding box center.
void facet_rotate_local(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) try {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  double cz = (bb.min.z + bb.max.z) * 0.5;
  Manifold result = src->Translate({-cx, -cy, -cz}).Rotate(x, y, z).Translate({cx, cy, cz});
  wrap(new Manifold(std::move(result)), out);
} catch (...) { facetClear(out); }

// Mirror pivoting at the bounding box center.
void facet_mirror_local(ManifoldPtr* m, double nx, double ny, double nz, FacetSolidRet* out) try {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  double cz = (bb.min.z + bb.max.z) * 0.5;
  Manifold result = src->Translate({-cx, -cy, -cz}).Mirror({nx, ny, nz}).Translate({cx, cy, cz});
  wrap(new Manifold(std::move(result)), out);
} catch (...) { facetClear(out); }

// Rotate by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
void facet_rotate_at(ManifoldPtr* m, double rx, double ry, double rz, double ox, double oy, double oz, FacetSolidRet* out) try {
  Manifold result = as_cpp(m)->Translate({-ox, -oy, -oz}).Rotate(rx, ry, rz).Translate({ox, oy, oz});
  wrap(new Manifold(std::move(result)), out);
} catch (...) { facetClear(out); }

// Scale by (x, y, z) around pivot point (ox, oy, oz).
void facet_scale_at(ManifoldPtr* m, double x, double y, double z, double ox, double oy, double oz, FacetSolidRet* out) try {
  Manifold result = as_cpp(m)->Translate({-ox, -oy, -oz}).Scale({x, y, z}).Translate({ox, oy, oz});
  wrap(new Manifold(std::move(result)), out);
} catch (...) { facetClear(out); }

// Mirror across plane with normal (nx, ny, nz) at signed offset from origin —
// fused translate-mirror-translate. The mirror plane passes through
// offset * normalize(n); we translate by -offset*n_hat, mirror across the
// origin-passing plane, then translate back.
void facet_mirror_at(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidRet* out) try {
  double len = std::sqrt(nx*nx + ny*ny + nz*nz);
  if (len > 0) { nx /= len; ny /= len; nz /= len; }
  double tx = offset * nx, ty = offset * ny, tz = offset * nz;
  Manifold result = as_cpp(m)->Translate({-tx, -ty, -tz}).Mirror({nx, ny, nz}).Translate({tx, ty, tz});
  wrap(new Manifold(std::move(result)), out);
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

void facet_cs_translate(ManifoldCrossSection* cs, double x, double y, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Translate({x, y})), out);
} catch (...) { facetClear(out); }

void facet_cs_rotate(ManifoldCrossSection* cs, double degrees, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Rotate(degrees)), out);
} catch (...) { facetClear(out); }

void facet_cs_scale(ManifoldCrossSection* cs, double x, double y, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Scale({x, y})), out);
} catch (...) { facetClear(out); }

void facet_cs_mirror(ManifoldCrossSection* cs, double ax, double ay, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Mirror({ax, ay})), out);
} catch (...) { facetClear(out); }

// Rotate sketch pivoting at the bounding box center.
void facet_cs_rotate_local(ManifoldCrossSection* cs, double degrees, FacetSketchRet* out) try {
  CrossSection* src = as_cpp_cs(cs);
  auto bb = src->Bounds();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  CrossSection result = src->Translate({-cx, -cy}).Rotate(degrees).Translate({cx, cy});
  wrap_cs(new CrossSection(std::move(result)), out);
} catch (...) { facetClear(out); }

// Mirror sketch pivoting at the bounding box center.
void facet_cs_mirror_local(ManifoldCrossSection* cs, double ax, double ay, FacetSketchRet* out) try {
  CrossSection* src = as_cpp_cs(cs);
  auto bb = src->Bounds();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  CrossSection result = src->Translate({-cx, -cy}).Mirror({ax, ay}).Translate({cx, cy});
  wrap_cs(new CrossSection(std::move(result)), out);
} catch (...) { facetClear(out); }

void facet_cs_offset(ManifoldCrossSection* cs, double delta, int segments, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Offset(delta, CrossSection::JoinType::Round, 2.0, segments)), out);
} catch (...) { facetClear(out); }

// Scale by (x, y) around pivot point (px, py).
void facet_cs_scale_at(ManifoldCrossSection* cs, double x, double y, double px, double py, FacetSketchRet* out) try {
  CrossSection result = as_cpp_cs(cs)->Translate({-px, -py}).Scale({x, y}).Translate({px, py});
  wrap_cs(new CrossSection(std::move(result)), out);
} catch (...) { facetClear(out); }

// Mirror across axis (ax, ay) at signed offset from origin — fused
// translate-mirror-translate. The mirror line passes through offset * axis_hat.
void facet_cs_mirror_at(ManifoldCrossSection* cs, double ax, double ay, double offset, FacetSketchRet* out) try {
  double len = std::sqrt(ax*ax + ay*ay);
  if (len > 0) { ax /= len; ay /= len; }
  double tx = offset * ax, ty = offset * ay;
  CrossSection result = as_cpp_cs(cs)->Translate({-tx, -ty}).Mirror({ax, ay}).Translate({tx, ty});
  wrap_cs(new CrossSection(std::move(result)), out);
} catch (...) { facetClear(out); }

}  // extern "C"
