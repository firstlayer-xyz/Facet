CROSS_TARGETS := darwin-arm64 darwin-amd64 linux-amd64 linux-arm64 windows-amd64 windows-arm64

GO_TOOLCHAIN := $(CURDIR)/.go-toolchain
GO := $(GO_TOOLCHAIN)/bin/go
export GOROOT := $(GO_TOOLCHAIN)
export PATH := $(GO_TOOLCHAIN)/bin:$(PATH)
WAILS := $(HOME)/go/bin/wails

.PHONY: all manifold dev run build release clean cli mcp cross-libs cross-build cross-all test

all: manifold build

cross-all:
	@for t in $(CROSS_TARGETS); do \
		echo "=== Building $$t ==="; \
		$(MAKE) cross-build TARGET=$$t; \
	done

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

cross-libs:
	bash scripts/build-manifold.sh $(TARGET)

cross-build: go-toolchain cross-libs
	cd app/frontend && npm install && npm run build
	@SDK_FLAGS=""; \
	case "$(TARGET)" in \
		darwin-arm64)  ZIG=aarch64-macos;  SDK_FLAGS="-I$$(xcrun --show-sdk-path)/usr/include -L$$(xcrun --show-sdk-path)/usr/lib -F$$(xcrun --show-sdk-path)/System/Library/Frameworks -Wno-nullability-completeness -Wno-error" ;; \
		darwin-amd64)  ZIG=x86_64-macos;   SDK_FLAGS="-I$$(xcrun --show-sdk-path)/usr/include -L$$(xcrun --show-sdk-path)/usr/lib -F$$(xcrun --show-sdk-path)/System/Library/Frameworks -Wno-nullability-completeness -Wno-error" ;; \
		linux-amd64)   ZIG=x86_64-linux-gnu ;; \
		linux-arm64)   ZIG=aarch64-linux-gnu ;; \
		windows-amd64) ZIG=x86_64-windows-gnu ;; \
		windows-arm64) ZIG=aarch64-windows-gnu ;; \
		*) echo "Unknown TARGET=$(TARGET)"; exit 1 ;; \
	esac; \
	GOOS=$$(echo "$(TARGET)" | cut -d- -f1); \
	GOARCH=$$(echo "$(TARGET)" | cut -d- -f2); \
	EXT=""; \
	if [ "$$GOOS" = "windows" ]; then EXT=".exe"; fi; \
	CGO_ENABLED=1 GOOS=$$GOOS GOARCH=$$GOARCH \
		CC="zig cc -target $$ZIG $$SDK_FLAGS" \
		CXX="zig c++ -target $$ZIG $$SDK_FLAGS" \
		PKG_CONFIG="$(CURDIR)/scripts/fake-pkg-config.sh" \
		$(GO) build -tags crossbuild -o build/bin/facet-$(TARGET)$$EXT ./app

release: go-toolchain
	rm -rf artifacts
	mkdir -p artifacts
	@for t in $(CROSS_TARGETS); do \
		echo "=== Release $$t ==="; \
		$(MAKE) cross-build TARGET=$$t; \
		GOOS=$$(echo "$$t" | cut -d- -f1); \
		EXT=""; \
		if [ "$$GOOS" = "windows" ]; then EXT=".exe"; fi; \
		cp build/bin/facet-$$t$$EXT artifacts/facet-$$t$$EXT; \
	done

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
	rm -rf build artifacts
	rm -rf app/frontend/dist
