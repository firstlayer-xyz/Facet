package manifold

import "errors"

// errInsertNoShell is returned by Insert when every disconnected piece of
// (a - b) lies within b's convex hull, so there is no outer shell to keep.
// Seating b would silently discard the entire base solid, which is never a
// valid result. Shared by the native (booleans.go) and wasm (booleans_js.go)
// builds, both of which detect the condition via a null result from the C++
// facet_insert.
var errInsertNoShell = errors.New("Insert: every piece of the base solid lies within the inserted part's convex hull, leaving no outer shell to seat it into")
