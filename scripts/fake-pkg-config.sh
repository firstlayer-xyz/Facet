#!/bin/sh
# Fake pkg-config for cross-compilation.
# Returns success with empty output so cgo pkg-config directives
# don't fail when the target system's dev packages aren't installed.
exit 0
