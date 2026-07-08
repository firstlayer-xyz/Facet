package checker

// KnownBuiltins is the set of every _-prefixed builtin the evaluator registers
// (builtinRegistry). The checker only carries argument signatures for a subset
// (builtinSigs) plus a few hand-special-cased names; the rest are valid but
// unsigned, and used to type-check as unknown() with no diagnostic. Enumerating
// the full set lets checkBuiltinCall reject a typo'd or stale _-name at compile
// time, matching the evaluator's "unknown builtin" hard error, while still
// letting the unsigned-but-registered builtins through.
//
// A drift test in package evaluator (which can import checker) asserts this set
// equals the live builtinRegistry keys, so the two cannot silently diverge.
var KnownBuiltins = map[string]bool{
	"_abs": true, "_acos": true, "_ambo": true, "_area": true,
	"_asin": true, "_atan2": true, "_bounding_box": true, "_canonicalize": true,
	"_ceil": true, "_chamfer": true, "_circle": true, "_color": true,
	"_color_from_hex": true, "_color_hex": true, "_color_to_hex": true, "_compose": true,
	"_contains": true, "_cos": true, "_cube": true, "_cube_rounded": true,
	"_cylinder": true, "_decompose": true, "_difference": true, "_display_mesh": true,
	"_dual": true, "_expand": true, "_extrude": true, "_face_normals": true,
	"_fillet": true, "_find_index": true, "_find_indices": true, "_floor": true,
	"_frustum_rounded": true, "_genus": true, "_has_prefix": true, "_has_suffix": true,
	"_hull": true, "_index_of": true, "_index_of_arr": true, "_indices_of": true,
	"_insert": true, "_intersection": true, "_kis": true, "_layout": true,
	"_length": true, "_lerp": true, "_level_set": true, "_load_mesh": true,
	"_loft": true, "_match": true, "_max": true, "_mesh": true,
	"_min": true, "_min_gap": true, "_mirror": true, "_number": true,
	"_offset": true, "_polygon": true, "_polymesh": true, "_pow": true,
	"_project": true, "_refine": true, "_refine_to_length": true, "_replace": true,
	"_revolve": true, "_rotate": true, "_rotate_origin": true, "_round": true,
	"_scale": true, "_scale_to_radius": true, "_scale_uniform": true, "_simplify": true,
	"_sin": true, "_size": true, "_slice": true, "_smooth": true,
	"_snub": true, "_solid": true, "_solid_from_mesh": true, "_solid_offset": true,
	"_sphere": true, "_split": true, "_split_plane": true, "_sqrt": true,
	"_square": true, "_string": true, "_sub_str": true, "_surface_area": true,
	"_sweep": true, "_tan": true, "_text": true, "_to_lower": true,
	"_to_upper": true, "_translate": true, "_trim": true, "_trim_str": true,
	"_truncate": true, "_union": true, "_utc_date": true, "_utc_time": true,
	"_vertex_normals": true, "_volume": true, "_warp": true,
}
