#!/usr/bin/env python3
"""Resume a real multi-role demo on an explicitly disposable loopback repository.

Prerequisite: prepare-competition-demo.py manifest; main has api-version.json
{version:1,requires:1}, new-consumer.json {requires:1}, and one field_equals rule
from api-version.json:version to api-version.json:requires. Role providers and
Actions Worker must already be configured. No credentials are copied to evidence.
Mutations are never automatically retried after an ambiguous response.
"""
import argparse
import concurrent.futures
import http.cookiejar
import json
import os
import pathlib
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--manifest', required=True)
p.add_argument('--cookies', required=True)
p.add_argument('--output', required=True)
p.add_argument('--ci-command', default='node --test ci.test.mjs && git -C /tmp/ci-source rev-parse HEAD')
a = p.parse_args()
os.umask(0o077)
fixture = json.loads(pathlib.Path(a.manifest).read_text())
u = urllib.parse.urlsplit(fixture['base'])
if u.scheme != 'http' or u.hostname not in ('127.0.0.1', 'localhost') or u.username or u.password:
    p.error('only isolated loopback previews are permitted')
if not fixture['artifact_name'].startswith('codex-') or len(fixture['attempts']) != 2:
    p.error('two explicitly disposable Attempts are required')
root = '/api/repositories/' + '/'.join(urllib.parse.quote(v, safe='') for v in fixture['repository'].split('/'))
out = pathlib.Path(a.output)
state = json.loads(out.read_text()) if out.exists() else {'work_id': fixture['work_id'], 'steps': {}, 'timeline': []}
if state['work_id'] != fixture['work_id']:
    p.error('evidence belongs to another Work')
lock = threading.Lock()

def save():
    temporary = out.with_suffix(out.suffix + '.tmp')
    temporary.write_text(json.dumps(state, indent=2) + '\n')
    os.chmod(temporary, 0o600)
    temporary.replace(out)

def call(method, path, body=None):
    jar = http.cookiejar.MozillaCookieJar(a.cookies)
    jar.load(ignore_discard=True, ignore_expires=True)
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
    req = urllib.request.Request(fixture['base'].rstrip('/') + path, method=method,
        headers={'Content-Type': 'application/json'}, data=json.dumps(body).encode() if body is not None else None)
    started = time.monotonic()
    try:
        with client.open(req, timeout=170) as response:
            result = json.load(response)
            status = response.status
    except urllib.error.HTTPError as response:
        # Persist failure classification, never model text or credentials.
        result = json.load(response)
        raise RuntimeError(f'{method} {path}: HTTP {response.code} {result.get("error", "request_failed")}') from None
    return {'status': status, 'result': result, 'seconds': round(time.monotonic()-started, 3)}

