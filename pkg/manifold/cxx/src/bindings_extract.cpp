#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <algorithm>
#include <cmath>
#ifndef M_PI
#define M_PI 3.14159265358979323846
#endif
#include <cstdlib>
#include <cstring>
#include <functional>
#include <unordered_map>
#include <utility>
#include <vector>

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, wrap, wrap_solid_from_mesh, facetClear

namespace {

// Extract expanded mesh from a single MeshGL.
void extract_expanded_from_meshgl(
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

// Build face IDs from MeshGL run data
std::vector<uint32_t> buildFaceIDs(const MeshGL& mesh) {
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

}  // namespace

extern "C" {

// ---------------------------------------------------------------------------
// Mesh Extraction
// ---------------------------------------------------------------------------

void facet_extract_mesh(ManifoldPtr* m,
                        float** out_vertices, int* out_num_verts,
                        uint32_t** out_indices, int* out_num_tris) try {
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
} catch (...) {
  *out_vertices = nullptr;
  *out_num_verts = 0;
  *out_indices = nullptr;
  *out_num_tris = 0;
}

void facet_extract_display_mesh(ManifoldPtr* m,
                                float** out_vertices, int* out_num_verts, int* out_num_prop,
                                uint32_t** out_indices, int* out_num_tris,
                                uint32_t** out_face_ids, int* out_num_face_ids) try {
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
} catch (...) {
  *out_vertices = nullptr;
  *out_num_verts = 0;
  *out_num_prop = 0;
  *out_indices = nullptr;
  *out_num_tris = 0;
  *out_face_ids = nullptr;
  *out_num_face_ids = 0;
}

void facet_solid_from_mesh(float* verts, size_t n_verts,
                           uint32_t* indices, size_t n_tris, FacetSolidRet* out) try {
  MeshGL mesh;
  mesh.numProp = 3;
  mesh.vertProperties.assign(verts, verts + n_verts * 3);
  mesh.triVerts.assign(indices, indices + n_tris * 3);
  wrap_solid_from_mesh(mesh, out);
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// Merged Display Mesh Extraction
// ---------------------------------------------------------------------------

void facet_merge_extract_display_mesh(
    ManifoldPtr** solids, size_t count,
    float** out_vertices, int* out_num_verts, int* out_num_prop,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_face_ids, int* out_num_face_ids) try {

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
} catch (...) {
  *out_vertices = nullptr;
  *out_num_verts = 0;
  *out_num_prop = 0;
  *out_indices = nullptr;
  *out_num_tris = 0;
  *out_face_ids = nullptr;
  *out_num_face_ids = 0;
}

// ---------------------------------------------------------------------------
// Expanded Mesh Extraction (non-indexed, with edge lines)
// ---------------------------------------------------------------------------

void facet_extract_expanded_mesh(
    ManifoldPtr* m,
    float** out_positions, int* out_num_positions,
    uint32_t** out_face_ids, int* out_num_face_ids,
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg) try {

  MeshGL mesh = as_cpp(m)->GetMeshGL();
  auto faceIDs = buildFaceIDs(mesh);
  extract_expanded_from_meshgl(mesh, faceIDs,
      out_positions, out_num_positions,
      out_face_ids, out_num_face_ids,
      out_edge_lines, out_num_edges,
      edge_threshold_deg);
} catch (...) {
  *out_positions = nullptr; *out_num_positions = 0;
  *out_face_ids = nullptr; *out_num_face_ids = 0;
  *out_edge_lines = nullptr; *out_num_edges = 0;
}

void facet_merge_extract_expanded_mesh(
    ManifoldPtr** solids, size_t count,
    float** out_positions, int* out_num_positions,
    uint32_t** out_face_ids, int* out_num_face_ids,
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg) try {

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
} catch (...) {
  *out_positions = nullptr; *out_num_positions = 0;
  *out_face_ids = nullptr; *out_num_face_ids = 0;
  *out_edge_lines = nullptr; *out_num_edges = 0;
}

void facet_extract_mesh_with_runs(ManifoldPtr* m,
    float** out_vertices, int* out_num_verts,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_run_original_id, uint32_t** out_run_index,
    int* out_num_runs, int* out_num_run_index) try {

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
} catch (...) {
  *out_vertices = nullptr;
  *out_num_verts = 0;
  *out_indices = nullptr;
  *out_num_tris = 0;
  *out_run_original_id = nullptr;
  *out_run_index = nullptr;
  *out_num_runs = 0;
  *out_num_run_index = 0;
}

}  // extern "C"
