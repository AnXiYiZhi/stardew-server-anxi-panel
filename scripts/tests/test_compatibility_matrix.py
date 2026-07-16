import copy
import importlib.util
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("compatibility_matrix", ROOT / "scripts" / "compatibility_matrix.py")
MATRIX = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MATRIX)


class CompatibilityMatrixTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.base = MATRIX.load(ROOT / "backend" / "internal" / "games" / "stardew_junimo" / "config" / "runtime_stack_manifest.json")

    def test_embedded_recommended_is_valid(self):
        MATRIX.validate(self.base)
        self.assertEqual("recommended", self.base["status"])
        self.assertEqual("required", self.base["runtimeUpdatePolicy"])

    def test_update_policy_is_explicit(self):
        for policy in (None, "silent", ""):
            value = copy.deepcopy(self.base)
            value["runtimeUpdatePolicy"] = policy
            with self.assertRaisesRegex(MATRIX.MatrixError, "runtimeUpdatePolicy"):
                MATRIX.validate(value)

    def test_exact_component_digests_are_required(self):
        value = copy.deepcopy(self.base)
        value["server"]["digests"] = {}
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)

    def test_image_aliases_must_share_one_digest(self):
        value = copy.deepcopy(self.base)
        value["server"]["digests"][value["server"]["images"][1]] = "sha256:" + "f" * 64
        with self.assertRaisesRegex(MATRIX.MatrixError, "share one canonical digest"):
            MATRIX.validate(value)

    def test_required_remote_image_policy(self):
        self.assertTrue(MATRIX.required_remote_image("sdvd/server:1.2.3"))
        self.assertTrue(MATRIX.required_remote_image("ghcr.io/anxiyizhi/example:1.2.3"))
        self.assertTrue(MATRIX.required_remote_image("crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/example:1.2.3"))
        self.assertFalse(MATRIX.required_remote_image("docker.1ms.run/sdvd/server:1.2.3"))

    def test_latest_and_incomplete_auth_source_are_rejected(self):
        value = copy.deepcopy(self.base)
        value["steamAuth"]["tag"] = "latest"
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)
        value = copy.deepcopy(self.base)
        value["steamAuth"]["sourceRevision"] = "short"
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)

    def test_candidate_and_tested_statuses_are_not_part_of_release_flow(self):
        for status in ("candidate", "tested", "discovered"):
            value = copy.deepcopy(self.base)
            value["status"] = status
            with self.assertRaisesRegex(MATRIX.MatrixError, "status is invalid"):
                MATRIX.validate(value)

    def test_withdrawn_requires_fallback(self):
        value = copy.deepcopy(self.base)
        value["status"] = "withdrawn"
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)
        value["withdrawal"] = {"reason": "critical regression", "fallbackStackVersion": "previous-exact-stack"}
        MATRIX.validate(value)

    def test_release_digest_parser_and_panel_minimum(self):
        digest = "sha256:" + "c" * 64
        self.assertEqual(digest, MATRIX.digest_from_imagetools(f"Name: example/image:1\nDigest: {digest}\n"))
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.digest_from_imagetools("Digest: sha256:short")
        self.assertTrue(MATRIX.panel_version_satisfies("v0.1.14", "0.1.14"))
        self.assertFalse(MATRIX.panel_version_satisfies("0.1.13", "0.1.14"))


if __name__ == "__main__":
    unittest.main()
