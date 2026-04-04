#pragma once
#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque types (binary-compatible with Manifold C++ objects via reinterpret_cast)
typedef struct ManifoldManifold ManifoldManifold;
typedef struct ManifoldCrossSection ManifoldCrossSection;

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

void facet_delete_solid(ManifoldManifold* m);
void facet_delete_sketch(ManifoldCrossSection* cs);
size_t facet_solid_memory_size(ManifoldManifold* m);
size_t facet_sketch_memory_size(ManifoldCrossSection* cs);

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

ManifoldManifold* facet_cube(double x, double y, double z);
ManifoldManifold* facet_sphere(double radius, int segments);
ManifoldManifold* facet_cylinder(double height, double radius_low, double radius_high, int segments);

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

ManifoldManifold* facet_union(ManifoldManifold* a, ManifoldManifold* b);
ManifoldManifold* facet_difference(ManifoldManifold* a, ManifoldManifold* b);
ManifoldManifold* facet_intersection(ManifoldManifold* a, ManifoldManifold* b);
ManifoldManifold* facet_insert(ManifoldManifold* a, ManifoldManifold* b);

// Splits m into connected components. Returns count; fills *out_components with
// a malloc'd array of ManifoldManifold*. Caller frees each with facet_delete_solid,
// then frees *out_components with free().
int facet_decompose(ManifoldManifold* m, ManifoldManifold*** out_components);

// ---------------------------------------------------------------------------
// 2D Booleans
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_union(ManifoldCrossSection* a, ManifoldCrossSection* b);
ManifoldCrossSection* facet_cs_difference(ManifoldCrossSection* a, ManifoldCrossSection* b);
ManifoldCrossSection* facet_cs_intersection(ManifoldCrossSection* a, ManifoldCrossSection* b);

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

ManifoldManifold* facet_translate(ManifoldManifold* m, double x, double y, double z);
ManifoldManifold* facet_rotate(ManifoldManifold* m, double x, double y, double z);
ManifoldManifold* facet_scale(ManifoldManifold* m, double x, double y, double z);
ManifoldManifold* facet_mirror(ManifoldManifold* m, double nx, double ny, double nz);
ManifoldManifold* facet_scale_local(ManifoldManifold* m, double x, double y, double z);
ManifoldManifold* facet_rotate_local(ManifoldManifold* m, double x, double y, double z);
ManifoldManifold* facet_mirror_local(ManifoldManifold* m, double nx, double ny, double nz);
ManifoldManifold* facet_rotate_at(ManifoldManifold* m, double rx, double ry, double rz, double ox, double oy, double oz);

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

ManifoldManifold* facet_extrude(ManifoldCrossSection* cs, double height, int slices,
                                double twist, double scale_x, double scale_y);
ManifoldManifold* facet_revolve(ManifoldCrossSection* cs, int segments, double degrees);
ManifoldManifold* facet_sweep(ManifoldCrossSection* cs,
                              double* path_xyz, size_t n_path_points);
ManifoldManifold* facet_loft(ManifoldCrossSection** sketches, size_t n_sketches,
                             double* heights, size_t n_heights);

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_slice(ManifoldManifold* m, double height);
ManifoldCrossSection* facet_project(ManifoldManifold* m);

// ---------------------------------------------------------------------------
// 3D Hulls
// ---------------------------------------------------------------------------

ManifoldManifold* facet_hull(ManifoldManifold* m);
ManifoldManifold* facet_batch_hull(ManifoldManifold** solids, size_t count);
ManifoldManifold* facet_hull_points(double* xyz, size_t n_points);

// ---------------------------------------------------------------------------
// 2D Hulls
// ---------------------------------------------------------------------------

ManifoldCrossSection* facet_cs_hull(ManifoldCrossSection* cs);
ManifoldCrossSection* facet_cs_batch_hull(ManifoldCrossSection** sketches, size_t count);

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

ManifoldManifold* facet_trim_by_plane(ManifoldManifold* m, double nx, double ny, double nz, double offset);
ManifoldManifold* facet_smooth_out(ManifoldManifold* m, double min_sharp_angle, double min_smoothness);
ManifoldManifold* facet_refine(ManifoldManifold* m, int n);
ManifoldManifold* facet_simplify(ManifoldManifold* m, double tolerance);
ManifoldManifold* facet_refine_to_length(ManifoldManifold* m, double length);

// Split returns two manifolds: the part of m inside cutter and the part outside.
// Returned as a flat pair — caller frees each with facet_delete_solid.
typedef struct { ManifoldManifold* first; ManifoldManifold* second; } FacetManifoldPair;
FacetManifoldPair facet_split(ManifoldManifold* m, ManifoldManifold* cutter);
FacetManifoldPair facet_split_by_plane(ManifoldManifold* m, double nx, double ny, double nz, double offset);

// Compose assembles n non-overlapping manifolds into one without boolean operations.
// Components must not intersect; the result is undefined if they do.
ManifoldManifold* facet_compose(ManifoldManifold** manifolds, int n);

