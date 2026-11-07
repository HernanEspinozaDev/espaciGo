import copy
import datetime as dt
import unittest

import recovery_iam_policy as m

BID = "548b8eff-e489-4307-a77f-455b56aa4e57"
ROLE = "projects/espacigo-testprod/roles/espacigoRecoveryRead20261107a1b2c3d4"
NOW = dt.datetime(2026, 11, 7, 12, 0, tzinfo=dt.timezone.utc)
EXP = NOW + dt.timedelta(hours=1)
MEMBER = "serviceAccount:espacigo-release-job-builder@espacigo-testprod.iam.gserviceaccount.com"


class RecoveryIamPolicyTests(unittest.TestCase):
    def test_rejects_malformed_or_unexpanded_existing_binding(self):
        invalid = [None, {'role': 7, 'members': ['user:x']}, {'role': 'roles/viewer', 'members': 'user:x'},
                   {'role': 'roles/viewer_withcond_abc', 'members': ['user:x']},
                   {'role': 'roles/viewer', 'members': ['user:x'], 'condition': True},
                   {'role': 'roles/viewer', 'members': ['user:x'], 'condition': {'title': 'missing-expression'}}]
        for binding in invalid:
            policy = {'etag': 'e', 'version': 3, 'bindings': [binding]}
            with self.subTest(binding=binding), self.assertRaises(ValueError):
                m.grant_policy(policy, self.binding(), now=NOW)

    def binding(self, **kwargs):
        args = dict(parent_bid=BID, role_name=ROLE, expires_at=EXP, now=NOW)
        args.update(kwargs)
        return m.build_binding(**args)

    def test_binding_is_exact_named_conditional_allowlist(self):
        b = self.binding()
        self.assertEqual(b["role"], ROLE)
        self.assertEqual(b["members"], [MEMBER])
        c = b["condition"]
        self.assertEqual(c["title"], "recovery-readonly-" + BID)
        expr = c["expression"]
        self.assertIn("request.time < timestamp(", expr)
        self.assertIn("resource.name ==", expr)
        self.assertEqual(expr.count("resource.name =="), 6)
        self.assertNotIn("*", expr)
        self.assertIn('resource.name == "projects/_/buckets/', expr)
        self.assertNotIn('//storage.googleapis.com/', expr)
        self.assertNotIn('//run.googleapis.com/', expr)
        self.assertEqual(expr.count(' || '), 5)
        self.assertNotIn(' or ', expr)

    def test_policy_rejects_cel_injection_after_exact_selectors(self):
        tampered = self.binding()
        tampered['condition']['expression'] = tampered['condition']['expression'][:-1] + ' || true)'
        with self.assertRaisesRegex(ValueError, 'canonical resource allowlist'):
            m.grant_policy({'etag': 'e', 'version': 3, 'bindings': []}, tampered)

    def test_rejects_invalid_bid_role_and_expiry(self):
        for bid in ["x", BID.upper(), "548b8eff-e489-4307-a77f-455b56aa4e5"]:
            with self.subTest(bid=bid), self.assertRaises(ValueError):
                m.build_binding(bid, ROLE, EXP, now=NOW)
        for role in ["roles/viewer", "projects/other/roles/espacigoRecoveryRead20261107a1b2c3d4", "projects/espacigo-testprod/roles/espacigoRecoveryRead20261107ABCDEF12"]:
            with self.subTest(role=role), self.assertRaises(ValueError):
                m.build_binding(BID, role, EXP, now=NOW)
        for expiry in [NOW, NOW + dt.timedelta(hours=2, seconds=1), "2026-11-07T13:00:00"]:
            with self.subTest(expiry=expiry), self.assertRaises(ValueError):
                m.build_binding(BID, ROLE, expiry, now=NOW)

    def test_grant_preserves_unrelated_policy_and_rejects_conflicts(self):
        base = {"version": 1, "etag": "e1", "auditConfigs": [{"service": "x"}], "bindings": [{"role": "roles/viewer", "members": ["user:a"]}]}
        original = copy.deepcopy(base)
        updated = m.grant_policy(base, self.binding(), now=NOW)
        self.assertEqual(base, original)
        self.assertEqual(updated["version"], 3)
        self.assertEqual(updated["etag"], "e1")
        self.assertEqual(updated["auditConfigs"], base["auditConfigs"])
        for conflict in [
            {"version": 3, "etag": "e", "bindings": [{"role": ROLE, "members": ["user:other"]}]},
            {"version": 3, "etag": "e", "bindings": [self.binding()]},
        ]:
            with self.subTest(conflict=conflict), self.assertRaises(ValueError):
                m.grant_policy(conflict, self.binding(), now=NOW)

    def test_grant_rejects_bad_policy_or_binding(self):
        b = self.binding()
        for policy in [{"version": 1, "bindings": []}, {"etag": "e", "version": 2, "bindings": []}, {"etag": "e", "version": 3, "bindings": "bad"}]:
            with self.subTest(policy=policy), self.assertRaises(ValueError):
                m.grant_policy(policy, b)
        tampered = copy.deepcopy(b)
        tampered["members"] = ["user:attacker@example.com"]
        with self.assertRaises(ValueError):
            m.grant_policy({"etag": "e", "version": 3, "bindings": []}, tampered)

    def test_revoke_removes_only_exact_binding_and_preserves_concurrent_state(self):
        own = self.binding()
        other = {"role": "roles/viewer", "members": ["user:a"]}
        policy = {"etag": "new-etag", "version": 3, "bindings": [other, own], "auditConfigs": [{"service": "x"}], "futureField": True}
        out = m.revoke_policy(policy, own)
        self.assertEqual(out["etag"], "new-etag")
        self.assertEqual(out["bindings"], [other])
        self.assertEqual(out["auditConfigs"], policy["auditConfigs"])
        self.assertTrue(out["futureField"])
        for bindings in [[], [own, copy.deepcopy(own)]]:
            bad = dict(policy, bindings=bindings)
            with self.subTest(bindings=bindings), self.assertRaises(ValueError):
                m.revoke_policy(bad, own)

    def test_semantic_policy_normalizes_only_order_and_etag_version(self):
        a = {"etag": "a", "version": 1, "bindings": [{"role": "r", "members": ["user:b", "user:a"]}], "auditConfigs": [{"service": "x"}]}
        b = {"etag": "b", "version": 3, "bindings": [{"role": "r", "members": ["user:a", "user:b"]}], "auditConfigs": [{"service": "x"}]}
        self.assertEqual(m.semantic_policy(a), m.semantic_policy(b))
        b["auditConfigs"] = []
        self.assertNotEqual(m.semantic_policy(a), m.semantic_policy(b))


if __name__ == "__main__":
    unittest.main()
