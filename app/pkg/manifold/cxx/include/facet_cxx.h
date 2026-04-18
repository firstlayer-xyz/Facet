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
// Lifecycle
// ---------------------------------------------------------------------------

void facet_delete_solid(ManifoldPtr* m);
void facet_delete_sketch(ManifoldCrossSection* cs);
size_t facet_solid_memory_size(ManifoldPtr* m);
size_t facet_sketch_memory_size(ManifoldCrossSection* cs);

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

ManifoldPtr* facet_cube(double x, double y, double z);
ManifoldPtr* facet_sphere(double radius, int segments);
ManifoldPtr* facet_cylinder(double height, double radius_low, double radius_high, int segments);

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_square(double x, double y);
ManifoldCrossSection* facet_circle(double radius, int segments);
ManifoldCrossSection* facet_polygon(double* xy_pairs, size_t n_points);
ManifoldCrossSection* facet_cs_empty(void);

// ---------------------------------------------------------------------------
// 3D Booleans
// ---------------------------------------------------------------------------

ManifoldPtr* facet_union(ManifoldPtr* a, ManifoldPtr* b);
ManifoldPtr* facet_difference(ManifoldPtr* a, ManifoldPtr* b);
ManifoldPtr* facet_intersection(ManifoldPtr* a, ManifoldPtr* b);
ManifoldPtr* facet_insert(ManifoldPtr* a, ManifoldPtr* b);

// Splits m into connected components. Returns count; fills *out_components with
// a malloc'd array of ManifoldPtr*. Caller frees each with facet_delete_solid,
// then frees *out_components with free().
int facet_decompose(ManifoldPtr* m, ManifoldPtr*** out_components);

// ---------------------------------------------------------------------------
// 2D Booleans
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_union(ManifoldCrossSection* a, ManifoldCrossSection* b);
ManifoldCrossSection* facet_cs_difference(ManifoldCrossSection* a, ManifoldCrossSection* b);
ManifoldCrossSection* facet_cs_intersection(ManifoldCrossSection* a, ManifoldCrossSection* b);

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

ManifoldPtr* facet_translate(ManifoldPtr* m, double x, double y, double z);
ManifoldPtr* facet_rotate(ManifoldPtr* m, double x, double y, double z);
ManifoldPtr* facet_scale(ManifoldPtr* m, double x, double y, double z);
ManifoldPtr* facet_mirror(ManifoldPtr* m, double nx, double ny, double nz);
ManifoldPtr* facet_scale_local(ManifoldPtr* m, double x, double y, double z);
ManifoldPtr* facet_rotate_local(ManifoldPtr* m, double x, double y, double z);
ManifoldPtr* facet_mirror_local(ManifoldPtr* m, double nx, double ny, double nz);
ManifoldPtr* facet_rotate_at(ManifoldPtr* m, double rx, double ry, double rz, double ox, double oy, double oz);

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_translate(ManifoldCrossSection* cs, double x, double y);
ManifoldCrossSection* facet_cs_rotate(ManifoldCrossSection* cs, double degrees);
ManifoldCrossSection* facet_cs_scale(ManifoldCrossSection* cs, double x, double y);
ManifoldCrossSection* facet_cs_mirror(ManifoldCrossSection* cs, double ax, double ay);
ManifoldCrossSection* facet_cs_rotate_local(ManifoldCrossSection* cs, double degrees);
ManifoldCrossSection* facet_cs_mirror_local(ManifoldCrossSection* cs, double ax, double ay);
ManifoldCrossSection* facet_cs_offset(ManifoldCrossSection* cs, double delta, int segments);

// ---------------------------------------------------------------------------
// 2D → 3D
// ---------------------------------------------------------------------------

ManifoldPtr* facet_extrude(ManifoldCrossSection* cs, double height, int slices,
                                double twist, double scale_x, double scale_y);
ManifoldPtr* facet_revolve(ManifoldCrossSection* cs, int segments, double degrees);
ManifoldPtr* facet_sweep(ManifoldCrossSection* cs,
                              double* path_xyz, size_t n_path_points);
ManifoldPtr* facet_loft(ManifoldCrossSection** sketches, size_t n_sketches,
                             double* heights, size_t n_heights);

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_slice(ManifoldPtr* m, double height);
ManifoldCrossSection* facet_project(ManifoldPtr* m);

// ---------------------------------------------------------------------------
// 3D Hulls
// ---------------------------------------------------------------------------

ManifoldPtr* facet_hull(ManifoldPtr* m);
ManifoldPtr* facet_batch_hull(ManifoldPtr** solids, size_t count);
ManifoldPtr* facet_hull_points(double* xyz, size_t n_points);

// ---------------------------------------------------------------------------
// 2D Hulls
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_hull(ManifoldCrossSection* cs);
ManifoldCrossSection* facet_cs_batch_hull(ManifoldCrossSection** sketches, size_t count);

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

