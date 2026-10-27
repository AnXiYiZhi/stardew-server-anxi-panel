import importlib.util
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("panel_release_policy", ROOT / "scripts" / "panel_release_policy.py")
POLICY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(POLICY)


class ReleasePolicyTests(unittest.TestCase):
    def test_version_order_ignores_latest_pointer(self):
        self.assertEqual("v0.10.0", max(["v0.7.3", "v0.9.0", "v0.10.0"], key=POLICY.version))

    def test_major_release_requires_bridge(self):
        content = {"schemaVersion": 1, "version": "0.8.0", "title": "Update", "summary": "Summary", "changes": ["Change"]}
        with self.assertRaisesRegex(ValueError, "0.7.3"):
            POLICY.validate(content, "0.8.0")
        content["minimumUpdaterVersion"] = "0.7.3"
        self.assertIs(content, POLICY.validate(content, "0.8.0"))

    def test_invalid_confirmation_and_block_are_rejected(self):
        content = {"schemaVersion": 1, "version": "0.7.3", "title": "Update", "summary": "Summary", "changes": ["Change"], "blocked": "false"}
        with self.assertRaises(ValueError):
            POLICY.validate(content, "0.7.3")
        content["blocked"] = False
        content["notices"] = [{"level": "warning", "title": "Maintenance", "message": "Restart", "requiresConfirmation": "true"}]
        with self.assertRaises(ValueError):
            POLICY.validate(content, "0.7.3")

    def test_publication_pins_legacy_entry(self):
        release = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        self.assertIn("make_latest: ${{ steps.identity.outputs.version == '0.7.3' && 'true' || 'false' }}", release)
        for name in ["release.yml", "release-candidate.yml", "release-after-candidate.yml"]:
            workflow = (ROOT / ".github/workflows" / name).read_text(encoding="utf-8")
            self.assertNotIn("releases/latest", workflow)
            self.assertIn("panel_release_policy.py newest", workflow)

    def test_notes_follow_exact_release_version(self):
        self.assertEqual(ROOT / "deploy/updates/0.8.0.json", POLICY.policy_path("v0.8.0", root=ROOT))
        self.assertEqual(ROOT / "deploy/panel-update.json", POLICY.policy_path("0.7.3", root=ROOT))
        content = {"schemaVersion": 1, "version": "0.7.3", "title": "Update", "summary": "Summary", "changes": ["Change"]}
        with self.assertRaisesRegex(ValueError, "exact target version"):
            POLICY.validate(content, "0.7.4")


if __name__ == "__main__":
    unittest.main()