// ---------------------------------------------------------------------------
// 3D Measurements
// ---------------------------------------------------------------------------

double facet_volume(ManifoldManifold* m);
double facet_surface_area(ManifoldManifold* m);
int    facet_genus(ManifoldManifold* m);
double facet_min_gap(ManifoldManifold* a, ManifoldManifold* b, double search_length);
void facet_bounding_box(ManifoldManifold* m,
                        double* min_x, double* min_y, double* min_z,
                        double* max_x, double* max_y, double* max_z);
int facet_num_components(ManifoldManifold* m);

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
void facet_extract_mesh(ManifoldManifold* m,
                        float** out_vertices, int* out_num_verts,
                        uint32_t** out_indices, int* out_num_tris);

// Extracts display mesh with optional face group IDs. Caller must free all outputs.
// out_face_ids may be NULL if no face IDs present; out_num_face_ids will be 0.
void facet_extract_display_mesh(ManifoldManifold* m,
                                float** out_vertices, int* out_num_verts, int* out_num_prop,
                                uint32_t** out_indices, int* out_num_tris,
                                uint32_t** out_face_ids, int* out_num_face_ids);

// ---------------------------------------------------------------------------
// Import / Export
// ---------------------------------------------------------------------------

// Returns NULL if the file has no vertices.
ManifoldManifold* facet_import_mesh(const char* path);

void facet_export_mesh(ManifoldManifold* m, const char* path);

// Create a Manifold from raw mesh data (vertices as flat float xyz, indices as uint32).
ManifoldManifold* facet_solid_from_mesh(float* verts, size_t n_verts,
                                        uint32_t* indices, size_t n_tris);

// ---------------------------------------------------------------------------
// Merged Display Mesh Extraction
// ---------------------------------------------------------------------------

// Extracts and merges display meshes from multiple solids into one.
// Combines vertex data, offsets triangle indices, and offsets face group IDs.
// Caller must free all output arrays.
void facet_merge_extract_display_mesh(
    ManifoldManifold** solids, size_t count,
    float** out_vertices, int* out_num_verts, int* out_num_prop,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_face_ids, int* out_num_face_ids);

// Extracts an expanded (non-indexed) display mesh ready for direct GPU upload.
// Vertices are expanded per-triangle (3 verts * numTri), eliminating the need
// for index buffers and JS-side toNonIndexed(). Edge lines are computed for
// edges above edge_threshold_deg (in degrees). Caller must free all outputs.
void facet_extract_expanded_mesh(
    ManifoldManifold* m,
    // Expanded vertices: 3 floats (xyz) per vertex, 3 vertices per triangle
    float** out_positions, int* out_num_positions,
    // Per-triangle face group IDs (for click/highlight)
    uint32_t** out_face_ids, int* out_num_face_ids,
    // Edge line segments: pairs of xyz (6 floats per edge)
    float** out_edge_lines, int* out_num_edges,
    float edge_threshold_deg);

// Same as above but for multiple solids merged into one.
void facet_merge_extract_expanded_mesh(
    ManifoldManifold** solids, size_t count,
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
ManifoldManifold* facet_warp(ManifoldManifold* m, int callback_id);

// LevelSet creates a solid from a signed-distance-field (SDF) callback.
// Points where sdf(p) <= level form the interior; the surface is at sdf(p) = level.
// callback_id: Go closure ID; bounds: axis-aligned box to sample; edge_length: mesh resolution.
ManifoldManifold* facet_level_set(int callback_id,
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
int facet_original_id(ManifoldManifold* m);

// Marks a manifold as an original, assigning it a unique originalID.
// Returns a new manifold (caller owns).
ManifoldManifold* facet_as_original(ManifoldManifold* m);

// Extracts mesh data with run information for per-face color mapping.
// runOriginalID[i] is the originalID for triangle run i.
// runIndex[i] is the start index (in triVerts) for run i; runIndex[numRuns] = total triVerts.
// Caller must free all output arrays.
void facet_extract_mesh_with_runs(ManifoldManifold* m,
    float** out_vertices, int* out_num_verts,
    uint32_t** out_indices, int* out_num_tris,
    uint32_t** out_run_original_id, uint32_t** out_run_index,
    int* out_num_runs);

// ---------------------------------------------------------------------------
// PolyMesh
// ---------------------------------------------------------------------------

ManifoldManifold* facet_solid_from_mesh_with_face_ids(
    float* vert_props, size_t n_verts,
    uint32_t* tri_verts, size_t n_tris,
    uint32_t* face_ids, size_t n_face_ids
);

void facet_extract_polymesh(
    ManifoldManifold* manifold,
    double** out_vertices, int* out_num_verts,
    int** out_face_indices, int* out_face_indices_len,
    int** out_face_sizes, int* out_num_faces
);

#ifdef __cplusplus
}
#endif
