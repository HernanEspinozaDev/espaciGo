#!/usr/bin/env python3
"""Deterministically compare schema-only pg_dump files across F5 restore."""
import argparse
import hashlib
import re
import sys
from pathlib import Path

V82_SHA = "b53fe7f6f03af4d9081e4a378053ad0780d8b532962326488108201eb098ed96"
V83_SHA = "405c2f02f8a29e962421a44cbbd07e6e7fae4aa4696483e9d45ab209e82e9333"
FUNCTION = "public.document_verification_space_owner_changed()"
FUNC_RE = re.compile(r"(CREATE FUNCTION public\.document_verification_space_owner_changed\(\) RETURNS trigger.*?AS \$\$)(.*?)(\$\$;)", re.S | re.I)
GRANT = "GRANT UPDATE(id) ON TABLE public.document_verification_reviewer_grant TO espacigo_runtime;"


class VerificationError(ValueError):
    pass


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def migration_sources(directory: Path) -> tuple[bytes, bytes]:
    matches = {V82_SHA: [], V83_SHA: []}
    for path in directory.rglob("*"):
        if not path.is_file():
            continue
        try:
            data = path.read_bytes()
        except OSError:
            continue
        digest = sha(data)
        if digest in matches:
            matches[digest].append(data)
    for digest, found in matches.items():
        if len(found) != 1:
            raise VerificationError(f"expected exactly one migration source with SHA {digest}; found {len(found)}")
    return matches[V82_SHA][0], matches[V83_SHA][0]


def function_match(text: str):
    found = list(FUNC_RE.finditer(text))
    if len(found) != 1:
        raise VerificationError(f"expected exactly one {FUNCTION}; found {len(found)}")
    return found[0]


def source_body(source: bytes) -> str:
    text = source.decode("utf-8")
    # Migration source may use the canonical $function$ delimiter.
    pattern = re.compile(r"CREATE (?:OR REPLACE )?FUNCTION public\.document_verification_space_owner_changed\(\) RETURNS trigger.*?AS \$([A-Za-z_]*)\$(.*?)\$\1\$;", re.S | re.I)
    found = list(pattern.finditer(text))
    if len(found) != 1:
        raise VerificationError("pinned migration must contain exactly one target function")
    return found[0].group(2).strip()


def normalize(text: str) -> str:
    # Preserve text in dollar/single/double quoted literals and block comments.
    # Only discard complete pg_dump metadata lines outside these constructs.
    out = []
    quote = None
    block_depth = 0
    for line in text.splitlines():
        if quote is None and block_depth == 0:
            if line.startswith("--") or re.fullmatch(r"\\(?:un)?restrict\s+\S+", line) or not line.strip():
                continue
        out.append(line)
        i = 0
        while i < len(line):
            if block_depth:
                if line.startswith('/*', i):
                    block_depth += 1
                    i += 2
                elif line.startswith('*/', i):
                    block_depth -= 1
                    i += 2
                else:
                    i += 1
            elif quote:
                if line.startswith(quote, i):
                    if quote in ("'", '"') and line.startswith(quote * 2, i):
                        i += 2
                    else:
                        i += len(quote)
                        quote = None
                elif quote == "'" and line[i] == '\\':
                    i += 2
                else:
                    i += 1
            elif line.startswith('--', i):
                break
            elif line.startswith('/*', i):
                block_depth = 1
                i += 2
            elif line[i] in ("'", '"'):
                quote = line[i]
                i += 1
            elif line[i] == '$':
                match = re.match(r'\$(?:[A-Za-z_][A-Za-z_0-9]*)?\$', line[i:])
                if match:
                    quote = match.group()
                    i += len(quote)
                else:
                    i += 1
            else:
                i += 1
    if quote or block_depth:
        raise VerificationError('unterminated SQL quoting or comment')
    return "\n".join(out) + "\n"


def verify(before_path: Path, after_path: Path, migration_dir: Path) -> tuple[str, str]:
    before_src, after_src = migration_sources(migration_dir)
    before_sha, after_sha = sha(before_src), sha(after_src)
    before = before_path.read_text(encoding="utf-8")
    after = after_path.read_text(encoding="utf-8")
    bm, am = function_match(before), function_match(after)
    bbody, abody = bm.group(2).strip(), am.group(2).strip()
    if bbody != source_body(before_src) or abody != source_body(after_src):
        raise VerificationError("schema function body does not exactly match its pinned migration source")
    # Retain the complete function declaration/settings; allow only its source-
    # verified body to differ. Arbitrary adjacent statements are never allowed.
    before = before[:bm.start(2)] + '\nF5_PINNED_BODY\n' + before[bm.end(2):]
    after = after[:am.start(2)] + '\nF5_PINNED_BODY\n' + after[am.end(2):]
    if before.count(GRANT) != 0 or after.count(GRANT) != 1:
        raise VerificationError('expected exactly one additional V83 UPDATE(id) grant')
    lines = after.splitlines()
    if lines.count(GRANT) != 1:
        raise VerificationError('expected V83 grant as an exact statement line')
    lines.remove(GRANT)
    if normalize('\n'.join(lines)) != normalize(before):
        raise VerificationError('schema differs beyond the pinned body and exact V83 grant')
    return before_sha, after_sha


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before-schema", required=True, type=Path)
    parser.add_argument("--after-schema", required=True, type=Path)
    parser.add_argument("--migration-dir", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        old, new = verify(args.before_schema, args.after_schema, args.migration_dir)
    except (OSError, UnicodeError, VerificationError) as exc:
        print(f"verification failed: {exc}", file=sys.stderr)
        return 1
    print(f"verified schema equivalence: V82={old} V83={new}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
