#include "facet_cxx.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"
#include "manifold/polygon.h"
#include "manifold/meshIO.h"

#include <cmath>
#include <cstdlib>
#include <cstring>
#include <vector>

using namespace manifold;

static Manifold* as_cpp(ManifoldPtr* m) {
  return reinterpret_cast<Manifold*>(m);
}
static ManifoldPtr* as_c(Manifold* m) {
  return reinterpret_cast<ManifoldPtr*>(m);
}
static CrossSection* as_cpp_cs(ManifoldCrossSection* cs) {
  return reinterpret_cast<CrossSection*>(cs);
}
static ManifoldCrossSection* as_c_cs(CrossSection* cs) {
  return reinterpret_cast<ManifoldCrossSection*>(cs);
}

extern "C" {

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

void facet_delete_solid(ManifoldPtr* m) { delete as_cpp(m); }
void facet_delete_sketch(ManifoldCrossSection* cs) { delete as_cpp_cs(cs); }

size_t facet_solid_memory_size(ManifoldPtr* m) {
  auto* cpp = as_cpp(m);
  size_t nv = cpp->NumVert();
  size_t nt = cpp->NumTri();
  size_t np = cpp->NumProp();
  return nv * (24 + np * 8) + nt * 108;
}

size_t facet_sketch_memory_size(ManifoldCrossSection* cs) {
  auto* cpp = as_cpp_cs(cs);
  return cpp->NumVert() * 16 + cpp->NumContour() * 24;
}

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

ManifoldPtr* facet_cube(double x, double y, double z) {
  return as_c(new Manifold(Manifold::Cube({x, y, z}, false).AsOriginal()));
}

ManifoldPtr* facet_sphere(double radius, int segments) {
  return as_c(new Manifold(Manifold::Sphere(radius, segments).AsOriginal()));
}

ManifoldPtr* facet_cylinder(double height, double radius_low, double radius_high, int segments) {
  return as_c(new Manifold(Manifold::Cylinder(height, radius_low, radius_high, segments).AsOriginal()));
}

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_square(double x, double y) {
  return as_c_cs(new CrossSection(CrossSection::Square({x, y}, true)));
}

ManifoldCrossSection* facet_circle(double radius, int segments) {
  return as_c_cs(new CrossSection(CrossSection::Circle(radius, segments)));
}

ManifoldCrossSection* facet_polygon(double* xy_pairs, size_t n_points) {
  SimplePolygon poly(n_points);
  for (size_t i = 0; i < n_points; i++) {
    poly[i] = {xy_pairs[i * 2], xy_pairs[i * 2 + 1]};
  }
  return as_c_cs(new CrossSection(CrossSection({poly}, CrossSection::FillRule::Positive)));
}

ManifoldCrossSection* facet_cs_empty(void) {
  return as_c_cs(new CrossSection());
}

// ---------------------------------------------------------------------------
// 3D Booleans
// ---------------------------------------------------------------------------

ManifoldPtr* facet_union(ManifoldPtr* a, ManifoldPtr* b) {
  return as_c(new Manifold(*as_cpp(a) + *as_cpp(b)));
}

ManifoldPtr* facet_difference(ManifoldPtr* a, ManifoldPtr* b) {
  return as_c(new Manifold(*as_cpp(a) - *as_cpp(b)));
}

ManifoldPtr* facet_intersection(ManifoldPtr* a, ManifoldPtr* b) {
  return as_c(new Manifold(*as_cpp(a) ^ *as_cpp(b)));
}

ManifoldPtr* facet_insert(ManifoldPtr* a, ManifoldPtr* b) {
  Manifold diff = *as_cpp(a) - *as_cpp(b);
  auto components = diff.Decompose();
  Manifold pierced;
  if (components.size() <= 1) {
    pierced = std::move(diff);
  } else {
    Box b_bbox = as_cpp(b)->BoundingBox();
    std::vector<Manifold> outer;
    for (auto& comp : components) {
      if (!b_bbox.Contains(comp.BoundingBox()))
        outer.push_back(std::move(comp));
    }
    pierced = outer.empty() ? std::move(diff) : Manifold::Compose(outer);
  }
  return as_c(new Manifold(pierced + *as_cpp(b)));
}

// Returns count of connected components; fills *out_components with a malloc'd
// array of ManifoldPtr* (one per component). Caller must free each
// element with facet_delete_solid, then free(*out_components).
int facet_decompose(ManifoldPtr* m, ManifoldPtr*** out_components) {
  auto components = as_cpp(m)->Decompose();
  int n = (int)components.size();
  if (n == 0) {
    *out_components = nullptr;
    return 0;
  }
  ManifoldPtr** arr = (ManifoldPtr**)malloc(n * sizeof(ManifoldPtr*));
  for (int i = 0; i < n; i++)
    arr[i] = as_c(new Manifold(std::move(components[i])));
  *out_components = arr;
  return n;
}

// ---------------------------------------------------------------------------
// 2D Booleans
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_union(ManifoldCrossSection* a, ManifoldCrossSection* b) {
  return as_c_cs(new CrossSection(*as_cpp_cs(a) + *as_cpp_cs(b)));
}

ManifoldCrossSection* facet_cs_difference(ManifoldCrossSection* a, ManifoldCrossSection* b) {
  return as_c_cs(new CrossSection(*as_cpp_cs(a) - *as_cpp_cs(b)));
}

ManifoldCrossSection* facet_cs_intersection(ManifoldCrossSection* a, ManifoldCrossSection* b) {
  return as_c_cs(new CrossSection(*as_cpp_cs(a) ^ *as_cpp_cs(b)));
}

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

ManifoldPtr* facet_translate(ManifoldPtr* m, double x, double y, double z) {
  return as_c(new Manifold(as_cpp(m)->Translate({x, y, z})));
}

ManifoldPtr* facet_rotate(ManifoldPtr* m, double x, double y, double z) {
  return as_c(new Manifold(as_cpp(m)->Rotate(x, y, z)));
}

ManifoldPtr* facet_scale(ManifoldPtr* m, double x, double y, double z) {
  return as_c(new Manifold(as_cpp(m)->Scale({x, y, z})));
}

ManifoldPtr* facet_mirror(ManifoldPtr* m, double nx, double ny, double nz) {
  return as_c(new Manifold(as_cpp(m)->Mirror({nx, ny, nz})));
}

// Pivot operations — translate-op-translate fused into a single C++ call.

// Scale pivoting at the bounding box min corner (bottom-left-front stays fixed).
ManifoldPtr* facet_scale_local(ManifoldPtr* m, double x, double y, double z) {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double mx = bb.min.x, my = bb.min.y, mz = bb.min.z;
  Manifold result = src->Translate({-mx, -my, -mz}).Scale({x, y, z}).Translate({mx, my, mz});
  return as_c(new Manifold(std::move(result)));
}

// Rotate pivoting at the bounding box center.
ManifoldPtr* facet_rotate_local(ManifoldPtr* m, double x, double y, double z) {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  double cz = (bb.min.z + bb.max.z) * 0.5;
  Manifold result = src->Translate({-cx, -cy, -cz}).Rotate(x, y, z).Translate({cx, cy, cz});
  return as_c(new Manifold(std::move(result)));
}

// Mirror pivoting at the bounding box center.
ManifoldPtr* facet_mirror_local(ManifoldPtr* m, double nx, double ny, double nz) {
  Manifold* src = as_cpp(m);
  auto bb = src->BoundingBox();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  double cz = (bb.min.z + bb.max.z) * 0.5;
  Manifold result = src->Translate({-cx, -cy, -cz}).Mirror({nx, ny, nz}).Translate({cx, cy, cz});
  return as_c(new Manifold(std::move(result)));
}

// Rotate by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
ManifoldPtr* facet_rotate_at(ManifoldPtr* m, double rx, double ry, double rz, double ox, double oy, double oz) {
  Manifold result = as_cpp(m)->Translate({-ox, -oy, -oz}).Rotate(rx, ry, rz).Translate({ox, oy, oz});
  return as_c(new Manifold(std::move(result)));
}

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_translate(ManifoldCrossSection* cs, double x, double y) {
  return as_c_cs(new CrossSection(as_cpp_cs(cs)->Translate({x, y})));
}

ManifoldCrossSection* facet_cs_rotate(ManifoldCrossSection* cs, double degrees) {
  return as_c_cs(new CrossSection(as_cpp_cs(cs)->Rotate(degrees)));
}

ManifoldCrossSection* facet_cs_scale(ManifoldCrossSection* cs, double x, double y) {
  return as_c_cs(new CrossSection(as_cpp_cs(cs)->Scale({x, y})));
}

ManifoldCrossSection* facet_cs_mirror(ManifoldCrossSection* cs, double ax, double ay) {
  return as_c_cs(new CrossSection(as_cpp_cs(cs)->Mirror({ax, ay})));
}

// Rotate sketch pivoting at the bounding box center.
ManifoldCrossSection* facet_cs_rotate_local(ManifoldCrossSection* cs, double degrees) {
  CrossSection* src = as_cpp_cs(cs);
  auto bb = src->Bounds();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  CrossSection result = src->Translate({-cx, -cy}).Rotate(degrees).Translate({cx, cy});
  return as_c_cs(new CrossSection(std::move(result)));
}

// Mirror sketch pivoting at the bounding box center.
ManifoldCrossSection* facet_cs_mirror_local(ManifoldCrossSection* cs, double ax, double ay) {
  CrossSection* src = as_cpp_cs(cs);
  auto bb = src->Bounds();
  double cx = (bb.min.x + bb.max.x) * 0.5;
  double cy = (bb.min.y + bb.max.y) * 0.5;
  CrossSection result = src->Translate({-cx, -cy}).Mirror({ax, ay}).Translate({cx, cy});
  return as_c_cs(new CrossSection(std::move(result)));
}

ManifoldCrossSection* facet_cs_offset(ManifoldCrossSection* cs, double delta, int segments) {
  return as_c_cs(new CrossSection(as_cpp_cs(cs)->Offset(delta, CrossSection::JoinType::Round, 2.0, segments)));
}

// ---------------------------------------------------------------------------
// 2D → 3D
// ---------------------------------------------------------------------------

ManifoldPtr* facet_extrude(ManifoldCrossSection* cs, double height, int slices,
                                double twist, double scale_x, double scale_y) {
  auto polys = as_cpp_cs(cs)->ToPolygons();
  return as_c(new Manifold(Manifold::Extrude(polys, height, slices, twist, {scale_x, scale_y}).AsOriginal()));
}

ManifoldPtr* facet_revolve(ManifoldCrossSection* cs, int segments, double degrees) {
  auto polys = as_cpp_cs(cs)->ToPolygons();
  return as_c(new Manifold(Manifold::Revolve(polys, segments, degrees).AsOriginal()));
}

ManifoldPtr* facet_sweep(ManifoldCrossSection* cs,
                              double* path_xyz, size_t n_path_points) {
  if (n_path_points < 2) return as_c(new Manifold());

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
  if (nCS == 0) return as_c(new Manifold());
  size_t nPath = n_path_points;

  // Read path points.
  std::vector<vec3> path(nPath);
  for (size_t i = 0; i < nPath; i++) {
    path[i] = {path_xyz[i*3], path_xyz[i*3+1], path_xyz[i*3+2]};
  }

  // Compute tangent vectors at each path point.
  std::vector<vec3> tangents(nPath);
  for (size_t i = 0; i < nPath; i++) {
    vec3 t;
    if (i == 0) {
      t = path[1] - path[0];
    } else if (i == nPath - 1) {
      t = path[nPath-1] - path[nPath-2];
    } else {
      t = path[i+1] - path[i-1];
    }
    double len = std::sqrt(t.x*t.x + t.y*t.y + t.z*t.z);
    if (len > 0) t = t / len;
    tangents[i] = t;
  }

  // Compute rotation-minimizing frames (tangent, normal, binormal).
  std::vector<vec3> normals(nPath);
  std::vector<vec3> binormals(nPath);

  // Initial normal: find a vector not parallel to the first tangent.
  {
    vec3 t0 = tangents[0];
    vec3 up = {0, 0, 1};
    if (std::abs(t0.x*up.x + t0.y*up.y + t0.z*up.z) > 0.9) {
      up = {0, 1, 0};
    }
    // binormal = normalize(cross(t0, up))
    vec3 b = {t0.y*up.z - t0.z*up.y,
              t0.z*up.x - t0.x*up.z,
              t0.x*up.y - t0.y*up.x};
    double blen = std::sqrt(b.x*b.x + b.y*b.y + b.z*b.z);
    if (blen > 0) b = b / blen;
    // normal = cross(b, t0)
    vec3 n = {b.y*t0.z - b.z*t0.y,
              b.z*t0.x - b.x*t0.z,
              b.x*t0.y - b.y*t0.x};
    normals[0] = n;
    binormals[0] = b;
  }

  // Propagate frames using rotation-minimizing approach:
  // project previous normal onto plane perpendicular to current tangent.
  for (size_t i = 1; i < nPath; i++) {
    vec3 t = tangents[i];
    vec3 prevN = normals[i-1];
    // Remove component along tangent: n = prevN - dot(prevN, t) * t
    double d = prevN.x*t.x + prevN.y*t.y + prevN.z*t.z;
    vec3 n = {prevN.x - d*t.x, prevN.y - d*t.y, prevN.z - d*t.z};
    double nlen = std::sqrt(n.x*n.x + n.y*n.y + n.z*n.z);
    if (nlen > 1e-12) {
      n = n / nlen;
    } else {
      // Degenerate: use previous normal (path doubles back)
      n = normals[i-1];
    }
    // binormal = cross(t, n)
    vec3 b = {t.y*n.z - t.z*n.y,
              t.z*n.x - t.x*n.z,
              t.x*n.y - t.y*n.x};
    normals[i] = n;
    binormals[i] = b;
  }

  // Build vertex positions: for each path point, place cross-section in 3D.
  // Vertex layout: path_index * nCS + cs_index
  std::vector<float> vertProps(nPath * nCS * 3);
  for (size_t pi = 0; pi < nPath; pi++) {
    vec3 p = path[pi];
    vec3 n = normals[pi];
    vec3 b = binormals[pi];
    for (size_t ci = 0; ci < nCS; ci++) {
      vec2 cv = csVerts[ci];
      // 3D position = path_point + cv.x * normal + cv.y * binormal
      float x = (float)(p.x + cv.x * n.x + cv.y * b.x);
      float y = (float)(p.y + cv.x * n.y + cv.y * b.y);
      float z = (float)(p.z + cv.x * n.z + cv.y * b.z);
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
  return as_c(new Manifold(Manifold(mesh).AsOriginal()));
}

ManifoldPtr* facet_loft(ManifoldCrossSection** sketches, size_t n_sketches,
                             double* heights, size_t n_heights) {
  if (n_sketches < 2 || n_sketches != n_heights) return as_c(new Manifold());

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
  if (maxContours == 0) return as_c(new Manifold());

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
  if (ringVerts == 0) return as_c(new Manifold());

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
  return as_c(new Manifold(Manifold(mesh).AsOriginal()));
}

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_slice(ManifoldPtr* m, double height) {
  auto polys = as_cpp(m)->Slice(height);
  return as_c_cs(new CrossSection(CrossSection(polys, CrossSection::FillRule::Positive)));
}

ManifoldCrossSection* facet_project(ManifoldPtr* m) {
  auto polys = as_cpp(m)->Project();
  return as_c_cs(new CrossSection(CrossSection(polys, CrossSection::FillRule::Positive)));
}

// ---------------------------------------------------------------------------
// 3D Hulls
// ---------------------------------------------------------------------------

ManifoldPtr* facet_hull(ManifoldPtr* m) {
  return as_c(new Manifold(as_cpp(m)->Hull().AsOriginal()));
}

ManifoldPtr* facet_batch_hull(ManifoldPtr** solids, size_t count) {
  std::vector<Manifold> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp(solids[i]);
  }
  return as_c(new Manifold(Manifold::Hull(vec).AsOriginal()));
}

ManifoldPtr* facet_hull_points(double* xyz, size_t n_points) {
  std::vector<vec3> pts(n_points);
  for (size_t i = 0; i < n_points; i++) {
    pts[i] = {xyz[i * 3], xyz[i * 3 + 1], xyz[i * 3 + 2]};
  }
  return as_c(new Manifold(Manifold::Hull(pts).AsOriginal()));
}

// ---------------------------------------------------------------------------
// 2D Hulls
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_hull(ManifoldCrossSection* cs) {
  return as_c_cs(new CrossSection(as_cpp_cs(cs)->Hull()));
}

ManifoldCrossSection* facet_cs_batch_hull(ManifoldCrossSection** sketches, size_t count) {
  std::vector<CrossSection> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp_cs(sketches[i]);
  }
  return as_c_cs(new CrossSection(CrossSection::Hull(vec)));
}

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

ManifoldPtr* facet_trim_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset) {
  return as_c(new Manifold(as_cpp(m)->TrimByPlane({nx, ny, nz}, offset)));
}

