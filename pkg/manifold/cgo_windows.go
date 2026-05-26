//go:build windows

package manifold

// Platform-specific search paths and C++ stdlib.  See cgo_common.go for
// the shared library list.

/*
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/cxx/build-windows-amd64 -L${SRCDIR}/../../third_party/manifold/build-windows-amd64/src -L${SRCDIR}/../../third_party/manifold/build-windows-amd64/_deps/clipper2-build -L${SRCDIR}/../../third_party/manifold/build-windows-amd64/tbb -L${SRCDIR}/../../third_party/assimp/install-windows-amd64/lib -L${SRCDIR}/../../third_party/freetype/install-windows-amd64/lib
#cgo windows,arm64 LDFLAGS: -L${SRCDIR}/cxx/build-windows-arm64 -L${SRCDIR}/../../third_party/manifold/build-windows-arm64/src -L${SRCDIR}/../../third_party/manifold/build-windows-arm64/_deps/clipper2-build -L${SRCDIR}/../../third_party/manifold/build-windows-arm64/tbb -L${SRCDIR}/../../third_party/assimp/install-windows-arm64/lib -L${SRCDIR}/../../third_party/freetype/install-windows-arm64/lib
#cgo windows LDFLAGS: -lstdc++
*/
import "C"
