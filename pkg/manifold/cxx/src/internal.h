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
// C call, avoiding a separate facet_solid_memory_size cgo crossing per
// allocated handle.
static inline std::size_t solid_size(const manifold::Manifold* m) {
  return (std::size_t)m->NumVert() * (24 + (std::size_t)m->NumProp() * 8)
       + (std::size_t)m->NumTri() * 108;
}
static inline std::size_t sketch_size(const manifold::CrossSection* cs) {
  return (std::size_t)cs->NumVert() * 16 + (std::size_t)cs->NumContour() * 24;
}

// The ONLY sanctioned way to construct a C handle from a freshly-allocated
// C++ object. Writes the Go-side memory size through out_size and returns
// the opaque C handle. out_size is REQUIRED by contract — callers must
// always pass a valid pointer (no NULL check; passing NULL is a programmer
// bug that should fail loudly).
static inline ManifoldPtr* wrap(manifold::Manifold* m, std::size_t* out_size) {
  *out_size = solid_size(m);
  return reinterpret_cast<ManifoldPtr*>(m);
}
static inline ManifoldCrossSection* wrap_cs(manifold::CrossSection* cs, std::size_t* out_size) {
  *out_size = sketch_size(cs);
  return reinterpret_cast<ManifoldCrossSection*>(cs);
}

}  // namespace facet_cxx_internal
