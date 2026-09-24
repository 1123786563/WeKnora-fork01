#!/usr/bin/env python3
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
import os

log = os.environ.get('LOG', '/tmp/mock-log.jsonl')

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass
    def do_GET(self):
        if self.path == '/redirect':
            self.send_response(307)
            self.send_header('Location', 'http://direct-listener:8080/v1/chat/completions')
            self.end_headers()
            return
        self._write(200, {'data': [{'id': 'mock-model', 'object': 'model'}]})
    def do_POST(self):
        n = int(self.headers.get('content-length', '0'))
        body = self.rfile.read(n)
        with open(log, 'a') as f:
            f.write(json.dumps({'method': 'POST', 'path': self.path,
                                'headers': dict(self.headers),
                                'body': body.decode('utf-8', 'replace')}) + '\n')
        self._write(200, {'id': 'mock-response', 'object': 'chat.completion',
                          'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': 'probe-ok'}, 'finish_reason': 'stop'}],
                          'usage': {'prompt_tokens': 1, 'completion_tokens': 1, 'total_tokens': 2}})
    def _write(self, status, value):
        data = json.dumps(value).encode()
        self.send_response(status)
        self.send_header('content-type', 'application/json')
        self.send_header('content-length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)

HTTPServer(('0.0.0.0', int(os.environ.get('PORT', '8080'))), Handler).serve_forever()