ManifoldPtr* facet_smooth_out(ManifoldPtr* m, double min_sharp_angle, double min_smoothness) {
  return as_c(new Manifold(as_cpp(m)->SmoothOut(min_sharp_angle, min_smoothness)));
}

ManifoldPtr* facet_refine(ManifoldPtr* m, int n) {
  return as_c(new Manifold(as_cpp(m)->Refine(n)));
}

ManifoldPtr* facet_simplify(ManifoldPtr* m, double tolerance) {
  return as_c(new Manifold(as_cpp(m)->Simplify(tolerance)));
}

ManifoldPtr* facet_refine_to_length(ManifoldPtr* m, double length) {
  return as_c(new Manifold(as_cpp(m)->RefineToLength(length)));
}

FacetManifoldPair facet_split(ManifoldPtr* m, ManifoldPtr* cutter) {
  auto [first, second] = as_cpp(m)->Split(*as_cpp(cutter));
  return { as_c(new Manifold(std::move(first))), as_c(new Manifold(std::move(second))) };
}

FacetManifoldPair facet_split_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset) {
  auto [first, second] = as_cpp(m)->SplitByPlane({nx, ny, nz}, offset);
  return { as_c(new Manifold(std::move(first))), as_c(new Manifold(std::move(second))) };
}

