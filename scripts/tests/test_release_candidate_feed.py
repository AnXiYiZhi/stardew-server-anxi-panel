import json
import sys
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


if __name__ == "__main__":
    unittest.main()
