#!/usr/bin/env python3
"""Forward isolated Mailpit HTTP and SMTP to loopback only."""
from __future__ import annotations
import argparse
import asyncio
import json
import signal
import socket
import subprocess
import sys

PROJECT = "espacigo_wizard_20261106"
CONTAINER = PROJECT + "-mail"
NETWORK = PROJECT + "-internal"
IMAGE = "axllent/mailpit@sha256:b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48"
IMAGE_ID = "sha256:b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48"
LOOPBACK = "127.0.0.1"
FORWARDS = ((18087, 8025), (11036, 1025))
LIMIT = 65536


def inspect_target(docker="docker", runner=subprocess.run):
    def run(args):
        result = runner(args, text=True, capture_output=True, check=False, timeout=15)
        if result.returncode:
            raise ValueError
        return json.loads(result.stdout)[0]
    try:
        item = run([docker, "inspect", CONTAINER])
        net = item["NetworkSettings"]["Networks"][NETWORK]
        host = item["HostConfig"]
        # No capabilities, mounts or writable container filesystem; the only
        # permitted mount is an ephemeral tmpfs if the container declares one.
        if (item["Name"] != "/" + CONTAINER
                or item["Config"]["Labels"].get("com.espacigo.trial") != PROJECT
                or item["Config"]["Image"] != IMAGE or item["Image"] != IMAGE_ID
                or not item["State"]["Running"]
                or host.get("ReadonlyRootfs") is not True
                or host.get("CapAdd") not in (None, [])
                or "ALL" not in (host.get("CapDrop") or [])
                or any(m.get("Type") != "tmpfs" for m in item["Mounts"])
                or not net.get("IPAddress")):
            raise ValueError
        nw = run([docker, "network", "inspect", NETWORK])
        if (nw.get("Name") != NETWORK or nw.get("Internal") is not True
                or nw.get("Labels", {}).get("com.espacigo.trial") != PROJECT
                or CONTAINER not in {v.get("Name") for v in nw.get("Containers", {}).values()}):
            raise ValueError
        ip = net["IPAddress"]
        socket.inet_aton(ip)
        if ip.startswith("127."):
            raise ValueError
        return ip
    except (OSError, ValueError, KeyError, IndexError, TypeError, json.JSONDecodeError):
        raise RuntimeError("isolated Mailpit identity, image, restrictions, network, or state did not validate") from None


def preflight(ip):
    # Bind probes reject occupied ports; SO_REUSEADDR is deliberately absent.
    probes = []
    try:
        for port, _ in FORWARDS:
            s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            probes.append(s)
            s.bind((LOOPBACK, port))
        for _, target_port in FORWARDS:
            with socket.create_connection((ip, target_port), timeout=3):
                pass
    finally:
        for s in probes:
            s.close()


async def relay(reader, writer):
    try:
        while data := await reader.read(LIMIT):
            writer.write(data)
            await writer.drain()
    except (ConnectionError, asyncio.CancelledError):
        pass
    finally:
        writer.close()
        try:
            await writer.wait_closed()
        except (ConnectionError, asyncio.CancelledError):
            pass


async def handle(client_reader, client_writer, target, target_port):
    upstream_writer = None
    try:
        upstream_reader, upstream_writer = await asyncio.open_connection(target, target_port, limit=LIMIT)
        await asyncio.gather(relay(client_reader, upstream_writer), relay(upstream_reader, client_writer))
    except (OSError, asyncio.CancelledError):
        pass
    finally:
        writers = [w for w in (client_writer, upstream_writer) if w is not None]
        for writer in writers:
            writer.close()
        await asyncio.gather(*(w.wait_closed() for w in writers), return_exceptions=True)


async def serve(ip):
    servers = []
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    try:
        for local_port, target_port in FORWARDS:
            server = await asyncio.start_server(
                lambda r, w, p=target_port: handle(r, w, ip, p),
                LOOPBACK, local_port, limit=LIMIT)
            servers.append(server)
        for sig in (signal.SIGINT, signal.SIGTERM):
            loop.add_signal_handler(sig, stop.set)
        await stop.wait()
    finally:
        for server in servers:
            server.close()
        await asyncio.gather(*(s.wait_closed() for s in servers))


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--check", action="store_true", help="validate identity and TCP reachability only")
    args = p.parse_args(argv)
    try:
        ip = inspect_target()
        preflight(ip)
        if args.check:
            print(f"preflight ok: isolated Mailpit TCP reachable at {ip}:8025 and {ip}:1025")
            return 0
        print("loopback forwards: 127.0.0.1:18087 -> Mailpit:8025; 127.0.0.1:11036 -> Mailpit:1025", flush=True)
        asyncio.run(serve(ip))
        return 0
    except (RuntimeError, OSError) as exc:
        print(f"blocked: {exc if isinstance(exc, RuntimeError) else 'identity, loopback port, or target TCP preflight failed'}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
