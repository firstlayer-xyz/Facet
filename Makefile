GO_TOOLCHAIN := $(CURDIR)/.go-toolchain
GO := $(GO_TOOLCHAIN)/bin/go
GOFMT := $(GO_TOOLCHAIN)/bin/gofmt
export GOROOT := $(GO_TOOLCHAIN)
export PATH := $(GO_TOOLCHAIN)/bin:$(PATH)

# Pin the Wails CLI to match go.mod's require, so `make dev` / `make build`
# never silently use whatever a contributor happens to have in ~/go/bin/wails.
# Installed into the project-local Go toolchain dir alongside `go`.
WAILS_VERSION := v2.12.0
WAILS := $(GO_TOOLCHAIN)/bin/wails

.PHONY: all manifold lint dev run build clean cli scad2facet wasm wasm-cxx serve-web check-shims test test-race test-web test-desktop test-desktop-go wails-cli

all: manifold build

go-toolchain: $(GO)
$(GO):
	bash scripts/setup-go.sh

# wails-cli installs the pinned Wails build tool into the project's local
# toolchain. The marker file lets make skip the install when the binary is
# already current; delete .go-toolchain/bin/wails (or `make clean`) to force
# a reinstall.
#
# GOBIN must be set explicitly: `go install` ignores GOROOT and otherwise
# defaults to $GOPATH/bin (~/go/bin), which is exactly the location we are
# trying to avoid depending on.
wails-cli: $(WAILS)
$(WAILS): go-toolchain
	@echo "installing wails@$(WAILS_VERSION) into $(WAILS)..."
	GOBIN=$(GO_TOOLCHAIN)/bin $(GO) install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)

manifold:
	bash scripts/build-manifold.sh $(TARGET)

dev: go-toolchain manifold wails-cli
	cd desktop && $(WAILS) dev

run: build
	open desktop/build/bin/Facet.app

build: go-toolchain manifold wails-cli
	cd desktop && $(WAILS) build
	bash scripts/build-quicklook.sh

cli: go-toolchain manifold
	$(GO) build -o build/bin/facetc ./cmd/facetc

# OpenSCAD -> Facet transpiler. Pure Go (no geometry layer), so it needs
# neither manifold nor CGO.
scad2facet: go-toolchain
	$(GO) build -o build/bin/scad2facet ./cmd/scad2facet

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

# Dev server for web/. Plain static server (single-threaded wasm needs no
# SharedArrayBuffer), mirroring how GitHub Pages serves the bundle.
serve-web: go-toolchain
	$(GO) run scripts/serve-web.go

# Verify the wasm JS bridge (web/index.html _mf_* shims) stays in sync with the
# native cgo build, so a hand-written shim can't silently diverge from the C++
# kernel. Source scan only — fast, no build needed.
check-shims: go-toolchain
	$(GO) run scripts/check-shims.go

# Formatting + vet gate. gofmt runs over all our Go (no compile needed); vet
# covers the non-desktop packages (desktop needs the //go:embed frontend, so its
# vet rides along in test-desktop-go's build).
lint: go-toolchain manifold
	@files=$$($(GOFMT) -l pkg cmd desktop scripts); \
	if [ -n "$$files" ]; then echo "gofmt needed on:"; echo "$$files"; exit 1; fi
	CGO_ENABLED=1 $(GO) vet ./pkg/... ./cmd/...

# Every non-desktop Go package. The glob (rather than an explicit list) keeps new
# packages from silently escaping CI; desktop/ is separate (test-desktop-go)
# because its //go:embed needs a built frontend.
test: go-toolchain manifold
	CGO_ENABLED=1 $(GO) test ./pkg/... ./cmd/...

