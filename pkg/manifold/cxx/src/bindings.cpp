#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"
#include "manifold/polygon.h"

#include <cmath>
#ifndef M_PI
#define M_PI 3.14159265358979323846
#endif
#include <cstdlib>
#include <cstring>
#include <limits>
#include <string>
#include <vector>

#ifdef FACET_WASM
// Under wasm, the warp/levelset callback bridges are provided by the JS
// host. We declare them as imported functions here; the JS-side glue (see
// webspike/) supplies a concrete implementation. For the initial spike a
// minimal identity-warp / zero-density-levelset is enough; real callback
// dispatch will land alongside the syscall/js manifold bridge.
#include <emscripten/emscripten.h>
EM_JS(void, facetWarpBridge, (int callback_id, double* x, double* y, double* z), {
  if (Module["facetWarpBridge"]) Module["facetWarpBridge"](callback_id, x, y, z);
});
EM_JS(double, facetLevelSetBridge, (int callback_id, double x, double y, double z), {
  if (Module["facetLevelSetBridge"]) return Module["facetLevelSetBridge"](callback_id, x, y, z);
  return 0.0;
});
#else
extern "C" {
  void facetWarpBridge(int callback_id, double* x, double* y, double* z);
  double facetLevelSetBridge(int callback_id, double x, double y, double z);
}
#endif

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, as_cpp_cs, wrap, wrap_cs, solid_size, sketch_size

namespace {
constexpr double kFourPi = 4.0 * 3.14159265358979323846;

// Squared distance from p to triangle (a,b,c). Ericson, Real-Time Collision
// Detection — closest point on triangle, returns the squared distance.
double dist2_point_tri(const vec3& p, const vec3& a, const vec3& b, const vec3& c) {
  const double abx = b.x - a.x, aby = b.y - a.y, abz = b.z - a.z;
  const double acx = c.x - a.x, acy = c.y - a.y, acz = c.z - a.z;
  const double apx = p.x - a.x, apy = p.y - a.y, apz = p.z - a.z;
  const double d1 = abx * apx + aby * apy + abz * apz;
  const double d2 = acx * apx + acy * apy + acz * apz;
  if (d1 <= 0 && d2 <= 0) return apx * apx + apy * apy + apz * apz;
  const double bpx = p.x - b.x, bpy = p.y - b.y, bpz = p.z - b.z;
  const double d3 = abx * bpx + aby * bpy + abz * bpz;
  const double d4 = acx * bpx + acy * bpy + acz * bpz;
  if (d3 >= 0 && d4 <= d3) return bpx * bpx + bpy * bpy + bpz * bpz;
  const double vc = d1 * d4 - d3 * d2;
  if (vc <= 0 && d1 >= 0 && d3 <= 0) {
    const double v = d1 / (d1 - d3);
    const double qx = apx - v * abx, qy = apy - v * aby, qz = apz - v * abz;
    return qx * qx + qy * qy + qz * qz;
  }
  const double cpx = p.x - c.x, cpy = p.y - c.y, cpz = p.z - c.z;
  const double d5 = abx * cpx + aby * cpy + abz * cpz;
  const double d6 = acx * cpx + acy * cpy + acz * cpz;
  if (d6 >= 0 && d5 <= d6) return cpx * cpx + cpy * cpy + cpz * cpz;
  const double vb = d5 * d2 - d1 * d6;
  if (vb <= 0 && d2 >= 0 && d6 <= 0) {
    const double w = d2 / (d2 - d6);
    const double qx = apx - w * acx, qy = apy - w * acy, qz = apz - w * acz;
    return qx * qx + qy * qy + qz * qz;
  }
  const double va = d3 * d6 - d5 * d4;
  if (va <= 0 && (d4 - d3) >= 0 && (d5 - d6) >= 0) {
    const double w = (d4 - d3) / ((d4 - d3) + (d5 - d6));
    const double bcx = c.x - b.x, bcy = c.y - b.y, bcz = c.z - b.z;
    const double qx = bpx - w * bcx, qy = bpy - w * bcy, qz = bpz - w * bcz;
    return qx * qx + qy * qy + qz * qz;
  }
  const double denom = 1.0 / (va + vb + vc);
  const double v = vb * denom, w = vc * denom;
  const double qx = apx - v * abx - w * acx;
  const double qy = apy - v * aby - w * acy;
  const double qz = apz - v * abz - w * acz;
  return qx * qx + qy * qy + qz * qz;
}

// Signed solid angle subtended by triangle (A,B,C) at p (Van Oosterom &
// Strackee). Summed over a closed mesh and divided by 4*pi, |sum| > 0.5 means p
// is inside — robust for the watertight meshes Manifold guarantees.
double signed_solid_angle(const vec3& p, const vec3& A, const vec3& B, const vec3& C) {
  const double ax = A.x - p.x, ay = A.y - p.y, az = A.z - p.z;
  const double bx = B.x - p.x, by = B.y - p.y, bz = B.z - p.z;
  const double cx = C.x - p.x, cy = C.y - p.y, cz = C.z - p.z;
  const double la = std::sqrt(ax * ax + ay * ay + az * az);
  const double lb = std::sqrt(bx * bx + by * by + bz * bz);
  const double lc = std::sqrt(cx * cx + cy * cy + cz * cz);
  const double crx = by * cz - bz * cy;
  const double cry = bz * cx - bx * cz;
  const double crz = bx * cy - by * cx;
  const double num = ax * crx + ay * cry + az * crz;
  const double ab = ax * bx + ay * by + az * bz;
  const double bc = bx * cx + by * cy + bz * cz;
  const double ca = cx * ax + cy * ay + cz * az;
  const double den = la * lb * lc + ab * lc + bc * la + ca * lb;
  return 2.0 * std::atan2(num, den);
}
}  // namespace

extern "C" {

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

void facet_delete_solid(ManifoldPtr* m) { delete as_cpp(m); }
void facet_delete_sketch(ManifoldCrossSection* cs) { delete as_cpp_cs(cs); }

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

void facet_cube(double x, double y, double z, FacetSolidRet* out) {
  wrap(new Manifold(Manifold::Cube({x, y, z}, false).AsOriginal()), out);
}

void facet_sphere(double radius, int segments, FacetSolidRet* out) {
  // Manifold::Sphere is centered at origin; translate so bbox starts at (0,0,0).
  auto s = Manifold::Sphere(radius, segments).Translate({radius, radius, radius});
  wrap(new Manifold(s.AsOriginal()), out);
}

void facet_cylinder(double height, double radius_low, double radius_high, int segments, FacetSolidRet* out) {
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
}

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

void facet_square(double x, double y, FacetSketchRet* out) {
  // Not centered — bbox min at (0,0), matching cube/sphere/cylinder convention.
  wrap_cs(new CrossSection(CrossSection::Square({x, y}, false)), out);
}

void facet_circle(double radius, int segments, FacetSketchRet* out) {
  // CrossSection::Circle is origin-centered; translate so bbox min is at (0,0),
  // matching square / cube / sphere / cylinder convention.
  auto c = CrossSection::Circle(radius, segments).Translate({radius, radius});
  wrap_cs(new CrossSection(std::move(c)), out);
}

void facet_polygon(
  const double* outer_xy_pairs, size_t outer_n,
  const double* holes_xy_pairs, const size_t* hole_sizes, size_t n_holes,
  FacetSketchRet* out) {
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
}

void facet_cs_empty(FacetSketchRet* out) {
  wrap_cs(new CrossSection(), out);
}

// ---------------------------------------------------------------------------
// 3D Booleans
// ---------------------------------------------------------------------------

void facet_union(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) {
  wrap(new Manifold(*as_cpp(a) + *as_cpp(b)), out);
}

void facet_difference(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) {
  wrap(new Manifold(*as_cpp(a) - *as_cpp(b)), out);
}

void facet_intersection(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) {
  wrap(new Manifold(*as_cpp(a) ^ *as_cpp(b)), out);
}

// Insert seats b into a: cut b's shape out of a, drop any piece of a that b
// traps inside itself (a "plug"), then union b back in. A piece is a plug iff
// it lies entirely within b's convex hull, tested by exact boolean
// containment: (piece - hull) is empty. The hull is rotation-invariant and
// tight to b's geometry.
//
// When every piece is a plug there is no outer shell and seating b would
// discard all of a, which is never a valid result. out->ptr is left null to
// signal this; the Go layer reports it as errInsertNoShell.
void facet_insert(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) {
  Manifold diff = *as_cpp(a) - *as_cpp(b);
  auto components = diff.Decompose();
  Manifold pierced;
  if (components.size() <= 1) {
    // b severed nothing, so there is no plug to remove.
    pierced = std::move(diff);
  } else {
    Manifold b_hull = as_cpp(b)->Hull();
    std::vector<Manifold> outer;
    for (auto& comp : components) {
      if (!(comp - b_hull).IsEmpty())  // escapes b's hull -> outer shell, keep
        outer.push_back(std::move(comp));
    }
    if (outer.empty()) {
      out->ptr = nullptr;
      out->size = 0;
      out->original_id = -1;
      return;
    }
    pierced = Manifold::Compose(outer);
  }
  wrap(new Manifold(pierced + *as_cpp(b)), out);
}

// Returns count of connected components; fills *out_components with a malloc'd
// array of FacetSolidRet (one per component). Caller must free each
// component's ptr with facet_delete_solid, then free(*out_components).
int facet_decompose(ManifoldPtr* m, FacetSolidRet** out_components) {
  auto components = as_cpp(m)->Decompose();
  int n = (int)components.size();
  if (n == 0) {
    *out_components = nullptr;
    return 0;
  }
  FacetSolidRet* arr = (FacetSolidRet*)malloc(n * sizeof(FacetSolidRet));
  for (int i = 0; i < n; i++)
    wrap(new Manifold(std::move(components[i])), &arr[i]);
  *out_components = arr;
  return n;
}

// ---------------------------------------------------------------------------
// 2D Booleans
// ---------------------------------------------------------------------------

void facet_cs_union(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out) {
  wrap_cs(new CrossSection(*as_cpp_cs(a) + *as_cpp_cs(b)), out);
}

void facet_cs_difference(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out) {
  wrap_cs(new CrossSection(*as_cpp_cs(a) - *as_cpp_cs(b)), out);
}

void facet_cs_intersection(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out) {
  wrap_cs(new CrossSection(*as_cpp_cs(a) ^ *as_cpp_cs(b)), out);
}

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

void facet_translate(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Translate({x, y, z})), out);
}

