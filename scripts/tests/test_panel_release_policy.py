import importlib.util
import json
import subprocess
import sys
import tempfile
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
        for target in ("0.8.1", "0.8.2"):
            content["version"] = target
            self.assertIs(content, POLICY.validate(content, target))

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
        patch = POLICY.load_policy("0.8.1", root=ROOT)
        self.assertEqual(patch["version"], "0.8.1")
        self.assertEqual(patch["minimumUpdaterVersion"], "0.7.3")
        bridge_path = ROOT / "deploy/updates/0.7.3.json"
        self.assertEqual(bridge_path, POLICY.policy_path("0.7.3", root=ROOT))
        POLICY.validate(json.loads(bridge_path.read_text(encoding="utf-8")), "0.7.3")
        content = {"schemaVersion": 1, "version": "0.7.3", "title": "Update", "summary": "Summary", "changes": ["Change"]}
        with self.assertRaisesRegex(ValueError, "exact target version"):
            POLICY.validate(content, "0.7.4")

    def test_unarchived_release_uses_current_policy(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.assertEqual(root / "deploy/panel-update.json", POLICY.policy_path("0.7.4", root=root))

    def test_future_patch_notes_materialize_the_unversioned_template(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "deploy").mkdir()
            (root / "deploy/panel-update.json").write_text((ROOT / "deploy/panel-update.json").read_text(encoding="utf-8"), encoding="utf-8")
            notes = root / "notes.md"
            notes.write_text("## v0.8.2\n\nFuture patch details.\n", encoding="utf-8")
            subprocess.run([sys.executable, str(ROOT / "scripts/panel_release_policy.py"), "notes", "--version", "0.8.2", "--output", str(notes)], cwd=root, check=True, capture_output=True, text=True, encoding="utf-8", timeout=10)
            generated = notes.read_text(encoding="utf-8")
            raw = generated.split("<!-- anxi-panel-update\n", 1)[1].split("\n-->", 1)[0]
            content = json.loads(raw)
            self.assertEqual(content["version"], "0.8.2")
            self.assertEqual(content["minimumUpdaterVersion"], "0.7.3")
            self.assertTrue(content["title"].startswith("v0.8.2"))
            self.assertNotIn("v0.8.1", generated)
            self.assertEqual(content, POLICY.load_policy("0.8.2", root=root))

    def test_explicit_wrong_version_never_becomes_a_template(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "deploy").mkdir()
            explicit = POLICY.load_policy("0.8.1", root=ROOT)
            path = root / "deploy/panel-update.json"
            path.write_text(json.dumps(explicit), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "exact target version"):
                POLICY.load_policy("0.8.2", root=root)
            with self.assertRaisesRegex(ValueError, "exact target version"):
                POLICY.load_policy("0.8.2", override=path, root=root)

    def test_maintenance_template_retains_required_content_guards(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "deploy").mkdir()
            content = json.loads((ROOT / "deploy/panel-update.json").read_text(encoding="utf-8"))
            content.pop("title")
            (root / "deploy/panel-update.json").write_text(json.dumps(content), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "title"):
                POLICY.load_policy("0.8.2", root=root)

    def test_previous_release_selector_uses_highest_stable_tag(self):
        for tags, expected in (("v0.7.3\nv0.8.0\nv0.8.1-rc.1\n", "v0.8.0"), ("v0.7.3\nv0.8.0\nv0.8.1\nv0.8.2-rc.1\n", "v0.8.1")):
            result = subprocess.run([sys.executable, str(ROOT / "scripts/panel_release_policy.py"), "newest"], input=tags, check=True, capture_output=True, text=True, encoding="utf-8", timeout=10)
            self.assertEqual(result.stdout.strip(), expected)


if __name__ == "__main__":
    unittest.main()
