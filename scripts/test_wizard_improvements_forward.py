from __future__ import annotations
import importlib.util
import json
import subprocess
import unittest
from pathlib import Path

PATH = Path(__file__).with_name("wizard-improvements-forward.py")
spec = importlib.util.spec_from_file_location("wizard_forward", PATH)
assert spec and spec.loader
forward = importlib.util.module_from_spec(spec)
spec.loader.exec_module(forward)


def fixture(network=None, label=None):
    return [{"Name": "/" + forward.CONTAINER,
             "Config": {"Labels": {"com.espacigo.trial": label or forward.PROJECT}, "Image": forward.IMAGE},
             "State": {"Running": True},
             "Mounts": [{"Type": "volume", "Name": forward.VOLUME, "Destination": "/var/lib/postgresql"}],
             "NetworkSettings": {"Networks": {network or forward.NETWORK: {"IPAddress": "172.26.0.2"}}}}]


class ForwardGuards(unittest.TestCase):
    def runner(self, container, argv):
        if argv[1] == "inspect" and argv[2] == forward.CONTAINER:
            return subprocess.CompletedProcess(argv, 0, json.dumps(container), "")
        if argv[1] in ("network", "volume") and argv[2] == "inspect":
            name = argv[3]
            data = [{"Name": name, "Labels": {"com.espacigo.trial": forward.PROJECT}}]
            return subprocess.CompletedProcess(argv, 0, json.dumps(data), "")
        raise AssertionError(f"unexpected Docker command kind: {argv[1]}")

    def test_exact_owned_trial_identity_is_accepted(self):
        self.assertEqual(forward.inspect_target(runner=lambda args, **kw: self.runner(fixture(), args)), "172.26.0.2")

    def test_wrong_label_is_rejected(self):
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            forward.inspect_target(runner=lambda args, **kw: self.runner(fixture(label="other"), args))

    def test_wrong_network_ip_target_is_rejected(self):
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            forward.inspect_target(runner=lambda args, **kw: self.runner(fixture(network="other-network"), args))

    def test_wrong_volume_label_is_rejected(self):
        def runner(args, **kw):
            if len(args) > 2 and args[1] == "volume":
                return subprocess.CompletedProcess(args, 0, json.dumps([{"Name": forward.VOLUME, "Labels": {}}]), "")
            return self.runner(fixture(), args)
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            forward.inspect_target(runner=runner)

if __name__ == "__main__":
    unittest.main()