void facet_rotate(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Rotate(x, y, z)), out);
}

void facet_scale(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Scale({x, y, z})), out);
}

void facet_mirror(ManifoldPtr* m, double nx, double ny, double nz, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Mirror({nx, ny, nz})), out);
}

// Pivot operations — translate-op-translate fused into a single C++ call.

// Scale pivoting at the bounding box min corner (bottom-left-front stays fixed).
void facet_scale_local(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double mx = bb.min.x, my = bb.min.y, mz = bb.min.z;
  Manifold result = src->Translate({-mx, -my, -mz}).Scale({x, y, z}).Translate({mx, my, mz});
  wrap(new Manifold(std::move(result)), out);
}

// Rotate pivoting at the bounding box center.
void facet_rotate_local(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out) {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  double cz = (bb.min.z + bb.max.z) * 0.5;
  Manifold result = src->Translate({-cx, -cy, -cz}).Rotate(x, y, z).Translate({cx, cy, cz});
  wrap(new Manifold(std::move(result)), out);
}

// Mirror pivoting at the bounding box center.
void facet_mirror_local(ManifoldPtr* m, double nx, double ny, double nz, FacetSolidRet* out) {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  double cz = (bb.min.z + bb.max.z) * 0.5;
  Manifold result = src->Translate({-cx, -cy, -cz}).Mirror({nx, ny, nz}).Translate({cx, cy, cz});
  wrap(new Manifold(std::move(result)), out);
}

// Rotate by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
void facet_rotate_at(ManifoldPtr* m, double rx, double ry, double rz, double ox, double oy, double oz, FacetSolidRet* out) {
  Manifold result = as_cpp(m)->Translate({-ox, -oy, -oz}).Rotate(rx, ry, rz).Translate({ox, oy, oz});
  wrap(new Manifold(std::move(result)), out);
}

// Scale by (x, y, z) around pivot point (ox, oy, oz).
void facet_scale_at(ManifoldPtr* m, double x, double y, double z, double ox, double oy, double oz, FacetSolidRet* out) {
  Manifold result = as_cpp(m)->Translate({-ox, -oy, -oz}).Scale({x, y, z}).Translate({ox, oy, oz});
  wrap(new Manifold(std::move(result)), out);
}

// Mirror across plane with normal (nx, ny, nz) at signed offset from origin —
// fused translate-mirror-translate. The mirror plane passes through
// offset * normalize(n); we translate by -offset*n_hat, mirror across the
// origin-passing plane, then translate back.
void facet_mirror_at(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidRet* out) {
  double len = std::sqrt(nx*nx + ny*ny + nz*nz);
  if (len > 0) { nx /= len; ny /= len; nz /= len; }
  double tx = offset * nx, ty = offset * ny, tz = offset * nz;
  Manifold result = as_cpp(m)->Translate({-tx, -ty, -tz}).Mirror({nx, ny, nz}).Translate({tx, ty, tz});
  wrap(new Manifold(std::move(result)), out);
}

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

void facet_cs_translate(ManifoldCrossSection* cs, double x, double y, FacetSketchRet* out) {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Translate({x, y})), out);
}

void facet_cs_rotate(ManifoldCrossSection* cs, double degrees, FacetSketchRet* out) {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Rotate(degrees)), out);
}

void facet_cs_scale(ManifoldCrossSection* cs, double x, double y, FacetSketchRet* out) {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Scale({x, y})), out);
}

void facet_cs_mirror(ManifoldCrossSection* cs, double ax, double ay, FacetSketchRet* out) {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Mirror({ax, ay})), out);
}

// Rotate sketch pivoting at the bounding box center.
void facet_cs_rotate_local(ManifoldCrossSection* cs, double degrees, FacetSketchRet* out) {
  CrossSection* src = as_cpp_cs(cs);
  auto bb = src->Bounds();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  CrossSection result = src->Translate({-cx, -cy}).Rotate(degrees).Translate({cx, cy});
  wrap_cs(new CrossSection(std::move(result)), out);
}

// Mirror sketch pivoting at the bounding box center.
void facet_cs_mirror_local(ManifoldCrossSection* cs, double ax, double ay, FacetSketchRet* out) {
  CrossSection* src = as_cpp_cs(cs);
  auto bb = src->Bounds();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  CrossSection result = src->Translate({-cx, -cy}).Mirror({ax, ay}).Translate({cx, cy});
  wrap_cs(new CrossSection(std::move(result)), out);
}

void facet_cs_offset(ManifoldCrossSection* cs, double delta, int segments, FacetSketchRet* out) {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Offset(delta, CrossSection::JoinType::Round, 2.0, segments)), out);
}

// Scale by (x, y) around pivot point (px, py).
void facet_cs_scale_at(ManifoldCrossSection* cs, double x, double y, double px, double py, FacetSketchRet* out) {
  CrossSection result = as_cpp_cs(cs)->Translate({-px, -py}).Scale({x, y}).Translate({px, py});
  wrap_cs(new CrossSection(std::move(result)), out);
}

// Mirror across axis (ax, ay) at signed offset from origin — fused
// translate-mirror-translate. The mirror line passes through offset * axis_hat.
void facet_cs_mirror_at(ManifoldCrossSection* cs, double ax, double ay, double offset, FacetSketchRet* out) {
  double len = std::sqrt(ax*ax + ay*ay);
  if (len > 0) { ax /= len; ay /= len; }
  double tx = offset * ax, ty = offset * ay;
  CrossSection result = as_cpp_cs(cs)->Translate({-tx, -ty}).Mirror({ax, ay}).Translate({tx, ty});
  wrap_cs(new CrossSection(std::move(result)), out);
}

// ---------------------------------------------------------------------------
// 2D → 3D
// ---------------------------------------------------------------------------

void facet_extrude(ManifoldCrossSection* cs, double height, int slices,
                   double twist, double scale_x, double scale_y, FacetSolidRet* out) {
  auto polys = as_cpp_cs(cs)->ToPolygons();
  wrap(new Manifold(Manifold::Extrude(polys, height, slices, twist, {scale_x, scale_y}).AsOriginal()), out);
}

void facet_revolve(ManifoldCrossSection* cs, int segments, double degrees, FacetSolidRet* out) {
  auto polys = as_cpp_cs(cs)->ToPolygons();
  wrap(new Manifold(Manifold::Revolve(polys, segments, degrees).AsOriginal()), out);
}

