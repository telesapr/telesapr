#!/usr/bin/env python3
"""Serves the browser build for trying it out: scripts/wasm-serve.py [dir [port]].

Like python3 -m http.server, but it sends telesapr-web.wasm.br, when the
build wrote one and the browser accepts brotli, as the compressed
telesapr-web.wasm.
"""
import http.server
import os
import sys


class Handler(http.server.SimpleHTTPRequestHandler):
    def send_head(self):
        path = self.translate_path(self.path)
        if (path.endswith(".wasm") and "br" in self.headers.get("Accept-Encoding", "")
                and os.path.exists(path + ".br")):
            f = open(path + ".br", "rb")
            self.send_response(200)
            self.send_header("Content-Type", "application/wasm")
            self.send_header("Content-Encoding", "br")
            self.send_header("Content-Length", str(os.fstat(f.fileno()).st_size))
            self.end_headers()
            return f
        return super().send_head()


if __name__ == "__main__":
    root = sys.argv[1] if len(sys.argv) > 1 else "bin/web"
    port = int(sys.argv[2]) if len(sys.argv) > 2 else 8080
    handler = lambda *a, **k: Handler(*a, directory=root, **k)
    http.server.ThreadingHTTPServer(("127.0.0.1", port), handler).serve_forever()
