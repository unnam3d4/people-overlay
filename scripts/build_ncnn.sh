#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
NCNN_SRC="$ROOT/_ncnn_src"
if [[ ! -f "$NCNN_SRC/CMakeLists.txt" ]]; then
 git clone --depth 1 --branch 20240410 https://github.com/Tencent/ncnn.git "$NCNN_SRC"
fi
for config in "x64 x86_64-w64-mingw32" "x86 i686-w64-mingw32"; do
 read -r arch prefix <<< "$config"
 echo "== Building Win7 CPU native engine: $arch =="
 build="$ROOT/_ncnn_build_$arch"
 install="$ROOT/_ncnn_sdk_$arch"
 cmake -S "$NCNN_SRC" -B "$build" -G Ninja \
  -DCMAKE_SYSTEM_NAME=Windows \
  -DCMAKE_C_COMPILER="${prefix}-gcc-posix" \
  -DCMAKE_CXX_COMPILER="${prefix}-g++-posix" \
  -DCMAKE_RC_COMPILER="${prefix}-windres" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX="$install" \
  -DNCNN_VULKAN=OFF \
  -DNCNN_OPENMP=OFF \
  -DNCNN_SHARED_LIB=OFF \
  -DNCNN_AVX=OFF \
  -DNCNN_AVX2=OFF \
  -DNCNN_BUILD_TOOLS=OFF \
  -DNCNN_BUILD_EXAMPLES=OFF \
  -DNCNN_BUILD_TESTS=OFF \
  -DNCNN_BUILD_BENCHMARK=OFF \
  -DNCNN_INSTALL_SDK=ON
 cmake --build "$build" --parallel 4
 cmake --install "$build"
 lib="$install/lib/libncnn.a"
 if [[ ! -f "$lib" ]];then echo "Missing $lib";exit 1;fi
 mkdir -p "$ROOT/_output/$arch"
 "${prefix}-g++-posix" -std=c++11 -O2 -D_WIN32_WINNT=0x0601 -DWINVER=0x0601 \
  -I"$install/include/ncnn" -shared "${BRIDGE_SOURCE:-$ROOT/native/bridge.cpp}" "$lib" \
  -Wl,--kill-at -static -static-libgcc -static-libstdc++ \
  -o "$ROOT/_output/$arch/ncnnbridge.dll"
 "${prefix}-objdump" -p "$ROOT/_output/$arch/ncnnbridge.dll" | grep "DLL Name:"
 if "${prefix}-objdump" -p "$ROOT/_output/$arch/ncnnbridge.dll" | grep "DLL Name:" | grep -i -E '(vcruntime|msvcp|libgcc|libstdc\+\+|libwinpthread)' ; then
    echo "ERROR: Native DLL has forbidden, unbundled DLL dependencies"
    exit 1
 fi
 done
