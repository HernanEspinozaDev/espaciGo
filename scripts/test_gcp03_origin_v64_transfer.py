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


class ExistingDestinationBlob:
    def __init__(self) -> None:
        self.generation = 42
        self.size = 12
        self.crc32c = "c2VudGlubA=="
        self.persisted_metadata = {"sha256": "b" * 64, "transfer_id": "prior-invocation"}
        self.requested_metadata: dict[str, str] = {}
        self.delete_calls: list[int] = []

    @property
    def metadata(self) -> dict[str, str]:
        return self.persisted_metadata

    @metadata.setter
    def metadata(self, value: dict[str, str]) -> None:
        self.requested_metadata = value

    def rewrite(self, *args, **kwargs):
        raise RuntimeError("precondition failed: generation already exists")

    def delete(self, *, if_generation_match: int) -> None:
        self.delete_calls.append(if_generation_match)


class SourceBucket:
    def blob(self, name: str, *, generation: int):
        return object()


class ExistingDestinationBucket:
    def __init__(self) -> None:
        self.existing = ExistingDestinationBlob()

    def blob(self, name: str) -> ExistingDestinationBlob:
        return self.existing


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

    def test_create_only_conflict_never_deletes_preexisting_generation(self) -> None:
        files = object.__new__(GCSFiles)
        files.source = SourceBucket()
        files.target = ExistingDestinationBucket()
        source = {
            "source_name": "runs/g03o-1017/11111111-1111-4111-8111-111111111111.blob",
            "generation": 7,
            "size": 12,
            "sha256": "a" * 64,
            "crc32c": "c2VudGlubA==",
        }

        with self.assertRaises(TransferStopped) as error:
            files.copy_create_only(source, "g03o-1017/11111111-1111-4111-8111-111111111111.blob")

        self.assertEqual(error.exception.code, "PRIVATE_FILE_COPY_FAILED")
        self.assertEqual(files.target.existing.generation, 42)
        self.assertEqual(files.target.existing.persisted_metadata["transfer_id"], "prior-invocation")
        self.assertEqual(files.target.existing.delete_calls, [])


if __name__ == "__main__":
    unittest.main()
