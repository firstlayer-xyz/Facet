//go:build linux

package manifold

/*
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/cxx/build-linux-amd64 -L${SRCDIR}/../../third_party/manifold/build-linux-amd64/src -L${SRCDIR}/../../third_party/manifold/build-linux-amd64/_deps/clipper2-build -L${SRCDIR}/../../third_party/manifold/build-linux-amd64/tbb -L${SRCDIR}/../../third_party/assimp/install-linux-amd64/lib -L${SRCDIR}/../../third_party/freetype/install-linux-amd64/lib
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/cxx/build-linux-arm64 -L${SRCDIR}/../../third_party/manifold/build-linux-arm64/src -L${SRCDIR}/../../third_party/manifold/build-linux-arm64/_deps/clipper2-build -L${SRCDIR}/../../third_party/manifold/build-linux-arm64/tbb -L${SRCDIR}/../../third_party/assimp/install-linux-arm64/lib -L${SRCDIR}/../../third_party/freetype/install-linux-arm64/lib
#cgo linux LDFLAGS: -lfacet_cxx -lmanifold -lClipper2 -ltbb -lassimp -lzlibstatic -lfreetype -lc++ -lm
*/
import "C"
