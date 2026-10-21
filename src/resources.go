package main

// The offline release appends an exact allowlisted model+engine ZIP to the EXE.
import("archive/zip";"bytes";"encoding/binary";"errors";"fmt";"io";"os";"path/filepath";"strings")
var bundleMagic=[]byte("PPLAI06!")
func decodeFooter(r io.ReaderAt,size int64)(start,length int64,err error){
 if size<16{return 0,0,errors.New("no embedded AI bundle")}
 b:=make([]byte,16)
 if _,err=r.ReadAt(b,size-16);err!=nil{return}
 if !bytes.Equal(b[:8],bundleMagic){return 0,0,errors.New("no embedded AI bundle")}
 n:=binary.LittleEndian.Uint64(b[8:])
 if n<128||n>uint64(size-16)||n>256*1024*1024{return 0,0,errors.New("invalid AI bundle size")}
 return size-16-int64(n),int64(n),nil
}
func extractBundle(exe,dest string)(string,error){
 f,e:=os.Open(exe);if e!=nil{return "",e};defer f.Close()
 st,e:=f.Stat();if e!=nil{return "",e}
 start,length,e:=decodeFooter(f,st.Size());if e!=nil{return "",e}
 zr,e:=zip.NewReader(io.NewSectionReader(f,start,length),length);if e!=nil{return "",e}
 entries:=map[string]*zip.File{}
 for _,v:=range zr.File{
  allowed:=false; for _,want:=range resourceFiles(){if v.Name==want{allowed=true;break}};if !allowed{return "",fmt.Errorf("unexpected resource %q",v.Name)}
  if _,ok:=entries[v.Name];ok{return "",errors.New("duplicate bundle member")}
  if v.UncompressedSize64<64||v.UncompressedSize64>150*1024*1024{return "",errors.New("bundle member size invalid")}
  entries[v.Name]=v
 }
 if len(entries)!=len(resourceFiles()){return "",errors.New("model or inference runtime missing from EXE")}
 if e=os.MkdirAll(dest,0700);e!=nil{return "",e}
 for _,name:=range resourceFiles(){
  v:=entries[name];destFile:=filepath.Join(dest,name)
  if s,e:=os.Stat(destFile);e==nil&&s.Size()==int64(v.UncompressedSize64){continue}
  in,e:=v.Open();if e!=nil{return "",e}
  tmp,e:=os.CreateTemp(dest,"extract-*");if e!=nil{in.Close();return "",e}
  n,copyErr:=io.Copy(tmp,io.LimitReader(in,int64(v.UncompressedSize64)+1))
  closeErr:=tmp.Close();in.Close()
  if copyErr==nil{copyErr=closeErr}
  if copyErr!=nil||n!=int64(v.UncompressedSize64){os.Remove(tmp.Name());return "",fmt.Errorf("extract %s: %v",name,copyErr)}
  if e=os.Rename(tmp.Name(),destFile);e!=nil{
   _=os.Remove(destFile)
   if e=os.Rename(tmp.Name(),destFile);e!=nil{os.Remove(tmp.Name());return "",e}
  }
 }
 return dest,nil
}
func locateResources()(string,error){
 exe,e:=os.Executable();if e!=nil{return "",e}
 root:=filepath.Dir(exe)
 installed:=true
 for _,name:=range resourceFiles(){if _,e=os.Stat(filepath.Join(root,name));e!=nil{installed=false;break}}
 if installed{return root,nil}
 home:=os.Getenv("LOCALAPPDATA");if home==""{home=os.TempDir()}
 st,e:=os.Stat(exe);if e!=nil{return "",e}
 safe:=strings.TrimSuffix(filepath.Base(exe),filepath.Ext(exe))
 dest:=filepath.Join(home,"PeopleOverlay","assets-"+safe+fmt.Sprintf("-%d",st.Size()))
 return extractBundle(exe,dest)
}
