from __future__ import annotations
import importlib.util
import json
import subprocess
from unittest.mock import MagicMock, patch
import unittest
from pathlib import Path

PATH = Path(__file__).with_name("wizard-improvements-mail-forward.py")
spec = importlib.util.spec_from_file_location("mail_forward", PATH)
assert spec and spec.loader
forward = importlib.util.module_from_spec(spec)
spec.loader.exec_module(forward)


def fixture(*, image=None, image_id=None, label=None, network=None, internal=True):
    return ([{"Name": "/" + forward.CONTAINER,
              "Config": {"Labels": {"com.espacigo.trial": label or forward.PROJECT},
                         "Image": image or forward.IMAGE},
              "Image": image_id or forward.IMAGE_ID,
              "State": {"Running": True},
              "HostConfig": {"ReadonlyRootfs": True, "CapAdd": [], "CapDrop": ["ALL"]},
              "Mounts": [],
              "NetworkSettings": {"Networks": {network or forward.NETWORK: {"IPAddress": "172.26.0.3"}}}}],
            [{"Name": network or forward.NETWORK, "Internal": internal,
              "Labels": {"com.espacigo.trial": forward.PROJECT},
              "Containers": {"id": {"Name": forward.CONTAINER}}}])


class MailForwardGuards(unittest.TestCase):
    def runner(self, objects, network, args):
        if args[1] == "inspect" and args[2] == forward.CONTAINER:
            data = objects
        elif args[1:3] == ["network", "inspect"]:
            data = network
        else:
            raise AssertionError(args)
        return subprocess.CompletedProcess(args, 0, json.dumps(data), "")

    def inspect(self, **kw):
        objects, network = fixture(**kw)
        return forward.inspect_target(runner=lambda args, **opts: self.runner(objects, network, args))

    def test_exact_target_identity_and_internal_network(self):
        self.assertEqual(self.inspect(), "172.26.0.3")

    def test_wrong_digest_rejected(self):
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            self.inspect(image="axllent/mailpit:latest")
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            self.inspect(image_id="sha256:" + "0" * 64)

    def test_wrong_label_rejected(self):
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            self.inspect(label="other")

    def test_wrong_or_noninternal_network_rejected(self):
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            self.inspect(network="other-network")
        with self.assertRaisesRegex(RuntimeError, "did not validate"):
            self.inspect(internal=False)

    def test_forward_mapping_is_exactly_two_loopback_targets(self):
        self.assertEqual(forward.LOOPBACK, "127.0.0.1")
        self.assertEqual(forward.FORWARDS, ((18087, 8025), (11036, 1025)))

    def test_preflight_rejects_occupied_host_port_without_adopting(self):
        fake_probe = MagicMock()
        fake_probe.bind.side_effect = OSError("occupied")
        create_connection = MagicMock()
        with patch.object(forward.socket, "socket", return_value=fake_probe), patch.object(
                forward.socket, "create_connection", create_connection):
            with self.assertRaisesRegex(OSError, "occupied"):
                forward.preflight("172.26.0.3")
        fake_probe.bind.assert_called_once_with(("127.0.0.1", 18087))
        fake_probe.close.assert_called_once()
        create_connection.assert_not_called()

if __name__ == "__main__":
    unittest.main()
