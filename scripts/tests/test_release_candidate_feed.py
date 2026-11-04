import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from release_candidate_feed import feeds


class CandidateFeedTests(unittest.TestCase):
    root = Path(__file__).resolve().parents[2]

    def test_bridge_feed_contains_the_publication_policy(self):
        latest, releases = feeds(self.root, "0.7.3")
        self.assertEqual(latest["tag_name"], "v0.7.3")
        self.assertEqual(releases, [latest])
        raw = latest["body"].split("<!-- anxi-panel-update\n", 1)[1].split("\n-->", 1)[0]
        policy = json.loads(raw)
        self.assertEqual(policy["version"], "0.7.3")
        self.assertTrue(policy["changes"])

    def test_major_feed_keeps_legacy_latest_on_the_bridge(self):
        latest, releases = feeds(self.root, "0.8.0")
        self.assertEqual(latest["tag_name"], "v0.7.3")
        self.assertEqual([release["tag_name"] for release in releases], ["v0.8.0", "v0.7.3"])
        raw = releases[0]["body"].split("<!-- anxi-panel-update\n", 1)[1].split("\n-->", 1)[0]
        policy = json.loads(raw)
        self.assertEqual(policy["minimumUpdaterVersion"], "0.7.3")
        self.assertTrue(any(notice["requiresConfirmation"] for notice in policy["notices"]))

    def test_current_patch_is_discoverable_by_the_bridge_list(self):
        latest, releases = feeds(self.root, "0.8.1")
        self.assertEqual(latest["tag_name"], "v0.7.3")
        self.assertEqual([release["tag_name"] for release in releases], ["v0.8.1", "v0.7.3"])
        raw = releases[0]["body"].split("<!-- anxi-panel-update\n", 1)[1].split("\n-->", 1)[0]
        policy = json.loads(raw)
        self.assertEqual(policy["version"], "0.8.1")
        self.assertEqual(policy["minimumUpdaterVersion"], "0.7.3")
        self.assertTrue(any("v0.7.3" in notice["title"] for notice in policy["notices"]))

    def test_future_patch_feed_uses_the_publication_template(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "deploy").mkdir()
            (root / "deploy/panel-update.json").write_text((self.root / "deploy/panel-update.json").read_text(encoding="utf-8"), encoding="utf-8")
            latest, releases = feeds(root, "0.8.2")
            self.assertEqual((latest, releases), feeds(root, "v0.8.2"))
            self.assertEqual(latest["tag_name"], "v0.7.3")
            self.assertEqual([release["tag_name"] for release in releases], ["v0.8.2", "v0.7.3"])
            raw = releases[0]["body"].split("<!-- anxi-panel-update\n", 1)[1].split("\n-->", 1)[0]
            policy = json.loads(raw)
            self.assertEqual(policy["version"], "0.8.2")
            self.assertEqual(policy["minimumUpdaterVersion"], "0.7.3")
            self.assertTrue(policy["title"].startswith("v0.8.2"))
            self.assertNotIn("v0.8.1", releases[0]["body"])


if __name__ == "__main__":
    unittest.main()
