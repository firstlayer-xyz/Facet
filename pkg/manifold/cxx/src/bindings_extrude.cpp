#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"
#include "manifold/polygon.h"

#include <cmath>
#include <vector>

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, as_cpp_cs, wrap, wrap_cs, wrap_solid_from_mesh, facetClear

extern "C" {

// ---------------------------------------------------------------------------
// 2D → 3D
// ---------------------------------------------------------------------------

void facet_extrude(ManifoldCrossSection* cs, double height, int slices,
                   double twist, double scale_x, double scale_y, FacetSolidRet* out) try {
  auto polys = as_cpp_cs(cs)->ToPolygons();
  wrap(new Manifold(Manifold::Extrude(polys, height, slices, twist, {scale_x, scale_y}).AsOriginal()), out);
} catch (...) { facetClear(out); }

void facet_revolve(ManifoldCrossSection* cs, int segments, double degrees, FacetSolidRet* out) try {
  auto polys = as_cpp_cs(cs)->ToPolygons();
  wrap(new Manifold(Manifold::Revolve(polys, segments, degrees).AsOriginal()), out);
} catch (...) { facetClear(out); }

void facet_sweep(ManifoldCrossSection* cs,
                 double* path_xyz, size_t n_path_points, FacetSolidRet* out) try {
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
    // TriangulateIdx throws geometryErr on a self-intersecting profile; a C++
    // exception crossing this extern "C" boundary into Go is undefined behavior.
    // Catch it and return an empty manifold, matching the degenerate-input
    // guards above (Go reads a null/empty result as a failed sweep).
    std::vector<ivec3> capTris;
    try {
      capTris = TriangulateIdx(polysIdx);
    } catch (...) {
      wrap(new Manifold(), out);
      return;
    }
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
  wrap_solid_from_mesh(mesh, out);
} catch (...) { facetClear(out); }

void facet_loft(ManifoldCrossSection** sketches, size_t n_sketches,
                double* heights, size_t n_heights, FacetSolidRet* out) try {
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
      // Bound the index before reading edgeLens[edgeIdx]; floating-point
      // arc-length accumulation can overshoot on the final point, so clamp to
      // the last edge rather than wrapping (which would teleport the point to
      // the contour start).
      while (edgeIdx + 1 < n && walked + edgeLens[edgeIdx] - edgeWalked < target - 1e-12) {
        walked += edgeLens[edgeIdx] - edgeWalked;
        edgeIdx++;
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

  // Oversample each ring. Angular correspondence (below) samples at uniform
  // angles, but a high-curvature feature (a rounded-rect corner) spans a small
  // angular range, so at the raw vertex count it collapses to a chamfer.
  // Sampling several times denser resolves corners back into smooth arcs while
  // keeping every ring's count identical (so the correspondence stays exact).
  for (auto& t : contourTargets) {
    if (t == 0) continue;
    size_t dense = t * 6;
    if (dense > 1024) dense = 1024;
    t = dense;
  }

  // Total vertices per ring.
  size_t ringVerts = 0;
  for (auto t : contourTargets) ringVerts += t;
  if (ringVerts == 0) { wrap(new Manifold(), out); return; }

  // resampleContourAngular places targetN points where rays cast from the
  // contour's centroid at uniform angles cross the contour. Unlike arc-length,
  // this aligns the same angular feature across every ring (a corner at angle θ
  // maps to the corner at θ on the next ring), so lofting profiles whose corners
  // occupy different fractions of the perimeter — rounded rects of different
  // radii, a tapered foot — no longer twists. Sampling follows the contour's
  // winding (CCW vs CW hole). Returns false if the contour is not star-shaped
  // from its centroid (a ray misses), so the caller can fall back to arc-length.
  const double kPi = 3.14159265358979323846;
  auto resampleContourAngular = [&](const SimplePolygon& contour,
                                    size_t targetN) -> std::pair<SimplePolygon, bool> {
    size_t n = contour.size();
    if (n < 3 || targetN < 3) return {{}, false};
    vec2 ctr = {0, 0};
    double area2 = 0;
    for (size_t e = 0; e < n; e++) {
      const vec2& A = contour[e];
      const vec2& B = contour[(e + 1) % n];
      ctr.x += A.x;
      ctr.y += A.y;
      area2 += A.x * B.y - B.x * A.y;
    }
    ctr.x /= (double)n;
    ctr.y /= (double)n;
    double sign = (area2 < 0) ? -1.0 : 1.0;  // match the contour's winding
    SimplePolygon result;
    result.reserve(targetN);
    for (size_t i = 0; i < targetN; i++) {
      double theta = sign * 2.0 * kPi * (double)i / (double)targetN;
      vec2 dir = {std::cos(theta), std::sin(theta)};
      double bestT = -1.0;
      vec2 bestP = {0, 0};
      for (size_t e = 0; e < n; e++) {
        const vec2& A = contour[e];
        const vec2& B = contour[(e + 1) % n];
        vec2 edge = {B.x - A.x, B.y - A.y};
        vec2 w = {A.x - ctr.x, A.y - ctr.y};
        double dxe = dir.x * edge.y - dir.y * edge.x;  // dir × edge
        if (std::abs(dxe) < 1e-12) continue;           // ray parallel to edge
        double tpar = (w.x * edge.y - w.y * edge.x) / dxe;  // (w × edge)/(dir × edge)
        double spar = (w.x * dir.y - w.y * dir.x) / dxe;    // (w × dir)/(dir × edge)
        if (tpar >= -1e-9 && spar >= -1e-9 && spar <= 1.0 + 1e-9 && tpar > bestT) {
          bestT = tpar;
          bestP = {ctr.x + tpar * dir.x, ctr.y + tpar * dir.y};
        }
      }
      if (bestT < 0) return {{}, false};  // ray missed → not star-shaped
      result.push_back(bestP);
    }
    return {result, true};
  };

  // Resample all sketches so each has maxContours contours with matching vertex
  // counts. Per contour index, prefer angular correspondence (used for every
  // ring or none, so the rings stay consistent); fall back to arc-length when
  // any ring's contour is missing or not star-shaped.
  std::vector<std::vector<SimplePolygon>> resampled(n_sketches);
  for (size_t s = 0; s < n_sketches; s++) resampled[s].resize(maxContours);

  for (size_t c = 0; c < maxContours; c++) {
    size_t targetN = contourTargets[c];
    std::vector<SimplePolygon> angular(n_sketches);
    bool allAngular = true;
    for (size_t s = 0; s < n_sketches; s++) {
      if (c >= allPolys[s].size()) { allAngular = false; break; }
      auto r = resampleContourAngular(allPolys[s][c], targetN);
      if (!r.second) { allAngular = false; break; }
      angular[s] = std::move(r.first);
    }
    for (size_t s = 0; s < n_sketches; s++) {
      if (allAngular) {
        resampled[s][c] = angular[s];
      } else if (c < allPolys[s].size()) {
        resampled[s][c] = resampleContour(allPolys[s][c], targetN);
      } else {
        // Degenerate: fill a missing contour with the sketch's centroid.
        vec2 centroid = {0, 0};
        size_t totalPts = 0;
        for (auto& poly : allPolys[s]) {
          for (auto& v : poly) { centroid.x += v.x; centroid.y += v.y; totalPts++; }
        }
        if (totalPts > 0) { centroid.x /= totalPts; centroid.y /= totalPts; }
        resampled[s][c] = SimplePolygon(targetN, centroid);
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
    // TriangulateIdx throws geometryErr on a self-intersecting profile; a C++
    // exception crossing this extern "C" boundary into Go is undefined behavior.
    // Catch it and return an empty manifold, matching the degenerate-input
    // guards above (Go reads a null/empty result as a failed loft).
    std::vector<ivec3> capTris;
    try {
      capTris = TriangulateIdx(polysIdx);
    } catch (...) {
      wrap(new Manifold(), out);
      return;
    }
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
  wrap_solid_from_mesh(mesh, out);
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

void facet_slice(ManifoldPtr* m, double height, FacetSketchRet* out) try {
  auto polys = as_cpp(m)->Slice(height);
  wrap_cs(new CrossSection(CrossSection(polys, CrossSection::FillRule::Positive)), out);
} catch (...) { facetClear(out); }

void facet_project(ManifoldPtr* m, FacetSketchRet* out) try {
  auto polys = as_cpp(m)->Project();
  wrap_cs(new CrossSection(CrossSection(polys, CrossSection::FillRule::Positive)), out);
} catch (...) { facetClear(out); }

}  // extern "C"
