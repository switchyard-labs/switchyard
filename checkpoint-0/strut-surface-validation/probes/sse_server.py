#!/usr/bin/env python3
"""Tiny SSE server used by probe_sse.p."""
import http.server, socketserver, time, sys

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        for i in range(1, 6):
            try:
                self.wfile.write(f"event: event-{['one','two','three','four','five'][i-1]}\n".encode())
                self.wfile.write(f"data: payload-{i}\n\n".encode())
                self.wfile.flush()
                time.sleep(0.1)
            except BrokenPipeError:
                return
    def log_message(self, *a): pass

socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(("127.0.0.1", 8090), H) as s:
    s.serve_forever()