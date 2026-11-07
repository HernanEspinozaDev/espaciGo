#!/usr/bin/env python3
"""Local/CI structural validation for the Cloud Build-only rollout."""
from __future__ import annotations

import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError as exc:  # pragma: no cover - runner dependency diagnostic
    raise SystemExit("PyYAML is required to validate cloudbuild.yaml") from exc


BUILTIN_SUBSTITUTIONS = {"BUILD_ID", "COMMIT_SHA", "PROJECT_ID"}


def quality_gate_errors(config: dict) -> list[str]:
    """No image, lock, IAM, migration or rollout step may bypass quality."""
    steps = config.get("steps", [])
    if not steps or steps[0].get("id") != "verify-source-quality":
        return ["the first Cloud Build step must verify quality for the exact source SHA"]
    gate = steps[0]
    body = "\n".join(gate.get("args", []))
    errors = []
    if not re.fullmatch(r"gcr.io/google.com/cloudsdktool/google-cloud-cli:stable@sha256:[a-f0-9]{64}", gate.get("name", "")):
        errors.append("quality must run in the pinned SDK image")
    if 'scripts/gcp/quality_gate.py --sha="$COMMIT_SHA" --scope="${_DEPLOY_SCOPE}" --wait-seconds=1200' not in body:
        errors.append("quality must require the parent SHA and exact deployment scope")
    for step in steps:
        if step.get("waitFor") is not None or step.get("allowFailure") or step.get("allowExitCodes"):
            errors.append("orchestration must preserve sequential fail-closed dependencies after quality")
    return errors


def unescaped_cloud_build_dollars(value: str, allowed: set[str]) -> list[str]:
    """Return shell expansions Cloud Build would parse before bash sees them."""
    found: list[str] = []
    index = 0
    while index < len(value):
        if value[index] != "$":
            index += 1
            continue
        if value.startswith("$$", index):
            index += 2
            continue
        if value.startswith("$(", index):
            found.append("$(...)")
            index += 2
            continue
        if value.startswith("${", index):
            end = value.find("}", index + 2)
            if end < 0:
                found.append("${... (unterminated)")
                index = len(value)
                continue
            name = value[index + 2 : end]
            if name not in allowed:
                found.append("${" + name + "}")
            index = end + 1
            continue
        if index + 1 < len(value) and (value[index + 1].isalpha() or value[index + 1] == "_"):
            end = index + 2
            while end < len(value) and (value[end].isalnum() or value[end] == "_"):
                end += 1
            name = value[index + 1 : end]
            if name not in allowed:
                found.append("$" + name)
            index = end
            continue
        if index + 1 < len(value) and (value[index + 1].isdigit() or value[index + 1] in "@*#?!-"):
            found.append("$" + value[index + 1])
            index += 2
            continue
        # A bare dollar not followed by shell syntax is usually a regex anchor.
        index += 1
    return found


def expand_cloud_build_substitutions(value: str, values: dict[str, str]) -> str:
    """Exercise Cloud Build's $$ escaping and custom/built-in substitution forms."""
    sentinel = "\u0000"
    protected = value.replace("$$", sentinel)
    pattern = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)")

    def replace(match: re.Match[str]) -> str:
        name = match.group(1) or match.group(2)
        if name not in values:
            raise ValueError(f"unknown Cloud Build substitution {name}")
        return values[name]

    return pattern.sub(replace, protected).replace(sentinel, "$")


def all_strings(value: object):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for nested in value.values():
            yield from all_strings(nested)
    elif isinstance(value, list):
        for nested in value:
            yield from all_strings(nested)


def main() -> int:
    config=yaml.safe_load(Path('cloudbuild.yaml').read_text())
    errors=quality_gate_errors(config)
    steps=config.get('steps',[])
    if [s.get('id') for s in steps]!=['verify-source-quality','release-main']:
        errors.append('only one main release executor may follow source quality')
    if config.get('substitutions')!={'_DEPLOY_SCOPE':'full'} or config.get('timeout')!='7200s':
        errors.append('main must use the complete classified release with bounded runtime')
    expected_sa='projects/espacigo-testprod/serviceAccounts/espacigo-cloud-build-deployer@espacigo-testprod.iam.gserviceaccount.com'
    if config.get('serviceAccount')!=expected_sa:errors.append('unexpected main identity')
    body='\n'.join(steps[-1].get('args',[])) if steps else ''
    if 'scripts/gcp/release_main.py --execute --sha="$COMMIT_SHA" --build-id="$BUILD_ID"' not in body:
        errors.append('main executor must bind the native source SHA/build ID')
    for step in steps:
        if step.get('name')!=steps[0].get('name'):errors.append('all orchestration steps must use the same pinned SDK')
    for filename in ['cloudbuild-images.yaml','cloudbuild-migrator-image.yaml']:
        child=yaml.safe_load(Path(filename).read_text())
        if child.get('options',{}).get('requestedVerifyOption')!='VERIFIED':errors.append('image provenance must be mandatory')
        if child.get('serviceAccount')!=expected_sa:errors.append('image builder identity differs')
    provision=yaml.safe_load(Path('cloudbuild-job-provision.yaml').read_text())
    if provision.get('serviceAccount')==expected_sa or 'espacigo-release-job-builder@' not in provision.get('serviceAccount',''):
        errors.append('Job configuration must use a separate identity')
    for filename in ['cloudbuild.yaml','cloudbuild-job-provision.yaml']:
        doc=yaml.safe_load(Path(filename).read_text())
        if doc.get('options',{}).get('substitutionOption')!='MUST_MATCH':errors.append('unknown substitutions must fail closed')
        allowed=set(doc.get('substitutions',{}))|BUILTIN_SUBSTITUTIONS
        if any(unescaped_cloud_build_dollars(s,allowed) for s in all_strings(doc)):errors.append('unescaped shell substitution')
    if errors:
        print('\n'.join('ERROR: '+e for e in errors),file=sys.stderr);return 1
    print('Cloud Build valid: one classified main executor, quality first, separate Job provisioner and mandatory image provenance.');return 0


if __name__ == '__main__':
    raise SystemExit(main())
