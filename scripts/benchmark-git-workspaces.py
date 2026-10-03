#!/usr/bin/env python3
"""Measure disposable clones versus a locked mirror and isolated worktrees.

This is a Git substrate benchmark, not an end-to-end provider/semantic benchmark.
All repositories and candidates are temporary; no credentials or remote pushes.
"""
import concurrent.futures
import json
import os
from pathlib import Path
import statistics
import subprocess
import tempfile
import threading
import time


def git(path, *args):
    env = {**os.environ, 'GIT_CONFIG_NOSYSTEM': '1',
           'GIT_AUTHOR_NAME': 'Benchmark', 'GIT_AUTHOR_EMAIL': 'benchmark@example.invalid',
           'GIT_COMMITTER_NAME': 'Benchmark', 'GIT_COMMITTER_EMAIL': 'benchmark@example.invalid',
           'GIT_AUTHOR_DATE': '2026-01-01T00:00:00Z', 'GIT_COMMITTER_DATE': '2026-01-01T00:00:00Z'}
    return subprocess.run(['git', '-C', str(path), *args], env=env, check=True,
                          capture_output=True, text=True, timeout=30).stdout.strip()


def benchmark():
    rows = []
    with tempfile.TemporaryDirectory(prefix='switchyard-c23-') as root:
        root = Path(root)
        source = root / 'source'
        source.mkdir()
        git(source, 'init', '-b', 'main')
        for i in range(20):
            (source / 'routes.txt').write_text('north=yard\nsouth=depot\n' + f'# revision {i}\n')
            git(source, 'add', 'routes.txt')
            git(source, 'commit', '-qm', f'Revision {i}')
        base = git(source, 'rev-parse', 'HEAD')
        git(source, 'checkout', '-qb', 'candidate')
        (source / 'signal.txt').write_text('north=green\n')
        git(source, 'add', 'signal.txt')
        git(source, 'commit', '-qm', 'Add signal')
        candidate = git(source, 'rev-parse', 'HEAD')
        git(source, 'checkout', '-q', 'main')
        remote, mirror = root / 'remote.git', root / 'mirror.git'
        git(root, 'clone', '--bare', '--no-local', str(source), str(remote))
        mirror_started = time.perf_counter()
        git(root, 'clone', '--mirror', '--no-local', str(remote), str(mirror))
        cold_mirror_ms = (time.perf_counter() - mirror_started) * 1000
        lock = threading.Lock()

        def task(mode, operation, index):
            path = root / f'{mode}-{operation}-{concurrency}-{index}'
            began = time.perf_counter()
            if mode == 'clone':
                git(root, 'clone', '--no-local', '--quiet', '--no-checkout', str(remote), str(path))
                git(path, 'checkout', '--quiet', '--detach', base)
            else:
                with lock:
                    git(mirror, 'worktree', 'add', '--quiet', '--detach', str(path), base)
            try:
                if git(path, 'rev-parse', 'HEAD') != base:
                    raise AssertionError('Workspace did not pin the exact base')
                if operation == 'browse':
                    result = git(path, 'ls-tree', '-r', base)
                elif operation == 'preview':
                    git(path, 'merge', '--no-commit', '--no-ff', candidate)
                    result = git(path, 'write-tree')
                elif operation == 'comparison':
                    result = git(path, 'diff', '--name-status', base, candidate)
                else:
                    git(path, 'merge', '--no-ff', '-m', 'Integration candidate', candidate)
                    result = git(path, 'rev-parse', 'HEAD^{tree}')
                return (time.perf_counter() - began) * 1000, result
            finally:
                if mode == 'mirror':
                    with lock:
                        git(mirror, 'worktree', 'remove', '--force', str(path))

        for operation in ['browse', 'preview', 'comparison', 'integration-candidate']:
            for concurrency in [1, 3, 10, 25]:
                expected = None
                for mode in ['clone', 'mirror']:
                    if mode == 'mirror':
                        with lock:
                            git(mirror, 'fetch', '--quiet', '--prune', 'origin')
                    started = time.perf_counter()
                    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as pool:
                        results = list(pool.map(lambda n: task(mode, operation, n), range(concurrency)))
                    duration = (time.perf_counter() - started) * 1000
                    identities = {result for _, result in results}
                    if len(identities) != 1:
                        raise AssertionError('Concurrent workspaces produced different results')
                    identity = identities.pop()
                    if expected is not None and expected != identity:
                        raise AssertionError('Clone and mirror results differ')
                    expected = identity
                    rows.append({'operation': operation, 'concurrency': concurrency, 'mode': mode,
                                 'batch_ms': round(duration, 2),
                                 'median_workspace_ms': round(statistics.median(ms for ms, _ in results), 2),
                                 'exact_base_verified': True, 'result_matches': True})
        if git(remote, 'rev-parse', 'refs/heads/main') != base:
            raise AssertionError('Benchmark changed the source branch')
    return {'scope': 'local Git substrate; no provider/network or semantic engine timings',
            'cold_mirror_ms': round(cold_mirror_ms, 2), 'rows': rows}


if __name__ == '__main__':
    print(json.dumps(benchmark(), indent=2))
