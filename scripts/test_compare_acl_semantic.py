import copy
import importlib.util
import unittest
from pathlib import Path

MODULE_PATH = Path(__file__).with_name("compare_acl_semantic.py")
spec = importlib.util.spec_from_file_location("compare_acl_semantic", MODULE_PATH)
assert spec is not None and spec.loader is not None
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def snapshot():
    return {
        "database": "espacigo_f5_gate_20261101", "server_version_num": 180006,
        "acl_inventory": list(module.ACL_COLUMNS), "owner_inventory": list(module.OWNER_COLUMNS),
        "owners": [{"class": "pg_class", "identity": "table public.t", "owner": "postgres"}],
        "acls": [{"class": "pg_class", "identity": "table public.t", "column": "", "grantee": "PUBLIC", "grantor": "postgres", "privilege": "SELECT", "grantable": False, "source": "effective"}],
        "initial_privileges": [], "default_acls": [],
        "acl_states": [{"class":"pg_class","identity":"table public.t","column":"","source":"effective","state":"explicit"}],
        "default_acl_objects": [],
    }


class SemanticSnapshotTests(unittest.TestCase):
    def assert_drift(self, mutate):
        left, right = snapshot(), snapshot()
        mutate(right)
        with self.assertRaises(module.SnapshotError):
            module.compare_snapshots(left, right)

    def test_permuted_rows_compare_equal(self):
        left, right = snapshot(), snapshot()
        left["acls"].append({**left["acls"][0], "privilege": "INSERT"})
        right["acls"].append({**right["acls"][0], "privilege": "INSERT"})
        right["acls"].reverse()
        right["acl_inventory"].reverse()
        module.compare_snapshots(left, right)

    def test_detects_acl_grant_added_or_missing(self):
        self.assert_drift(lambda s: s["acls"].clear())
        self.assert_drift(lambda s: s["acls"].append({**s["acls"][0], "privilege": "UPDATE"}))

    def test_detects_column_grant_and_grant_option(self):
        self.assert_drift(lambda s: s["acls"][0].update(column="relname"))
        self.assert_drift(lambda s: s["acls"][0].update(grantable=True))

    def test_detects_owner_default_inventory_and_system_public_changes(self):
        self.assert_drift(lambda s: s["owners"][0].update(owner="other"))
        self.assert_drift(lambda s: s["default_acls"].append({"role":"postgres","schema":"public","object_type":"r","grantee":"PUBLIC","grantor":"postgres","privilege":"SELECT","grantable":False}))
        self.assert_drift(lambda s: s["acl_inventory"].pop())
        self.assert_drift(lambda s: s.update(database="wrong"))
        self.assert_drift(lambda s: s.update(server_version_num=180005))
        self.assert_drift(lambda s: s["acls"][0].update(identity="relation pg_catalog.pg_class"))

    def test_detects_oid_values_not_just_oid_named_keys(self):
        self.assert_drift(lambda s: s["owners"][0].update(identity="table public.t (oid 123)"))

    def test_empty_defaults_are_equal_and_order_independent(self):
        left, right = snapshot(), snapshot()
        left["default_acls"] = [{"role":"r","schema":"","object_type":"r","grantee":"PUBLIC","grantor":"r","privilege":"SELECT","grantable":False}]
        right["default_acls"] = copy.deepcopy(left["default_acls"])
        module.compare_snapshots(left, right)
        left["default_acls"].clear()
        with self.assertRaises(module.SnapshotError):
            module.compare_snapshots(left, right)

    def test_empty_acl_state_and_empty_default_objects_are_compared(self):
        self.assert_drift(lambda s: s["acl_states"][0].update(state="empty"))
        self.assert_drift(lambda s: s["default_acl_objects"].append({"role":"postgres","schema":"public","object_type":"r","state":"empty"}))

    def test_rejects_malformed_rows_inventories_and_oid_fields(self):
        self.assert_drift(lambda s: s["acls"][0].update(grantable="false"))
        self.assert_drift(lambda s: s["acls"][0].update(grantee=None))
        self.assert_drift(lambda s: s["default_acls"].append({"role":"r","schema":"","object_type":"x","grantee":"PUBLIC","grantor":"r","privilege":"SELECT","grantable":False}))
        self.assert_drift(lambda s: s["acl_inventory"].append(s["acl_inventory"][0]))
        self.assert_drift(lambda s: s["owner_inventory"].clear())
        self.assert_drift(lambda s: s["owners"][0].update(object_oid=123))


if __name__ == "__main__":
    unittest.main()
