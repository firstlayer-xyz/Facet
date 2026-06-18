#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <cstdlib>
#include <vector>

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, as_cpp_cs, wrap, wrap_cs, facetClear

extern "C" {

// ---------------------------------------------------------------------------
// 3D Booleans
// ---------------------------------------------------------------------------

void facet_union(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) try {
  wrap(new Manifold(*as_cpp(a) + *as_cpp(b)), out);
} catch (...) { facetClear(out); }

void facet_difference(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) try {
  wrap(new Manifold(*as_cpp(a) - *as_cpp(b)), out);
} catch (...) { facetClear(out); }

void facet_intersection(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) try {
  wrap(new Manifold(*as_cpp(a) ^ *as_cpp(b)), out);
} catch (...) { facetClear(out); }

// facet_batch_boolean unions/subtracts/intersects N solids at once via the
// kernel's tree-reduction BatchBoolean (op: 0=Add, 1=Subtract, 2=Intersect,
// matching OpType). This is asymptotically better than folding pairwise booleans
// over a growing accumulator. No .AsOriginal() (unlike hull): BatchBoolean
// preserves the per-solid originalIDs the FaceMap/color path depends on.
void facet_batch_boolean(ManifoldPtr** solids, size_t count, int op, FacetSolidRet* out) try {
  std::vector<Manifold> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp(solids[i]);
  }
  wrap(new Manifold(Manifold::BatchBoolean(vec, static_cast<OpType>(op))), out);
} catch (...) { facetClear(out); }

// facet_cs_batch_boolean is the 2D (CrossSection) counterpart.
void facet_cs_batch_boolean(ManifoldCrossSection** sketches, size_t count, int op, FacetSketchRet* out) try {
  std::vector<CrossSection> vec(count);
  for (size_t i = 0; i < count; i++) {
    vec[i] = *as_cpp_cs(sketches[i]);
  }
  wrap_cs(new CrossSection(CrossSection::BatchBoolean(vec, static_cast<OpType>(op))), out);
} catch (...) { facetClear(out); }

// Insert seats b into a: cut b's shape out of a, drop any piece of a that b
// traps inside itself (a "plug"), then union b back in. A piece is a plug iff
// it lies entirely within b's convex hull, tested by exact boolean
// containment: (piece - hull) is empty. The hull is rotation-invariant and
// tight to b's geometry.
//
// When every piece is a plug there is no outer shell and seating b would
// discard all of a, which is never a valid result. out->ptr is left null to
// signal this; the Go layer reports it as errInsertNoShell.
void facet_insert(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out) try {
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
} catch (...) { facetClear(out); }

// Returns count of connected components; fills *out_components with a malloc'd
// array of FacetSolidRet (one per component). Caller must free each
// component's ptr with facet_delete_solid, then free(*out_components).
int facet_decompose(ManifoldPtr* m, FacetSolidRet** out_components) try {
  auto components = as_cpp(m)->Decompose();
  int n = (int)components.size();
  if (n == 0) {
    *out_components = nullptr;
    return 0;
  }
  FacetSolidRet* arr = (FacetSolidRet*)malloc(n * sizeof(FacetSolidRet));
  if (!arr) {
    *out_components = nullptr;
    return 0;
  }
  int built = 0;
  try {
    for (; built < n; built++)
      wrap(new Manifold(std::move(components[built])), &arr[built]);
  } catch (...) {
    // The outer function-try-block's catch can't reach arr/built, so free the
    // components built so far and the array here before signaling failure.
    for (int j = 0; j < built; j++) delete reinterpret_cast<Manifold*>(arr[j].ptr);
    free(arr);
    *out_components = nullptr;
    return 0;
  }
  *out_components = arr;
  return n;
} catch (...) { if (out_components) *out_components = nullptr; return 0; }

void facet_compose(ManifoldPtr** manifolds, int n, FacetSolidRet* out) try {
  std::vector<Manifold> v;
  v.reserve(n);
  for (int i = 0; i < n; i++) v.push_back(*as_cpp(manifolds[i]));
  wrap(new Manifold(Manifold::Compose(v)), out);
} catch (...) { facetClear(out); }

// ---------------------------------------------------------------------------
// 2D Booleans
// ---------------------------------------------------------------------------

void facet_cs_union(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(*as_cpp_cs(a) + *as_cpp_cs(b)), out);
} catch (...) { facetClear(out); }

void facet_cs_difference(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(*as_cpp_cs(a) - *as_cpp_cs(b)), out);
} catch (...) { facetClear(out); }

void facet_cs_intersection(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out) try {
  wrap_cs(new CrossSection(*as_cpp_cs(a) ^ *as_cpp_cs(b)), out);
} catch (...) { facetClear(out); }

}  // extern "C"
