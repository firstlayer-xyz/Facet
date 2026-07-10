// Internal helpers shared between facet_cxx translation units (bindings.cpp,
// polymesh.cpp, text.cpp). Not part of the public C ABI; do not install.

#pragma once
#include "facet_cxx.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <climits>   // INT_MAX
#include <cstddef>
#include <cstdlib>   // malloc/free
#include <memory>    // unique_ptr
#include <new>       // std::bad_alloc
#include <stdexcept> // std::length_error

namespace facet_cxx_internal {

// MallocPtr owns a malloc'd buffer and frees it (via free()) on scope exit. It
// is what xmalloc returns, so an allocation can never exist un-owned: a throw
// before the buffer is handed to Go frees it automatically. Call release() when
// storing the pointer in an out-parameter — Go then owns and frees it.
struct FreeDeleter {
  void operator()(void* p) const { std::free(p); }
};
using MallocPtr = std::unique_ptr<void, FreeDeleter>;

// xmalloc allocates n bytes and returns an owning MallocPtr, throwing
// std::bad_alloc on failure instead of returning NULL. The extract/polymesh
// entry points copy into the buffer immediately; an unchecked NULL would
// segfault at the next memcpy (a signal the try/catch barriers can't catch).
// Throwing routes a failed allocation through the same barrier as any other
// bad_alloc (Go gets a clean null result, not a crash), and the owning return
// means a throw before handoff frees whatever was already allocated.
static inline MallocPtr xmalloc(std::size_t n) {
  void* p = std::malloc(n);
  if (!p && n != 0) throw std::bad_alloc();
  return MallocPtr(p);
}

// int_count narrows a size_t count to the int the C ABI out-params use, throwing
// (routed through each extractor's catch(...) to a null result) if it exceeds
// INT_MAX rather than wrapping negative. The kernel's int-indexed Halfedge keeps
// real meshes far below this, so it is a defensive tripwire, not a live path.
static inline int int_count(std::size_t n) {
  if (n > static_cast<std::size_t>(INT_MAX)) {
    throw std::length_error("mesh count exceeds INT_MAX");
  }
  return static_cast<int>(n);
}

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