ManifoldPtr* facet_compose(ManifoldPtr** manifolds, int n) {
  std::vector<Manifold> v;
  v.reserve(n);
  for (int i = 0; i < n; i++) v.push_back(*as_cpp(manifolds[i]));
  return as_c(new Manifold(Manifold::Compose(v)));
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

// ---------------------------------------------------------------------------
// Import / Export
// ---------------------------------------------------------------------------

ManifoldPtr* facet_import_mesh(const char* path) {
  MeshGL mesh = ImportMesh(std::string(path), true);
  if (mesh.NumVert() == 0) return nullptr;
  return as_c(new Manifold(Manifold(mesh).AsOriginal()));
}

void facet_export_mesh(ManifoldPtr* m, const char* path) {
  MeshGL mesh = as_cpp(m)->GetMeshGL();
  ExportOptions opts;
  opts.faceted = true;
  ExportMesh(std::string(path), mesh, opts);
}

ManifoldPtr* facet_solid_from_mesh(float* verts, size_t n_verts,
                                        uint32_t* indices, size_t n_tris) {
  MeshGL mesh;
  mesh.numProp = 3;
  mesh.vertProperties.assign(verts, verts + n_verts * 3);
  mesh.triVerts.assign(indices, indices + n_tris * 3);
  return as_c(new Manifold(Manifold(mesh).AsOriginal()));
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

ManifoldPtr* facet_warp(ManifoldPtr* m, int callback_id) {
  return as_c(new Manifold(as_cpp(m)->Warp([callback_id](vec3& v) {
    double x = v.x, y = v.y, z = v.z;
    facetWarpBridge(callback_id, &x, &y, &z);
    v.x = x; v.y = y; v.z = z;
  })));
}

ManifoldPtr* facet_level_set(int callback_id,
                                   double min_x, double min_y, double min_z,
                                   double max_x, double max_y, double max_z,
                                   double edge_length) {
  Box bounds{vec3{min_x, min_y, min_z}, vec3{max_x, max_y, max_z}};
  return as_c(new Manifold(Manifold::LevelSet(
      [callback_id](vec3 v) -> double {
        return facetLevelSetBridge(callback_id, v.x, v.y, v.z);
      },
      bounds, edge_length, 0.0, -1.0, false).AsOriginal()));
}

// ---------------------------------------------------------------------------
// OriginalID tracking
// ---------------------------------------------------------------------------

int facet_original_id(ManifoldPtr* m) {
  return as_cpp(m)->OriginalID();
}

ManifoldPtr* facet_as_original(ManifoldPtr* m) {
  return as_c(new Manifold(as_cpp(m)->AsOriginal()));
}

void facet_extract_mesh_with_runs(ManifoldPtr* m,
    float** out_vertices, int* out_num_verts,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_run_original_id, uint32_t** out_run_index,
    int* out_num_runs) {

  MeshGL mesh = as_cpp(m)->GetMeshGL();

  int nv = mesh.NumVert();
  int nt = mesh.NumTri();
  int np = mesh.numProp;

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

    // runIndex has numRuns+1 entries (last entry = total triVerts size)
    int riLen = (int)mesh.runIndex.size();
    *out_run_index = (uint32_t*)malloc(riLen * sizeof(uint32_t));
    memcpy(*out_run_index, mesh.runIndex.data(), riLen * sizeof(uint32_t));
  } else {
    *out_run_original_id = nullptr;
    *out_run_index = nullptr;
  }
}

}  // extern "C"
