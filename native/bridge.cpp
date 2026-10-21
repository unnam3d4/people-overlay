// SPDX-License-Identifier: AGPL-3.0-or-later
// Tiny NCNN->stdcall bridge: builds as a standalone MinGW DLL without MSVC CRT.
#include "net.h"
#include <cmath>
#include <cstring>
#include <cstdio>
#include <memory>
#include <vector>

static std::unique_ptr<ncnn::Net> g_net;
static const char* g_input = "in0";
static const char* g_output = "out0";

extern "C" __declspec(dllexport) int __stdcall PPLLoad(const wchar_t* param, const wchar_t* weights) {
    if (!param || !weights) return -1;
    g_net.reset(new ncnn::Net());
    g_net->opt.num_threads = 1;
    g_net->opt.use_vulkan_compute = false;
    g_net->opt.use_fp16_storage = false;
    g_net->opt.use_fp16_arithmetic = false;
    g_net->opt.use_int8_inference = false;
    // MinGW NCNN exposes FILE* rather than wchar_t* overload; _wfopen
    // preserves non-ASCII usernames and file paths on Windows 7.
    FILE* pf = _wfopen(param, L"rb");
    if (!pf) { g_net.reset(); return -1; }
    int rc = g_net->load_param(pf);
    fclose(pf);
    if (rc != 0) { g_net.reset(); return -2; }
    FILE* wf = _wfopen(weights, L"rb");
    if (!wf) { g_net.reset(); return -3; }
    rc = g_net->load_model(wf);
    fclose(wf);
    if (rc != 0) { g_net.reset(); return -4; }
    const std::vector<const char*> input_names = g_net->input_names();
    const std::vector<const char*> output_names = g_net->output_names();
    if (!input_names.empty()) g_input = input_names[0];
    if (!output_names.empty()) g_output = output_names[0];
    return 0;
}

extern "C" __declspec(dllexport) int __stdcall PPLInfer(const float* planar, int length, float* output, int cap, int* outN) {
    if (!g_net || !planar || !output || !outN) return -10;
    if (length != 3 * 640 * 640 || cap < 84 * 8400) return -11;
    ncnn::Mat in(640, 640, 3);
    if (in.empty()) return -12;
    for (int c=0; c<3; ++c) {
       float* dest = in.channel(c);
       std::memcpy(dest, planar + c*640*640, 640*640*sizeof(float));
    }
    ncnn::Extractor ex = g_net->create_extractor();
    ex.set_num_threads(1);
    if (ex.input(g_input, in) != 0) return -13;
    ncnn::Mat out;
    if (ex.extract(g_output, out) != 0 || out.empty()) return -14;
    if (out.elemsize != sizeof(float) || out.elempack != 1) return -15;

    // Ultralytics NCNN output is normally [84,8400], stored as 84 rows.
    // Some exporter versions transpose it; handle both layouts explicitly.
    if (out.dims == 2 && out.h == 84 && out.w > 0 && out.w <= 8400) {
       *outN = out.w;
       for (int c=0;c<84;c++) std::memcpy(output+c*out.w,out.row(c),sizeof(float)*out.w);
    } else if (out.dims == 2 && out.w == 84 && out.h > 0 && out.h <= 8400) {
       *outN = out.h;
       for (int j=0;j<out.h;j++) {
          const float* row = out.row(j);
          for (int c=0;c<84;c++) output[c*out.h+j] = row[c];
       }
    } else if (out.dims == 3 && out.c == 84 && out.h==1 && out.w > 0 && out.w <= 8400) {
       *outN = out.w;
       for (int c=0;c<84;c++) std::memcpy(output+c*out.w,out.channel(c),sizeof(float)*out.w);
    } else {
       return -16;
    }
    return 0;
}

extern "C" __declspec(dllexport) void __stdcall PPLClose() {
    g_net.reset();
    g_input = "in0";
    g_output = "out0";
}
