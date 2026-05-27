cmake_minimum_required(VERSION 3.18)
project(facet_cxx LANGUAGES CXX)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Detect platform and arch for FreeType install path
if(CMAKE_SYSTEM_NAME STREQUAL "Darwin")
  set(_OS "darwin")
elseif(CMAKE_SYSTEM_NAME STREQUAL "Linux")
  set(_OS "linux")
elseif(CMAKE_SYSTEM_NAME STREQUAL "Windows")
  set(_OS "windows")
endif()
# On Darwin prefer CMAKE_OSX_ARCHITECTURES when set — it's the source of
# truth for cross-arch builds. CMAKE_SYSTEM_PROCESSOR there reflects the
# host (since we don't use a toolchain file on macOS), so it's wrong for
# cross-compile from x86 host to arm64 target. Linux/Windows pick from
# CMAKE_SYSTEM_PROCESSOR as before.
if(CMAKE_SYSTEM_NAME STREQUAL "Darwin" AND CMAKE_OSX_ARCHITECTURES)
  if(CMAKE_OSX_ARCHITECTURES STREQUAL "arm64")
    set(_ARCH "arm64")
  else()
    set(_ARCH "amd64")
  endif()
elseif(CMAKE_SYSTEM_PROCESSOR MATCHES "arm64|aarch64|ARM64")
  set(_ARCH "arm64")
else()
  set(_ARCH "amd64")
endif()
set(FREETYPE_INSTALL_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../../third_party/freetype/install-${_OS}-${_ARCH}")
set(ASSIMP_INSTALL_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../../third_party/assimp/install-${_OS}-${_ARCH}")

add_library(facet_cxx STATIC src/bindings.cpp src/polymesh.cpp src/text.cpp)

# TBB info header — used by facet_tbb_default_concurrency() for runtime
# parallelism diagnostics. Located in Manifold's bundled TBB clone tree.
set(MANIFOLD_BUILD_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../../third_party/manifold/build-${_OS}-${_ARCH}")

target_include_directories(facet_cxx
  PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/include
  PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/../../../third_party/manifold/include
          ${MANIFOLD_BUILD_DIR}/_deps/tbb-src/include
          ${FREETYPE_INSTALL_DIR}/include/freetype2
          ${ASSIMP_INSTALL_DIR}/include
)

# Forward Manifold's MANIFOLD_PAR build choice into facet_cxx so the
# diagnostic helpers can report what Manifold was built with. We default
# to ON (matches scripts/build-manifold.sh); pass -DFACET_MANIFOLD_PAR=OFF
# only if the upstream build is configured serial.
option(FACET_MANIFOLD_PAR "Manifold built with TBB-backed parallelism" ON)
if(FACET_MANIFOLD_PAR)
  target_compile_definitions(facet_cxx PRIVATE FACET_MANIFOLD_PAR=1)
else()
  target_compile_definitions(facet_cxx PRIVATE FACET_MANIFOLD_PAR=-1)
endif()
