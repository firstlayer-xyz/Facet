#pragma once
#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque types (binary-compatible with Manifold C++ objects via reinterpret_cast)
typedef struct ManifoldPtr ManifoldPtr;
typedef struct ManifoldCrossSection ManifoldCrossSection;

// ---------------------------------------------------------------------------
// Return bundles
// ---------------------------------------------------------------------------
//
// Every Solid/Sketch creator writes its result through a FacetSolidRet* or
// FacetSketchRet* out parameter. One cgo crossing produces the opaque
// handle and all bookkeeping the Go side needs:
//   - size:        Go's runtime.ExternalAlloc accounting
//   - original_id: Solid.FaceMap key (or -1 if not marked as an original)
//
// Out-pointer (not by-value return) keeps the same signature usable by both
// Go cgo and emscripten/cwrap; an emscripten caller allocates one
// shared scratch slot of the right size and reads back the fields.

typedef struct {
    ManifoldPtr* ptr;
    size_t       size;
    int          original_id;  // -1 if the C++ side did not call AsOriginal()
} FacetSolidRet;

typedef struct {
    ManifoldCrossSection* ptr;
    size_t                size;
} FacetSketchRet;

// Split / SplitByPlane produce two Solids; each is fully described by a
// FacetSolidRet, so the pair contains two of them.
typedef struct {
    FacetSolidRet first;
    FacetSolidRet second;
} FacetSolidPair;

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

void facet_delete_solid(ManifoldPtr* m);
void facet_delete_sketch(ManifoldCrossSection* cs);

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

void facet_cube(double x, double y, double z, FacetSolidRet* out);
void facet_sphere(double radius, int segments, FacetSolidRet* out);
void facet_cylinder(double height, double radius_low, double radius_high, int segments, FacetSolidRet* out);

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

void facet_square(double x, double y, FacetSketchRet* out);
// CrossSection::Circle is origin-centered. facet_circle translates the
// result so its bbox starts at (0,0) — matching cube/sphere/cylinder
// convention and avoiding a separate Go-side Translate cgo call.
void facet_circle(double radius, int segments, FacetSketchRet* out);
// Polygon with optional holes: outer outline plus N inner outlines.
// Holes are concatenated into `holes_xy_pairs`; `hole_sizes[i]` is the
// point count for the i-th hole. Uses FillRule::EvenOdd so the caller
// doesn't need to manage winding direction — any ring nested in another
// flips fill. n_holes=0 reduces to a plain polygon.
void facet_polygon(
  const double* outer_xy_pairs, size_t outer_n,
  const double* holes_xy_pairs, const size_t* hole_sizes, size_t n_holes,
  FacetSketchRet* out);
void facet_cs_empty(FacetSketchRet* out);

// ---------------------------------------------------------------------------
// 3D Booleans
// ---------------------------------------------------------------------------

void facet_union(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out);
void facet_difference(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out);
void facet_intersection(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out);
void facet_insert(ManifoldPtr* a, ManifoldPtr* b, FacetSolidRet* out);

// Splits m into connected components. Returns count; fills *out_components
// with a malloc'd array of FacetSolidRet (one per component). Caller frees
// each component's ptr with facet_delete_solid, then frees *out_components
// with free().
int facet_decompose(ManifoldPtr* m, FacetSolidRet** out_components);

// ---------------------------------------------------------------------------
// 2D Booleans
// ---------------------------------------------------------------------------

void facet_cs_union(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out);
void facet_cs_difference(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out);
void facet_cs_intersection(ManifoldCrossSection* a, ManifoldCrossSection* b, FacetSketchRet* out);

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

