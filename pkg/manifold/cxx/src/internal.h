// Internal helpers shared between facet_cxx translation units (bindings.cpp,
// polymesh.cpp, text.cpp). Not part of the public C ABI; do not install.

#pragma once
#include "facet_cxx.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <cstddef>

namespace facet_cxx_internal {

static inline manifold::Manifold* as_cpp(ManifoldPtr* m) {
  return reinterpret_cast<manifold::Manifold*>(m);
}
static inline manifold::CrossSection* as_cpp_cs(ManifoldCrossSection* cs) {
  return reinterpret_cast<manifold::CrossSection*>(cs);
}

// Approximate Go-side memory footprint, used by Go's runtime.ExternalAlloc.
// Written inline so every creator can return both pointer and size in one
// C call without a separate facet_solid_memory_size cgo crossing.
static inline std::size_t solid_size(const manifold::Manifold* m) {
  return (std::size_t)m->NumVert() * (24 + (std::size_t)m->NumProp() * 8)
       + (std::size_t)m->NumTri() * 108;
}
static inline std::size_t sketch_size(const manifold::CrossSection* cs) {
  return (std::size_t)cs->NumVert() * 16 + (std::size_t)cs->NumContour() * 24;
}

// Fill a FacetSolidRet for a freshly-allocated Manifold: stores the opaque
// handle, the bookkeeping size, and the original-ID (or -1 if the C++ side
// didn't mark it as original). The single sanctioned way to construct a
// FacetSolidRet — anything else risks skipping size or ID accounting that
// the Go side relies on.
//
// `out` is REQUIRED. Passing NULL is a programmer bug that fails loudly.
static inline void wrap(manifold::Manifold* m, FacetSolidRet* out) {
  out->ptr         = reinterpret_cast<ManifoldPtr*>(m);
  out->size        = solid_size(m);
  out->original_id = m->OriginalID();  // -1 if not an original
}
static inline void wrap_cs(manifold::CrossSection* cs, FacetSketchRet* out) {
  out->ptr  = reinterpret_cast<ManifoldCrossSection*>(cs);
  out->size = sketch_size(cs);
}

// Clears an out-struct to the "failed/empty" state Go already recognizes
// (null ptr). Used by the per-function exception barriers so a C++ exception
// never unwinds across the extern "C" boundary into Go (UB).
static inline void facetClear(FacetSolidRet* out)  { if (out) { out->ptr = nullptr; out->size = 0; out->original_id = -1; } }
static inline void facetClear(FacetSketchRet* out) { if (out) { out->ptr = nullptr; out->size = 0; } }
static inline void facetClear(FacetSolidPair* out) { if (out) { facetClear(&out->first); facetClear(&out->second); } }

// Construct a Solid from a triangle mesh, welding coincident vertices first.
// MeshGL::Merge() fills the merge vectors for vertices that coincide within
// tolerance along open edges, so a mesh assembled from independent per-face
// vertices — e.g. a subdivision that computes each shared edge's midpoint once
// per adjacent triangle — closes into a manifold. It is a no-op when the mesh
// already carries merge vectors or is manifold. Welding joins coincident points
// only: a genuinely non-manifold mesh still yields an empty Manifold, so this
// does not paper over broken topology. The single entry point for turning raw
// triangle data into a Solid (with or without face IDs).
static inline void wrap_solid_from_mesh(manifold::MeshGL& mesh,
                                        FacetSolidRet* out) {
  mesh.Merge();
  wrap(new manifold::Manifold(manifold::Manifold(mesh).AsOriginal()), out);
}

}  // namespace facet_cxx_internal