void facet_sweep(ManifoldCrossSection* cs,
                 double* path_xyz, size_t n_path_points, FacetSolidRet* out) {
  if (n_path_points < 2) { wrap(new Manifold(), out); return; }

  auto polys = as_cpp_cs(cs)->ToPolygons();

  // Collect all cross-section vertices across contours.
  std::vector<vec2> csVerts;
  // Track contour boundaries for quad-strip connectivity.
  std::vector<size_t> contourStart;
  std::vector<size_t> contourSize;
  for (auto& poly : polys) {
    contourStart.push_back(csVerts.size());
    contourSize.push_back(poly.size());
    for (auto& v : poly) {
      csVerts.push_back(v);
    }
  }
  size_t nCS = csVerts.size();
  if (nCS == 0) { wrap(new Manifold(), out); return; }
  size_t nPath = n_path_points;

  // Read path points.
  std::vector<vec3> path(nPath);
  for (size_t i = 0; i < nPath; i++) {
    path[i] = {path_xyz[i*3], path_xyz[i*3+1], path_xyz[i*3+2]};
  }

  // Compute tangent vectors at each path point, matching BOSL2 path_sweep's
  // deriv-based tangents (uniform spacing): interior points use a central
  // difference; the endpoints use a 3-point one-sided difference, so the end
  // caps tilt toward the path the way BOSL2's do rather than sitting perpendicular
  // to only the first/last segment. A 2-point path falls back to the segment dir.
  std::vector<vec3> tangents(nPath);
  for (size_t i = 0; i < nPath; i++) {
    vec3 t;
    if (i == 0) {
      if (nPath >= 3) {
        t = {-3*path[0].x + 4*path[1].x - path[2].x,
             -3*path[0].y + 4*path[1].y - path[2].y,
             -3*path[0].z + 4*path[1].z - path[2].z};
      } else {
        t = path[1] - path[0];
      }
    } else if (i == nPath - 1) {
      size_t e = nPath - 1;
      if (nPath >= 3) {
        t = {3*path[e].x - 4*path[e-1].x + path[e-2].x,
             3*path[e].y - 4*path[e-1].y + path[e-2].y,
             3*path[e].z - 4*path[e-1].z + path[e-2].z};
      } else {
        t = path[e] - path[e-1];
      }
    } else {
      t = path[i+1] - path[i-1];
    }
    double len = std::sqrt(t.x*t.x + t.y*t.y + t.z*t.z);
    if (len > 0) t = t / len;
    tangents[i] = t;
  }

  // Rotation-minimizing frame: one in-plane normal per path point, kept as stable
  // as possible around bends. The frame's in-plane X axis is derived at placement
  // time as cross(normal, tangent), matching BOSL2 frame_map(y=normal, z=tangent).
  std::vector<vec3> normals(nPath);

  // Initial normal, matching BOSL2: start from BACK when the first tangent is
  // steep (|z| > 1/sqrt2) else UP, then project perpendicular to the tangent.
  {
    vec3 t0 = tangents[0];
    vec3 up = {0, 0, 1};
    if (std::abs(t0.z) > 0.70710678118) {
      up = {0, 1, 0};
    }
    double d = up.x*t0.x + up.y*t0.y + up.z*t0.z;
    vec3 n = {up.x - d*t0.x, up.y - d*t0.y, up.z - d*t0.z};
    double nlen = std::sqrt(n.x*n.x + n.y*n.y + n.z*n.z);
    if (nlen > 1e-12) n = n / nlen;
    normals[0] = n;
  }

  // Propagate with the double-reflection method (Wang, Jüttler, Zheng & Liu 2008),
  // as BOSL2's "incremental" path_sweep does. Two reflections carry the normal
  // across each segment with far less drift than a plain projection — which is
  // what makes sharp corners match BOSL2 rather than skewing the frame.
  for (size_t i = 1; i < nPath; i++) {
    vec3 v1 = path[i] - path[i-1];
    double c1 = v1.x*v1.x + v1.y*v1.y + v1.z*v1.z;
    vec3 r = normals[i-1];
    vec3 tp = tangents[i-1];
    // First reflection across the plane bisecting the segment.
    double kr = (c1 > 1e-12) ? 2.0*(v1.x*r.x + v1.y*r.y + v1.z*r.z)/c1 : 0.0;
    vec3 rL = {r.x - kr*v1.x, r.y - kr*v1.y, r.z - kr*v1.z};
    double kt = (c1 > 1e-12) ? 2.0*(v1.x*tp.x + v1.y*tp.y + v1.z*tp.z)/c1 : 0.0;
    vec3 tL = {tp.x - kt*v1.x, tp.y - kt*v1.y, tp.z - kt*v1.z};
    // Second reflection, aligning the reflected tangent with the next tangent.
    vec3 v2 = {tangents[i].x - tL.x, tangents[i].y - tL.y, tangents[i].z - tL.z};
    double c2 = v2.x*v2.x + v2.y*v2.y + v2.z*v2.z;
    vec3 n;
    if (c2 > 1e-12) {
      double kr2 = 2.0*(v2.x*rL.x + v2.y*rL.y + v2.z*rL.z)/c2;
      n = {rL.x - kr2*v2.x, rL.y - kr2*v2.y, rL.z - kr2*v2.z};
    } else {
      n = rL;  // straight segment: tangent unchanged, no extra rotation
    }
    // Re-orthonormalize against the current tangent to suppress numeric drift.
    vec3 t = tangents[i];
    double d = n.x*t.x + n.y*t.y + n.z*t.z;
    n = {n.x - d*t.x, n.y - d*t.y, n.z - d*t.z};
    double nlen = std::sqrt(n.x*n.x + n.y*n.y + n.z*n.z);
    if (nlen > 1e-12) {
      n = {n.x/nlen, n.y/nlen, n.z/nlen};
    } else {
      n = normals[i-1];
    }
    normals[i] = n;
  }

  // Build vertex positions: place the cross-section in 3D at each path point.
  // The frame X axis = cross(normal, tangent), so a 2D profile point (x, y) maps
  // to x*Xaxis + y*normal — matching BOSL2 frame_map(y=normal, z=tangent).
  // Vertex layout: path_index * nCS + cs_index
  std::vector<float> vertProps(nPath * nCS * 3);
  for (size_t pi = 0; pi < nPath; pi++) {
    vec3 p = path[pi];
    vec3 n = normals[pi];
    vec3 t = tangents[pi];
    vec3 xax = {n.y*t.z - n.z*t.y, n.z*t.x - n.x*t.z, n.x*t.y - n.y*t.x};
    for (size_t ci = 0; ci < nCS; ci++) {
      vec2 cv = csVerts[ci];
      float x = (float)(p.x + cv.x * xax.x + cv.y * n.x);
      float y = (float)(p.y + cv.x * xax.y + cv.y * n.y);
      float z = (float)(p.z + cv.x * xax.z + cv.y * n.z);
      size_t vi = (pi * nCS + ci) * 3;
      vertProps[vi]   = x;
      vertProps[vi+1] = y;
      vertProps[vi+2] = z;
    }
  }

  // Build triangle indices: connect adjacent rings with quad strips per contour.
  std::vector<uint32_t> triVerts;
  for (size_t pi = 0; pi < nPath - 1; pi++) {
    for (size_t c = 0; c < contourStart.size(); c++) {
      size_t cStart = contourStart[c];
      size_t cSize = contourSize[c];
      for (size_t j = 0; j < cSize; j++) {
        size_t jNext = (j + 1) % cSize;
        uint32_t v0 = (uint32_t)(pi * nCS + cStart + j);
        uint32_t v1 = (uint32_t)(pi * nCS + cStart + jNext);
        uint32_t v2 = (uint32_t)((pi+1) * nCS + cStart + j);
        uint32_t v3 = (uint32_t)((pi+1) * nCS + cStart + jNext);
        // Two triangles per quad
        triVerts.push_back(v0); triVerts.push_back(v1); triVerts.push_back(v2);
        triVerts.push_back(v1); triVerts.push_back(v3); triVerts.push_back(v2);
      }
    }
  }

  // Cap the start and end with triangulated polygons.
  {
    size_t idx = 0;
    PolygonsIdx polysIdx;
    for (auto& poly : polys) {
      SimplePolygonIdx simpleIdx;
      for (auto& v : poly) {
        simpleIdx.push_back({v, (int)idx});
        idx++;
      }
      polysIdx.push_back(simpleIdx);
    }
    std::vector<ivec3> capTris = TriangulateIdx(polysIdx);
    // Start cap (ring 0): reverse winding so normals face outward (backward)
    for (auto& tri : capTris) {
      triVerts.push_back((uint32_t)tri[0]);
      triVerts.push_back((uint32_t)tri[2]);
      triVerts.push_back((uint32_t)tri[1]);
    }
    // End cap (ring nPath-1): normal winding
    uint32_t endOff = (uint32_t)((nPath - 1) * nCS);
    for (auto& tri : capTris) {
      triVerts.push_back(endOff + (uint32_t)tri[0]);
      triVerts.push_back(endOff + (uint32_t)tri[1]);
      triVerts.push_back(endOff + (uint32_t)tri[2]);
    }
  }

  MeshGL mesh;
  mesh.numProp = 3;
  mesh.vertProperties = std::move(vertProps);
  mesh.triVerts.assign(triVerts.begin(), triVerts.end());
  wrap(new Manifold(Manifold(mesh).AsOriginal()), out);
}

