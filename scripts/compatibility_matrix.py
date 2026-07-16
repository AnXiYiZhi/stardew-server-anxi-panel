#!/usr/bin/env python3
"""Validate the exact component manifest embedded in a Panel release."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path
from urllib.parse import urlparse
from urllib.request import Request, urlopen

STATUSES = ("recommended", "withdrawn")
SHA256_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
HEX64_RE = re.compile(r"^[0-9a-f]{64}$")
REVISION_RE = re.compile(r"^[0-9a-f]{40}$")
VERSION_RE = re.compile(r"^[0-9]+(?:\.[0-9]+){2,3}$")


class MatrixError(ValueError):
    pass


def load(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as handle:
        value = json.load(handle)
    if not isinstance(value, dict):
        raise MatrixError(f"{path}: root must be an object")
    return value


def require(condition: bool, message: str) -> None:
    if not condition:
        raise MatrixError(message)


def validate_image_component(name: str, component: object, strict: bool) -> None:
    require(isinstance(component, dict), f"{name} must be an object")
    tag = component.get("tag")
    images = component.get("images")
    digests = component.get("digests")
    require(isinstance(tag, str) and tag and tag.lower() != "latest", f"{name}.tag must be exact")
    require(isinstance(images, list) and images and len(images) == len(set(images)), f"{name}.images must be non-empty and unique")
    require(isinstance(digests, dict), f"{name}.digests must be an object")
    for image in images:
        require(isinstance(image, str) and "@" not in image and image.endswith(":" + tag), f"{name} image tag mismatch: {image!r}")
        digest = digests.get(image)
        if strict:
            require(isinstance(digest, str) and SHA256_RE.fullmatch(digest) is not None, f"{name} image lacks reviewed digest: {image}")
        elif digest is not None:
            require(isinstance(digest, str) and SHA256_RE.fullmatch(digest) is not None, f"{name} digest is invalid: {image}")
    require(set(digests).issubset(set(images)), f"{name}.digests contains an image not listed in images")
    reviewed = {digests[image] for image in images if image in digests}
    require(len(reviewed) == 1, f"{name} image aliases must share one canonical digest")


def validate(matrix: dict) -> None:
    require(matrix.get("schemaVersion") == 1, "schemaVersion must equal 1")
    require(isinstance(matrix.get("stackVersion"), str) and matrix["stackVersion"], "stackVersion is required")
    require(matrix.get("channel") in ("stable", "preview"), "channel must be stable or preview")
    require(VERSION_RE.fullmatch(str(matrix.get("minimumPanelVersion", ""))) is not None, "minimumPanelVersion must be exact")
    require(matrix.get("runtimeUpdatePolicy") in {"recommended", "required"}, "runtimeUpdatePolicy must be recommended or required")
    status = matrix.get("status")
    require(status in STATUSES, "status is invalid")
    validate_image_component("server", matrix.get("server"), True)
    validate_image_component("steamAuth", matrix.get("steamAuth"), True)

    auth = matrix["steamAuth"]
    require(isinstance(auth.get("upstreamRef"), str) and auth["upstreamRef"].startswith("refs/tags/"), "steamAuth.upstreamRef must be an exact tag ref")
    require(REVISION_RE.fullmatch(str(auth.get("sourceRevision", ""))) is not None, "steamAuth.sourceRevision must be a full revision")

    for name, app_id in (("game", "413150"), ("sdk", "1007")):
        component = matrix.get(name)
        require(isinstance(component, dict), f"{name} must be an object")
        require(str(component.get("appId")) == app_id, f"{name}.appId must be {app_id}")
        require(str(component.get("buildId", "")).isdigit() and str(component["buildId"]) != "0", f"{name}.buildId must be exact")

    smapi = matrix.get("smapi")
    require(isinstance(smapi, dict) and VERSION_RE.fullmatch(str(smapi.get("version", ""))) is not None, "smapi.version must be exact")
    require(HEX64_RE.fullmatch(str(smapi.get("sha256", ""))) is not None, "smapi.sha256 is invalid")
    urls = smapi.get("urls")
    require(isinstance(urls, list) and urls, "smapi.urls is required")
    official_path = f"/Pathoschild/SMAPI/releases/download/{smapi['version']}/SMAPI-{smapi['version']}-installer.zip"
    for raw in urls:
        parsed = urlparse(raw)
        require(parsed.scheme == "https" and parsed.hostname == "github.com" and parsed.path == official_path, "SMAPI URL is not an exact official installer")

    control = matrix.get("controlMod")
    require(isinstance(control, dict) and VERSION_RE.fullmatch(str(control.get("version", ""))) is not None, "controlMod.version must be exact")
    require(isinstance(control.get("commandResultVersion"), int) and control["commandResultVersion"] > 0, "commandResultVersion must be positive")
    require(HEX64_RE.fullmatch(str(control.get("dllSha256", ""))) is not None, "controlMod.dllSha256 is invalid")
    require(isinstance(matrix.get("releaseNotes"), list) and matrix["releaseNotes"], "releaseNotes are required")

    withdrawal = matrix.get("withdrawal")
    if status == "withdrawn":
        require(isinstance(withdrawal, dict) and withdrawal.get("reason") and withdrawal.get("fallbackStackVersion"), "withdrawn requires reason and fallbackStackVersion")
    else:
        require(withdrawal is None, "withdrawal is only allowed for withdrawn")


def digest_from_imagetools(output: str) -> str:
    for line in output.splitlines():
        match = re.match(r"^Digest:\s+(sha256:[0-9a-f]{64})\s*$", line.strip())
        if match:
            return match.group(1)
    raise MatrixError("docker buildx imagetools output did not contain a manifest digest")


def required_remote_image(image: str) -> bool:
    repository = image.rsplit(":", 1)[0]
    first_segment = repository.split("/", 1)[0]
    canonical_docker_hub = "." not in first_segment and ":" not in first_segment and first_segment != "localhost"
    owned_mirror = repository.startswith("ghcr.io/anxiyizhi/") or repository.startswith(
        "crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/"
    )
    return canonical_docker_hub or owned_mirror


def verify_remote_artifacts(matrix: dict) -> None:
    validate(matrix)
    require(matrix["status"] == "recommended", "remote verification requires the embedded recommended manifest")
    for name in ("server", "steamAuth"):
        for image in matrix[name]["images"]:
            completed = subprocess.run(
                ["docker", "buildx", "imagetools", "inspect", image],
                check=False,
                capture_output=True,
                text=True,
                timeout=120,
            )
            if completed.returncode != 0:
                if required_remote_image(image):
                    raise MatrixError(f"cannot inspect required image {image}: {completed.stderr.strip()}")
                print(f"compatibility matrix warning: optional mirror unavailable: {image}", file=sys.stderr)
                continue
            actual = digest_from_imagetools(completed.stdout)
            require(actual == matrix[name]["digests"][image], f"tag/digest mismatch for {image}: expected {matrix[name]['digests'][image]}, got {actual}")

    smapi = matrix["smapi"]
    request = Request(smapi["urls"][0], headers={"User-Agent": "stardew-anxi-panel-release-gate/1"})
    digest = hashlib.sha256()
    total = 0
    with urlopen(request, timeout=120) as response:
        require(urlparse(response.geturl()).hostname in set(smapi["trustedHosts"]), "SMAPI redirect host is not trusted")
        while True:
            chunk = response.read(1024 * 1024)
            if not chunk:
                break
            total += len(chunk)
            require(total <= smapi["maxArchiveBytes"], "SMAPI download exceeds maxArchiveBytes")
            digest.update(chunk)
    require(total == smapi["archiveBytes"], f"SMAPI archive size mismatch: expected {smapi['archiveBytes']}, got {total}")
    require(digest.hexdigest() == smapi["sha256"], "SMAPI SHA256 mismatch")

    with tempfile.TemporaryDirectory(prefix="anxi-matrix-trace-") as directory:
        commands = (
            ["git", "init", "--quiet", directory],
            ["git", "-C", directory, "fetch", "--quiet", "--no-tags", "--depth=500", "https://github.com/AnXiYiZhi/junimo-server-steam-service-cn.git", f"+{matrix['steamAuth']['sourceRevision']}:refs/verify/auth-source"],
            ["git", "-C", directory, "fetch", "--quiet", "--no-tags", "--depth=1", "https://github.com/stardew-valley-dedicated-server/server.git", f"+{matrix['steamAuth']['upstreamRef']}:refs/verify/upstream"],
        )
        for command in commands:
            completed = subprocess.run(command, check=False, capture_output=True, text=True, timeout=180)
            require(completed.returncode == 0, f"auth upstream traceability fetch failed: {completed.stderr.strip()}")
        ancestry = subprocess.run(
            ["git", "-C", directory, "merge-base", "--is-ancestor", "refs/verify/upstream", "refs/verify/auth-source"],
            check=False,
            capture_output=True,
            text=True,
            timeout=30,
        )
        require(ancestry.returncode == 0, "steamAuth sourceRevision does not contain upstreamRef")


def panel_version_satisfies(current: str, minimum: str) -> bool:
    def parsed(value: str) -> tuple[int, ...] | None:
        value = value.removeprefix("v")
        if VERSION_RE.fullmatch(value) is None:
            return None
        parts = tuple(int(part) for part in value.split("."))
        return parts + (0,) * (4 - len(parts))
    left, right = parsed(current), parsed(minimum)
    return left is not None and right is not None and left >= right


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    validate_cmd = sub.add_parser("validate")
    validate_cmd.add_argument("paths", nargs="+", type=Path)
    remote_cmd = sub.add_parser("verify-remote-artifacts")
    remote_cmd.add_argument("path", type=Path)
    panel_cmd = sub.add_parser("check-panel-version")
    panel_cmd.add_argument("path", type=Path)
    panel_cmd.add_argument("--version", required=True)
    args = parser.parse_args()
    try:
        if args.command == "validate":
            for path in args.paths:
                validate(load(path))
        elif args.command == "verify-remote-artifacts":
            verify_remote_artifacts(load(args.path))
        else:
            matrix = load(args.path)
            validate(matrix)
            require(panel_version_satisfies(args.version, matrix["minimumPanelVersion"]), f"Panel {args.version} is below minimum {matrix['minimumPanelVersion']}")
    except (OSError, json.JSONDecodeError, MatrixError) as exc:
        print(f"compatibility matrix error: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