ManifoldPtr* facet_trim_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset);
ManifoldPtr* facet_smooth_out(ManifoldPtr* m, double min_sharp_angle, double min_smoothness);
ManifoldPtr* facet_refine(ManifoldPtr* m, int n);
ManifoldPtr* facet_simplify(ManifoldPtr* m, double tolerance);
ManifoldPtr* facet_refine_to_length(ManifoldPtr* m, double length);

// Split returns two manifolds: the part of m inside cutter and the part outside.
// Returned as a flat pair — caller frees each with facet_delete_solid.
typedef struct { ManifoldPtr* first; ManifoldPtr* second; } FacetManifoldPair;
FacetManifoldPair facet_split(ManifoldPtr* m, ManifoldPtr* cutter);
FacetManifoldPair facet_split_by_plane(ManifoldPtr* m, double nx, double ny, double nz, double offset);

// Compose assembles n non-overlapping manifolds into one without boolean operations.
// Components must not intersect; the result is undefined if they do.
ManifoldPtr* facet_compose(ManifoldPtr** manifolds, int n);

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

// ---------------------------------------------------------------------------
// Import / Export
// ---------------------------------------------------------------------------

// Imports a mesh file via Assimp. Returns a new Manifold (caller owns via
// facet_delete_solid) on success; returns NULL on failure with *out_err set
// to a malloc'd, null-terminated error string (caller frees via
// facet_free_string). On success *out_err is set to NULL. out_err must be
// non-NULL.
ManifoldPtr* facet_import_mesh(const char* path, char** out_err);

// Exports a Manifold to a mesh file via Assimp. Returns NULL on success, or
// a malloc'd, null-terminated error string on failure (caller frees via
// facet_free_string).
char* facet_export_mesh(ManifoldPtr* m, const char* path);

// Frees a string returned via out-pointer or return value by the import/export
// API. Safe to call with NULL.
void facet_free_string(char* s);

// Create a Manifold from raw mesh data (vertices as flat float xyz, indices as uint32).
ManifoldPtr* facet_solid_from_mesh(float* verts, size_t n_verts,
                                        uint32_t* indices, size_t n_tris);

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

// Renders text string to a CrossSection using FreeType.
// Returns empty CrossSection if text is empty or font fails to load.
// Returns NULL on error (caller should check).
ManifoldCrossSection* facet_text_to_cross_section(
    const char* font_path, const char* text, double size_mm);

// ---------------------------------------------------------------------------
// Callback operations
// ---------------------------------------------------------------------------

// Forward declarations for Go bridge functions (resolved at link time via //export).
extern void facetWarpBridge(int id, double* x, double* y, double* z);
extern double facetLevelSetBridge(int id, double x, double y, double z);

// Warp deforms each vertex of a solid using a per-vertex callback.
// callback_id identifies the Go closure to invoke for each vertex.
ManifoldPtr* facet_warp(ManifoldPtr* m, int callback_id);

// LevelSet creates a solid from a signed-distance-field (SDF) callback.
// Points where sdf(p) <= level form the interior; the surface is at sdf(p) = level.
// callback_id: Go closure ID; bounds: axis-aligned box to sample; edge_length: mesh resolution.
ManifoldPtr* facet_level_set(int callback_id,
                                  double min_x, double min_y, double min_z,
                                  double max_x, double max_y, double max_z,
                                  double edge_length);

// ---------------------------------------------------------------------------
// Color
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// OriginalID tracking (for faceID-based color)
// ---------------------------------------------------------------------------

// Returns the originalID of a manifold (-1 if not marked as original).
int facet_original_id(ManifoldPtr* m);

// Marks a manifold as an original, assigning it a unique originalID.
// Returns a new manifold (caller owns).
ManifoldPtr* facet_as_original(ManifoldPtr* m);

// Extracts mesh data with run information for per-face color mapping.
// runOriginalID[i] is the originalID for triangle run i.
// runIndex[i] is the start index (in triVerts) for run i; runIndex[numRuns] = total triVerts.
// Caller must free all output arrays.
void facet_extract_mesh_with_runs(ManifoldPtr* m,
    float** out_vertices, int* out_num_verts,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_run_original_id, uint32_t** out_run_index,
    int* out_num_runs);

// ---------------------------------------------------------------------------
// PolyMesh
// ---------------------------------------------------------------------------

ManifoldPtr* facet_solid_from_mesh_with_face_ids(
    float* vert_props, size_t n_verts,
    uint32_t* tri_verts, size_t n_tris,
    uint32_t* face_ids, size_t n_face_ids
);

void facet_extract_polymesh(
    ManifoldPtr* manifold,
    double** out_vertices, int* out_num_verts,
    int** out_face_indices, int* out_face_indices_len,
    int** out_face_sizes, int* out_num_faces
);

#ifdef __cplusplus
}
#endif