void facet_loft(ManifoldCrossSection** sketches, size_t n_sketches,
                double* heights, size_t n_heights, FacetSolidRet* out) {
  if (n_sketches < 2 || n_sketches != n_heights) { wrap(new Manifold(), out); return; }

  // Extract polygons for each sketch.
  std::vector<Polygons> allPolys(n_sketches);
  for (size_t i = 0; i < n_sketches; i++) {
    allPolys[i] = as_cpp_cs(sketches[i])->ToPolygons();
  }

  // Find the maximum contour count across all sketches.
  size_t maxContours = 0;
  for (auto& polys : allPolys) {
    if (polys.size() > maxContours) maxContours = polys.size();
  }
  if (maxContours == 0) { wrap(new Manifold(), out); return; }

  // Resample helper: walk a contour's edges and place targetN points at
  // uniform arc-length spacing.
  auto resampleContour = [](const SimplePolygon& contour, size_t targetN) -> SimplePolygon {
    if (contour.size() == 0 || targetN == 0) return {};
    if (targetN == 1) {
      // Degenerate: centroid
      vec2 c = {0, 0};
      for (auto& v : contour) { c.x += v.x; c.y += v.y; }
      c.x /= (double)contour.size();
      c.y /= (double)contour.size();
      return {c};
    }
    // Compute edge lengths and total perimeter.
    size_t n = contour.size();
    std::vector<double> edgeLens(n);
    double totalLen = 0;
    for (size_t i = 0; i < n; i++) {
      size_t j = (i + 1) % n;
      double dx = contour[j].x - contour[i].x;
      double dy = contour[j].y - contour[i].y;
      edgeLens[i] = std::sqrt(dx * dx + dy * dy);
      totalLen += edgeLens[i];
    }
    if (totalLen < 1e-15) {
      // Degenerate contour — all points identical.
      SimplePolygon result(targetN, contour[0]);
      return result;
    }
    // Walk the perimeter, placing targetN points at equal arc-length intervals.
    SimplePolygon result;
    result.reserve(targetN);
    double step = totalLen / (double)targetN;
    double walked = 0;
    size_t edgeIdx = 0;
    double edgeWalked = 0;
    for (size_t i = 0; i < targetN; i++) {
      double target = i * step;
      while (walked + edgeLens[edgeIdx] - edgeWalked < target - 1e-12 && edgeIdx < n) {
        walked += edgeLens[edgeIdx] - edgeWalked;
        edgeIdx = (edgeIdx + 1) % n;
        edgeWalked = 0;
      }
      double rem = target - walked;
      double t = (edgeLens[edgeIdx] > 1e-15) ? (edgeWalked + rem) / edgeLens[edgeIdx] : 0;
      if (t > 1.0) t = 1.0;
      size_t a = edgeIdx;
      size_t b = (edgeIdx + 1) % n;
      vec2 p = {contour[a].x + t * (contour[b].x - contour[a].x),
                contour[a].y + t * (contour[b].y - contour[a].y)};
      result.push_back(p);
    }
    return result;
  };

  // Determine per-contour target vertex count: max across all sketches.
  std::vector<size_t> contourTargets(maxContours, 0);
  for (auto& polys : allPolys) {
    for (size_t c = 0; c < polys.size(); c++) {
      if (polys[c].size() > contourTargets[c])
        contourTargets[c] = polys[c].size();
    }
  }

  // Total vertices per ring.
  size_t ringVerts = 0;
  for (auto t : contourTargets) ringVerts += t;
  if (ringVerts == 0) { wrap(new Manifold(), out); return; }

  // Resample all sketches so each has maxContours contours, each with
  // matching vertex count. Missing contours degenerate to centroid of
  // the existing contours.
  std::vector<std::vector<SimplePolygon>> resampled(n_sketches);
  for (size_t s = 0; s < n_sketches; s++) {
    resampled[s].resize(maxContours);
    // Compute centroid of existing contours for degenerate fill.
    vec2 centroid = {0, 0};
    size_t totalPts = 0;
    for (auto& poly : allPolys[s]) {
      for (auto& v : poly) { centroid.x += v.x; centroid.y += v.y; totalPts++; }
    }
    if (totalPts > 0) { centroid.x /= totalPts; centroid.y /= totalPts; }
    for (size_t c = 0; c < maxContours; c++) {
      if (c < allPolys[s].size()) {
        resampled[s][c] = resampleContour(allPolys[s][c], contourTargets[c]);
      } else {
        // Degenerate: fill missing contour with centroid points.
        resampled[s][c] = SimplePolygon(contourTargets[c], centroid);
      }
    }
  }

  // Build vertex positions: for each sketch ring, place resampled 2D points at its Z height.
  std::vector<float> vertProps(n_sketches * ringVerts * 3);
  for (size_t si = 0; si < n_sketches; si++) {
    double z = heights[si];
    size_t vi = 0;
    for (size_t c = 0; c < maxContours; c++) {
      for (auto& p : resampled[si][c]) {
        size_t idx = (si * ringVerts + vi) * 3;
        vertProps[idx]     = (float)p.x;
        vertProps[idx + 1] = (float)p.y;
        vertProps[idx + 2] = (float)z;
        vi++;
      }
    }
  }

  // Build triangle indices: connect adjacent rings with quad strips per contour.
  std::vector<uint32_t> triVerts;
  for (size_t si = 0; si < n_sketches - 1; si++) {
    size_t cOff = 0;
    for (size_t c = 0; c < maxContours; c++) {
      size_t cSize = contourTargets[c];
      for (size_t j = 0; j < cSize; j++) {
        size_t jNext = (j + 1) % cSize;
        uint32_t v0 = (uint32_t)(si * ringVerts + cOff + j);
        uint32_t v1 = (uint32_t)(si * ringVerts + cOff + jNext);
        uint32_t v2 = (uint32_t)((si + 1) * ringVerts + cOff + j);
        uint32_t v3 = (uint32_t)((si + 1) * ringVerts + cOff + jNext);
        triVerts.push_back(v0); triVerts.push_back(v1); triVerts.push_back(v2);
        triVerts.push_back(v1); triVerts.push_back(v3); triVerts.push_back(v2);
      }
      cOff += cSize;
    }
  }

  // Cap bottom (ring 0, reversed winding) and top (ring N-1).
  // Build PolygonsIdx for the first ring's resampled contours.
  {
    PolygonsIdx polysIdx;
    size_t idx = 0;
    for (size_t c = 0; c < maxContours; c++) {
      SimplePolygonIdx simpleIdx;
      for (size_t j = 0; j < contourTargets[c]; j++) {
        simpleIdx.push_back({resampled[0][c][j], (int)idx});
        idx++;
      }
      polysIdx.push_back(simpleIdx);
    }
    std::vector<ivec3> capTris = TriangulateIdx(polysIdx);
    // Start cap (ring 0): reverse winding
    for (auto& tri : capTris) {
      triVerts.push_back((uint32_t)tri[0]);
      triVerts.push_back((uint32_t)tri[2]);
      triVerts.push_back((uint32_t)tri[1]);
    }
    // End cap (ring N-1): normal winding
    uint32_t endOff = (uint32_t)((n_sketches - 1) * ringVerts);
    for (auto& tri : capTris) {
      triVerts.push_back(endOff + (uint32_t)tri[0]);
      triVerts.push_back(endOff + (uint32_t)tri[1]);
      triVerts.push_back(endOff + (uint32_t)tri[2]);
    }
  }

  MeshGL mesh;
  mesh.numProp = 3;
  mesh.vertProperties = std::move(vertProps);
  mesh.triVerts.assign(triVerts.begin(), triVerts.end());
  wrap(new Manifold(Manifold(mesh).AsOriginal()), out);
}

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