# Tests for the desktop (Wails app) Go package — the Go-side counterpart to
# `test-desktop` (which runs the frontend Playwright suite). Separate from
# `test` because the package does `//go:embed all:frontend/dist`, so it only
# compiles once the frontend has been built — which also needs the generated
# wailsjs bindings. We build the frontend (real bindings + vite, no app
# packaging) only when dist is absent, so this runs fast after a `make build`
# or in CI right after `wails build`, but still works from a clean checkout.
#
# On that clean-checkout path, dist must be seeded with a placeholder before
# `wails generate module`: generating bindings compiles the desktop package,
# whose embed pattern refuses to match an absent/empty dist. The `all:` prefix
# makes the embed accept the dotfile, and vite empties dist on build, so the
# placeholder never survives into a real embed.
test-desktop-go: go-toolchain manifold wails-cli
	@if [ ! -d desktop/frontend/dist ] || [ -z "$$(ls -A desktop/frontend/dist 2>/dev/null)" ]; then \
		echo "frontend/dist missing — building frontend for the embed..."; \
		mkdir -p desktop/frontend/dist && \
		touch desktop/frontend/dist/.placeholder && \
		( cd desktop && $(WAILS) generate module ) && \
		( cd desktop/frontend && npm ci && npm run build ); \
	fi
	CGO_ENABLED=1 $(GO) test ./desktop/...

test-race: go-toolchain manifold
	CGO_ENABLED=1 $(GO) test -race ./pkg/... ./cmd/...

# Browser-side smoke tests via Playwright (Node.js + headless Chromium).
# Starts serve-web in the background, runs the playwright suite under
# web/test/, then kills the server. Requires the wasm artifacts to be
# present in web/ — run `make wasm` first if you haven't.
#
# The server is built and exec'd directly (not `go run`) so the recorded PID
# is the listener itself — killing a `go run` wrapper leaves its child server
# alive, squatting on port 8000 long after the test run. The until-loop also
# fails loudly if the server dies (e.g. port already in use) instead of
# silently testing against whatever else is answering on 8000.
test-web: go-toolchain
	@if [ ! -d web/test/node_modules ]; then \
		echo "installing playwright..."; \
		(cd web/test && npm install); \
	fi
	@echo "starting serve-web..."
	@$(GO) build -o build/bin/serve-web scripts/serve-web.go
	@build/bin/serve-web > /tmp/facet-test-web.log 2>&1 & \
		echo $$! > /tmp/facet-test-web.pid; \
		trap 'kill $$(cat /tmp/facet-test-web.pid) 2>/dev/null; rm -f /tmp/facet-test-web.pid' EXIT; \
		until curl -sf http://localhost:8000/ > /dev/null 2>&1; do \
			if ! kill -0 $$(cat /tmp/facet-test-web.pid) 2>/dev/null; then \
				echo "serve-web failed to start:"; cat /tmp/facet-test-web.log; exit 1; \
			fi; \
			sleep 0.2; \
		done; \
		(cd web/test && npm test)

# Desktop frontend Playwright suite (vite + mocked Wails harness).
# First run on a fresh checkout: generates wailsjs stubs, runs npm ci,
# and downloads the chromium browser. Subsequent runs skip those steps.
# On Linux you may need `npx playwright install --with-deps chromium`
# once for system libs (CI does this for the runner).
test-desktop:
	@if [ ! -d desktop/frontend/wailsjs ]; then \
		echo "generating wailsjs stubs..."; \
		bash scripts/gen-wailsjs-stubs.sh; \
	fi
	@if [ ! -d desktop/frontend/node_modules ]; then \
		echo "installing frontend deps + chromium browser..."; \
		(cd desktop/frontend && npm ci && npx playwright install chromium); \
	fi
	cd desktop/frontend && npm test

clean:
	rm -rf $(GO_TOOLCHAIN)
	rm -rf third_party/manifold/build third_party/manifold/build-*
	rm -rf third_party/assimp/build third_party/assimp/build-* third_party/assimp/install third_party/assimp/install-*
	rm -rf third_party/freetype/build third_party/freetype/build-* third_party/freetype/install third_party/freetype/install-*
	rm -rf third_party/.zig-wrappers third_party/.zig-wrappers-*
	rm -rf pkg/manifold/cxx/build pkg/manifold/cxx/build-*
	rm -rf build
	rm -rf desktop/frontend/dist
