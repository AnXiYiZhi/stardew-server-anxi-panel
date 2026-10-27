#!/usr/bin/env python3
"""Shared stable-release ordering and the upgrade policy embedded in releases."""
import argparse
import json
import re
import sys
from pathlib import Path

STABLE = re.compile(r"v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z")


def version(value):
    match = STABLE.fullmatch(value.strip())
    if not match:
        raise ValueError("expected an exact stable version")
    return tuple(map(int, match.groups()))


def validate(content, target):
    if content.get("schemaVersion") != 1:
        raise ValueError("unsupported update policy")
    if version(content.get("version", "")) != version(target):
        raise ValueError("update notes must describe the exact target version")
    minimum = version(content.get("minimumUpdaterVersion", "0.0.0"))
    if version(target) >= (0, 8, 0) and minimum < (0, 7, 3):
        raise ValueError("0.8.0 and later require the 0.7.3 updater")
    if not content.get("title") or not content.get("summary") or not content.get("changes"):
        raise ValueError("user-facing title, summary and changes are required")
    for notice in content.get("notices", []):
        if notice.get("level") not in ("info", "warning", "danger") or not notice.get("title") or not notice.get("message") or not isinstance(notice.get("requiresConfirmation"), bool):
            raise ValueError("invalid important notice")
    if "blocked" in content and not isinstance(content["blocked"], bool):
        raise ValueError("blocked must be boolean")
    return content


def policy_path(target, override=None, root=Path(".")):
    if override:
        return Path(override)
    exact = root / "deploy" / "updates" / (".".join(map(str, version(target))) + ".json")
    return exact if exact.is_file() else root / "deploy" / "panel-update.json"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("newest", "notes", "validate"))
    parser.add_argument("--version")
    parser.add_argument("--policy")
    parser.add_argument("--output")
    args = parser.parse_args()
    if args.command == "newest":
        values = [line.strip() for line in sys.stdin if STABLE.fullmatch(line.strip())]
        if not values:
            raise ValueError("no published stable release")
        print(max(values, key=version))
        return
    content = validate(json.loads(policy_path(args.version, args.policy).read_text(encoding="utf-8")), args.version)
    if args.command == "notes":
        path = Path(args.output)
        original = path.read_text(encoding="utf-8")
        if "<!-- anxi-panel-update" in original:
            raise ValueError("release already contains an upgrade policy")
        visible = content["title"] + "\n\n" + content["summary"] + "\n\n"
        visible += "\n".join("- " + change for change in content["changes"]) + "\n"
        for notice in content.get("notices", []):
            visible += "\n**" + notice["title"] + "**\n\n" + notice["message"] + "\n"
        metadata = "\n<!-- anxi-panel-update\n" + json.dumps(content, ensure_ascii=False, separators=(",", ":")) + "\n-->\n"
        path.write_text(visible + "\n" + original + metadata, encoding="utf-8", newline="\n")


if __name__ == "__main__":
    main()
