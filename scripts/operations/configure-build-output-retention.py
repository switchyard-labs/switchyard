#!/usr/bin/env python3
"""Preserve existing R2 rules and expire only temporary Actions outputs after 30 days."""
import argparse,json,os,pathlib,re,urllib.request
RULE={'id':'switchyard-build-output-retention','enabled':True,'conditions':{'prefix':'build-assets/'},'deleteObjectsTransition':{'condition':{'type':'Age','maxAge':30*86400}}}
def merged_rules(current):
    rules=current['rules']
    if not isinstance(rules,list):raise ValueError('Invalid lifecycle rules')
    matching=[r for r in rules if r.get('id')==RULE['id']]
    if matching and matching!=[RULE]:raise ValueError('Existing managed rule differs; review before replacing')
    return {'rules':rules if matching else rules+[RULE]}
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--account',required=True);p.add_argument('--bucket',required=True);p.add_argument('--backup',type=pathlib.Path,required=True);p.add_argument('--apply',action='store_true');a=p.parse_args()
    if not re.fullmatch(r'[a-f0-9]{32}',a.account) or not re.fullmatch(r'[a-z0-9][a-z0-9-]{1,61}[a-z0-9]',a.bucket):p.error('Invalid account or bucket')
    token=os.environ.get('CLOUDFLARE_API_TOKEN','');
    if not token:p.error('CLOUDFLARE_API_TOKEN required')
    url=f'https://api.cloudflare.com/client/v4/accounts/{a.account}/r2/buckets/{a.bucket}/lifecycle'
    def call(method,data=None):
        r=urllib.request.Request(url,method=method,data=json.dumps(data).encode() if data is not None else None,headers={'Authorization':'Bearer '+token,'Content-Type':'application/json','User-Agent':'Switchyard-Operations/1.0'})
        with urllib.request.urlopen(r,timeout=30) as response:v=json.load(response)
        if not v.get('success'):raise RuntimeError('Lifecycle API rejected request')
        return v['result']
    before=call('GET');desired=merged_rules(before)
    # Exclusive private backup: never overwrite an earlier recovery artifact.
    fd=os.open(a.backup,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'w') as f:json.dump(before,f,indent=2)
    if a.apply:
        if call('GET')!=before:raise RuntimeError('Rules changed during preparation; retry with a new backup')
        call('PUT',desired)
        if call('GET')!=desired:raise RuntimeError('Readback differs; inspect private backup')
    print(json.dumps({'applied':a.apply,'prefix':'build-assets/','retention_days':30,'preserved_rules':len(before['rules']),'backup':str(a.backup)}))
if __name__=='__main__':main()