void facet_translate(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out);
void facet_rotate(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out);
void facet_scale(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out);
void facet_mirror(ManifoldPtr* m, double nx, double ny, double nz, FacetSolidRet* out);
void facet_scale_local(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out);
void facet_rotate_local(ManifoldPtr* m, double x, double y, double z, FacetSolidRet* out);
void facet_mirror_local(ManifoldPtr* m, double nx, double ny, double nz, FacetSolidRet* out);
void facet_rotate_at(ManifoldPtr* m, double rx, double ry, double rz, double ox, double oy, double oz, FacetSolidRet* out);
// Scale (x,y,z) pivoting at point (ox,oy,oz) — fused translate-scale-translate.
void facet_scale_at(ManifoldPtr* m, double x, double y, double z, double ox, double oy, double oz, FacetSolidRet* out);
// Mirror across plane with normal (nx,ny,nz) at signed offset from origin —
// fused translate-mirror-translate. The normal is normalized in C.
void facet_mirror_at(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidRet* out);

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

void facet_cs_translate(ManifoldCrossSection* cs, double x, double y, FacetSketchRet* out);
void facet_cs_rotate(ManifoldCrossSection* cs, double degrees, FacetSketchRet* out);
void facet_cs_scale(ManifoldCrossSection* cs, double x, double y, FacetSketchRet* out);
void facet_cs_mirror(ManifoldCrossSection* cs, double ax, double ay, FacetSketchRet* out);
void facet_cs_rotate_local(ManifoldCrossSection* cs, double degrees, FacetSketchRet* out);
void facet_cs_mirror_local(ManifoldCrossSection* cs, double ax, double ay, FacetSketchRet* out);
void facet_cs_offset(ManifoldCrossSection* cs, double delta, int segments, FacetSketchRet* out);
// Scale (x,y) pivoting at point (px,py) — fused translate-scale-translate.
void facet_cs_scale_at(ManifoldCrossSection* cs, double x, double y, double px, double py, FacetSketchRet* out);
// Mirror across axis (ax,ay) at signed offset from origin — fused
// translate-mirror-translate. The axis is normalized in C.
void facet_cs_mirror_at(ManifoldCrossSection* cs, double ax, double ay, double offset, FacetSketchRet* out);

// ---------------------------------------------------------------------------
// 2D → 3D
// ---------------------------------------------------------------------------

void facet_extrude(ManifoldCrossSection* cs, double height, int slices,
                   double twist, double scale_x, double scale_y, FacetSolidRet* out);
void facet_revolve(ManifoldCrossSection* cs, int segments, double degrees, FacetSolidRet* out);
void facet_sweep(ManifoldCrossSection* cs, double* path_xyz, size_t n_path_points, FacetSolidRet* out);
void facet_loft(ManifoldCrossSection** sketches, size_t n_sketches,
                double* heights, size_t n_heights, FacetSolidRet* out);

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

void facet_slice(ManifoldPtr* m, double height, FacetSketchRet* out);
void facet_project(ManifoldPtr* m, FacetSketchRet* out);

// ---------------------------------------------------------------------------
// 3D Hulls
// ---------------------------------------------------------------------------

void facet_hull(ManifoldPtr* m, FacetSolidRet* out);
void facet_batch_hull(ManifoldPtr** solids, size_t count, FacetSolidRet* out);
void facet_hull_points(double* xyz, size_t n_points, FacetSolidRet* out);

// ---------------------------------------------------------------------------
// 2D Hulls
// ---------------------------------------------------------------------------

void facet_cs_hull(ManifoldCrossSection* cs, FacetSketchRet* out);
void facet_cs_batch_hull(ManifoldCrossSection** sketches, size_t count, FacetSketchRet* out);

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

void facet_trim_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidRet* out);
void facet_smooth_out(ManifoldPtr* m, double min_sharp_angle, double min_smoothness, FacetSolidRet* out);
void facet_refine(ManifoldPtr* m, int n, FacetSolidRet* out);
void facet_simplify(ManifoldPtr* m, double tolerance, FacetSolidRet* out);
void facet_refine_to_length(ManifoldPtr* m, double length, FacetSolidRet* out);

