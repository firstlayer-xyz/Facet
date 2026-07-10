#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <cmath>
#ifndef M_PI
#define M_PI 3.14159265358979323846
#endif
#include <limits>
#include <vector>

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, as_cpp_cs, wrap, wrap_cs, facetClear

namespace {
constexpr double kFourPi = 4.0 * M_PI;

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
// 3D Hulls
// ---------------------------------------------------------------------------

void facet_hull(ManifoldPtr* m, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Hull().AsOriginal()), out);
} catch (...) { facetClear(out); }

void facet_batch_hull(ManifoldPtr** solids, size_t count, FacetSolidRet* out) try {
  std::vector<Manifold> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp(solids[i]);
  }
  wrap(new Manifold(Manifold::Hull(vec).AsOriginal()), out);
} catch (...) { facetClear(out); }

void facet_hull_points(double* xyz, size_t n_points, FacetSolidRet* out) try {
  std::vector<vec3> pts(n_points);
  for (size_t i = 0; i < n_points; i++) {
    pts[i] = {xyz[i * 3], xyz[i * 3 + 1], xyz[i * 3 + 2]};
  }
  wrap(new Manifold(Manifold::Hull(pts).AsOriginal()), out);
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// 2D Hulls
// ---------------------------------------------------------------------------

void facet_cs_hull(ManifoldCrossSection* cs, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(as_cpp_cs(cs)->Hull()), out);
} catch (...) { facetClear(out); }

void facet_cs_batch_hull(ManifoldCrossSection** sketches, size_t count, FacetSketchRet* out) try {
  std::vector<CrossSection> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp_cs(sketches[i]);
  }
  wrap_cs(new CrossSection(CrossSection::Hull(vec)), out);
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

void facet_trim_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->TrimByPlane({nx, ny, nz}, offset)), out);
} catch (...) { facetClear(out); }

void facet_smooth_out(ManifoldPtr* m, double min_sharp_angle, double min_smoothness, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->SmoothOut(min_sharp_angle, min_smoothness)), out);
} catch (...) { facetClear(out); }

void facet_refine(ManifoldPtr* m, int n, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Refine(n)), out);
} catch (...) { facetClear(out); }

void facet_simplify(ManifoldPtr* m, double tolerance, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Simplify(tolerance)), out);
} catch (...) { facetClear(out); }

void facet_offset(ManifoldPtr* m, double delta, double edge_length, FacetSolidRet* out) try {
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
} catch (...) { facetClear(out); }

void facet_refine_to_length(ManifoldPtr* m, double length, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->RefineToLength(length)), out);
} catch (...) { facetClear(out); }

// Both halves are held in unique_ptr until BOTH wraps succeed, then released to
// Go. wrap() sets out->ptr before forcing lazy evaluation (solid_size), which
// can itself throw, so owning both halves to the end means any throw — from
// `new Manifold` or from either wrap — unwinds with the heap objects still owned
// and deleted, instead of leaking the first half when the second throws.
void facet_split(ManifoldPtr* m, ManifoldPtr* cutter, FacetSolidPair* out) try {
  auto [first, second] = as_cpp(m)->Split(*as_cpp(cutter));
  auto a = std::make_unique<Manifold>(std::move(first));
  auto b = std::make_unique<Manifold>(std::move(second));
  wrap(a.get(), &out->first);
  wrap(b.get(), &out->second);
  a.release();
  b.release();
} catch (...) { facetClear(out); }

void facet_split_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidPair* out) try {
  auto [first, second] = as_cpp(m)->SplitByPlane({nx, ny, nz}, offset);
  auto a = std::make_unique<Manifold>(std::move(first));
  auto b = std::make_unique<Manifold>(std::move(second));
  wrap(a.get(), &out->first);
  wrap(b.get(), &out->second);
  a.release();
  b.release();
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// OriginalID tracking
// ---------------------------------------------------------------------------

void facet_as_original(ManifoldPtr* m, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->AsOriginal()), out);
} catch (...) { facetClear(out); }

}  // extern "C"
