package manifold

import "errors"

// errInsertNoShell is returned by Insert when every disconnected piece of
// (a - b) lies within b's convex hull, so there is no outer shell to keep.
// Seating b would silently discard the entire base solid, which is never a
// valid result. Shared by the native (booleans.go) and wasm (booleans_js.go)
// builds, both of which detect the condition via a null result from the C++
// facet_insert.
var errInsertNoShell = errors.New("Insert: every piece of the base solid lies within the inserted part's convex hull, leaving no outer shell to seat it into")

var errBatchBooleanEmpty = errors.New("BatchBoolean: no operands")
var errBatchBooleanFailed = errors.New("BatchBoolean: kernel returned no result")

// BoolOp selects the operation for a batch boolean. The values match manifold's
// OpType (Add/Subtract/Intersect) so they cross the C boundary as a plain int.
type BoolOp int

const (
	OpUnion        BoolOp = iota // OpType::Add
	OpDifference                 // OpType::Subtract
	OpIntersection               // OpType::Intersect
)

// mergedFaceMaps folds the inputs' face maps in input order, so a key present in
// an earlier solid wins — matching the precedence of pairwise a.Union(b). Shared
// by the native and wasm BatchBoolean so both carry colors identically.
func mergedFaceMaps(solids []*Solid) map[uint32]FaceInfo {
	if len(solids) == 0 {
		return nil
	}
	m := solids[0].FaceMap
	for _, s := range solids[1:] {
		m = mergeFaceMaps(m, s.FaceMap)
	}
	return m
}
