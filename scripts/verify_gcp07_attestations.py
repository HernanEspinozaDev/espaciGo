#!/usr/bin/env python3
"""Verify SPDX SBOM and SLSA provenance attestations in BuildKit OCI archives."""

from __future__ import annotations

import hashlib
import json
import re
import sys
import tarfile
from pathlib import Path
from typing import Any


class AttestationError(Exception):
    """Raised when an OCI archive lacks a valid expected attestation."""


def _read_member(archive: tarfile.TarFile, name: str) -> bytes:
    try:
        member = archive.getmember(name)
    except KeyError as exc:
        raise AttestationError("OCI archive is missing a referenced member") from exc
    if not member.isfile():
        raise AttestationError("OCI archive contains a non-file attestation member")
    stream = archive.extractfile(member)
    if stream is None:
        raise AttestationError("OCI archive member could not be read")
    return stream.read()


def _read_blob(archive: tarfile.TarFile, digest: str) -> bytes:
    if not isinstance(digest, str):
        raise AttestationError("OCI archive contains an unsupported digest")
    match = re.fullmatch(r"sha256:([0-9a-f]{64})", digest)
    if match is None:
        raise AttestationError("OCI archive contains an unsupported digest")
    payload = _read_member(archive, f"blobs/sha256/{match.group(1)}")
    if hashlib.sha256(payload).hexdigest() != match.group(1):
        raise AttestationError("OCI archive blob digest does not match")
    return payload


def _decode_json(payload: bytes) -> dict[str, Any]:
    try:
        value = json.loads(payload)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise AttestationError("OCI attestation is not valid JSON") from exc
    if not isinstance(value, dict):
        raise AttestationError("OCI attestation JSON has an unexpected structure")
    return value


def verify_archive(path: Path) -> set[str]:
    try:
        archive = tarfile.open(path, mode="r:*")
    except (OSError, tarfile.TarError) as exc:
        raise AttestationError("OCI archive could not be opened") from exc

    predicates: set[str] = set()
    with archive:
        index = _decode_json(_read_member(archive, "index.json"))
        descriptors = index.get("manifests")
        if not isinstance(descriptors, list):
            raise AttestationError("OCI index has no manifest list")

        pending = list(descriptors)
        visited: set[str] = set()
        image_digests: set[str] = set()
        attestation_manifests: list[tuple[dict[str, Any], dict[str, Any]]] = []
        while pending:
            descriptor = pending.pop()
            if not isinstance(descriptor, dict):
                continue
            digest = descriptor.get("digest", "")
            if not isinstance(digest, str):
                raise AttestationError("OCI archive contains an unsupported digest")
            if digest in visited:
                continue
            visited.add(digest)
            manifest = _decode_json(_read_blob(archive, digest))
            annotations = descriptor.get("annotations", {})
            is_attestation = (
                isinstance(annotations, dict)
                and annotations.get("vnd.docker.reference.type") == "attestation-manifest"
            ) or descriptor.get("artifactType") == "application/vnd.docker.attestation.manifest.v1+json"
            if not is_attestation:
                nested = manifest.get("manifests")
                if isinstance(nested, list):
                    pending.extend(nested)
                elif isinstance(manifest.get("layers"), list):
                    image_digests.add(digest)
                continue

            attestation_manifests.append((descriptor, manifest))

        for descriptor, manifest in attestation_manifests:
            annotations = descriptor.get("annotations", {})
            referenced_digest = (
                annotations.get("vnd.docker.reference.digest")
                if isinstance(annotations, dict)
                else None
            )
            if (
                not isinstance(referenced_digest, str)
                or not re.fullmatch(r"sha256:[0-9a-f]{64}", referenced_digest)
                or referenced_digest not in image_digests
            ):
                raise AttestationError("Attestation descriptor does not reference an image manifest")

            layers = manifest.get("layers")
            if not isinstance(layers, list):
                raise AttestationError("Attestation manifest has no layer list")
            for layer in layers:
                if not isinstance(layer, dict):
                    continue
                statement = _decode_json(_read_blob(archive, layer.get("digest", "")))
                predicate_type = statement.get("predicateType")
                subjects = statement.get("subject")
                if not isinstance(subjects, list):
                    raise AttestationError("Attestation statement has no subject")
                subject_matches_image = any(
                    isinstance(subject, dict)
                    and isinstance(subject.get("digest"), dict)
                    and subject["digest"].get("sha256") == referenced_digest.removeprefix("sha256:")
                    for subject in subjects
                )
                if not subject_matches_image:
                    raise AttestationError("Attestation subject does not match its referenced image")
                if isinstance(predicate_type, str):
                    predicates.add(predicate_type)

    if "https://spdx.dev/Document" not in predicates:
        raise AttestationError("OCI archive has no SPDX SBOM attestation")
    if not any(value.startswith("https://slsa.dev/provenance/") for value in predicates):
        raise AttestationError("OCI archive has no SLSA provenance attestation")
    return predicates


def main(argv: list[str]) -> int:
    if not argv:
        print("Usage: verify_gcp07_attestations.py IMAGE.oci.tar [IMAGE.oci.tar ...]", file=sys.stderr)
        return 2

    try:
        for value in argv:
            verify_archive(Path(value))
    except AttestationError as exc:
        print(f"GCP-07 attestation verification failed: {exc}", file=sys.stderr)
        return 1

    print(f"Verified SBOM and provenance attestations for {len(argv)} OCI image archive(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
