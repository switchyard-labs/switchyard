#!/usr/bin/env python3
"""Push a disposable branch and capture automatic SSE/Actions evidence.

Requires an approved Actions definition for the branch. Never calls event ingest.
Git capabilities live only in the child environment, not argv or Git config.
"""
import argparse
import http.cookiejar
import json
import os
import pathlib
import subprocess
import tempfile
import threading
import time
import urllib.request

p = argparse.ArgumentParser()
p.add_argument('--base', required=True)
p.add_argument('--cookies', required=True)
p.add_argument('--owner', required=True)
p.add_argument('--repo', required=True)
p.add_argument('--branch', required=True)
p.add_argument('--marker', required=True)
p.add_argument('--output', required=True)
p.add_argument('--create', action='store_true')
a = p.parse_args()
if not a.branch.startswith('event-') or '..' in a.branch:
    p.error('Use a disposable event-* branch')
jar = http.cookiejar.MozillaCookieJar(a.cookies)
jar.load(ignore_discard=True, ignore_expires=True)
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
base = a.base.rstrip('/')
repo_url = base + '/api/repositories/' + a.owner + '/' + a.repo
def get(url):
    with opener.open(url, timeout=30) as response:
        return json.load(response)
request = urllib.request.Request(repo_url + '/git-credential',
    data=b'{"scope":"write","ttl_seconds":300}',
    headers={'Content-Type': 'application/json'})
with opener.open(request, timeout=30) as response:
    capability = json.load(response)
env = dict(os.environ, GIT_CONFIG_COUNT='1', GIT_CONFIG_KEY_0='http.extraHeader',
    GIT_CONFIG_VALUE_0='Authorization: Bearer ' + capability['token'],
    GIT_TERMINAL_PROMPT='0', GIT_CONFIG_GLOBAL='/dev/null', GIT_CONFIG_NOSYSTEM='1')
def git(*args, cwd=None):
    result = subprocess.run(['git', *args], cwd=cwd, env=env,
        capture_output=True, timeout=90)
    if result.returncode:
        raise RuntimeError('Git step failed: ' + args[0] + '; output withheld')
    return result.stdout.decode().strip()
frames = []
ready = threading.Event()
def stream():
    try:
        with opener.open(base + '/api/events/stream', timeout=60) as response:
            frame = []
            for line in response:
                text = line.decode().strip()
                if text:
                    frame.append(text)
                else:
                    frames.append({'received_unix': time.time(), 'frame': '\n'.join(frame)})
                    if 'event: ready' in frame:
                        ready.set()
                    frame = []
    except (OSError, TimeoutError):
        pass
threading.Thread(target=stream, daemon=True).start()
if not ready.wait(10):
    raise RuntimeError('Authenticated SSE stream did not become ready')
with tempfile.TemporaryDirectory(prefix='switchyard-event-cert-') as temp:
    checkout = pathlib.Path(temp) / 'repo'
    git('clone', '--quiet', capability['remote'], str(checkout))
    branch_args = [] if a.create else ['origin/' + a.branch]
    git('checkout', '-b', a.branch, *branch_args, cwd=checkout)
    (checkout / 'event-certification.txt').write_text(a.marker + '\n')
    git('add', 'event-certification.txt', cwd=checkout)
    git('-c', 'user.name=Alice', '-c', 'user.email=alice@example.invalid',
        'commit', '-m', 'test: certify automatic event response', cwd=checkout)
    sha = git('rev-parse', 'HEAD', cwd=checkout)
    started = time.time()
    git('push', '--quiet', 'origin', 'HEAD:refs/heads/' + a.branch, cwd=checkout)
    accepted = time.time()
    config = (checkout / '.git/config').read_text()
    assert capability['token'] not in config and 'extraheader' not in config.lower()
result = {'sha': sha, 'ref': 'refs/heads/' + a.branch,
    'push_started_unix': started, 'push_completed_unix': accepted,
    'push_seconds': accepted - started, 'manual_ingest': False}
deadline = time.monotonic() + 120
while time.monotonic() < deadline:
    events = [item for item in get(base + '/api/events')['items'] if item.get('payload', {}).get('after') == sha]
    runs = [item for item in get(repo_url + '/actions')['items'] if item['source_sha'] == sha]
    sse = [item for item in frames if sha in item['frame']]
    result.update(events=events, runs=runs, sse=sse, observed_unix=time.time())
    if events and runs and sse and runs[0]['state'].get('status') in ('success', 'failure', 'cancelled'):
        break
    time.sleep(1)
pathlib.Path(a.output).write_text(json.dumps(result, indent=2))
print(json.dumps({'sha': sha, 'events': len(result['events']), 'runs': len(result['runs']),
    'sse_frames': len(result['sse']), 'output': a.output}))
if not result['events'] or not result['runs'] or not result['sse']:
    raise SystemExit('Automatic event/Actions/SSE certification incomplete')
