GO_TOOLCHAIN := $(CURDIR)/.go-toolchain
GO := $(GO_TOOLCHAIN)/bin/go
export GOROOT := $(GO_TOOLCHAIN)
export PATH := $(GO_TOOLCHAIN)/bin:$(PATH)
WAILS := $(HOME)/go/bin/wails

.PHONY: all manifold dev run build clean cli mcp test

all: manifold build

go-toolchain: $(GO)
$(GO):
	bash scripts/setup-go.sh

manifold:
	bash scripts/build-manifold.sh

dev: go-toolchain manifold
	cd app && $(WAILS) dev

run: build
	open app/build/bin/facet.app

build: go-toolchain manifold
	cd app && $(WAILS) build

cli: go-toolchain manifold
	$(GO) build -o build/bin/facetc ./cmd/facetc

mcp: go-toolchain manifold
	$(GO) build -o build/bin/facet-mcp ./cmd/facet-mcp

test: go-toolchain manifold
	CGO_ENABLED=1 $(GO) test ./app/pkg/fctlang/... ./app/pkg/manifold/...

test-race: go-toolchain manifold
	CGO_ENABLED=1 $(GO) test -race ./app/pkg/fctlang/... ./app/pkg/manifold/...

clean:
	rm -rf $(GO_TOOLCHAIN)
	rm -rf app/third_party/manifold/build app/third_party/manifold/build-*
	rm -rf app/third_party/assimp/build app/third_party/assimp/build-* app/third_party/assimp/install app/third_party/assimp/install-*
	rm -rf app/third_party/freetype/build app/third_party/freetype/build-* app/third_party/freetype/install app/third_party/freetype/install-*
	rm -rf app/third_party/.zig-wrappers app/third_party/.zig-wrappers-*
	rm -rf app/pkg/manifold/cxx/build app/pkg/manifold/cxx/build-*
	rm -rf build
	rm -rf app/frontend/dist
