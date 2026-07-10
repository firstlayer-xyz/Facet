#include "facet_cxx.h"
#include "internal.h"
#include "manifold/manifold.h"
#include "manifold/cross_section.h"

#include <limits>

using namespace manifold;
using namespace facet_cxx_internal;  // as_cpp, as_cpp_cs, solid_size, sketch_size

extern "C" {

// ---------------------------------------------------------------------------
// 3D Measurements
// ---------------------------------------------------------------------------

double facet_volume(ManifoldPtr* m) try {
  return as_cpp(m)->Volume();
} catch (...) { return 0.0; }

double facet_surface_area(ManifoldPtr* m) try {
  return as_cpp(m)->SurfaceArea();
} catch (...) { return 0.0; }

int facet_genus(ManifoldPtr* m) try {
  return as_cpp(m)->Genus();
} catch (...) { return 0; }

double facet_min_gap(ManifoldPtr* a, ManifoldPtr* b, double search_length) try {
  return as_cpp(a)->MinGap(*as_cpp(b), search_length);
  // NaN, not 0.0: a genuine 0.0 means the solids touch, so returning it on a
  // kernel exception would report a real measurement for a failed query. NaN is
  // an out-of-band failure signal.
} catch (...) { return std::numeric_limits<double>::quiet_NaN(); }

void facet_bounding_box(ManifoldPtr* m,
                        double* min_x, double* min_y, double* min_z,
                        double* max_x, double* max_y, double* max_z) try {
  Box box = as_cpp(m)->BoundingBox();
  *min_x = box.min.x; *min_y = box.min.y; *min_z = box.min.z;
  *max_x = box.max.x; *max_y = box.max.y; *max_z = box.max.z;
} catch (...) {
  *min_x = 0.0; *min_y = 0.0; *min_z = 0.0;
  *max_x = 0.0; *max_y = 0.0; *max_z = 0.0;
}

int facet_num_components(ManifoldPtr* m) try {
  auto comps = as_cpp(m)->Decompose();
  return static_cast<int>(comps.size());
} catch (...) { return 0; }

size_t facet_solid_size(ManifoldPtr* m) try {
  return solid_size(as_cpp(m));
} catch (...) { return 0; }

size_t facet_sketch_size(ManifoldCrossSection* cs) try {
  return sketch_size(as_cpp_cs(cs));
} catch (...) { return 0; }

// ---------------------------------------------------------------------------
// 2D Measurements
// ---------------------------------------------------------------------------

double facet_cs_area(ManifoldCrossSection* cs) try {
  return as_cpp_cs(cs)->Area();
} catch (...) { return 0.0; }

void facet_cs_bounds(ManifoldCrossSection* cs,
                     double* min_x, double* min_y, double* max_x, double* max_y) try {
  Rect rect = as_cpp_cs(cs)->Bounds();
  *min_x = rect.min.x; *min_y = rect.min.y;
  *max_x = rect.max.x; *max_y = rect.max.y;
} catch (...) {
  *min_x = 0.0; *min_y = 0.0; *max_x = 0.0; *max_y = 0.0;
}

// ---------------------------------------------------------------------------
// OriginalID tracking
// ---------------------------------------------------------------------------

int facet_original_id(ManifoldPtr* m) try {
  return as_cpp(m)->OriginalID();
  // -1, not 0: honors the header contract ("-1 if not marked as original") and
  // matches facetClear; the face-color path skips ids < 0, so a bogus key 0 is
  // never seeded on failure.
} catch (...) { return -1; }

}  // extern "C"
