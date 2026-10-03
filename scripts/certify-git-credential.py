#!/usr/bin/env python3
"""Certify a read capability against a disposable repository without emitting secrets."""
import argparse, http.cookiejar, json, os, pathlib, subprocess, tempfile, urllib.request

p = argparse.ArgumentParser()
p.add_argument('--base', required=True)
p.add_argument('--cookies', required=True)
p.add_argument('--owner', required=True)
p.add_argument('--repo', required=True)
a = p.parse_args()
jar = http.cookiejar.MozillaCookieJar(a.cookies)
jar.load(ignore_discard=True, ignore_expires=True)
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
request = urllib.request.Request(a.base.rstrip('/')+'/api/repositories/'+a.owner+'/'+a.repo+'/git-credential', data=b'{"scope":"read","ttl_seconds":120}', headers={'Content-Type':'application/json'})
with opener.open(request, timeout=60) as response:
    assert response.headers.get('Cache-Control') == 'no-store'
    capability = json.load(response)
assert capability['permissions'] == ['read']
env = dict(os.environ, GIT_CONFIG_COUNT='1', GIT_CONFIG_KEY_0='http.extraHeader', GIT_CONFIG_VALUE_0='Authorization: Bearer '+capability['token'], GIT_TERMINAL_PROMPT='0', GIT_CONFIG_GLOBAL='/dev/null', GIT_CONFIG_NOSYSTEM='1')
def git(*args, cwd=None):
    result = subprocess.run(['git', *args], cwd=cwd, env=env, capture_output=True, timeout=90)
    if result.returncode:
        raise RuntimeError('Git capability check failed; subprocess output withheld')
    return result.stdout
with tempfile.TemporaryDirectory(prefix='switchyard-clone-capability-') as directory:
    checkout = pathlib.Path(directory)/'repo'
    git('clone', '--quiet', capability['remote'], str(checkout))
    git('fetch', '--quiet', 'origin', cwd=checkout)
    config = (checkout/'.git/config').read_text()
    assert capability['token'] not in config and 'extraheader' not in config.lower()
    sha = git('rev-parse', 'HEAD', cwd=checkout).decode().strip()
print(json.dumps({'credential_status':200,'permissions':capability['permissions'],'expires_at':capability['expires_at'],'clone':'PASS','fetch':'PASS','token_in_argv':False,'token_in_git_config':False,'head':sha}))