def step(name, method, path, body=None):
    with lock:
        old = state['steps'].get(name)
        if old:
            if old['state'] != 'done':
                raise RuntimeError(f'{name} has ambiguous or failed state; inspect execution/refs before explicit recovery')
            return old['response']['result']
        state['steps'][name] = {'state': 'pending'}
        save()
    response = call(method, path, body)
    with lock:
        state['steps'][name] = {'state': 'done', 'response': response}
        state['timeline'].append({'step': name, 'at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()), 'seconds': response['seconds']})
        save()
    print(name + ': completed', flush=True)
    return response['result']

def wait(name, path, select, timeout=180):
    old = state['steps'].get(name)
    if old and old['state'] == 'done':
        return old['response']['result']
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        response = call('GET', path)
        selected = select(response['result'])
        if selected:
            state['steps'][name] = {'state': 'done', 'response': {'result': selected}}
            save()
            print(name + ': verified', flush=True)
            return selected
        time.sleep(3)
    raise RuntimeError(name + ': timed out; GET polling can safely resume')

def ci(attempt, sha, label):
    def select(data):
        runs = [r for r in data.get('items', []) if r.get('source_sha') == sha and r.get('ref') == 'refs/heads/'+attempt['branch'] and r.get('trigger') == 'push']
        for run in runs:
            status = run.get('state', {}).get('status')
            if status in ('failure', 'cancelled'):
                raise RuntimeError(label + ': native CI failed')
            if status == 'success':
                return run
    return wait(label, root+'/actions', select)

def clean_preview(attempt, label):
    result = step(label, 'POST', '/api/attempts/'+attempt['id']+'/preview', {})
    if result.get('conflict') or result.get('semantic_conflict'):
        raise RuntimeError(label + ': preview is not clean')
    return result

def review(attempt, label):
    result = step(label, 'POST', '/api/attempts/'+attempt['id']+'/review', {})
    if not result.get('execution') or any(f.get('severity') == 'error' for f in result.get('findings', [])):
        raise RuntimeError(label + ': reviewer reported an error')
    return result

def integrate(attempt, pr, label):
    queued = step(label+'-enqueue', 'POST', '/api/prs/'+pr['id']+'/enqueue', {})
    def select(data):
        for item in data.get('items', []):
            if item.get('id') != queued['id']:
                continue
            if item.get('status') == 'failed':
                raise RuntimeError(label + ': queue failed')
            if item.get('status') == 'done':
                return item
    return wait(label+'-published', '/api/queue', select)

try:
    settings = step('original-actions-settings', 'GET', root+'/settings/actions')
    source = 'export default '+json.dumps({'refs': ['refs/heads/main']+['refs/heads/'+x['branch'] for x in fixture['attempts']], 'jobs': [{'id':'verify','steps':[{'id':'unit','command':a.ci_command,'timeout_ms':60000}]}]})+';'
    step('approve-actions-before-push', 'PUT', root+'/settings/actions', {'source':source, 'expected_revision':settings['settings']['revision'], 'required_jobs':['verify']})
    first, second = fixture['attempts']
    def implement(attempt):
        if attempt['lane'] == 'a':
            body = {'file':'api-version.json','prompt':'Preserve this JSON object. Set both version and requires to number 2. Return only this complete valid JSON file.'}
        else:
            body = {'file':'switchyard.contract.json','prompt':'Preserve the existing rule. Append exactly one field_equals rule with a="api-version.json:version" and b="new-consumer.json:requires". Return only this complete valid JSON file.'}
        return step('implementation-'+attempt['lane'], 'POST', '/api/attempts/'+attempt['id']+'/run', body)
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        implementations = list(pool.map(implement, fixture['attempts']))
    prs = []
    for attempt, implementation in zip(fixture['attempts'], implementations):
        if not implementation.get('new_sha') or not implementation.get('execution'):
            raise RuntimeError('real implementation failed')
        ci(attempt, implementation['new_sha'], 'native-ci-'+attempt['lane'])
        clean_preview(attempt, 'initial-preview-'+attempt['lane'])
        review(attempt, 'initial-review-'+attempt['lane'])
        prs.append(step('pr-'+attempt['lane'], 'POST', '/api/attempts/'+attempt['id']+'/pr', {'title':'Semantic competition '+attempt['lane']}))
    integrate(first, prs[0], 'lane-a')
    conflict = step('semantic-conflict', 'POST', '/api/attempts/'+second['id']+'/preview', {})
    if conflict.get('conflict') or not conflict.get('semantic_conflict'):
        raise RuntimeError('expected a genuine textually clean semantic conflict')
    repair = step('real-resolver', 'POST', '/api/attempts/'+second['id']+'/resolve', {})
    if repair.get('status') != 'repair_proposed' or not repair.get('new_sha'):
        raise RuntimeError('real resolver did not propose a repair')
    clean_preview(second, 'repair-repreview')
    review(second, 'repair-rereview')
    ci(second, repair['new_sha'], 'repair-native-ci')
    integrate(second, prs[1], 'lane-b')
    step('final-refs', 'GET', root+'/refs')
    step('final-provenance', 'GET', '/api/work/'+fixture['work_id']+'/provenance')
    executions = call('GET', '/api/executions')['result']
    state['executions'] = [{k:v for k,v in ex.items() if k != 'output'} for ex in executions.get('items', []) if ex.get('attempt_id') in {x['id'] for x in fixture['attempts']}]
    state['result'] = 'PASS'
    save()
    print('PASS: real multi-role workflow, native exact-SHA CI, queue publication and provenance', flush=True)
except Exception as error:
    state['result'] = 'INCOMPLETE'
    state['error'] = str(error)
    save()
    raise SystemExit(str(error)) from None
