#!/usr/bin/env python3
"""Quiesced Switchyard state snapshot/restore. Never prints config or key bytes.
Stop Switchyard before snapshot. Trestle SQLite backup uses its online protocol.
Restore only to a new private directory; configuring service paths is explicit.
"""
import argparse, hashlib, json, os, shutil, sqlite3
from pathlib import Path
p=argparse.ArgumentParser();sub=p.add_subparsers(dest='operation',required=True)
b=sub.add_parser('backup');b.add_argument('--trestle-db',required=True);b.add_argument('--data',required=True);b.add_argument('--config',required=True);b.add_argument('--target',required=True)
r=sub.add_parser('restore');r.add_argument('--source',required=True);r.add_argument('--target',required=True)
a=p.parse_args();os.umask(0o077);target=Path(a.target);target.mkdir(mode=0o700)
def manifest(root):
 result={}
 for path in sorted(root.rglob('*')):
  if path.is_symlink():raise RuntimeError('Symlink in state snapshot; preserve needed external state explicitly')
  if path.is_file() and path!=root/'manifest.json':result[str(path.relative_to(root))]=hashlib.sha256(path.read_bytes()).hexdigest()
 return result
if a.operation=='backup':
 source=sqlite3.connect('file:'+str(Path(a.trestle_db).resolve())+'?mode=ro',uri=True)
 with sqlite3.connect(target/'trestle.db') as destination:
  source.backup(destination)
  if destination.execute('PRAGMA integrity_check').fetchone()[0]!='ok':raise RuntimeError('SQLite integrity failed')
 destination.close()
 source.close()
 (target/"trestle-state").mkdir()
 db_path=Path(a.trestle_db).resolve()
 for item in db_path.parent.iterdir():
  if item.name in {db_path.name,db_path.name+"-wal",db_path.name+"-shm"}:continue
  if item.is_symlink():raise RuntimeError("Symlink in Trestle state")
  if item.is_dir():shutil.copytree(item,target/"trestle-state"/item.name)
  else:shutil.copy2(item,target/"trestle-state"/item.name)
 shutil.copytree(a.data,target/'data',symlinks=True)
 shutil.copy2(a.config,target/'deployment.env')
 (target/'deployment.env').chmod(0o600)
 (target/'manifest.json').write_text(json.dumps(manifest(target),indent=2)+'\n')
else:
 source=Path(a.source);expected=json.loads((source/'manifest.json').read_text())
 if manifest(source)!=expected:raise RuntimeError('Backup manifest mismatch')
 shutil.copytree(source,target,dirs_exist_ok=True,symlinks=True)
 if manifest(target)!=expected:raise RuntimeError('Restored state mismatch')
 with sqlite3.connect('file:'+str((target/'trestle.db').resolve())+'?mode=ro',uri=True) as db:
  if db.execute('PRAGMA integrity_check').fetchone()[0]!='ok':raise RuntimeError('Restored SQLite integrity failed')
print(a.operation+' verified; state remains private')
