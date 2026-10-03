#!/usr/bin/env python3
"""Disposable loopback fixture for testing the real editor assets without Git writes."""
from http.server import ThreadingHTTPServer, SimpleHTTPRequestHandler
from pathlib import Path
import json
from urllib.parse import urlparse, parse_qs
ROOT = Path(__file__).resolve().parents[1] / 'public'
FILES = {'first.go':'package first\n', 'nested/second.go':'package second\n'}
class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs): super().__init__(*args, directory=str(ROOT), **kwargs)
    def respond(self, value, code=200):
        data=json.dumps(value).encode(); self.send_response(code); self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(data)
    def do_GET(self):
        url=urlparse(self.path)
        if url.path=='/api/auth/me': return self.respond({'authed':True,'user':'fixture'})
        if url.path=='/api/demo': return self.respond({'enabled':False,'guest':False})
        if url.path.endswith('/refs'): return self.respond({'refs':{'refs/heads/main':'a'*40}})
        if url.path.endswith('/tree'): return self.respond({'tree':[{'path':p} for p in FILES]})
        if url.path.endswith('/content'):
            data=FILES.get(parse_qs(url.query).get('path',[''])[0])
            self.send_response(200 if data is not None else 404);self.send_header('Content-Type','text/plain');self.end_headers();self.wfile.write((data or '').encode());return
        if url.path.startswith('/api/drafts/'):return self.respond({'error':'no_draft'},404)
        if url.path.startswith('/api/'):return self.respond({'items':[]})
        super().do_GET()
    def do_POST(self):
        if self.path=='/api/diff':return self.respond({'diff':'fixture diff'})
        return self.respond({'error':'fixture_is_read_only'},403)
    def log_message(self,*args): pass
if __name__=='__main__': ThreadingHTTPServer(('127.0.0.1',8127),Handler).serve_forever()
