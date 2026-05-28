GO_TOOLCHAIN := $(CURDIR)/.go-toolchain
GO := $(GO_TOOLCHAIN)/bin/go
export GOROOT := $(GO_TOOLCHAIN)
export PATH := $(GO_TOOLCHAIN)/bin:$(PATH)
WAILS := $(HOME)/go/bin/wails

.PHONY: all manifold dev run build clean cli wasm wasm-cxx serve-web test

all: manifold build

go-toolchain: $(GO)
$(GO):
	bash scripts/setup-go.sh

manifold:
	bash scripts/build-manifold.sh $(TARGET)

dev: go-toolchain manifold
	cd desktop && $(WAILS) dev

run: build
	open desktop/build/bin/facet.app

build: go-toolchain manifold
	cd desktop && $(WAILS) build

cli: go-toolchain manifold
	$(GO) build -o build/bin/facetc ./cmd/facetc

FACET_WEB := $(CURDIR)/web

WASM_OPT_FLAGS := --enable-bulk-memory --enable-sign-ext --enable-nontrapping-float-to-int --enable-mutable-globals

# Build our facet_cxx geometry layer to wasm via Emscripten. Produces
# build/bin/facet_cxx.{js,wasm} from pkg/manifold/cxx/src/*.cpp linked
# against Manifold (also built for emscripten). emsdk must be on PATH.
wasm-cxx:
	bash scripts/build-wasm.sh

wasm: go-toolchain wasm-cxx
	GOOS=js GOARCH=wasm $(GO) build \
		-trimpath \
		-ldflags "-s -w" \
		-o build/bin/facet.wasm \
		./web/wasm
	wasm-opt -Oz $(WASM_OPT_FLAGS) -o build/bin/facet.wasm build/bin/facet.wasm
	cp $(GO_TOOLCHAIN)/lib/wasm/wasm_exec.js build/bin/wasm_exec.js
	@if [ -d "$(FACET_WEB)" ]; then \
		cp build/bin/facet.wasm build/bin/wasm_exec.js \
		   build/bin/facet_cxx.js build/bin/facet_cxx.wasm $(FACET_WEB)/; \
	else \
		echo "(skipped copy to $(FACET_WEB) — directory does not exist)"; \
	fi

# Dev server for web/. Sends Cross-Origin-Opener-Policy + Cross-Origin-Embedder-Policy
# headers so SharedArrayBuffer (and pthread-based wasm) work in the browser.
serve-web: go-toolchain
	$(GO) run scripts/serve-web.go

test: go-toolchain manifold
	CGO_ENABLED=1 $(GO) test ./pkg/fctlang/... ./pkg/manifold/...

test-race: go-toolchain manifold
	CGO_ENABLED=1 $(GO) test -race ./pkg/fctlang/... ./pkg/manifold/...

# Browser-side smoke tests via Playwright (Node.js + headless Chromium).
# Starts serve-web in the background, runs the playwright suite under
# web/test/, then kills the server. Requires the wasm artifacts to be
# present in web/ — run `make wasm` first if you haven't.
test-web: go-toolchain
	@if [ ! -d web/test/node_modules ]; then \
		echo "installing playwright..."; \
		(cd web/test && npm install); \
	fi
	@echo "starting serve-web..."
	@$(GO) run scripts/serve-web.go > /tmp/facet-test-web.log 2>&1 & \
		echo $$! > /tmp/facet-test-web.pid; \
		trap 'kill $$(cat /tmp/facet-test-web.pid) 2>/dev/null; rm -f /tmp/facet-test-web.pid' EXIT; \
		until curl -sf http://localhost:8000/ > /dev/null 2>&1; do sleep 0.2; done; \
		(cd web/test && npm test)

clean:
	rm -rf $(GO_TOOLCHAIN)
	rm -rf third_party/manifold/build third_party/manifold/build-*
	rm -rf third_party/assimp/build third_party/assimp/build-* third_party/assimp/install third_party/assimp/install-*
	rm -rf third_party/freetype/build third_party/freetype/build-* third_party/freetype/install third_party/freetype/install-*
	rm -rf third_party/.zig-wrappers third_party/.zig-wrappers-*
	rm -rf pkg/manifold/cxx/build pkg/manifold/cxx/build-*
	rm -rf build
	rm -rf desktop/frontend/dist
