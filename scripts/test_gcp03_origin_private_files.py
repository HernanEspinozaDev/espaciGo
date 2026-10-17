import hashlib
import io
import json
import tarfile
import unittest

from gcp03_origin_private_files import PrivateFilesError, copy_private_files, validate_private_files_tar


def archive(*, filename="private-evidence/sample.bin", payload=b"synthetic private bytes", recorded_hash=None):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w") as tf:
        root = tarfile.TarInfo("private-evidence")
        root.type, root.mode = tarfile.DIRTYPE, 0o700
        tf.addfile(root)
        other_root = tarfile.TarInfo("privacy-replay")
        other_root.type, other_root.mode = tarfile.DIRTYPE, 0o700
        tf.addfile(other_root)
        parent_name = filename.rsplit("/", 1)[0] if "/" in filename else ""
        if parent_name and parent_name != "private-evidence":
            parent = tarfile.TarInfo(parent_name)
            parent.type, parent.mode = tarfile.DIRTYPE, 0o700
            tf.addfile(parent)
        item = tarfile.TarInfo(filename)
        item.size, item.mode = len(payload), 0o600
        tf.addfile(item, io.BytesIO(payload))
        relative = filename.removeprefix("private-evidence/")
        manifest = {"format": "espacigo.gcp03.local-origin-capture.v1.files", "files": [{
            "root": "private-evidence", "relative_path": relative, "bytes": len(payload),
            "sha256": recorded_hash or hashlib.sha256(payload).hexdigest(), "mode_octal": "0600"}]}
        encoded = json.dumps(manifest, sort_keys=True, separators=(",", ":")).encode()
        info = tarfile.TarInfo("capture-file-manifest.json")
        info.size, info.mode = len(encoded), 0o600
        tf.addfile(info, io.BytesIO(encoded))
    return output.getvalue()


class Store:
    def __init__(self):
        self.objects = {}

    def create(self, name, data, sha256):
        if name in self.objects:
            raise RuntimeError("create-only collision")
        if hashlib.sha256(data).hexdigest() != sha256:
            raise RuntimeError("hash mismatch")
        self.objects[name] = data


class PrivateFilesTests(unittest.TestCase):
    def test_valid_archive_is_copied_after_full_validation(self):
        payload = b"synthetic private bytes"
        archive_bytes = archive(payload=payload)
        files = validate_private_files_tar(archive_bytes, run_id="g03o-1017")
        self.assertEqual(len(files), 1)
        store = Store()
        report = copy_private_files(archive_bytes, run_id="g03o-1017", store=store)
        self.assertEqual(store.objects, {"runs/g03o-1017/private-evidence/sample.bin": payload})
        self.assertEqual(report[0]["bytes"], len(payload))

    def test_bad_inventory_creates_no_objects(self):
        store = Store()
        with self.assertRaises(PrivateFilesError):
            copy_private_files(archive(recorded_hash="0" * 64), run_id="g03o-1017", store=store)
        self.assertEqual(store.objects, {})

    def test_traversal_member_rejected(self):
        with self.assertRaises(PrivateFilesError):
            validate_private_files_tar(archive(filename="private-evidence/../escape"), run_id="g03o-1017")


if __name__ == "__main__":
    unittest.main()
