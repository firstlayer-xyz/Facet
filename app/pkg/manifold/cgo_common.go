package manifold

// Shared library-link list for all platforms.  The per-platform files
// (cgo_darwin.go, cgo_linux.go, cgo_windows.go) only contribute the
// platform-specific search paths (-L) and C++ stdlib choice (-lc++ vs
// -lstdc++).  Keeping the library names here prevents the drift that
// comes from adding a dependency to one OS file and forgetting the
// others — a class of bug that fails silently on whichever platform
// the author happened not to be building.

/*
#cgo LDFLAGS: -lfacet_cxx -lmanifold -lClipper2 -ltbb -lassimp -lzlibstatic -lfreetype -lm
*/
import "C"
