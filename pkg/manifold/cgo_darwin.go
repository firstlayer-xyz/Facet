//go:build darwin

package manifold

// Platform-specific search paths and C++ stdlib.  See cgo_common.go for
// the shared library list.

/*
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/cxx/build-darwin-arm64 -L${SRCDIR}/../../third_party/manifold/build-darwin-arm64/src -L${SRCDIR}/../../third_party/manifold/build-darwin-arm64/_deps/clipper2-build -L${SRCDIR}/../../third_party/manifold/build-darwin-arm64/tbb -L${SRCDIR}/../../third_party/assimp/install-darwin-arm64/lib -L${SRCDIR}/../../third_party/freetype/install-darwin-arm64/lib -L/opt/homebrew/lib
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/cxx/build-darwin-amd64 -L${SRCDIR}/../../third_party/manifold/build-darwin-amd64/src -L${SRCDIR}/../../third_party/manifold/build-darwin-amd64/_deps/clipper2-build -L${SRCDIR}/../../third_party/manifold/build-darwin-amd64/tbb -L${SRCDIR}/../../third_party/assimp/install-darwin-amd64/lib -L${SRCDIR}/../../third_party/freetype/install-darwin-amd64/lib -L/opt/homebrew/lib
#cgo darwin LDFLAGS: -lc++
*/
import "C"
