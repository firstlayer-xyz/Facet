#include "facet_cxx.h"
#include "manifold/manifold.h"

#include <cstdlib>
#include <cstring>
#include <map>
#include <set>
#include <unordered_map>
#include <vector>

using namespace manifold;

// Same casting pattern as Manifold's conv.h
static Manifold* as_cpp(ManifoldManifold* m) {
  return reinterpret_cast<Manifold*>(m);
}
static ManifoldManifold* as_c(Manifold* m) {
  return reinterpret_cast<ManifoldManifold*>(m);
}

extern "C" {

ManifoldManifold* facet_solid_from_mesh_with_face_ids(
    float* vert_props, size_t n_verts,
    uint32_t* tri_verts, size_t n_tris,
    uint32_t* face_ids, size_t n_face_ids) {
  MeshGL mesh;
  mesh.numProp = 3;
  mesh.vertProperties.assign(vert_props, vert_props + n_verts * 3);
  mesh.triVerts.assign(tri_verts, tri_verts + n_tris * 3);
  if (face_ids && n_face_ids > 0) {
    mesh.faceID.assign(face_ids, face_ids + n_face_ids);
  }
  return as_c(new Manifold(Manifold(mesh).AsOriginal()));
}

void facet_extract_polymesh(
    ManifoldManifold* manifold,
    double** out_vertices, int* out_num_verts,
    int** out_face_indices, int* out_face_indices_len,
    int** out_face_sizes, int* out_num_faces) {
  auto* m = as_cpp(manifold);
  MeshGL mesh = m->GetMeshGL();

  size_t numVert = mesh.NumVert();
  size_t numTri = mesh.NumTri();
  size_t numProp = mesh.numProp;

  if (numVert == 0 || numTri == 0) {
    *out_vertices = nullptr;
    *out_num_verts = 0;
    *out_face_indices = nullptr;
    *out_face_indices_len = 0;
    *out_face_sizes = nullptr;
    *out_num_faces = 0;
    return;
  }

  // Build canonical vertex map using merge pairs (union-find)
  std::vector<int> canonical(numVert);
  for (size_t i = 0; i < numVert; i++) canonical[i] = (int)i;

  auto find = [&](int x) -> int {
    while (canonical[x] != x) {
      canonical[x] = canonical[canonical[x]];
      x = canonical[x];
    }
    return x;
  };

  for (size_t i = 0; i < mesh.mergeFromVert.size(); i++) {
    int from = (int)mesh.mergeFromVert[i];
    int to = (int)mesh.mergeToVert[i];
    int rf = find(from);
    int rt = find(to);
    if (rf < rt)
      canonical[rt] = rf;
    else
      canonical[rf] = rt;
  }
  // Path compression
  for (size_t i = 0; i < numVert; i++) canonical[i] = find((int)i);

  // Build deduplicated vertex list
  std::map<int, int> canonToNew;
  std::vector<double> vertices;
  for (size_t i = 0; i < numVert; i++) {
    int c = canonical[i];
    if (canonToNew.find(c) == canonToNew.end()) {
      canonToNew[c] = (int)(vertices.size() / 3);
      vertices.push_back((double)mesh.vertProperties[i * numProp + 0]);
      vertices.push_back((double)mesh.vertProperties[i * numProp + 1]);
      vertices.push_back((double)mesh.vertProperties[i * numProp + 2]);
    }
  }

  // Build per-triangle face IDs from runOriginalID
  std::vector<uint32_t> triFaceID(numTri);
  if (!mesh.runOriginalID.empty() && !mesh.runIndex.empty()) {
    size_t numRuns = mesh.runOriginalID.size();
    for (size_t r = 0; r < numRuns; r++) {
      uint32_t origID = mesh.runOriginalID[r];
      size_t startTri = mesh.runIndex[r] / 3;
      size_t endTri = mesh.runIndex[r + 1] / 3;
      for (size_t t = startTri; t < endTri; t++) {
        triFaceID[t] = origID;
      }
    }
  } else {
    // No run data — each triangle is its own face
    for (size_t t = 0; t < numTri; t++) triFaceID[t] = (uint32_t)t;
  }

  // Group triangles by face ID
  struct Tri {
    int v0, v1, v2;
  };
  std::map<uint32_t, std::vector<Tri>> faceGroups;
  for (size_t ti = 0; ti < numTri; ti++) {
    int v0 = canonToNew[canonical[(int)mesh.triVerts[ti * 3 + 0]]];
    int v1 = canonToNew[canonical[(int)mesh.triVerts[ti * 3 + 1]]];
    int v2 = canonToNew[canonical[(int)mesh.triVerts[ti * 3 + 2]]];
    faceGroups[triFaceID[ti]].push_back({v0, v1, v2});
  }

  // For each face group, find boundary edges and chain into polygon loop
  std::vector<std::vector<int>> faces;
  for (auto& [fid, tris] : faceGroups) {
    if (tris.size() == 1) {
      faces.push_back({tris[0].v0, tris[0].v1, tris[0].v2});
      continue;
    }

    // Count half-edges
    struct HE {
      int from, to;
      bool operator<(const HE& o) const {
        return from < o.from || (from == o.from && to < o.to);
      }
    };
    std::map<HE, int> heCount;
    for (auto& t : tris) {
      heCount[{t.v0, t.v1}]++;
      heCount[{t.v1, t.v2}]++;
      heCount[{t.v2, t.v0}]++;
    }

    // Boundary edges: half-edges whose reverse doesn't appear or appears less
    std::map<int, int> boundary;  // from -> to
    for (auto& [he, count] : heCount) {
      HE rev = {he.to, he.from};
      auto it = heCount.find(rev);
      if (it == heCount.end() || count > it->second) {
        boundary[he.from] = he.to;
      }
    }

    if (boundary.size() < 3) {
      // Fallback: individual triangles
      for (auto& t : tris) {
        faces.push_back({t.v0, t.v1, t.v2});
      }
      continue;
    }

    // Chain boundary into loop
    int start = boundary.begin()->first;
    std::set<int> visited;
    std::vector<int> loop;
    int cur = start;
    while (loop.size() < boundary.size() + 1) {
      if (visited.count(cur)) break;
      visited.insert(cur);
      loop.push_back(cur);
      auto it = boundary.find(cur);
      if (it == boundary.end()) break;
      cur = it->second;
    }

    if (loop.size() >= 3) {
      faces.push_back(loop);
    } else {
      for (auto& t : tris) {
        faces.push_back({t.v0, t.v1, t.v2});
      }
    }
  }

  // Output vertices
  *out_num_verts = (int)(vertices.size() / 3);
  *out_vertices = (double*)malloc(vertices.size() * sizeof(double));
  memcpy(*out_vertices, vertices.data(), vertices.size() * sizeof(double));

  // Output faces as flat index array + sizes
  *out_num_faces = (int)faces.size();
  *out_face_sizes = (int*)malloc(faces.size() * sizeof(int));
  int totalIndices = 0;
  for (size_t i = 0; i < faces.size(); i++) {
    (*out_face_sizes)[i] = (int)faces[i].size();
    totalIndices += (int)faces[i].size();
  }
  *out_face_indices_len = totalIndices;
  *out_face_indices = (int*)malloc(totalIndices * sizeof(int));
  int idx = 0;
  for (auto& face : faces) {
    for (int vi : face) {
      (*out_face_indices)[idx++] = vi;
    }
  }
}

}  // extern "C"
