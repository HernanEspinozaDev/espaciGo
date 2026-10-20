import hashlib
import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path

from verify_gcp07_attestations import AttestationError, verify_archive


def _digest(payload: bytes) -> str:
    return "sha256:" + hashlib.sha256(payload).hexdigest()


def _add_blob(archive: tarfile.TarFile, payload: bytes) -> str:
    digest = _digest(payload)
    info = tarfile.TarInfo(f"blobs/sha256/{digest.removeprefix('sha256:')}")
    info.size = len(payload)
    archive.addfile(info, io.BytesIO(payload))
    return digest


def _make_archive(
    path: Path,
    include_sbom: bool = True,
    include_provenance: bool = True,
    nested_index: bool = False,
) -> None:
    predicates = []
    if include_sbom:
        predicates.append("https://spdx.dev/Document")
    if include_provenance:
        predicates.append("https://slsa.dev/provenance/v0.2")

    with tarfile.open(path, mode="w") as archive:
        descriptors = []
        for predicate in predicates:
            statement = json.dumps({"_type": "https://in-toto.io/Statement/v0.1", "predicateType": predicate}).encode()
            layer_digest = _add_blob(archive, statement)
            manifest = json.dumps({"schemaVersion": 2, "layers": [{"digest": layer_digest}]}).encode()
            manifest_digest = _add_blob(archive, manifest)
            descriptors.append(
                {
                    "digest": manifest_digest,
                    "annotations": {"vnd.docker.reference.type": "attestation-manifest"},
                }
            )

        if nested_index:
            nested = json.dumps({"schemaVersion": 2, "manifests": descriptors}).encode()
            nested_digest = _add_blob(archive, nested)
            descriptors = [{"digest": nested_digest, "mediaType": "application/vnd.oci.image.index.v1+json"}]

        index = json.dumps({"schemaVersion": 2, "manifests": descriptors}).encode()
        info = tarfile.TarInfo("index.json")
        info.size = len(index)
        archive.addfile(info, io.BytesIO(index))


class VerifyAttestationsTest(unittest.TestCase):
    def test_accepts_spdx_and_slsa_attestations(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            image = Path(directory) / "image.oci.tar"
            _make_archive(image)
            predicates = verify_archive(image)
        self.assertIn("https://spdx.dev/Document", predicates)
        self.assertTrue(any(item.startswith("https://slsa.dev/provenance/") for item in predicates))

    def test_rejects_archive_without_provenance(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            image = Path(directory) / "image.oci.tar"
            _make_archive(image, include_provenance=False)
            with self.assertRaises(AttestationError):
                verify_archive(image)

    def test_accepts_attestations_in_nested_oci_index(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            image = Path(directory) / "image.oci.tar"
            _make_archive(image, nested_index=True)
            predicates = verify_archive(image)
        self.assertIn("https://spdx.dev/Document", predicates)
        self.assertIn("https://slsa.dev/provenance/v0.2", predicates)


if __name__ == "__main__":
    unittest.main()
