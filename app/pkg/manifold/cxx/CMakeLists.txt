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
if(CMAKE_SYSTEM_PROCESSOR MATCHES "arm64|aarch64|ARM64")
  set(_ARCH "arm64")
else()
  set(_ARCH "amd64")
endif()
set(FREETYPE_INSTALL_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../../third_party/freetype/install-${_OS}-${_ARCH}")

add_library(facet_cxx STATIC src/bindings.cpp src/polymesh.cpp src/text.cpp)
target_include_directories(facet_cxx
  PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/include
  PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/../../../third_party/manifold/include
          ${FREETYPE_INSTALL_DIR}/include/freetype2
)
