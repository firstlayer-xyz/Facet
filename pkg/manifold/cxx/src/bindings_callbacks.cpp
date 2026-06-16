#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

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
using namespace facet_cxx_internal;  // as_cpp, wrap, facetClear

extern "C" {

// ---------------------------------------------------------------------------
// Callback operations
// ---------------------------------------------------------------------------

void facet_warp(ManifoldPtr* m, int callback_id, FacetSolidRet* out) try {
  wrap(new Manifold(as_cpp(m)->Warp([callback_id](vec3& v) {
    double x = v.x, y = v.y, z = v.z;
    facetWarpBridge(callback_id, &x, &y, &z);
    v.x = x; v.y = y; v.z = z;
  })), out);
} catch (...) { facetClear(out); }

void facet_level_set(int callback_id,
                     double min_x, double min_y, double min_z,
                     double max_x, double max_y, double max_z,
                     double edge_length, FacetSolidRet* out) try {
  Box bounds{vec3{min_x, min_y, min_z}, vec3{max_x, max_y, max_z}};
  wrap(new Manifold(Manifold::LevelSet(
      [callback_id](vec3 v) -> double {
        return facetLevelSetBridge(callback_id, v.x, v.y, v.z);
      },
      bounds, edge_length, 0.0, -1.0, false).AsOriginal()), out);
} catch (...) { facetClear(out); }

}  // extern "C"
