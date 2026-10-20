import copy
import unittest

from verify_gcp07_environment import is_ready
from verify_gcp07_plan import ALLOWED, IMAGE_PATH, PolicyError, validate


def environment():
    return {
        "protection_rules": [{"type": "required_reviewers", "prevent_self_review": True,
                              "reviewers": [{"type": "User", "reviewer": {"login": "HernanEspinozaDev", "id": 176093371}}]}],
        "deployment_branch_policy": {"protected_branches": False, "custom_branch_policies": True},
    }


def resource(address, image):
    base = {"name": "service", "template": [{"containers": [{"image": "old"}]}]}
    after = copy.deepcopy(base)
    after["template"][0]["containers"][0]["image"] = image
    return {"address": address, "change": {"actions": ["update"], "before": base, "after": after}}


class Gcp07dPolicyTests(unittest.TestCase):
    def test_environment_requires_reviewer_and_exact_main_policy(self):
        self.assertTrue(is_ready(environment(), {"branch_policies": [{"name": "main"}]}))
        self.assertFalse(is_ready(environment(), {"branch_policies": [{"name": "main"}, {"name": "release/*"}]}))
        wrong = environment()
        wrong["protection_rules"][0]["prevent_self_review"] = False
        self.assertFalse(is_ready(wrong, {"branch_policies": [{"name": "main"}]}))

    def test_accepts_single_allowlisted_digest_update(self):
        image = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        address = next(key for key, value in ALLOWED.items() if value[0] == "api")
        result = validate({"applyable": True, "resource_changes": [resource(address, image)]}, {"api": image}, "b" * 40, "123")
        self.assertEqual(len(result["fingerprint"]), 64)

    def test_rejects_create_wrong_image_and_non_image_change(self):
        image = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        address = next(key for key, value in ALLOWED.items() if value[0] == "api")
        bad = resource(address, image)
        bad["change"]["actions"] = ["delete", "create"]
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": [bad]}, {"api": image}, "b" * 40, "123")
        bad = resource(address, image)
        bad["change"]["after"]["name"] = "other"
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": [bad]}, {"api": image}, "b" * 40, "123")
        bad = resource(address, "example.invalid/image:latest")
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": [bad]}, {"api": bad["change"]["after"]["template"][0]["containers"][0]["image"]}, "b" * 40, "123")


if __name__ == "__main__":
    unittest.main()
