#!/usr/bin/env python3
"""Private curated-demo Git baselines and leased, resumable reset.

Credentials are inherited through Git's environment configuration, never URLs or
arguments. Each repository push is atomic; there is no cross-repository atomicity.
Keep the control plane and external demo Actions drained during reset.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
from urllib.parse import urlsplit

DEMOS = sorted('demo-basic demo-agents demo-conflict demo-semantic demo-workflow'.split())


def git(*args):
    env = {key: value for key, value in os.environ.items()
           if not key.startswith('GIT_TRACE') and key != 'GIT_CURL_VERBOSE'}
    try:
        result = subprocess.run(['git', *map(str, args)], capture_output=True, text=True, env=env, timeout=180)
    except subprocess.TimeoutExpired:
        raise ValueError('Git operation timed out; retain the private journal for recovery') from None
    if result.returncode:
        # Git diagnostics can contain a credential-bearing server response.
        raise ValueError('Git operation failed; no server response logged')
    return result.stdout


def refs(remote):
    result = {}
    for line in git('ls-remote', '--refs', remote).splitlines():
        sha, name = line.split('\t')
        if not name.startswith(('refs/heads/', 'refs/tags/')):
            raise ValueError('Unexpected remote ref namespace')
        result[name] = sha
    return result


def write(path, value):
    temp = path.with_suffix('.new')
    with open(temp, 'w', opener=lambda p, flags: os.open(p, flags, 0o600)) as out:
        json.dump(value, out, sort_keys=True)
        out.write('\n')
        out.flush()
        os.fsync(out.fileno())
    os.replace(temp, path)


def valid_remotes(remotes, fixture):
    if sorted(remotes) != DEMOS:
        raise ValueError('Remote map must contain exactly the five curated demos')
    for name, remote in remotes.items():
        parsed = urlsplit(remote)
        if fixture and parsed.scheme == 'file':
            path = Path(parsed.path).resolve()
            if not path.is_relative_to('/tmp') or path.name != name + '.git':
                raise ValueError('Fixture remote must be a named /tmp bare repository')
        elif (parsed.scheme != 'https' or parsed.username or parsed.password or parsed.query
              or parsed.fragment or not parsed.hostname or not parsed.path.endswith('/' + name + '.git')):
            raise ValueError('Remote must be token-free HTTPS for the exact curated demo')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=['baseline', 'plan', 'reset'])
    parser.add_argument('--remotes', required=True, help='Private JSON name-to-token-free-URL map')
    parser.add_argument('--baseline', required=True, help='Fresh directory for baseline, existing for reset')
    parser.add_argument('--journal', help='Fresh directory for recovery journal and pre-reset mirrors')
    parser.add_argument('--fixture', action='store_true', help='Permit /tmp file remotes for isolated tests')
    args = parser.parse_args()
    os.umask(0o077)
    remotes = json.loads(Path(args.remotes).read_text())
    valid_remotes(remotes, args.fixture)
    root = Path(args.baseline)
    if root.is_symlink():
        raise ValueError('Baseline directory cannot be a symlink')
    if args.operation == 'baseline':
        root.mkdir(mode=0o700)
        manifest = {'format': 1, 'remotes': remotes, 'refs': {}}
        for name, remote in remotes.items():
            before = refs(remote)
            git('clone', '--mirror', remote, root / (name + '.git'))
            if before != refs(remote) or before != refs(root / (name + '.git')):
                raise ValueError('Remote changed while baseline was captured')
            manifest['refs'][name] = before
        write(root / 'manifest.json', manifest)
        print('Five private Git baselines captured')
        return
    manifest = json.loads((root / 'manifest.json').read_text())
    if manifest.get('format') != 1 or manifest['remotes'] != remotes:
        raise ValueError('Baseline remote identity mismatch')
    for name in DEMOS:
        if refs(root / (name + '.git')) != manifest['refs'][name]:
            raise ValueError('Baseline mirror refs changed')
        git('--git-dir=' + str(root / (name + '.git')), 'fsck', '--full')
    current = {name: refs(remote) for name, remote in remotes.items()}
    print(json.dumps({'operation': args.operation, 'changed_repositories':
                      [n for n in DEMOS if current[n] != manifest['refs'][n]]}))
    if args.operation == 'plan':
        return
    if not args.journal:
        raise ValueError('Reset requires a private journal directory')
    journal = Path(args.journal)
    identity = hashlib.sha256(json.dumps(manifest, sort_keys=True).encode()).hexdigest()
    if journal.exists():
        state = json.loads((journal / 'state.json').read_text())
        if state['baseline_sha256'] != identity:
            raise ValueError('Recovery journal belongs to another baseline')
    else:
        journal.mkdir(mode=0o700)
        state = {'baseline_sha256': identity, 'before': current, 'completed': []}
        # Back up every repository before mutating the first one.
        for name, remote in remotes.items():
            git('clone', '--mirror', remote, journal / (name + '.git'))
            if refs(journal / (name + '.git')) != current[name] or refs(remote) != current[name]:
                raise ValueError('Remote changed while pre-reset backup was captured')
        write(journal / 'state.json', state)
    for name in DEMOS:
        desired = manifest['refs'][name]
        observed = refs(remotes[name])
        if observed == desired:
            if name not in state['completed']:
                state['completed'].append(name)
                write(journal / 'state.json', state)
            continue
        if observed != state['before'][name] or name in state['completed']:
            raise ValueError('Remote changed after reset planning; refusing overwrite')
        names = sorted(set(observed) | set(desired))
        leases = ['--force-with-lease=' + ref + ':' + observed.get(ref, '') for ref in names]
        specs = [desired.get(ref, '') + ':' + ref for ref in names]
        git('--git-dir=' + str(root / (name + '.git')), 'push', '--atomic', *leases, remotes[name], *specs)
        if refs(remotes[name]) != desired:
            raise ValueError('Reset ref verification failed')
        state['completed'].append(name)
        write(journal / 'state.json', state)
    print('Five Git repositories reset and verified; private recovery mirrors retained')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError) as error:
        raise SystemExit(str(error))
