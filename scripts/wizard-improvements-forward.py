#!/usr/bin/env python3
"""Forward only the isolated wizard trial PostgreSQL to loopback."""
from __future__ import annotations
import argparse
import asyncio
import json
import signal
import socket
import subprocess
import sys

PROJECT = "espacigo_wizard_20261106"
CONTAINER = PROJECT + "-db"
VOLUME = PROJECT + "-pgdata"
NETWORK = PROJECT + "-internal"
IMAGE = "postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677"
LOOPBACK = "127.0.0.1"
PORT = 15436


def inspect_target(docker="docker", runner=subprocess.run):
    args = [docker, "inspect", CONTAINER]
    result = runner(args, text=True, capture_output=True, check=False, timeout=15)
    if result.returncode:
        raise RuntimeError("isolated container inspection failed")
    try:
        item = json.loads(result.stdout)[0]
        labels = item["Config"]["Labels"] or {}
        mounts = item["Mounts"]
        networks = item["NetworkSettings"]["Networks"]
        net = networks[NETWORK]
        vols = [m for m in mounts if m.get("Type") == "volume" and m.get("Destination") == "/var/lib/postgresql"]
        if (item["Name"] != "/" + CONTAINER or labels.get("com.espacigo.trial") != PROJECT
                or item["Config"]["Image"] != IMAGE or not item["State"]["Running"]
                or len(vols) != 1 or vols[0].get("Name") != VOLUME
                or net.get("IPAddress", "") == ""):
            raise ValueError
        ip = net["IPAddress"]
        socket.inet_aton(ip)
        if ip.startswith("127."):
            raise ValueError
        # Confirm Docker's exact network and volume ownership labels.
        for kind, name in (("network", NETWORK), ("volume", VOLUME)):
            check = runner([docker, kind, "inspect", name], text=True, capture_output=True, check=False, timeout=15)
            if check.returncode:
                raise ValueError
            obj = json.loads(check.stdout)[0]
            if (obj.get("Name") != name or obj.get("Labels", {}).get("com.espacigo.trial") != PROJECT):
                raise ValueError
        return ip
    except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError, OSError):
        raise RuntimeError("isolated container identity, image, volume, network, or state did not validate") from None


async def relay(reader, writer):
    try:
        while True:
            data = await reader.read(65536)
            if not data:
                break
            writer.write(data)
            await writer.drain()
    except (ConnectionError, asyncio.CancelledError):
        pass
    finally:
        try:
            writer.close()
            await writer.wait_closed()
        except (ConnectionError, asyncio.CancelledError):
            pass


async def handle(client_reader, client_writer, target):
    upstream_writer = None
    try:
        upstream_reader, upstream_writer = await asyncio.open_connection(target, 5432, limit=65536)
        await asyncio.gather(relay(client_reader, upstream_writer), relay(upstream_reader, client_writer))
    except (OSError, asyncio.CancelledError):
        pass
    finally:
        for writer in (client_writer, upstream_writer):
            if writer is not None:
                writer.close()
        await asyncio.gather(*(w.wait_closed() for w in (client_writer, upstream_writer) if w is not None), return_exceptions=True)


async def serve(target):
    server = await asyncio.start_server(lambda r, w: handle(r, w, target), LOOPBACK, PORT, limit=65536)
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, stop.set)
    async with server:
        await stop.wait()
        server.close()
        await server.wait_closed()


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--check", action="store_true", help="validate target and test TCP reachability only")
    a = p.parse_args(argv)
    try:
        ip = inspect_target()
        with socket.create_connection((ip, 5432), timeout=3):
            pass
        if a.check:
            print(f"preflight ok: isolated PostgreSQL TCP reachable at {ip}:5432")
            return 0
        print(f"forwarding 127.0.0.1:{PORT} to isolated PostgreSQL {ip}:5432; Ctrl-C/SIGTERM stops", flush=True)
        asyncio.run(serve(ip))
        return 0
    except (RuntimeError, OSError) as exc:
        print(f"blocked: {exc if isinstance(exc, RuntimeError) else 'isolated PostgreSQL IP is not reachable from host'}", file=sys.stderr)
        return 2

if __name__ == "__main__":
    raise SystemExit(main())