// Offset (SDF re-mesh): grows (delta>0) or shrinks (delta<0) the solid by delta,
// computing a signed-distance field from the mesh and meshing the result with
// Manifold's positive-inside LevelSet at level=-delta. edge_length is the
// marching-cubes sampling resolution. Approximate; resamples the whole body.
void facet_offset(ManifoldPtr* m, double delta, double edge_length, FacetSolidRet* out);

// Split returns two manifolds: the part of m inside cutter and the part outside.
void facet_split(ManifoldPtr* m, ManifoldPtr* cutter, FacetSolidPair* out);
void facet_split_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset, FacetSolidPair* out);

// Compose assembles n non-overlapping manifolds into one without boolean operations.
// Components must not intersect; the result is undefined if they do.
void facet_compose(ManifoldPtr** manifolds, int n, FacetSolidRet* out);

// ---------------------------------------------------------------------------
// 3D Measurements
// ---------------------------------------------------------------------------

double facet_volume(ManifoldPtr* m);
double facet_surface_area(ManifoldPtr* m);
int    facet_genus(ManifoldPtr* m);
double facet_min_gap(ManifoldPtr* a, ManifoldPtr* b, double search_length);
void facet_bounding_box(ManifoldPtr* m,
                        double* min_x, double* min_y, double* min_z,
                        double* max_x, double* max_y, double* max_z);
int facet_num_components(ManifoldPtr* m);

// facet_solid_size / facet_sketch_size return the approximate Go-side memory
// footprint in bytes — the same accounting the native build embeds in
// FacetSolidRet.size / FacetSketchRet.size. The wasm build, which has no return
// struct to carry it, queries these per object so Go's GC can track the
// off-heap (C++) geometry via runtime.ExternalAlloc.
size_t facet_solid_size(ManifoldPtr* m);
size_t facet_sketch_size(ManifoldCrossSection* cs);

// ---------------------------------------------------------------------------
// 2D Measurements
// ---------------------------------------------------------------------------

double facet_cs_area(ManifoldCrossSection* cs);
void facet_cs_bounds(ManifoldCrossSection* cs,
                     double* min_x, double* min_y, double* max_x, double* max_y);

// ---------------------------------------------------------------------------
// Mesh Extraction
// ---------------------------------------------------------------------------

// Extracts shared-vertex mesh. Caller must free out_vertices, out_indices.
void facet_extract_mesh(ManifoldPtr* m,
                        float** out_vertices, int* out_num_verts,
                        uint32_t** out_indices, int* out_num_tris);

// Extracts display mesh with optional face group IDs. Caller must free all outputs.
// out_face_ids may be NULL if no face IDs present; out_num_face_ids will be 0.
void facet_extract_display_mesh(ManifoldPtr* m,
                                float** out_vertices, int* out_num_verts, int* out_num_prop,
                                uint32_t** out_indices, int* out_num_tris,
                                uint32_t** out_face_ids, int* out_num_face_ids);

// Create a Manifold from raw mesh data (vertices as flat float xyz, indices as uint32).
void facet_solid_from_mesh(float* verts, size_t n_verts,
                           uint32_t* indices, size_t n_tris, FacetSolidRet* out);

// ---------------------------------------------------------------------------
// Merged Display Mesh Extraction
// ---------------------------------------------------------------------------

// Extracts and merges display meshes from multiple solids into one.
// Combines vertex data, offsets triangle indices, and offsets face group IDs.
// Caller must free all output arrays.
void facet_merge_extract_display_mesh(
    ManifoldPtr** solids, size_t count,
    float** out_vertices, int* out_num_verts, int* out_num_prop,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_face_ids, int* out_num_face_ids);

// Extracts an expanded (non-indexed) display mesh ready for direct GPU upload.
// Vertices are expanded per-triangle (3 verts * numTri), eliminating the need
// for index buffers and JS-side toNonIndexed(). Edge lines are computed for
// edges above edge_threshold_deg (in degrees). Caller must free all outputs.
void facet_extract_expanded_mesh(
    ManifoldPtr* m,
    // Expanded vertices: 3 floats (xyz) per vertex, 3 vertices per triangle
    float** out_positions, int* out_num_positions,
    // Per-triangle face group IDs (for click/highlight)
    uint32_t** out_face_ids, int* out_num_face_ids,
    // Edge line segments: pairs of xyz (6 floats per edge)
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg);

