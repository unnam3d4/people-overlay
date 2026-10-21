"""Run Windows EXE replay sequentially and measure the replay process, not live RVi.

python run_pixel_benchmarks.py --exe label=path.exe --fixtures path --output path
Builder-only dependency: psutil. CPU machine-percent divides by logical CPUs;
core-percent does not. PNG decoding/report writing are included in process CPU.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import time
import psutil

def run(exe, manifest, output):
    output.parent.mkdir(parents=True, exist_ok=True)
    started=time.perf_counter()
    process=subprocess.Popen([str(exe.resolve()), '--benchmark', str(manifest.resolve()), str(output.resolve())])
    observed=psutil.Process(process.pid)
    cpu=peak=private=0
    while process.poll() is None:
        try:
            times=observed.cpu_times();cpu=max(cpu,times.user+times.system)
            mem=observed.memory_info();peak=max(peak,mem.rss);private=max(private,getattr(mem,'private',0))
        except (psutil.NoSuchProcess,psutil.AccessDenied):
            pass
        time.sleep(.05)
    wall=time.perf_counter()-started
    result={'exit_code':process.returncode,'wall_s':wall,'sampled_cpu_s':cpu,
            'cpu_core_percent':100*cpu/wall,'cpu_machine_percent':100*cpu/wall/(os.cpu_count() or 1),
            'logical_cpus':os.cpu_count(),'peak_sampled_rss_bytes':peak,'peak_sampled_private_bytes':private,
            'measurement':'offline replay process incl PNG reads; 50ms sampling; not live RVi CPU or concurrent inference/capture'}
    output.with_suffix('.process.json').write_text(json.dumps(result,indent=2))
    if process.returncode:
        raise RuntimeError(f'{exe.name} exited {process.returncode} on {manifest.name}')
    print(exe.name,manifest.name,round(wall,2),'s',round(result['cpu_machine_percent'],2),'% machine CPU',flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--exe',action='append',required=True,help='label=executable')
    p.add_argument('--fixtures',type=Path,required=True)
    p.add_argument('--output',type=Path,required=True)
    p.add_argument('--scenario',action='append')
    a=p.parse_args()
    for item in a.exe:
        label,path=item.split('=',1)
        for manifest in sorted(a.fixtures.glob('*fps.json')):
            if a.scenario and not any(s in manifest.stem for s in a.scenario):continue
            run(Path(path),manifest,a.output/f'{label}-{manifest.stem}.json')
