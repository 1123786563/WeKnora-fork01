"""THROWAWAY: serve the job-search UI prototype with one command."""

from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from functools import partial

ROOT = Path(__file__).resolve().parent
PORT = 4178

if __name__ == "__main__":
    handler = partial(SimpleHTTPRequestHandler, directory=str(ROOT))
    server = ThreadingHTTPServer(("127.0.0.1", PORT), handler)
    print(f"Prototype: http://127.0.0.1:{PORT}/?variant=C&platform=all", flush=True)
    server.serve_forever()