void facet_slice(ManifoldPtr* m, double height, FacetSketchRet* out) {
  auto polys = as_cpp(m)->Slice(height);
  wrap_cs(new CrossSection(CrossSection(polys, CrossSection::FillRule::Positive)), out);
}

void facet_project(ManifoldPtr* m, FacetSketchRet* out) {
  auto polys = as_cpp(m)->Project();
  wrap_cs(new CrossSection(CrossSection(polys, CrossSection::FillRule::Positive)), out);
}

// ---------------------------------------------------------------------------
// 3D Hulls
// ---------------------------------------------------------------------------

void facet_hull(ManifoldPtr* m, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Hull().AsOriginal()), out);
}

void facet_batch_hull(ManifoldPtr** solids, size_t count, FacetSolidRet* out) {
  std::vector<Manifold> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp(solids[i]);
  }
  wrap(new Manifold(Manifold::Hull(vec).AsOriginal()), out);
}

void facet_hull_points(double* xyz, size_t n_points, FacetSolidRet* out) {
  std::vector<vec3> pts(n_points);
  for (size_t i = 0; i < n_points; i++) {
    pts[i] = {xyz[i * 3], xyz[i * 3 + 1], xyz[i * 3 + 2]};
  }
  wrap(new Manifold(Manifold::Hull(pts).AsOriginal()), out);
}

// ---------------------------------------------------------------------------
// 2D Hulls
// ---------------------------------------------------------------------------

void facet_cs_hull(ManifoldCrossSection* cs, FacetSketchRet* out) {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Hull()), out);
}

void facet_cs_batch_hull(ManifoldCrossSection** sketches, size_t count, FacetSketchRet* out) {
  std::vector<CrossSection> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp_cs(sketches[i]);
  }
  wrap_cs(new CrossSection(CrossSection::Hull(vec)), out);
}

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

void facet_trim_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->TrimByPlane({nx, ny, nz}, offset)), out);
}

void facet_smooth_out(ManifoldPtr* m, double min_sharp_angle, double min_smoothness, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->SmoothOut(min_sharp_angle, min_smoothness)), out);
}

void facet_refine(ManifoldPtr* m, int n, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Refine(n)), out);
}

void facet_simplify(ManifoldPtr* m, double tolerance, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Simplify(tolerance)), out);
}

void facet_offset(ManifoldPtr* m, double delta, double edge_length, FacetSolidRet* out) {
  Manifold* mp = as_cpp(m);
  MeshGL mesh = mp->GetMeshGL();
  const uint32_t nProp = mesh.numProp;

  struct Tri { vec3 a, b, c; };
  auto V = [&](uint32_t i) -> vec3 {
    const size_t o = static_cast<size_t>(i) * nProp;
    return vec3{static_cast<double>(mesh.vertProperties[o]),
                static_cast<double>(mesh.vertProperties[o + 1]),
                static_cast<double>(mesh.vertProperties[o + 2])};
  };
  std::vector<Tri> tris;
  tris.reserve(mesh.triVerts.size() / 3);
  for (size_t t = 0; t + 2 < mesh.triVerts.size(); t += 3) {
    tris.push_back({V(mesh.triVerts[t]), V(mesh.triVerts[t + 1]), V(mesh.triVerts[t + 2])});
  }

  // Sampling region: the mesh bbox grown to enclose the offset surface.
  Box bb = mp->BoundingBox();
  const double pad = std::abs(delta) + 3.0 * edge_length;
  Box bounds{vec3{bb.min.x - pad, bb.min.y - pad, bb.min.z - pad},
             vec3{bb.max.x + pad, bb.max.y + pad, bb.max.z + pad}};

  // POSITIVE-inside signed distance (Manifold LevelSet convention).
  auto sdf = [&tris](vec3 p) -> double {
    double best2 = std::numeric_limits<double>::max();
    double omega = 0.0;
    for (const Tri& tr : tris) {
      const double d2 = dist2_point_tri(p, tr.a, tr.b, tr.c);
      if (d2 < best2) best2 = d2;
      omega += signed_solid_angle(p, tr.a, tr.b, tr.c);
    }
    const double dist = std::sqrt(best2);
    const bool inside = std::abs(omega / kFourPi) > 0.5;
    return inside ? dist : -dist;
  };

  // Offset by delta => mesh the {sdf > -delta} interior. canParallel=false to
  // match facet_level_set.
  wrap(new Manifold(
           Manifold::LevelSet(sdf, bounds, edge_length, -delta, -1.0, false).AsOriginal()),
       out);
}

void facet_refine_to_length(ManifoldPtr* m, double length, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->RefineToLength(length)), out);
}

void facet_split(ManifoldPtr* m, ManifoldPtr* cutter, FacetSolidPair* out) {
  auto [first, second] = as_cpp(m)->Split(*as_cpp(cutter));
  wrap(new Manifold(std::move(first)),  &out->first);
  wrap(new Manifold(std::move(second)), &out->second);
}

void facet_split_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidPair* out) {
  auto [first, second] = as_cpp(m)->SplitByPlane({nx, ny, nz}, offset);
  wrap(new Manifold(std::move(first)),  &out->first);
  wrap(new Manifold(std::move(second)), &out->second);
}

void facet_compose(ManifoldPtr** manifolds, int n, FacetSolidRet* out) {
  std::vector<Manifold> v;
  v.reserve(n);
  for (int i = 0; i < n; i++) v.push_back(*as_cpp(manifolds[i]));
  wrap(new Manifold(Manifold::Compose(v)), out);
}

// ---------------------------------------------------------------------------
// 3D Measurements
// ---------------------------------------------------------------------------

double facet_volume(ManifoldPtr* m) {
  return as_cpp(m)->Volume();
}

double facet_surface_area(ManifoldPtr* m) {
  return as_cpp(m)->SurfaceArea();
}

int facet_genus(ManifoldPtr* m) {
  return as_cpp(m)->Genus();
}

double facet_min_gap(ManifoldPtr* a, ManifoldPtr* b, double search_length) {
  return as_cpp(a)->MinGap(*as_cpp(b), search_length);
}

void facet_bounding_box(ManifoldPtr* m,
                        double* min_x, double* min_y, double* min_z,
                        double* max_x, double* max_y, double* max_z) {
  Box box = as_cpp(m)->BoundingBox();
  *min_x = box.min.x; *min_y = box.min.y; *min_z = box.min.z;
  *max_x = box.max.x; *max_y = box.max.y; *max_z = box.max.z;
}

int facet_num_components(ManifoldPtr* m) {
  auto comps = as_cpp(m)->Decompose();
  return static_cast<int>(comps.size());
}

size_t facet_solid_size(ManifoldPtr* m) {
  return solid_size(as_cpp(m));
}

size_t facet_sketch_size(ManifoldCrossSection* cs) {
  return sketch_size(as_cpp_cs(cs));
}

// ---------------------------------------------------------------------------
// 2D Measurements
// ---------------------------------------------------------------------------

double facet_cs_area(ManifoldCrossSection* cs) {
  return as_cpp_cs(cs)->Area();
}

void facet_cs_bounds(ManifoldCrossSection* cs,
                     double* min_x, double* min_y, double* max_x, double* max_y) {
  Rect rect = as_cpp_cs(cs)->Bounds();
  *min_x = rect.min.x; *min_y = rect.min.y;
  *max_x = rect.max.x; *max_y = rect.max.y;
}

// ---------------------------------------------------------------------------
// Mesh Extraction
// ---------------------------------------------------------------------------

