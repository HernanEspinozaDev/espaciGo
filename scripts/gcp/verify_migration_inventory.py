#!/usr/bin/env python3
"""Verify an offline dbmigrate inventory and compare source/image bytes."""
import argparse
import hashlib
import json
from pathlib import Path
import re

FORMAT = 'espacigo.migrations.inventory.v1'


def validate(report):
    if not isinstance(report, dict) or set(report) != {'format', 'migrations', 'inventory_sha256'} or report['format'] != FORMAT:
        raise ValueError('unsupported migration inventory contract')
    entries = report['migrations']
    if not isinstance(entries, list) or not entries:
        raise ValueError('empty migration inventory')
    digest = hashlib.sha256()
    for version, entry in enumerate(entries, 1):
        if not isinstance(entry, dict) or set(entry) != {'version', 'name', 'checksum'}:
            raise ValueError('invalid migration entry')
        if type(entry['version']) is not int or entry['version'] != version:
            raise ValueError('migration inventory must be contiguous from V1')
        if not isinstance(entry['name'], str) or not re.fullmatch(rf'V{version:06d}__[a-z0-9]+(?:_[a-z0-9]+)*\.sql', entry['name']):
            raise ValueError('migration name does not match its version')
        if not isinstance(entry['checksum'], str) or not re.fullmatch('[a-f0-9]{64}', entry['checksum']):
            raise ValueError('invalid migration checksum')
        digest.update(f"{entry['checksum']}  {entry['name']}\n".encode())
    if report['inventory_sha256'] != digest.hexdigest():
        raise ValueError('migration inventory fingerprint mismatch')
    return entries


def compare(expected, actual):
    validate(expected)
    validate(actual)
    if expected != actual:
        raise ValueError('source and runtime image migration inventories differ')
    return len(expected['migrations'])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('expected', type=Path)
    parser.add_argument('actual', type=Path)
    args = parser.parse_args()
    try:
        count = compare(json.loads(args.expected.read_text()), json.loads(args.actual.read_text()))
    except (OSError, ValueError, TypeError) as error:
        parser.exit(1, f'migration inventory rejected: {error}\n')
    print(f'PASS: source and runtime image contain the same {count} migrations and exact checksums')


if __name__ == '__main__':
    main()
