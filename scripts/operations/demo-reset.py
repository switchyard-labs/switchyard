#!/usr/bin/env python3
"""Quiesced curated-demo Git + state reset with a private full-state backup.

External demo CI/event ingestion must be drained before invoking this command.
The command does not delete external Actions manifests, audit, or R2 payloads.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys


def run(tool, *args):
    subprocess.run([sys.executable, str(Path(__file__).with_name(tool)), *map(str, args)], check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=['baseline', 'plan', 'reset'])
    parser.add_argument('--trestle-db', required=True)
    parser.add_argument('--stopped-service-manifest', required=True)
    parser.add_argument('--remotes', required=True)
    parser.add_argument('--baseline', required=True)
    parser.add_argument('--backup', help='Fresh full backup directory, or resume the same interrupted reset')
    parser.add_argument('--data', help='Switchyard private data directory; required for reset')
    parser.add_argument('--config', help='Deployment environment; required for reset')
    parser.add_argument('--external-demo-effects-drained', action='store_true',
                        help='Operator attests demo CI finished and event ingestion is paused')
    parser.add_argument('--fixture', action='store_true')
    args = parser.parse_args()
    os.umask(0o077)
    root = Path(args.baseline)
    if root.is_symlink():
        raise ValueError('Baseline must not be a symlink')
    state_args = ['--trestle-db', args.trestle_db, '--stopped-service-manifest', args.stopped_service_manifest,
                  '--baseline', root / 'state.json']
    git_args = ['--remotes', args.remotes, '--baseline', root / 'git']
    if args.fixture:
        git_args.append('--fixture')
    if not args.external_demo_effects_drained:
        raise ValueError('Drain external demo CI and pause event ingestion first')
    if args.operation == 'baseline':
        root.mkdir(mode=0o700)
        run('demo-state.py', 'baseline', *state_args)
        run('demo-git.py', 'baseline', *git_args)
        return
    run('demo-state.py', 'plan', *state_args)
    run('demo-git.py', 'plan', *git_args)
    if args.operation == 'plan':
        return
    if not all((args.backup, args.data, args.config)):
        raise ValueError('Reset requires backup, data and config paths')
    backup = Path(args.backup)
    receipt = {'baseline': str(root.resolve()), 'database': str(Path(args.trestle_db).resolve()),
               'remotes': str(Path(args.remotes).resolve()),
               'baseline_state_sha256': hashlib.sha256((root / 'state.json').read_bytes()).hexdigest(),
               'baseline_git_sha256': hashlib.sha256((root / 'git' / 'manifest.json').read_bytes()).hexdigest()}
    if backup.exists():
        if json.loads((backup / 'reset-identity.json').read_text()) != receipt:
            raise ValueError('Existing backup belongs to another reset')
    else:
        backup.mkdir(mode=0o700)
        subprocess.run([sys.executable, str(Path(__file__).parents[1] / 'state-backup.py'), 'backup',
                        '--trestle-db', args.trestle_db, '--data', args.data, '--config', args.config,
                        '--target', str(backup / 'snapshot')], check=True)
        with open(backup / 'reset-identity.json', 'x', opener=lambda p, f: os.open(p, f, 0o600)) as out:
            json.dump(receipt, out)
    # Git must finish before coordination records are restored. A failed Git
    # reset leaves state intact and can resume with the same private journal.
    run('demo-git.py', 'reset', *git_args, '--journal', backup / 'git-before')
    state_backup = backup / 'state-before-reset.db'
    resume = ['--resume-backup'] if state_backup.exists() else []
    run('demo-state.py', 'reset', *state_args, '--backup', state_backup, *resume)
    print('Curated demo Git and state reset complete; verify before resuming ingestion/services')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit(str(error))