// Same as above but for multiple solids merged into one.
void facet_merge_extract_expanded_mesh(
    ManifoldPtr** solids, size_t count,
    float** out_positions, int* out_num_positions,
    uint32_t** out_face_ids, int* out_num_face_ids,
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg);

// ---------------------------------------------------------------------------
// Text
// ---------------------------------------------------------------------------

// Renders text string to a CrossSection using FreeType. Writes empty
// CrossSection if text is empty or font fails to load. Sets out->ptr to NULL
// on failure (caller should check).
//
// halign: "left" (default — text starts at x=0), "center", "right".
// valign: "baseline" (default — y=0 is the baseline), "top" (y=0 is at
//   ascender), "center" (y=0 is mid-cap-height), "bottom" (y=0 is at
//   descender bottom). Empty string means default.
void facet_text_to_cross_section(
    const char* font_data, size_t font_len, const char* text, double size_mm,
    const char* halign, const char* valign, FacetSketchRet* out);

// ---------------------------------------------------------------------------
// Callback operations
// ---------------------------------------------------------------------------

// Forward declarations for Go bridge functions (resolved at link time via //export).
extern void facetWarpBridge(int id, double* x, double* y, double* z);
extern double facetLevelSetBridge(int id, double x, double y, double z);

// Warp deforms each vertex of a solid using a per-vertex callback.
// callback_id identifies the Go closure to invoke for each vertex.
void facet_warp(ManifoldPtr* m, int callback_id, FacetSolidRet* out);

// LevelSet creates a solid from a signed-distance-field (SDF) callback.
// Points where sdf(p) <= level form the interior; the surface is at sdf(p) = level.
// callback_id: Go closure ID; bounds: axis-aligned box to sample; edge_length: mesh resolution.
void facet_level_set(int callback_id,
                     double min_x, double min_y, double min_z,
                     double max_x, double max_y, double max_z,
                     double edge_length, FacetSolidRet* out);

// ---------------------------------------------------------------------------
// OriginalID tracking (for faceID-based color)
// ---------------------------------------------------------------------------

// Returns the originalID of a manifold (-1 if not marked as original).
// (Most callers should not need this — FacetSolidRet.original_id is
// populated by every creator. Kept here for diagnostic uses.)
int facet_original_id(ManifoldPtr* m);

// Marks a manifold as an original, assigning it a unique originalID.
void facet_as_original(ManifoldPtr* m, FacetSolidRet* out);

// Extracts mesh data with run information for per-face color mapping.
// runOriginalID[i] is the originalID for triangle run i.
// runIndex[i] is the start index (in triVerts) for run i; runIndex[numRuns] = total triVerts.
// out_num_run_index reports the actual run_index length (runIndex.size(), normally
// num_runs+1); the caller must size its read from it rather than assuming
// num_runs+1, so a shorter runIndex can't drive an out-of-bounds read.
// Caller must free all output arrays.
void facet_extract_mesh_with_runs(ManifoldPtr* m,
    float** out_vertices, int* out_num_verts,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_run_original_id, uint32_t** out_run_index,
    int* out_num_runs, int* out_num_run_index);

// ---------------------------------------------------------------------------
// PolyMesh
// ---------------------------------------------------------------------------

void facet_solid_from_mesh_with_face_ids(
    float* vert_props, size_t n_verts,
    uint32_t* tri_verts, size_t n_tris,
    uint32_t* face_ids, size_t n_face_ids,
    FacetSolidRet* out);

void facet_extract_polymesh(
    ManifoldPtr* manifold,
    double** out_vertices, int* out_num_verts,
    int** out_face_indices, int* out_face_indices_len,
    int** out_face_sizes, int* out_num_faces);

#ifdef __cplusplus
}
#endif
