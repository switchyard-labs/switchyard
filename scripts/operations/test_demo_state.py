import importlib.util
from pathlib import Path
import unittest
import contextlib
import io
import json
import sqlite3
import tempfile
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('demo_state', Path(__file__).with_name('demo-state.py'))
demo = importlib.util.module_from_spec(spec)
spec.loader.exec_module(demo)


def row(identity, **values):
    return ({'_id': identity}, values)


class ScopeTests(unittest.TestCase):
    def fixture(self):
        return {
            'attempts': [row('a', id='attempt', work_id='work', repo='demo-basic'),
                         row('other-a', id='other-attempt', work_id='other-work', repo='real')],
            'work': [row('w', id='work'), row('other-w', id='other-work')],
            'work_details': [row('d', work_id='work', repo='demo-basic')],
            'work_comments': [row('c', work_id='work'), row('other-c', work_id='other-work')],
            'prs': [row('p', id='pr', work_id='work', repo='demo-basic')],
            'findings': [row('f', target='attempt'), row('other-f', target='other-attempt')],
            'iq': [row('q', id='queue', repo='demo-basic', status='done')],
            'integration_effects': [row('e', queue_id='queue')],
            'executions': [row('x', id='exe', attempt_id='attempt', status='completed')],
            'action_runs': [row('run', repo='demo-basic', state='{"status":"success"}')],
            'credentials': [row('secret', repo='demo-basic')],
            'repository_meta': [row('owner', repo='demo-basic')],
            'releases': [row('release', repo='demo-basic')],
            'audit': [row('history', repo='demo-basic')],
        }

    def test_relations_and_protected_collections(self):
        scope = demo.select(self.fixture())
        ids = {r['_id'] for values in scope.values() for r in values}
        self.assertEqual(ids, {'a', 'w', 'd', 'c', 'p', 'f', 'q', 'e', 'x', 'run'})
        self.assertFalse({'credentials', 'repository_meta', 'releases', 'audit'} & scope.keys())

    def test_shared_work_refuses_entire_reset(self):
        rows = self.fixture()
        rows['attempts'].append(row('cross', work_id='work', repo='real'))
        with self.assertRaisesRegex(ValueError, 'non-demo'):
            demo.select(rows)

    def test_active_and_unknown_effects_refuse_reset(self):
        for name, field, value in [('iq', 'status', 'publishing'),
                                   ('executions', 'status', 'running'),
                                   ('action_runs', 'state', '{"status":"running"}'),
                                   ('action_runs', 'state', '{"status":"unknown"}')]:
            with self.subTest(collection=name, value=value):
                rows = self.fixture()
                rows[name][0][1][field] = value
                with self.assertRaisesRegex(ValueError, 'drain'):
                    demo.select(rows)

    def test_similarly_named_repo_is_not_a_demo(self):
        rows = {'attempts': [row('not-demo', repo='demo-basic-private', work_id='real')]}
        self.assertFalse(any(demo.select(rows).values()))


class DatabaseTests(unittest.TestCase):
    def test_reset_roundtrip_preserves_unrelated_rows_and_idempotency(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'db.sqlite'
            db = sqlite3.connect(path)
            db.execute('CREATE TABLE _trestle_collections(id TEXT,name TEXT)')
            db.execute('CREATE TABLE _trestle_fields(id TEXT,collection_id TEXT,name TEXT)')
            db.execute('CREATE TABLE _trestle_record_idempotency(collection_id TEXT,idempotency_key TEXT,record_id TEXT,created_at TEXT, PRIMARY KEY(collection_id,idempotency_key))')
            collections = {'attempts': ['id', 'work_id', 'repo', 'status'], 'work': ['id'],
                           'work_details': ['work_id', 'repo'], 'credentials': ['id', 'secret'],
                           'events': ['repo_name']}
            for name, fields in collections.items():
                cid = 'col-' + name
                db.execute('INSERT INTO _trestle_collections VALUES (?,?)', (cid, name))
                columns = ['_id TEXT PRIMARY KEY']
                for field in fields:
                    fid = name + '-' + field
                    db.execute('INSERT INTO _trestle_fields VALUES (?,?,?)', (fid, cid, field))
                    columns.append(demo.quoted('f_' + demo.digest(fid)) + ' TEXT')
                db.execute('CREATE TABLE ' + demo.quoted('_trestle_data_' + demo.digest(cid)) + '(' + ','.join(columns) + ')')
            db.commit()
            cat = demo.catalog(db)

            def insert(name, identity, **values):
                info = cat[name]
                columns = ['_id'] + [info['fields'][f] for f in values]
                db.execute('INSERT INTO ' + demo.quoted(info['table']) + '(' + ','.join(map(demo.quoted, columns))
                           + ') VALUES (' + ','.join('?' for _ in columns) + ')', [identity, *values.values()])

            insert('attempts', 'a', id='attempt', work_id='work', repo='demo-basic', status='open')
            insert('work', 'w', id='work')
            insert('work_details', 'd', work_id='work', repo='demo-basic')
            insert('credentials', 'secret', id='credential', secret='fixture-only')
            insert('events', 'real-event', repo_name='real')
            db.execute('INSERT INTO _trestle_record_idempotency VALUES (?,?,?,?)', (cat['attempts']['id'], 'baseline-key', 'a', 'now'))
            db.commit()
            baseline = root / 'baseline.json'
            common = ['--trestle-db', str(path), '--stopped-service-manifest', str(root / 'pids'), '--baseline', str(baseline)]

            def invoke(operation, *extra):
                with patch('sys.argv', ['demo-state.py', operation, *common, *extra]), patch.object(demo, 'stopped') as stopped, contextlib.redirect_stdout(io.StringIO()):
                    demo.main()
                    stopped.assert_called_once()

            invoke('baseline')
            original = demo.records(db, cat)
            db.execute('DELETE FROM ' + demo.quoted(cat['attempts']['table']))
            db.execute('DELETE FROM _trestle_record_idempotency')
            insert('attempts', 'new', id='new-attempt', work_id='new-work', repo='demo-basic')
            insert('work', 'new-work-row', id='new-work')
            insert('events', 'demo-event', repo_name='demo-basic')
            db.commit()
            invoke('reset', '--backup', str(root / 'before.sqlite'))
            self.assertEqual(demo.records(db, cat), original)
            self.assertEqual(db.execute('SELECT idempotency_key,record_id FROM _trestle_record_idempotency').fetchall(), [('baseline-key', 'a')])
            self.assertEqual((root / 'before.sqlite').stat().st_mode & 0o777, 0o600)
            # Backup is exclusive: a second reset cannot overwrite it.
            with self.assertRaises(FileExistsError):
                invoke('reset', '--backup', str(root / 'before.sqlite'))
            self.assertEqual(demo.records(db, cat), original)
            db.close()


if __name__ == '__main__':
    unittest.main()
