#!/usr/bin/env python3
"""Serves the browser build for trying it out: scripts/wasm-serve.py [dir [port]].

Like python3 -m http.server, but on 127.0.0.1 only, and it names .wasm
application/wasm on every Python, which WebAssembly.instantiateStreaming needs.
"""
import http.server
import sys


class Handler(http.server.SimpleHTTPRequestHandler):
    extensions_map = {**http.server.SimpleHTTPRequestHandler.extensions_map,
                      ".wasm": "application/wasm"}


if __name__ == "__main__":
    root = sys.argv[1] if len(sys.argv) > 1 else "bin/web"
    port = int(sys.argv[2]) if len(sys.argv) > 2 else 8080
    handler = lambda *a, **k: Handler(*a, directory=root, **k)
    http.server.ThreadingHTTPServer(("127.0.0.1", port), handler).serve_forever()