void facet_extract_mesh(ManifoldPtr* m,
                        float** out_vertices, int* out_num_verts,
                        uint32_t** out_indices, int* out_num_tris) {
  MeshGL mesh = as_cpp(m)->GetMeshGL();
  size_t numVert = mesh.NumVert();
  size_t numTri = mesh.NumTri();
  size_t numProp = mesh.numProp;

  if (numVert == 0 || numTri == 0) {
    *out_vertices = nullptr;
    *out_num_verts = 0;
    *out_indices = nullptr;
    *out_num_tris = 0;
    return;
  }

  // Stride-copy xyz from vertex properties
  float* verts = (float*)malloc(numVert * 3 * sizeof(float));
  for (size_t i = 0; i < numVert; i++) {
    verts[i * 3 + 0] = mesh.vertProperties[i * numProp + 0];
    verts[i * 3 + 1] = mesh.vertProperties[i * numProp + 1];
    verts[i * 3 + 2] = mesh.vertProperties[i * numProp + 2];
  }

  // Copy triangle indices
  uint32_t* idxs = (uint32_t*)malloc(numTri * 3 * sizeof(uint32_t));
  memcpy(idxs, mesh.triVerts.data(), numTri * 3 * sizeof(uint32_t));

  *out_vertices = verts;
  *out_num_verts = (int)numVert;
  *out_indices = idxs;
  *out_num_tris = (int)numTri;
}

void facet_extract_display_mesh(ManifoldPtr* m,
                                float** out_vertices, int* out_num_verts, int* out_num_prop,
                                uint32_t** out_indices, int* out_num_tris,
                                uint32_t** out_face_ids, int* out_num_face_ids) {
  MeshGL mesh = as_cpp(m)->GetMeshGL();
  size_t numVert = mesh.NumVert();
  size_t numTri = mesh.NumTri();
  size_t numProp = mesh.numProp;

  if (numVert == 0 || numTri == 0) {
    *out_vertices = nullptr;
    *out_num_verts = 0;
    *out_num_prop = 0;
    *out_indices = nullptr;
    *out_num_tris = 0;
    *out_face_ids = nullptr;
    *out_num_face_ids = 0;
    return;
  }

  // Copy full vertex properties (Go will stride-copy xyz)
  size_t propLen = numVert * numProp;
  float* props = (float*)malloc(propLen * sizeof(float));
  memcpy(props, mesh.vertProperties.data(), propLen * sizeof(float));

  // Copy triangle indices
  uint32_t* idxs = (uint32_t*)malloc(numTri * 3 * sizeof(uint32_t));
  memcpy(idxs, mesh.triVerts.data(), numTri * 3 * sizeof(uint32_t));

  // Build per-triangle face IDs from runOriginalID (the source of truth for face provenance)
  uint32_t* fids = nullptr;
  int nfids = 0;
  if (!mesh.runOriginalID.empty() && !mesh.runIndex.empty()) {
    nfids = (int)numTri;
    fids = (uint32_t*)malloc(nfids * sizeof(uint32_t));
    size_t numRuns = mesh.runOriginalID.size();
    for (size_t r = 0; r < numRuns; r++) {
      uint32_t origID = mesh.runOriginalID[r];
      size_t startTri = mesh.runIndex[r] / 3;
      size_t endTri = mesh.runIndex[r + 1] / 3;
      for (size_t t = startTri; t < endTri; t++) {
        fids[t] = origID;
      }
    }
  }

  *out_vertices = props;
  *out_num_verts = (int)numVert;
  *out_num_prop = (int)numProp;
  *out_indices = idxs;
  *out_num_tris = (int)numTri;
  *out_face_ids = fids;
  *out_num_face_ids = nfids;
}

void facet_solid_from_mesh(float* verts, size_t n_verts,
                           uint32_t* indices, size_t n_tris, FacetSolidRet* out) {
  MeshGL mesh;
  mesh.numProp = 3;
  mesh.vertProperties.assign(verts, verts + n_verts * 3);
  mesh.triVerts.assign(indices, indices + n_tris * 3);
  wrap_solid_from_mesh(mesh, out);
}

// ---------------------------------------------------------------------------
// Merged Display Mesh Extraction
// ---------------------------------------------------------------------------

void facet_merge_extract_display_mesh(
    ManifoldPtr** solids, size_t count,
    float** out_vertices, int* out_num_verts, int* out_num_prop,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_face_ids, int* out_num_face_ids) {

  // Extract all meshes first to compute totals
  std::vector<MeshGL> meshes(count);
  size_t totalVerts = 0, totalTris = 0, totalFaceIDs = 0;
  size_t commonNumProp = 0;
  bool hasFaceIDs = false;

  for (size_t i = 0; i < count; i++) {
    meshes[i] = as_cpp(solids[i])->GetMeshGL();
    totalVerts += meshes[i].NumVert();
    totalTris += meshes[i].NumTri();
    if (!meshes[i].runOriginalID.empty()) {
      hasFaceIDs = true;
    }
    if (meshes[i].numProp > commonNumProp) {
      commonNumProp = meshes[i].numProp;
    }
  }

  if (totalVerts == 0 || totalTris == 0) {
    *out_vertices = nullptr;
    *out_num_verts = 0;
    *out_num_prop = 0;
    *out_indices = nullptr;
    *out_num_tris = 0;
    *out_face_ids = nullptr;
    *out_num_face_ids = 0;
    return;
  }

  // Output numProp: use max across all meshes (6 if any has color, else 3).
  // Meshes with fewer props get zero-padded (matching Manifold boolean behavior).
  size_t outNumProp = commonNumProp < 3 ? 3 : commonNumProp;

  float* verts = (float*)malloc(totalVerts * outNumProp * sizeof(float));
  uint32_t* idxs = (uint32_t*)malloc(totalTris * 3 * sizeof(uint32_t));
  uint32_t* fids = nullptr;
  if (hasFaceIDs) {
    fids = (uint32_t*)malloc(totalTris * sizeof(uint32_t));
  }

  size_t vertOff = 0, triOff = 0;

  for (size_t i = 0; i < count; i++) {
    auto& mesh = meshes[i];
    size_t nv = mesh.NumVert();
    size_t nt = mesh.NumTri();
    size_t np = mesh.numProp;

    // Copy vertex properties, zero-padding if this mesh has fewer props
    for (size_t v = 0; v < nv; v++) {
      for (size_t p = 0; p < outNumProp; p++) {
        verts[(vertOff + v) * outNumProp + p] = (p < np)
          ? mesh.vertProperties[v * np + p]
          : 0.0f;
      }
    }

    // Copy indices with vertex offset
    for (size_t t = 0; t < nt * 3; t++) {
      idxs[triOff * 3 + t] = mesh.triVerts[t] + (uint32_t)vertOff;
    }

    // Build per-triangle face IDs from runOriginalID.
    // AsOriginal() assigns globally unique IDs, so no offset needed.
    if (hasFaceIDs) {
      if (!mesh.runOriginalID.empty() && !mesh.runIndex.empty()) {
        size_t numRuns = mesh.runOriginalID.size();
        for (size_t r = 0; r < numRuns; r++) {
          uint32_t origID = mesh.runOriginalID[r];
          size_t startTri = mesh.runIndex[r] / 3;
          size_t endTri = mesh.runIndex[r + 1] / 3;
          for (size_t t = startTri; t < endTri; t++) {
            fids[triOff + t] = origID;
          }
        }
      } else {
        // No runs — assign zero (unknown face group)
        for (size_t t = 0; t < nt; t++) {
          fids[triOff + t] = 0;
        }
      }
    }

    vertOff += nv;
    triOff += nt;
  }

  *out_vertices = verts;
  *out_num_verts = (int)totalVerts;
  *out_num_prop = (int)outNumProp;
  *out_indices = idxs;
  *out_num_tris = (int)totalTris;
  *out_face_ids = fids;
  *out_num_face_ids = hasFaceIDs ? (int)totalTris : 0;
}

// ---------------------------------------------------------------------------
// Expanded Mesh Extraction (non-indexed, with edge lines)
// ---------------------------------------------------------------------------

