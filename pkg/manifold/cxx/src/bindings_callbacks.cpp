#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#ifdef FACET_WASM
// Under wasm the warp/levelset callback bridges are EM_JS imports that dispatch
// to the Module["facet*Bridge"] hooks installed unconditionally in web/index.html,
// which route into the syscall/js Go dispatchers (manifold_warp_js.go /
// manifold_levelset_js.go). The hooks are always present by the time an op runs,
// so the calls are unguarded: a missing hook throws a JS TypeError (a loud failed
// eval) rather than silently warping with an identity / zero-density fallback —
// matching the native extern-C bridges and the Go-side loud panics.
#include <emscripten/emscripten.h>
EM_JS(void, facetWarpBridge, (int callback_id, double* x, double* y, double* z), {
  Module["facetWarpBridge"](callback_id, x, y, z);
});
EM_JS(double, facetLevelSetBridge, (int callback_id, double x, double y, double z), {
  return Module["facetLevelSetBridge"](callback_id, x, y, z);
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
  // Manifold's LevelSet treats POSITIVE values as inside, but Facet's SDF
  // contract (facet_cxx.h, std_mesh.fct, the Go wrappers, and the evaluator's
  // error default) is the standard negative-inside convention. Negate the
  // bridge so a user's negative-inside SDF yields the solid, not its complement.
  wrap(new Manifold(Manifold::LevelSet(
      [callback_id](vec3 v) -> double {
        return -facetLevelSetBridge(callback_id, v.x, v.y, v.z);
      },
      bounds, edge_length, 0.0, -1.0, false).AsOriginal()), out);
} catch (...) { facetClear(out); }

}  // extern "C"
