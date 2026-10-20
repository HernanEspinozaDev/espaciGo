from __future__ import annotations

import unittest

from gcp03_origin_v64_transfer import GCSFiles, TransferStopped


class Blob:
    name = "g03o-1017/11111111-1111-4111-8111-111111111111.blob"
    generation = 7
    size = 12
    crc32c = "c2VudGlubA=="
    metadata = {"sha256": "a" * 64}

    def reload(self, *, if_generation_match: int) -> None:
        if if_generation_match != self.generation:
            raise AssertionError("destination generation was not pinned")


class Bucket:
    def __init__(self) -> None:
        self.lookups: list[str] = []
        self.blob = Blob()

    def get_blob(self, name: str) -> Blob | None:
        self.lookups.append(name)
        return self.blob if name == self.blob.name else None

    def list_blobs(self, **kwargs):
        raise AssertionError("destination inventory must not list the bucket")


class DestinationInventoryTests(unittest.TestCase):
    def test_reads_only_exact_allowlisted_destination_names(self) -> None:
        files = object.__new__(GCSFiles)
        files.target = Bucket()
        names = {
            "g03o-1017/11111111-1111-4111-8111-111111111111.blob",
            "g03o-1017/privacy-replay/privacy-replay-v1.json",
        }

        found = files.destination_inventory(names)

        self.assertEqual(files.target.lookups, sorted(names))
        self.assertEqual(list(found), ["g03o-1017/11111111-1111-4111-8111-111111111111.blob"])
        self.assertEqual(found[next(iter(found))]["generation"], 7)

        files.source = Bucket()
        with self.assertRaises(TransferStopped):
            files.inventory({"11111111-1111-4111-8111-111111111111"})
        self.assertEqual(files.source.lookups, [])


if __name__ == "__main__":
    unittest.main()
