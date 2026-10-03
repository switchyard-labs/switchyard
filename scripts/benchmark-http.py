#!/usr/bin/env python3
"""Read-only endpoint timings. Cookie jar optional; neither bodies nor cookies are emitted."""
import argparse, http.cookiejar, json, statistics, time, urllib.request, urllib.error
p=argparse.ArgumentParser();p.add_argument('--base',required=True);p.add_argument('--cookies');p.add_argument('--samples',type=int,default=7);p.add_argument('paths',nargs='+');a=p.parse_args()
jar=http.cookiejar.MozillaCookieJar()
if a.cookies:jar.load(a.cookies,ignore_discard=True,ignore_expires=True)
opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar));rows=[]
for path in a.paths:
 times=[];statuses=[];server_timings=[]
 for i in range(a.samples):
  start=time.perf_counter()
  try:
   with opener.open(a.base.rstrip('/')+path,timeout=60) as response:response.read(8<<20);status=response.status;server_timings.append(response.headers.get("Server-Timing",""))
  except urllib.error.HTTPError as error:status=error.code
  except (TimeoutError,urllib.error.URLError):status='transport_error'
  times.append(round((time.perf_counter()-start)*1000,2));statuses.append(status)
 rows.append({'path':path,'samples':len(times),'first_request_ms':times[0],'median_ms':statistics.median(times),'min_ms':min(times),'max_ms':max(times),'statuses':statuses,'timings_ms':times,'server_timings':server_timings})
print(json.dumps({'scope':'read-only total HTTP response time from benchmark host; first request is not a guaranteed cold cache; no p95 for small sample','rows':rows},indent=2))
