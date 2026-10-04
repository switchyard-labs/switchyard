#!/usr/bin/env python3
"""Offline, selective curated-demo state baseline/reset. Git reset is separate.

Requires stopped Switchyard and Trestle processes, a schema-identical baseline,
and a fresh private backup. Never selects credentials, configuration, ownership,
release assets, audit history or global event cursors. No service is stopped here.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import datetime
import uuid

DEMOS = frozenset('demo-basic demo-agents demo-conflict demo-semantic demo-workflow'.split())
DIRECT = {
    'attempts': 'repo', 'prs': 'repo', 'iq': 'repo', 'runs': 'repo',
    'drafts': 'repo', 'commit_checks': 'repo', 'ref_obs': 'repo',
    'ref_updates': 'repo', 'events': 'repo_name', 'event_receipts': 'repo',
    'action_runs': 'repo', 'action_jobs': 'repo', 'action_steps': 'repo', 'action_checks': 'repo',
    'external_executions': 'repo', 'execution_metadata': 'repo',
}


def digest(value):
    return hashlib.sha256(value.encode()).hexdigest()[:16]


def quoted(value):
    return '"' + value.replace('"', '""') + '"'


def catalog(db):
    result = {}
    for cid, name in db.execute('SELECT id,name FROM _trestle_collections'):
        fields = dict(db.execute('SELECT name,id FROM _trestle_fields WHERE collection_id=?', (cid,)))
        table = '_trestle_data_' + digest(cid)
        columns = [r[1] for r in db.execute('PRAGMA table_info(' + quoted(table) + ')')]
        if not columns:
            raise ValueError('Unsupported Trestle collection storage')
        mapping = {name: 'f_' + digest(fid) for name, fid in fields.items()}
        if not set(mapping.values()).issubset(columns):
            raise ValueError('Unsupported Trestle field storage')
        result[name] = {'id': cid, 'table': table, 'columns': columns, 'fields': mapping}
    return result


def records(db, cat):
    result = {}
    for name, info in cat.items():
        rows = []
        for row in db.execute('SELECT * FROM ' + quoted(info['table'])):
            physical = dict(zip(info['columns'], row))
            logical = {field: physical[column] for field, column in info['fields'].items()}
            rows.append((physical, logical))
        result[name] = rows
    return result


def select(rows):
    """Build the complete relation scope before deleting anything."""
    for _, value in rows.get('workflow_runs', []):
        if value.get('status') not in {'completed', 'failed', 'cancelled'}:
            raise ValueError('Workflow effects must drain or be cancelled before demo reset')
    selected = {name: [] for name in DIRECT}
    for name, field in DIRECT.items():
        selected[name] = [raw for raw, value in rows.get(name, []) if value.get(field) in DEMOS]
    attempts = {v.get('id') for _, v in rows.get('attempts', []) if v.get('repo') in DEMOS}
    work = {v.get('work_id') for _, v in rows.get('attempts', []) if v.get('repo') in DEMOS}
    work |= {v.get('work_id') for _, v in rows.get('work_details', []) if v.get('repo') in DEMOS}
    work.discard(None)
    # Cross-repository Work must never be partially reset.
    for name in ('attempts', 'work_details', 'prs'):
        for _, value in rows.get(name, []):
            if value.get('work_id') in work and value.get('repo') not in DEMOS:
                raise ValueError('Demo Work also references a non-demo repository')
    prs = {v.get('id') for _, v in rows.get('prs', []) if v.get('repo') in DEMOS}
    queues = {v.get('id') for _, v in rows.get('iq', []) if v.get('repo') in DEMOS}
    executions = {v.get('execution_id') for _, v in rows.get('execution_metadata', []) if v.get('repo') in DEMOS}
    relations = {
        'work': ('id', work), 'work_details': ('work_id', work),
        'work_comments': ('work_id', work), 'findings': ('target', attempts),
        'executions': ('id', executions), 'pr_checks': ('pr_id', prs),
        'integration_effects': ('queue_id', queues),
    }
    for name, (field, ids) in relations.items():
        selected[name] = [raw for raw, value in rows.get(name, []) if value.get(field) in ids]
    # Older executions can lack metadata; include only an explicit demo Attempt.
    known = {r['_id'] for r in selected['executions']}
    selected['executions'] += [raw for raw, v in rows.get('executions', [])
                               if v.get('attempt_id') in attempts and raw['_id'] not in known]
    for name in ('executions', 'iq', 'action_runs'):
        ids = {r['_id'] for r in selected.get(name, [])}
        for raw, value in rows.get(name, []):
            if raw['_id'] not in ids:
                continue
            state = value.get('status')
            if name == 'action_runs':
                state = json.loads(value.get('state') or '{}').get('status')
            terminal = {'executions': {'completed', 'succeeded', 'failed', 'cancelled', 'success', 'error'},
                        'iq': {'done', 'failed', 'cancelled', 'conflict', 'blocked'},
                        'action_runs': {'success', 'failure', 'failed', 'timed_out', 'cancelled', 'skipped'}}
            if state not in terminal[name]:
                raise ValueError('Demo effects must drain before state reset')
    return selected


def stopped(pid_manifest, db_path):
    services = json.loads(Path(pid_manifest).read_text())
    if {s['name'] for s in services} != {'app', 'trestle'}:
        raise ValueError('Manifest must identify app and trestle')
    for service in services:
        pid = service['pid']
        if not isinstance(pid, int) or pid <= 1:
            raise ValueError('Invalid service PID')
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            continue
        raise ValueError('Services must be stopped before this operation')
    # A stale manifest alone is insufficient: detect any process with this DB open.
    for process in Path('/proc').iterdir():
        if not process.name.isdigit() or int(process.name) == os.getpid():
            continue
        try:
            for fd in (process / 'fd').iterdir():
                try:
                    target = fd.resolve()
                except FileNotFoundError:
                    continue
                if str(target) in {str(db_path), str(db_path) + '-wal', str(db_path) + '-shm'}:
                    raise ValueError('Trestle database still open by another process')
        except (FileNotFoundError, ProcessLookupError):
            continue
        except PermissionError:
            raise ValueError('Cannot verify quiescence; run with host process visibility')


def private_write(path, value):
    with open(path, 'x', opener=lambda p, flags: os.open(p, flags, 0o600)) as out:
        json.dump(value, out, sort_keys=True)
        out.write('\n')
        out.flush()
        os.fsync(out.fileno())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=['baseline', 'plan', 'reset'])
    parser.add_argument('--trestle-db', required=True)
    parser.add_argument('--stopped-service-manifest', required=True)
    parser.add_argument('--baseline', required=True)
    parser.add_argument('--backup', help='Fresh private SQLite backup required for reset')
    parser.add_argument('--resume-backup', action='store_true', help='Reuse only an integrity-checked backup from an interrupted reset')
    args = parser.parse_args()
    os.umask(0o077)
    path = Path(args.trestle_db)
    if path.is_symlink() or not path.is_file():
        raise ValueError('Database must be an existing regular file')
    path = path.resolve()
    stopped(args.stopped_service_manifest, path)
    db = sqlite3.connect('file:' + str(path) + '?mode=rw', uri=True)
    db.row_factory = None
    if db.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
        raise ValueError('Database integrity failed')
    cat = catalog(db)
    scope = select(records(db, cat))
    if args.operation == 'baseline':
        keys = []
        for name, values in scope.items():
            for row in values:
                keys += [list(r) for r in db.execute('SELECT * FROM _trestle_record_idempotency WHERE collection_id=? AND record_id=?',
                                                    (cat[name]['id'], row['_id']))]
        private_write(args.baseline, {'format': 1, 'catalog': cat, 'scope': scope, 'idempotency': keys})
        print('Private demo state baseline created')
        return
    baseline = json.loads(Path(args.baseline).read_text())
    if baseline.get('format') != 1 or baseline.get('catalog') != cat:
        raise ValueError('Baseline belongs to a different schema or database identity')
    # A tampered or manually edited baseline must not introduce non-demo rows.
    rebuilt = {}
    for name, values in baseline['scope'].items():
        if name not in scope:
            raise ValueError('Unexpected baseline collection')
        if name not in cat and not values:
            continue
        info = cat[name]
        rebuilt[name] = [(r, {f: r[c] for f, c in info['fields'].items()}) for r in values]
    if select(rebuilt) != baseline['scope']:
        raise ValueError('Baseline contains records outside the demo scope')
    identities = {(cat[n]['id'], r['_id']) for n, values in baseline['scope'].items() for r in values}
    for key in baseline.get('idempotency', []):
        if len(key) != 4 or (key[0], key[2]) not in identities:
            raise ValueError('Baseline idempotency record outside demo scope')
    print(json.dumps({'operation': args.operation, 'delete_counts': {k: len(v) for k, v in scope.items()},
                      'restore_counts': {k: len(v) for k, v in baseline['scope'].items()}}))
    if args.operation == 'plan':
        return
    if not args.backup:
        raise ValueError('Reset requires a fresh private backup')
    backup = Path(args.backup)
    checksum = Path(str(backup) + '.sha256')
    if args.resume_backup:
        if backup.is_symlink() or hashlib.sha256(backup.read_bytes()).hexdigest() != checksum.read_text().strip():
            raise ValueError('Interrupted-reset backup integrity mismatch')
    else:
        fd = os.open(backup, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
        os.close(fd)
        with sqlite3.connect(backup) as target:
            db.backup(target)
            if target.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
                raise ValueError('Reset backup integrity failed')
        with open(checksum, 'x', opener=lambda p, flags: os.open(p, flags, 0o600)) as out:
            out.write(hashlib.sha256(backup.read_bytes()).hexdigest() + '\n')
    db.execute('BEGIN EXCLUSIVE')
    try:
        retained = {r['_id'] for r in baseline['scope'].get('action_runs', [])}
        excluded_runs = [r for r in scope['action_runs'] if r['_id'] not in retained]
        if excluded_runs:
            info = cat.get('demo_action_exclusions')
            if not info:
                raise ValueError('Upgrade to schema 8 before resetting demo Actions')
            run_fields = cat['action_runs']['fields']
            now = datetime.datetime.now(datetime.timezone.utc).isoformat()
            for run in excluded_runs:
                repo, run_id = run[run_fields['repo']], run[run_fields['id']]
                identity = 'demo-reset-' + hashlib.sha256((repo + '\0' + run_id).encode()).hexdigest()
                if db.execute('SELECT 1 FROM ' + quoted(info['table']) + ' WHERE '
                              + quoted(info['fields']['id']) + '=?', (identity,)).fetchone():
                    continue
                row = {'_id': 'rec_' + uuid.uuid4().hex, '_version': 1, '_created': now, '_updated': now}
                row.update({info['fields'][k]: v for k, v in {'id': identity, 'repo': repo, 'run_id': run_id, 'created_at': now}.items()})
                columns = list(row)
                db.execute('INSERT INTO ' + quoted(info['table']) + ' (' + ','.join(map(quoted, columns))
                           + ') VALUES (' + ','.join('?' for _ in columns) + ')', [row[c] for c in columns])
        for name, values in scope.items():
            info = cat.get(name)
            if not info:
                continue
            for row in values:
                db.execute('DELETE FROM ' + quoted(info['table']) + ' WHERE _id=?', (row['_id'],))
                db.execute('DELETE FROM _trestle_record_idempotency WHERE collection_id=? AND record_id=?',
                           (info['id'], row['_id']))
            for row in baseline['scope'].get(name, []):
                columns = list(row)
                db.execute('INSERT INTO ' + quoted(info['table']) + ' (' + ','.join(map(quoted, columns))
                           + ') VALUES (' + ','.join('?' for _ in columns) + ')', [row[c] for c in columns])
        for key in baseline.get('idempotency', []):
            db.execute('INSERT INTO _trestle_record_idempotency VALUES (?,?,?,?)', key)
        if db.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
            raise ValueError('Reset integrity failed')
        db.commit()
    except BaseException:
        db.rollback()
        raise
    print('Demo state reset committed; Git refs and external Actions must be reset separately')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, sqlite3.Error, OSError) as error:
        raise SystemExit(str(error))
