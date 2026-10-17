import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from gcp03_origin_cloud_verify import VerifyError, verify_upload


class Blob:
    def __init__(self, name, generation, data):
        self.name, self.generation, self.data, self.size = name, generation, data, len(data)

    def reload(self, if_generation_match=None):
        if if_generation_match != self.generation:
            raise AssertionError("generation condition omitted")

    def download_as_bytes(self, if_generation_match=None):
        if if_generation_match != self.generation:
            raise AssertionError("generation condition omitted")
        return self.data


class Bucket:
    def __init__(self, objects):
        self.objects = objects

    def blob(self, name, generation=None):
        blob = self.objects[name]
        if generation is not None and generation != blob.generation:
            raise AssertionError("wrong generation selected")
        return blob


class Client:
    def __init__(self, objects):
        self._bucket = Bucket(objects)

    def bucket(self, name):
        if name != "espacigo-gcp03-origin-g03o-1017":
            raise AssertionError("unexpected bucket")
        return self._bucket


class CloudVerifyTests(unittest.TestCase):
    def test_verifies_all_ciphertext_and_support_objects_by_generation(self):
        names = ["database.dump.gcm", "private-files.tar.gcm", "contract-encryption-key.gcm",
                 "origin-metadata.json.gcm", "transport-manifest.json", "toc.list", "normalization.sql"]
        objects, refs = {}, {}
        for i, name in enumerate(names, 1):
            path = f"runs/g03o-1017/{name}"
            data = f"ciphertext-{name}".encode()
            objects[path] = Blob(path, i, data)
            ref = {"object_name": path, "generation": str(i), "sha256": hashlib.sha256(data).hexdigest(), "bytes": len(data)}
            if name.endswith(".gcm"):
                refs[name.removesuffix(".gcm")] = ref
            elif name == "transport-manifest.json":
                refs["transport_manifest"] = ref
            elif name == "toc.list":
                refs["toc_allowlist"] = ref
            else:
                refs["normalization_sql"] = ref
        receipt = {"format": "espacigo.gcp03.origin-upload-receipt.v1",
            "source_capture_run_id": "gcp03-merge-review-20261017", "target_run_id": "g03o-1017",
            "artifacts": {key: refs[key] for key in ("database.dump", "private-files.tar", "contract-encryption-key", "origin-metadata.json")},
            "transport_manifest": refs["transport_manifest"], "toc_allowlist": refs["toc_allowlist"],
            "normalization_sql": refs["normalization_sql"]}
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "receipt.json"
            raw = json.dumps(receipt, sort_keys=True).encode()
            path.write_bytes(raw)
            path.chmod(0o600)
            report = verify_upload(path, hashlib.sha256(raw).hexdigest(), Client(objects))
        self.assertEqual(report["verified_object_count"], 7)
        self.assertFalse(report["plaintext_written"])

    def test_rejects_invalid_receipt_before_cloud_access(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "receipt.json"
            path.write_text("{}")
            path.chmod(0o600)
            raw = path.read_bytes()
            with self.assertRaises(VerifyError):
                verify_upload(path, hashlib.sha256(raw).hexdigest(), Client({}))


if __name__ == "__main__":
    unittest.main()
