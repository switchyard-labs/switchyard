#!/usr/bin/env python3
"""Prepare two real disposable Attempts; never invokes a synthetic coding Agent.
Cookie input stays private, output is a resumable manifest, no mutation is retried.
"""
import argparse, http.cookiejar, json, os, pathlib, urllib.request, urllib.parse, uuid
p=argparse.ArgumentParser()
p.add_argument('--base',required=True);p.add_argument('--cookies',required=True)
p.add_argument('--repository',required=True,help='registered owner/repo backed by a disposable codex-* Artifacts repository')
p.add_argument('--output',required=True);a=p.parse_args()
u=urllib.parse.urlsplit(a.base)
if u.scheme!='http' or u.hostname not in ('127.0.0.1','localhost') or u.username or u.password or u.query or u.fragment:
 p.error('preparation is restricted to the isolated loopback demo')
out=pathlib.Path(a.output)
if out.exists():p.error('manifest exists; use it to resume instead of creating duplicate Work')
parts=a.repository.split('/')
if len(parts)!=2:p.error('canonical owner/repo required')
jar=http.cookiejar.MozillaCookieJar(a.cookies);jar.load(ignore_discard=True,ignore_expires=True)
client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
def call(method,path,body=None):
 req=urllib.request.Request(a.base.rstrip('/')+path,method=method,data=json.dumps(body).encode() if body is not None else None,headers={'Content-Type':'application/json'})
 with client.open(req,timeout=90) as r:return json.load(r)
def save(value):
 with os.fdopen(os.open(out,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600),'w') as f:json.dump(value,f,indent=2);f.write('\n')
root='/api/repositories/'+'/'.join(urllib.parse.quote(x,safe='') for x in parts)
repo=call('GET',root);physical=repo.get('artifact_name','')
if not physical.startswith('codex-') or not repo.get('can_write'):p.error('writable explicitly disposable repository required')
manifest={'repository':a.repository,'artifact_name':physical,'base':a.base,'run_id':uuid.uuid4().hex[:12],'attempts':[],'phase':'preparing','real_provider_certified':False}
work=call('POST','/api/work',{'title':'Competition: two competing implementations','kind':'feature','repo':physical,'body':'Two independent Attempts; exact-SHA CI, structured review and human integration choice. External coding-provider execution is a separate prerequisite.'})
manifest['work_id']=work['id'];save(manifest)
for lane in ('a','b'):
 branch='event-competition-'+manifest['run_id']+'-'+lane
 attempt=call('POST','/api/work/'+work['id']+'/attempts',{'repo':physical,'branch':branch})
 manifest['attempts'].append({'lane':lane,'id':attempt['id'],'branch':branch});save(manifest)
manifest['phase']='prepared-awaiting-real-provider';save(manifest)
print(json.dumps({'work_id':manifest['work_id'],'attempts':manifest['attempts'],'phase':manifest['phase'],'manifest':str(out)}))
