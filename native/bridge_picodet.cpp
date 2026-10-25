// SPDX-License-Identifier: Apache-2.0
// PicoDet-S 320 bridge for NCNN: no OpenCV, CUDA, Python or online services.
// DFL head layout as in PaddleDetection/deploy/third_engine/demo_ncnn.
#include "net.h"
#include <windows.h>
#include <algorithm>
#include <atomic>
#include <cmath>
#include <cstdio>
#include <memory>
#include <vector>

namespace {
std::unique_ptr<ncnn::Net> net;
std::atomic<int> threads(1);
constexpr int side=320,maximum=100;
struct Box {float x1,y1,x2,y2,score;};
float iou(const Box&a,const Box&b){
 float w=std::max(0.f,std::min(a.x2,b.x2)-std::max(a.x1,b.x1));
 float h=std::max(0.f,std::min(a.y2,b.y2)-std::max(a.y1,b.y1));
 float i=w*h,u=(a.x2-a.x1)*(a.y2-a.y1)+(b.x2-b.x1)*(b.y2-b.y1)-i;
 return u>0?i/u:0.f;
}
float dfl(const float* p){
 float mx=p[0];for(int j=1;j<8;j++)mx=std::max(mx,p[j]);
 float sum=0,weighted=0;
 for(int j=0;j<8;j++){float x=std::exp(p[j]-mx);sum+=x;weighted+=j*x;}
 return sum>0?weighted/sum:0.f;
}
const char* cls[4]={"transpose_0.tmp_0","transpose_2.tmp_0","transpose_4.tmp_0","transpose_6.tmp_0"};
const char* dist[4]={"transpose_1.tmp_0","transpose_3.tmp_0","transpose_5.tmp_0","transpose_7.tmp_0"};
const int strides[4]={8,16,32,64};
}
extern "C" __declspec(dllexport) int __stdcall PPLModelKind(){return 0x5049434f;}
extern "C" __declspec(dllexport) int __stdcall PPLLoad(const wchar_t*param,const wchar_t*weights){
 if(!param||!weights)return -1;
 net.reset(new ncnn::Net());net->opt.num_threads=1;
 net->opt.use_vulkan_compute=false;net->opt.use_fp16_arithmetic=false;
 net->opt.use_fp16_storage=false;net->opt.use_int8_inference=false;
 FILE*pf=_wfopen(param,L"rb");if(!pf){net.reset();return -2;}
 int rc=net->load_param(pf);fclose(pf);if(rc!=0){net.reset();return -3;}
 FILE*wf=_wfopen(weights,L"rb");if(!wf){net.reset();return -4;}
 rc=net->load_model(wf);fclose(wf);if(rc!=0){net.reset();return -5;}
 return 0;
}
extern "C" __declspec(dllexport) int __stdcall PPLInfer(
 const float*planar,int length,float*output,int cap,int*outN){
 if(!net||!planar||!output||!outN)return -10;
 *outN=0;if(length!=3*side*side||cap<6*maximum)return -11;
 ncnn::Mat input(side,side,3);if(input.empty())return -12;
 const float mean[3]={103.53f,116.28f,123.675f};
 const float invstd[3]={0.017429f,0.017507f,0.017125f};
 for(int c=0;c<3;c++){
   float*dst=input.channel(c);
   const float*src=planar+(2-c)*side*side; // Go planar RGB => Paddle BGR
   for(int i=0;i<side*side;i++)dst[i]=(src[i]*255.f-mean[c])*invstd[c];
 }
 ncnn::Extractor ex=net->create_extractor();ex.set_num_threads(threads.load());
 if(ex.input("image",input)!=0)return -13;
 std::vector<Box> cand;cand.reserve(128);
 for(int k=0;k<4;k++){
   ncnn::Mat scores,offsets;
   if(ex.extract(cls[k],scores)!=0||ex.extract(dist[k],offsets)!=0||
      scores.empty()||offsets.empty())return -14-k;
   const int stride=strides[k],cols=(side+stride-1)/stride,rows=cols*cols;
   if(scores.dims!=2||offsets.dims!=2||scores.w!=80||offsets.w!=32||
      scores.h!=rows||offsets.h!=rows||scores.elempack!=1||offsets.elempack!=1)
      return -20-k;
   for(int id=0;id<rows;id++){
     const float*cs=scores.row(id);float prob=cs[0];
     if(!std::isfinite(prob)||prob<.20f||prob>1.f)continue;
     bool person=true;
     for(int c=1;c<80;c++)if(cs[c]>prob){person=false;break;}
     if(!person)continue;
     const float*p=offsets.row(id);float edge[4];
     for(int q=0;q<4;q++){edge[q]=dfl(p+8*q)*stride;if(!std::isfinite(edge[q]))return -25;}
     float x=(id%cols+.5f)*stride,y=(id/cols+.5f)*stride;
     Box b={std::max(0.f,x-edge[0]),std::max(0.f,y-edge[1]),
            std::min(float(side),x+edge[2]),std::min(float(side),y+edge[3]),prob};
     if(b.x2>b.x1&&b.y2>b.y1)cand.push_back(b);
   }
 }
 std::sort(cand.begin(),cand.end(),[](const Box&a,const Box&b){return a.score>b.score;});
 std::vector<Box> kept;kept.reserve(maximum);
 int checked=std::min(int(cand.size()),800);
 for(int i=0;i<checked&&int(kept.size())<maximum;i++){
   bool duplicate=false;for(const Box&b:kept)if(iou(b,cand[i])>.45f){duplicate=true;break;}
   if(!duplicate)kept.push_back(cand[i]);
 }
 for(size_t i=0;i<kept.size();i++){
   const Box&b=kept[i];float*v=output+6*i;
   v[0]=b.x1;v[1]=b.y1;v[2]=b.x2;v[3]=b.y2;v[4]=b.score;v[5]=0.f;
 }
 *outN=int(kept.size());return 0;
}
extern "C" __declspec(dllexport) void __stdcall PPLSetThreads(int n){
 if(n<1)n=1;if(n>2)n=2;threads.store(n);
}
extern "C" __declspec(dllexport) void __stdcall PPLClose(){net.reset();}
