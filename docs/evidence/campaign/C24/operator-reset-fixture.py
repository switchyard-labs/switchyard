import hashlib,json,pathlib,sqlite3,subprocess,sys,tempfile
root=pathlib.Path(tempfile.mkdtemp(prefix='switchyard-reset-fixture-'))
tool=pathlib.Path(__file__).parent/'scripts/operations/demo-reset.py'
def run(*a): return subprocess.run(list(map(str,a)),check=True,capture_output=True,text=True).stdout
names='demo-basic demo-agents demo-conflict demo-semantic demo-workflow'.split(); rem={}
for n in names:
 r=root/(n+'.git');run('git','init','--bare',r);w=root/n;run('git','init',w);run('git','-C',w,'config','user.name','Reset fixture');run('git','-C',w,'config','user.email','fixture@example.invalid');(w/'README').write_text('baseline\n');run('git','-C',w,'add','.');run('git','-C',w,'commit','-m','baseline');run('git','-C',w,'branch','-M','main');run('git','-C',w,'push',r,'main');rem[n]=r.as_uri()
(root/'remotes.json').write_text(json.dumps(rem));(root/'pids.json').write_text(json.dumps([{'name':'app','pid':2147483647},{'name':'trestle','pid':2147483646}]))
(root/'trestle-state').mkdir(); dbpath=root/'trestle-state/trestle.db';db=sqlite3.connect(dbpath)
db.execute('CREATE TABLE _trestle_collections(id TEXT,name TEXT)');db.execute('CREATE TABLE _trestle_fields(id TEXT,collection_id TEXT,name TEXT)');db.execute('CREATE TABLE _trestle_record_idempotency(collection_id TEXT,idempotency_key TEXT,record_id TEXT,created_at TEXT,PRIMARY KEY(collection_id,idempotency_key))')
fields={'attempts':['id','work_id','repo','status'],'work':['id'],'work_details':['work_id','repo'],'credentials':['id','secret'],'events':['repo_name'],'action_runs':['id','repo','state'],'demo_action_exclusions':['id','repo','run_id','created_at']}
def h(s): return hashlib.sha256(s.encode()).hexdigest()[:16]
def q(s): return '"'+s+'"'
def table(n): return '_trestle_data_'+h('col-'+n)
for n,ff in fields.items():
 cid='col-'+n;db.execute('INSERT INTO _trestle_collections VALUES (?,?)',(cid,n));cc=['_id TEXT PRIMARY KEY','_version INTEGER','_created TEXT','_updated TEXT']
 for f in ff:
  fid=n+'-'+f;db.execute('INSERT INTO _trestle_fields VALUES (?,?,?)',(fid,cid,f));cc.append(q('f_'+h(fid))+' TEXT')
 db.execute('CREATE TABLE '+q(table(n))+'('+','.join(cc)+')')
def insert(n,i,**v):
 cc=['_id','_version','_created','_updated']+['f_'+h(n+'-'+f) for f in v];db.execute('INSERT INTO '+q(table(n))+'('+','.join(map(q,cc))+') VALUES ('+','.join('?' for _ in cc)+')',[i,1,'now','now',*v.values()])
for n in names:
 insert('attempts',n+'-a',id=n+'-attempt',work_id=n+'-work',repo=n,status='open');insert('work',n+'-w',id=n+'-work');insert('work_details',n+'-d',work_id=n+'-work',repo=n)
insert('credentials','credential',id='credential',secret='fixture-only');insert('events','real-event',repo_name='real')
db.execute('INSERT INTO _trestle_record_idempotency VALUES (?,?,?,?)',('col-attempts','baseline-key','demo-basic-a','now'));db.commit();db.close()
(root/'data').mkdir();(root/'data/key').write_text('fixture-only');(root/'config.env').write_text('FIXTURE_ONLY=true\n')
common=['--trestle-db',dbpath,'--stopped-service-manifest',root/'pids.json','--remotes',root/'remotes.json','--baseline',root/'baseline','--external-demo-effects-drained','--fixture']
run(sys.executable,tool,'baseline',*common)
def snapshot():
 c=sqlite3.connect(dbpath)
 try: return {n:c.execute('SELECT * FROM '+q(table(n))).fetchall() for n in fields if n!='demo_action_exclusions'}
 finally: c.close()
before=snapshot()
for n in names:
 w=root/n;(w/'README').write_text('mutated\n');run('git','-C',w,'commit','-am','mutate');run('git','-C',w,'push',rem[n],'main');run('git','-C',w,'push',rem[n],'HEAD:refs/heads/extra')
db=sqlite3.connect(dbpath)
insert('attempts','new-a',id='new-attempt',work_id='new-work',repo='demo-basic',status='open');insert('work','new-w',id='new-work');insert('events','demo-event',repo_name='demo-basic');insert('action_runs','ci-run',id='finished-ci',repo='demo-basic',state='{"status":"success"}');db.commit();db.close()
reset=['--backup',root/'backup','--data',root/'data','--config',root/'config.env']
run(sys.executable,tool,'reset',*common,*reset)
assert snapshot()==before
run(sys.executable,tool,'reset',*common,*reset)
assert snapshot()==before
with sqlite3.connect(dbpath) as c:
 assert c.execute('SELECT idempotency_key,record_id FROM _trestle_record_idempotency').fetchall()==[('baseline-key','demo-basic-a')]
 assert c.execute('SELECT COUNT(*) FROM '+q(table('demo_action_exclusions'))).fetchone()[0]==1
 assert c.execute('PRAGMA integrity_check').fetchone()[0]=='ok'
manifest=json.loads((root/'baseline/git/manifest.json').read_text())
for n in names:
 got=dict(line.split('\t')[::-1] for line in run('git','ls-remote','--refs',rem[n]).splitlines());assert got==manifest['refs'][n]
assert (root/'backup/snapshot/manifest.json').exists()
print(json.dumps({'result':'PASS','repositories':5,'git_baseline_restored':True,'extra_branches_removed':True,'state_roundtrip':True,'credentials_and_non_demo_activity_preserved':True,'idempotency_restored':True,'actions_rediscovery_exclusion':True,'repeat_reset_idempotent':True,'private_full_backup':True,'sqlite_integrity':'ok','fixture_root':str(root)}))