// Helper: extract expanded mesh from a single MeshGL.
static void extract_expanded_from_meshgl(
    const MeshGL& mesh,
    const std::vector<uint32_t>& faceIDs,
    float** out_positions, int* out_num_positions,
    uint32_t** out_face_ids, int* out_num_face_ids,
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg) {

  size_t numTri = mesh.NumTri();
  size_t numProp = mesh.numProp;

  if (numTri == 0) {
    *out_positions = nullptr; *out_num_positions = 0;
    *out_face_ids = nullptr; *out_num_face_ids = 0;
    *out_edge_lines = nullptr; *out_num_edges = 0;
    return;
  }

  // Expand vertices: 3 verts per triangle, 3 floats per vert
  size_t numVerts = numTri * 3;
  float* positions = (float*)malloc(numVerts * 3 * sizeof(float));
  for (size_t t = 0; t < numTri; t++) {
    for (int v = 0; v < 3; v++) {
      uint32_t vi = mesh.triVerts[t * 3 + v];
      positions[(t * 3 + v) * 3 + 0] = mesh.vertProperties[vi * numProp + 0];
      positions[(t * 3 + v) * 3 + 1] = mesh.vertProperties[vi * numProp + 1];
      positions[(t * 3 + v) * 3 + 2] = mesh.vertProperties[vi * numProp + 2];
    }
  }

  // Face IDs (per-triangle)
  uint32_t* fids = nullptr;
  int nfids = 0;
  if (!faceIDs.empty()) {
    nfids = (int)numTri;
    fids = (uint32_t*)malloc(nfids * sizeof(uint32_t));
    memcpy(fids, faceIDs.data(), nfids * sizeof(uint32_t));
  }

  // Edge lines: find edges above threshold angle
  float threshold_rad = edge_threshold_deg * (float)M_PI / 180.0f;
  float cos_threshold = cosf(threshold_rad);

  // Compute per-triangle normals
  struct Vec3 { float x, y, z; };
  auto cross = [](Vec3 a, Vec3 b) -> Vec3 {
    return {a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x};
  };
  auto sub = [](Vec3 a, Vec3 b) -> Vec3 {
    return {a.x-b.x, a.y-b.y, a.z-b.z};
  };
  auto normalize = [](Vec3 v) -> Vec3 {
    float len = sqrtf(v.x*v.x + v.y*v.y + v.z*v.z);
    if (len > 0) { v.x /= len; v.y /= len; v.z /= len; }
    return v;
  };
  auto dot = [](Vec3 a, Vec3 b) -> float {
    return a.x*b.x + a.y*b.y + a.z*b.z;
  };

  std::vector<Vec3> triNormals(numTri);
  for (size_t t = 0; t < numTri; t++) {
    const float* p0 = &positions[t * 9 + 0];
    const float* p1 = &positions[t * 9 + 3];
    const float* p2 = &positions[t * 9 + 6];
    Vec3 v0 = {p0[0], p0[1], p0[2]};
    Vec3 v1 = {p1[0], p1[1], p1[2]};
    Vec3 v2 = {p2[0], p2[1], p2[2]};
    triNormals[t] = normalize(cross(sub(v1, v0), sub(v2, v0)));
  }

  // Build edge adjacency: edge key → (tri0, tri1)
  // Use original indexed vertices for edge matching (shared vertices)
  struct Edge { uint32_t lo, hi; };
  struct EdgeHash {
    size_t operator()(const Edge& e) const {
      return std::hash<uint64_t>()(((uint64_t)e.lo << 32) | e.hi);
    }
  };
  struct EdgeEq {
    bool operator()(const Edge& a, const Edge& b) const {
      return a.lo == b.lo && a.hi == b.hi;
    }
  };
  std::unordered_map<Edge, std::pair<int, int>, EdgeHash, EdgeEq> edgeMap;
  for (size_t t = 0; t < numTri; t++) {
    uint32_t idx[3] = {mesh.triVerts[t*3], mesh.triVerts[t*3+1], mesh.triVerts[t*3+2]};
    for (int e = 0; e < 3; e++) {
      uint32_t a = idx[e], b = idx[(e+1)%3];
      Edge key = {std::min(a,b), std::max(a,b)};
      auto it = edgeMap.find(key);
      if (it == edgeMap.end()) {
        edgeMap[key] = {(int)t, -1};
      } else if (it->second.second == -1) {
        it->second.second = (int)t;
      }
    }
  }

  // Collect edge lines where angle between normals exceeds threshold
  std::vector<float> edgeLines;
  for (auto& [edge, tris] : edgeMap) {
    bool isEdge = false;
    if (tris.second == -1) {
      isEdge = true; // boundary edge
    } else {
      float d = dot(triNormals[tris.first], triNormals[tris.second]);
      if (d < cos_threshold) isEdge = true;
    }
    if (isEdge) {
      size_t np = mesh.numProp;
      const float* a = &mesh.vertProperties[edge.lo * np];
      const float* b = &mesh.vertProperties[edge.hi * np];
      edgeLines.push_back(a[0]); edgeLines.push_back(a[1]); edgeLines.push_back(a[2]);
      edgeLines.push_back(b[0]); edgeLines.push_back(b[1]); edgeLines.push_back(b[2]);
    }
  }

  *out_positions = positions;
  *out_num_positions = (int)numVerts;
  *out_face_ids = fids;
  *out_num_face_ids = nfids;

  if (!edgeLines.empty()) {
    size_t numEdges = edgeLines.size() / 6;
    *out_edge_lines = (float*)malloc(edgeLines.size() * sizeof(float));
    memcpy(*out_edge_lines, edgeLines.data(), edgeLines.size() * sizeof(float));
    *out_num_edges = (int)numEdges;
  } else {
    *out_edge_lines = nullptr;
    *out_num_edges = 0;
  }
}

// Helper: build face IDs from MeshGL run data
static std::vector<uint32_t> buildFaceIDs(const MeshGL& mesh) {
  std::vector<uint32_t> fids;
  size_t numTri = mesh.NumTri();
  if (!mesh.runOriginalID.empty() && !mesh.runIndex.empty()) {
    fids.resize(numTri);
    size_t numRuns = mesh.runOriginalID.size();
    for (size_t r = 0; r < numRuns; r++) {
      uint32_t origID = mesh.runOriginalID[r];
      size_t startTri = mesh.runIndex[r] / 3;
      size_t endTri = mesh.runIndex[r + 1] / 3;
      for (size_t t = startTri; t < endTri; t++) {
        fids[t] = origID;
      }
    }
  }
  return fids;
}

void facet_extract_expanded_mesh(
    ManifoldPtr* m,
    float** out_positions, int* out_num_positions,
    uint32_t** out_face_ids, int* out_num_face_ids,
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg) {

  MeshGL mesh = as_cpp(m)->GetMeshGL();
  auto faceIDs = buildFaceIDs(mesh);
  extract_expanded_from_meshgl(mesh, faceIDs,
      out_positions, out_num_positions,
      out_face_ids, out_num_face_ids,
      out_edge_lines, out_num_edges,
      edge_threshold_deg);
}

