#!/usr/bin/env python3
"""Verify the shipped Caddy policy against known-length and chunked uploads."""

import http.client
import http.server
import json
import pathlib
import re
import subprocess
import tempfile
import threading
import time


LIMIT = 33_554_432
ROOT = pathlib.Path(__file__).resolve().parents[2]


class Upstream(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Length", "2")
        self.end_headers()
        self.wfile.write(b"ok")

    def do_POST(self):
        try:
            if self.headers.get("Transfer-Encoding", "").lower() == "chunked":
                while True:
                    line = self.rfile.readline()
                    if not line:
                        return
                    size = int(line.split(b";", 1)[0], 16)
                    if size == 0:
                        self.rfile.readline()
                        break
                    if len(self.rfile.read(size)) != size:
                        return
                    self.rfile.read(2)
            else:
                remaining = int(self.headers.get("Content-Length", "0"))
                while remaining:
                    chunk = self.rfile.read(min(remaining, 65536))
                    if not chunk:
                        return
                    remaining -= len(chunk)
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", "2")
            self.end_headers()
            self.wfile.write(b"{}")
        except (BrokenPipeError, ConnectionResetError):
            pass

    def log_message(self, *_args):
        pass


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True).strip()


def check_upload(port, size, chunked, expected):
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=30)
    try:
        headers = {"Content-Type": "application/json"}
        if chunked:
            def chunks():
                remaining = size
                while remaining:
                    part = min(65536, remaining)
                    yield b" " * part
                    remaining -= part
            body = chunks()
        else:
            body = b" " * size
        try:
            connection.request("POST", "/api/desktop/v1/conversation-records", body, headers, encode_chunked=chunked)
        except BrokenPipeError:
            pass
        response = connection.getresponse()
        raw = response.read()
        assert response.status == expected, (size, chunked, response.status, raw[:1000])
        assert response.getheader("Cache-Control") == "no-store"
        request_id = response.getheader("X-Request-ID")
        assert request_id
        if expected == 413:
            error = json.loads(raw)["error"]
            assert error["code"] == "PAYLOAD_TOO_LARGE"
            assert error["request_id"] == request_id
            assert error["retryable"] is False and error["details"] == {}
        print(f"PASS size={size} chunked={chunked} status={expected}")
    finally:
        connection.close()


def main():
    upstream = http.server.ThreadingHTTPServer(("0.0.0.0", 0), Upstream)
    upstream.daemon_threads = True
    threading.Thread(target=upstream.serve_forever, daemon=True).start()
    container = None
    try:
        with tempfile.TemporaryDirectory(prefix="desktop-conversation-proxy-") as directory:
            source = (ROOT / "deploy/Caddyfile").read_text()
            source = source.replace("api.sub2api.com {", "http://:8080 {")
            source = re.sub(r"\n\ttls \{.*?\n\t\}", "", source, count=1, flags=re.S)
            source = source.replace("localhost:8080", f"host.docker.internal:{upstream.server_port}")
            config = pathlib.Path(directory) / "Caddyfile"
            config.write_text(source)
            container = docker("run", "--rm", "-d", "--add-host=host.docker.internal:host-gateway", "-p", "127.0.0.1::8080", "-v", f"{config}:/etc/caddy/Caddyfile:ro", "caddy:2.11-alpine")
            port = int(docker("port", container, "8080/tcp").rsplit(":", 1)[1])
            for _ in range(50):
                probe = http.client.HTTPConnection("127.0.0.1", port, timeout=1)
                try:
                    probe.request("GET", "/health")
                    probe.getresponse().read()
                    break
                except OSError:
                    time.sleep(0.1)
                finally:
                    probe.close()
            for chunked in (False, True):
                check_upload(port, LIMIT, chunked, 200)
                check_upload(port, LIMIT + 1, chunked, 413)
    finally:
        if container:
            docker("stop", "--time", "1", container)
        upstream.shutdown()
        upstream.server_close()


if __name__ == "__main__":
    main()
