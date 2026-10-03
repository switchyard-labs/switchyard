#!/usr/bin/env python3
"""Measure a prepared disposable Attempt preview; never publishes Git state."""
import argparse,http.cookiejar,json,math,statistics,time,urllib.parse,urllib.request
p=argparse.ArgumentParser();p.add_argument('--base',required=True);p.add_argument('--cookies',required=True);p.add_argument('--attempt',required=True);p.add_argument('--samples',type=int,default=12);a=p.parse_args()
if urllib.parse.urlsplit(a.base).hostname not in ('127.0.0.1','localhost'):p.error('isolated loopback preview required')
if not 1<=a.samples<=25:p.error('samples must be 1–25')
jar=http.cookiejar.MozillaCookieJar(a.cookies);jar.load(ignore_discard=True,ignore_expires=True);client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar));rows=[]
for _ in range(a.samples):
 start=time.perf_counter();req=urllib.request.Request(a.base+'/api/attempts/'+urllib.parse.quote(a.attempt,safe='')+'/preview',method='POST',data=b'{}',headers={'Content-Type':'application/json'})
 with client.open(req,timeout=90) as r:
  body=json.load(r);rows.append({'ms':round((time.perf_counter()-start)*1000,3),'http_status':r.status,'preview_status':body.get('status'),'server_timing':r.headers.get('Server-Timing')})
values=[r['ms'] for r in rows];print(json.dumps({'scope':'remote merged checkout plus local semantic validation; preview may record findings but never publishes Git','attempt':a.attempt,'n':len(rows),'median_ms':statistics.median(values),'p95_nearest_rank_ms':sorted(values)[math.ceil(.95*len(values))-1],'min_ms':min(values),'max_ms':max(values),'rows':rows},indent=2))