void facet_merge_extract_expanded_mesh(
    ManifoldPtr** solids, size_t count,
    float** out_positions, int* out_num_positions,
    uint32_t** out_face_ids, int* out_num_face_ids,
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg) {

  if (count == 0) {
    *out_positions = nullptr; *out_num_positions = 0;
    *out_face_ids = nullptr; *out_num_face_ids = 0;
    *out_edge_lines = nullptr; *out_num_edges = 0;
    return;
  }

  if (count == 1) {
    facet_extract_expanded_mesh(solids[0],
        out_positions, out_num_positions,
        out_face_ids, out_num_face_ids,
        out_edge_lines, out_num_edges,
        edge_threshold_deg);
    return;
  }

  // Extract all meshes, compute totals
  std::vector<MeshGL> meshes(count);
  size_t totalTris = 0;
  bool hasFaceIDs = false;
  for (size_t i = 0; i < count; i++) {
    meshes[i] = as_cpp(solids[i])->GetMeshGL();
    totalTris += meshes[i].NumTri();
    if (!meshes[i].runOriginalID.empty()) hasFaceIDs = true;
  }

  // Expand all into one buffer
  size_t totalVerts = totalTris * 3;
  float* positions = (float*)malloc(totalVerts * 3 * sizeof(float));
  uint32_t* fids = hasFaceIDs ? (uint32_t*)malloc(totalTris * sizeof(uint32_t)) : nullptr;
  std::vector<float> allEdgeLines;

  float threshold_rad = edge_threshold_deg * (float)M_PI / 180.0f;
  float cos_threshold = cosf(threshold_rad);

  size_t vertOff = 0, triOff = 0;
  for (size_t i = 0; i < count; i++) {
    auto& mesh = meshes[i];
    size_t numTri = mesh.NumTri();
    size_t numProp = mesh.numProp;

    // Expand vertices
    for (size_t t = 0; t < numTri; t++) {
      for (int v = 0; v < 3; v++) {
        uint32_t vi = mesh.triVerts[t * 3 + v];
        positions[(vertOff + t * 3 + v) * 3 + 0] = mesh.vertProperties[vi * numProp + 0];
        positions[(vertOff + t * 3 + v) * 3 + 1] = mesh.vertProperties[vi * numProp + 1];
        positions[(vertOff + t * 3 + v) * 3 + 2] = mesh.vertProperties[vi * numProp + 2];
      }
    }

    // Face IDs
    if (fids) {
      auto meshFids = buildFaceIDs(mesh);
      if (!meshFids.empty()) {
        memcpy(fids + triOff, meshFids.data(), numTri * sizeof(uint32_t));
      } else {
        memset(fids + triOff, 0, numTri * sizeof(uint32_t));
      }
    }

    // Edge lines for this sub-mesh
    // Compute normals from expanded positions
    struct Vec3 { float x, y, z; };
    auto cross = [](Vec3 a, Vec3 b) -> Vec3 {
      return {a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x};
    };
    auto sub = [](Vec3 a, Vec3 b) -> Vec3 { return {a.x-b.x, a.y-b.y, a.z-b.z}; };
    auto normalize = [](Vec3 v) -> Vec3 {
      float len = sqrtf(v.x*v.x + v.y*v.y + v.z*v.z);
      if (len > 0) { v.x /= len; v.y /= len; v.z /= len; }
      return v;
    };
    auto dot = [](Vec3 a, Vec3 b) -> float { return a.x*b.x + a.y*b.y + a.z*b.z; };

    std::vector<Vec3> triNormals(numTri);
    for (size_t t = 0; t < numTri; t++) {
      size_t base = (vertOff + t * 3) * 3;
      Vec3 v0 = {positions[base], positions[base+1], positions[base+2]};
      Vec3 v1 = {positions[base+3], positions[base+4], positions[base+5]};
      Vec3 v2 = {positions[base+6], positions[base+7], positions[base+8]};
      triNormals[t] = normalize(cross(sub(v1, v0), sub(v2, v0)));
    }

    struct Edge { uint32_t lo, hi; };
    struct EdgeHash {
      size_t operator()(const Edge& e) const {
        return std::hash<uint64_t>()(((uint64_t)e.lo << 32) | e.hi);
      }
    };
    struct EdgeEq {
      bool operator()(const Edge& a, const Edge& b) const {
        return a.lo == b.lo && a.hi == b.hi;
      }
    };
    std::unordered_map<Edge, std::pair<int,int>, EdgeHash, EdgeEq> edgeMap;
    for (size_t t = 0; t < numTri; t++) {
      uint32_t idx[3] = {mesh.triVerts[t*3], mesh.triVerts[t*3+1], mesh.triVerts[t*3+2]};
      for (int e = 0; e < 3; e++) {
        uint32_t a = idx[e], b = idx[(e+1)%3];
        Edge key = {std::min(a,b), std::max(a,b)};
        auto it = edgeMap.find(key);
        if (it == edgeMap.end()) edgeMap[key] = {(int)t, -1};
        else if (it->second.second == -1) it->second.second = (int)t;
      }
    }
    for (auto& [edge, tris] : edgeMap) {
      bool isEdge = false;
      if (tris.second == -1) isEdge = true;
      else {
        float d = dot(triNormals[tris.first], triNormals[tris.second]);
        if (d < cos_threshold) isEdge = true;
      }
      if (isEdge) {
        size_t np = mesh.numProp;
        const float* a = &mesh.vertProperties[edge.lo * np];
        const float* b = &mesh.vertProperties[edge.hi * np];
        allEdgeLines.push_back(a[0]); allEdgeLines.push_back(a[1]); allEdgeLines.push_back(a[2]);
        allEdgeLines.push_back(b[0]); allEdgeLines.push_back(b[1]); allEdgeLines.push_back(b[2]);
      }
    }

    vertOff += numTri * 3;
    triOff += numTri;
  }

  *out_positions = positions;
  *out_num_positions = (int)totalVerts;
  *out_face_ids = fids;
  *out_num_face_ids = hasFaceIDs ? (int)totalTris : 0;

  if (!allEdgeLines.empty()) {
    size_t numEdges = allEdgeLines.size() / 6;
    *out_edge_lines = (float*)malloc(allEdgeLines.size() * sizeof(float));
    memcpy(*out_edge_lines, allEdgeLines.data(), allEdgeLines.size() * sizeof(float));
    *out_num_edges = (int)numEdges;
  } else {
    *out_edge_lines = nullptr;
    *out_num_edges = 0;
  }
}

// ---------------------------------------------------------------------------
// Callback operations
// ---------------------------------------------------------------------------

void facet_warp(ManifoldPtr* m, int callback_id, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->Warp([callback_id](vec3& v) {
    double x = v.x, y = v.y, z = v.z;
    facetWarpBridge(callback_id, &x, &y, &z);
    v.x = x; v.y = y; v.z = z;
  })), out);
}

void facet_level_set(int callback_id,
                     double min_x, double min_y, double min_z,
                     double max_x, double max_y, double max_z,
                     double edge_length, FacetSolidRet* out) {
  Box bounds{vec3{min_x, min_y, min_z}, vec3{max_x, max_y, max_z}};
  wrap(new Manifold(Manifold::LevelSet(
      [callback_id](vec3 v) -> double {
        return facetLevelSetBridge(callback_id, v.x, v.y, v.z);
      },
      bounds, edge_length, 0.0, -1.0, false).AsOriginal()), out);
}

// ---------------------------------------------------------------------------
// OriginalID tracking
// ---------------------------------------------------------------------------

int facet_original_id(ManifoldPtr* m) {
  return as_cpp(m)->OriginalID();
}

void facet_as_original(ManifoldPtr* m, FacetSolidRet* out) {
  wrap(new Manifold(as_cpp(m)->AsOriginal()), out);
}

void facet_extract_mesh_with_runs(ManifoldPtr* m,
    float** out_vertices, int* out_num_verts,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_run_original_id, uint32_t** out_run_index,
    int* out_num_runs, int* out_num_run_index) {

  MeshGL mesh = as_cpp(m)->GetMeshGL();

  int nv = mesh.NumVert();
  int nt = mesh.NumTri();
  int np = mesh.numProp;

  // Empty mesh: allocate nothing and null every out-pointer, so the Go caller's
  // early-return frees nothing (mirrors facet_extract_mesh and the other
  // extractors). Without this the unconditional malloc below escapes unfreed
  // when exporting an empty solid.
  if (nv == 0 || nt == 0) {
    *out_vertices = nullptr;
    *out_num_verts = 0;
    *out_indices = nullptr;
    *out_num_tris = 0;
    *out_run_original_id = nullptr;
    *out_run_index = nullptr;
    *out_num_runs = 0;
    *out_num_run_index = 0;
    return;
  }

  *out_num_verts = nv;
  *out_num_tris = nt;

  // Extract positions (first 3 props per vertex)
  *out_vertices = (float*)malloc(nv * 3 * sizeof(float));
  for (int i = 0; i < nv; i++) {
    (*out_vertices)[i*3+0] = mesh.vertProperties[i*np+0];
    (*out_vertices)[i*3+1] = mesh.vertProperties[i*np+1];
    (*out_vertices)[i*3+2] = mesh.vertProperties[i*np+2];
  }

  // Copy triangle indices
  *out_indices = (uint32_t*)malloc(nt * 3 * sizeof(uint32_t));
  memcpy(*out_indices, mesh.triVerts.data(), nt * 3 * sizeof(uint32_t));

  // Copy run info
  int numRuns = (int)mesh.runOriginalID.size();
  *out_num_runs = numRuns;

  if (numRuns > 0) {
    *out_run_original_id = (uint32_t*)malloc(numRuns * sizeof(uint32_t));
    memcpy(*out_run_original_id, mesh.runOriginalID.data(), numRuns * sizeof(uint32_t));

    // runIndex normally has numRuns+1 entries (last entry = total triVerts size);
    // report its actual size so the caller never assumes the length.
    int riLen = (int)mesh.runIndex.size();
    *out_num_run_index = riLen;
    *out_run_index = (uint32_t*)malloc(riLen * sizeof(uint32_t));
    memcpy(*out_run_index, mesh.runIndex.data(), riLen * sizeof(uint32_t));
  } else {
    *out_run_original_id = nullptr;
    *out_run_index = nullptr;
    *out_num_run_index = 0;
  }
}

}  // extern "C"
