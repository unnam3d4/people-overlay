"""Build unchanged v1.9 processing code with the same optional replay harness."""
import argparse
import io
import os
from pathlib import Path
import shutil
import subprocess
import zipfile

BASELINE='e0929f20ef4376a32983b472b5087a68ab541e32'

def build(go, work, output):
    repo=Path(__file__).resolve().parent.parent
    work=work.resolve();work.mkdir(parents=True,exist_ok=True)
    archived=subprocess.check_output(['git','archive','--format=zip',BASELINE],cwd=repo)
    with zipfile.ZipFile(io.BytesIO(archived)) as z:
        z.extractall(work)
    shutil.copyfile(repo/'src/benchmark_windows.go',work/'src/benchmark_windows.go')
    main=work/'src/main_windows.go';text=main.read_text(encoding='utf-8')
    needle='func main(){'
    if text.count(needle)!=1:raise ValueError('unexpected baseline main')
    hook='''func main(){
 if len(os.Args)>1&&(os.Args[1]=="--benchmark"||os.Args[1]=="--detect-image"){
  if e:=runBenchmarkCLI(os.Args[1:]);e!=nil{logError(e);os.Exit(24)};return
 }
'''
    main.write_text(text.replace(needle,hook,1),encoding='utf-8')
    env=dict(os.environ,GOOS='windows',GOARCH='amd64',CGO_ENABLED='0')
    output=output.resolve();output.parent.mkdir(parents=True,exist_ok=True)
    subprocess.run([str(go.resolve()),'build','-tags','ncnn','-trimpath','-ldflags','-s -w -H windowsgui','-o',str(output),'.'],cwd=work/'src',env=env,check=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--go',type=Path,required=True);p.add_argument('--work',type=Path,required=True);p.add_argument('--output',type=Path,required=True)
    a=p.parse_args();build(a.go,a.work,a.output)
