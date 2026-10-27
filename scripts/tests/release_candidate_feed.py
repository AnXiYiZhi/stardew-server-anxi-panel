#!/usr/bin/env python3
"""Serve the candidate with the same release policy used by publication."""
import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from panel_release_policy import policy_path, validate, version  # noqa: E402


def feeds(root, target):
    content = validate(json.loads(policy_path(target, root=root).read_text(encoding="utf-8")), target)
    body = content["title"] + "\n\n" + content["summary"]
    body += "\n<!-- anxi-panel-update\n" + json.dumps(content, ensure_ascii=False, separators=(",", ":")) + "\n-->\n"
    release = {"tag_name": "v" + target, "name": content["title"], "body": body,
               "html_url": "https://example.invalid/releases/v" + target, "draft": False,
               "prerelease": False, "published_at": "2026-01-01T00:00:00Z"}
    if version(target) < (0, 8, 0):
        return release, [release]
    bridge = {"tag_name": "v0.7.3", "name": "v0.7.3", "body": "",
              "html_url": "https://example.invalid/releases/v0.7.3", "draft": False,
              "prerelease": False, "published_at": "2025-12-31T00:00:00Z"}
    return bridge, [release, bridge]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    latest, releases = feeds(args.root, args.version)
    args.output.mkdir(parents=True, exist_ok=True)
    for name, content in (("latest.json", latest), ("releases.json", releases)):
        (args.output / name).write_text(json.dumps(content, ensure_ascii=False), encoding="utf-8", newline="\n")


if __name__ == "__main__":
    main()
