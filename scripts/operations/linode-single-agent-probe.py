import os,json,time,subprocess,pathlib,hashlib
root=pathlib.Path('/var/lib/switchyard-agent/c25')
os.umask(0o077)
workspace=root/'single-workspace';workspace.mkdir(exist_ok=True);os.chown(workspace,999,988)
credential=root/'provider.key';os.chown(credential,999,988);credential.chmod(0o600)
unit='switchyard-c25-single.service'
user=['runuser','-u','switchyard-agent','--','env','XDG_RUNTIME_DIR=/run/user/999','DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/999/bus']
args=user+['systemd-run','--user','--wait','--pipe','--collect','--unit='+unit,'-p','MemoryMax=768M','-p','MemorySwapMax=0','-p','TasksMax=128','-p','CPUQuota=100%','-p','RuntimeMaxSec=110','-p','KillMode=control-group','/usr/local/libexec/switchyard-bwrap','--unshare-all','--share-net','--die-with-parent','--new-session','--clearenv']
for p in ['/usr','/bin','/lib','/lib64','/etc/resolv.conf','/etc/hosts','/etc/ssl']:args+=['--ro-bind',p,p]
args+=['--proc','/proc','--dev','/dev','--tmpfs','/tmp','--dir','/run','--dir','/runner','--dir','/home','--dir','/home/agent','--bind',str(workspace),'/workspace','--ro-bind',str(root/'switchyard-campaign-app'),'/runner/agent','--ro-bind',str(root/'opencode'),'/runner/opencode','--ro-bind',str(credential),'/run/credential','--chdir','/workspace','--setenv','HOME','/home/agent','--setenv','PATH','/usr/bin:/bin','--setenv','LANG','C.UTF-8','--setenv','SWITCHYARD_OPENCODE_BIN','/runner/opencode','--setenv','SWITCHYARD_CREDENTIAL_FILE','/run/credential','--','/runner/agent','agent-adapter']
def available():
 return int(next(x.split()[1] for x in pathlib.Path('/proc/meminfo').read_text().splitlines() if x.startswith('MemAvailable:')))*1024
before=available();samples=[];reason=None;start=time.monotonic()
out=open(root/'single.stdout','wb');err=open(root/'single.stderr','wb')
try:
 p=subprocess.Popen(args,stdin=subprocess.PIPE,stdout=out,stderr=err)
 task={'role':'implementer','provider':'opencode-go','model':'gpt-5.6-luna','repo':'disposable-capacity-probe','branch':'capacity-probe','file':'routes.mjs','current':'export const stations = ["north", "south"];\n','prompt':'Preserve existing code. Add only an exported function describeRoute(route) returning route.join(" → ") for arrays and "" otherwise.'}
 p.stdin.write(json.dumps(task).encode());p.stdin.close()
 while p.poll() is None:
  mem=available();sample={'seconds':round(time.monotonic()-start,3),'available_bytes':mem}
  for name,url in [('switchyard','http://127.0.0.1:8080/'),('trestle','http://127.0.0.1:7350/health'),('caddy','http://127.0.0.1/')]:
   r=subprocess.run(['curl','-s','-o','/dev/null','--max-time','2','-w','%{http_code} %{time_total}',url],capture_output=True,text=True)
   sample[name]=r.stdout.strip();sample[name+'_exit']=r.returncode
  samples.append(sample)
  if mem<192*1024*1024:reason='host_headroom_guard'
  if time.monotonic()-start>115:reason='wall_guard'
  if (root/'single.stdout').stat().st_size+(root/'single.stderr').stat().st_size>1024*1024:reason='output_guard'
  if reason:
   subprocess.run(user+['systemctl','--user','stop',unit],capture_output=True,timeout=10);break
  time.sleep(.5)
 p.wait(timeout=15)
 out.close();err.close()
 raw=(root/'single.stdout').read_bytes();result={}
 try:result=json.loads(raw)
 except Exception:pass
 content=result.get('files',{}).get('routes.mjs','')
 proof={'before_available_bytes':before,'after_available_bytes':available(),'duration_seconds':round(time.monotonic()-start,3),'returncode':p.returncode,'guard':reason,'failure_code':result.get('failure_code'),'timings_ns':result.get('timings_ns'),'output_has_original': 'export const stations' in content,'output_has_function':'function describeRoute' in content,'output_sha256':hashlib.sha256(content.encode()).hexdigest(),'samples':samples,'git_publication':'not exercised; standalone sandbox capacity task'}
 (root/'single-proof.json').write_text(json.dumps(proof,indent=2));print(json.dumps({k:v for k,v in proof.items() if k!='samples'}))
finally:
 credential.unlink(missing_ok=True)
 subprocess.run(user+['systemctl','--user','stop',unit],capture_output=True)
