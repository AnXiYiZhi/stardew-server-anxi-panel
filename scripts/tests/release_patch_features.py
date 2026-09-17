#!/usr/bin/env python3
"""Exercise patch contracts against a real, isolated candidate Panel."""
import argparse
import gzip
import http.cookiejar
import json
import os
from pathlib import Path
import re
import subprocess
import time
import urllib.error
import urllib.request


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True, timeout=60).strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--cookies", required=True)
    parser.add_argument("--data-dir", required=True)
    parser.add_argument("--error-image", required=True)
    parser.add_argument("--owner", required=True)
    args = parser.parse_args()
    require(os.environ.get("ANXI_RELEASE_CANDIDATE_ISOLATED_DOCKER") == "1",
            "patch fixture requires the candidate's isolated Docker daemon")
    jar = http.cookiejar.MozillaCookieJar(args.cookies)
    jar.load(ignore_discard=True, ignore_expires=True)
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

    def request(path, body=None, headers=None, method=None, anonymous=False):
        headers = dict(headers or {})
        if body is not None:
            body = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(args.url + path, body, headers, method=method)
        try:
            response = (urllib.request.urlopen if anonymous else client.open)(req, timeout=40)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            return response.status, response.headers, response.read()

    def api(path, body=None, expected=200):
        status, _, raw = request(path, body)
        require(status == expected, f"{path}: HTTP {status}, expected {expected}")
        return json.loads(raw)

    # Content negotiation, validation and byte ranges use the embedded build.
    status, _, html = request("/")
    require(status == 200, "candidate index missing")
    assets = re.findall(rb'/assets/index-[A-Za-z0-9_-]+\.js', html)
    require(len(set(assets)) == 1, "ambiguous frontend entry")
    asset = assets[0].decode()
    status, identity_headers, identity = request(asset)
    require(status == 200 and identity_headers.get("ETag"), "identity representation missing")
    status, compressed_headers, compressed = request(asset, headers={"Accept-Encoding": "gzip"})
    require(status == 200 and compressed_headers.get("Content-Encoding") == "gzip",
            "precompressed frontend representation missing")
    require(gzip.decompress(compressed) == identity, "gzip bytes differ from identity")
    require(compressed_headers["ETag"] != identity_headers["ETag"], "encoding ETags collide")
    for encoding, etag in (("identity", identity_headers["ETag"]), ("gzip", compressed_headers["ETag"])):
        status, _, raw = request(asset, headers={"Accept-Encoding": encoding, "If-None-Match": etag})
        require(status == 304 and not raw, "conditional request failed")
    status, _, raw = request(asset, method="HEAD")
    require(status == 200 and not raw, "HEAD returned a body")
    status, _, raw = request(asset, headers={"Range": "bytes=0-31"})
    require(status == 206 and raw == identity[:32], "range bytes changed")
    require("immutable" in identity_headers.get("Cache-Control", ""), "hashed asset not immutable")
    print("patch E2E: gzip, encoding ETags, 304, HEAD, Range and immutable assets passed", flush=True)

    require(request("/api/resources", anonymous=True)[0] == 401, "anonymous resources accepted")
    resource = api("/api/resources")
    require(resource["machine"]["scope"] == "machine", "machine scope missing")
    require(resource["machine"]["cpuCount"] > 0, "machine CPU denominator missing")
    game = next(item for item in resource["games"] if item["driverId"] == "stardew_junimo")
    require(game["sample"]["scope"] == "game" and game["worldCount"] >= 1, "game aggregation missing")
    metrics = api("/api/instances/stardew/metrics")
    require(metrics["sample"]["scope"] == "world" and metrics["machine"]["scope"] == "machine",
            "world metrics mixed with machine scope")
    deadline = time.monotonic() + 35
    while not game["sample"].get("storageTimestamp") and time.monotonic() < deadline:
        time.sleep(1)
        game = next(item for item in api("/api/resources")["games"] if item["driverId"] == "stardew_junimo")
    require(game["sample"].get("storageTimestamp"), "background storage sample did not complete")
    for _ in range(3):
        players = api("/api/instances/stardew/players")
        require(isinstance(players.get("players"), list), "player read contract changed")
    print("patch E2E: authenticated machine/game/world scopes, async storage and player reads passed", flush=True)

    # Run an actual Docker SteamCMD process with deterministic output. No Steam
    # credentials or network are used; install behavior is exercised through HTTP.
    instance = Path(args.data_dir) / "instances" / "stardew"
    env_path = instance / ".env"
    original = env_path.read_bytes()
    before_volumes = set(docker("volume", "ls", "--format", "{{.Name}}").splitlines())
    target_volume = args.owner + "-patch-install"
    require(target_volume not in before_volumes, "patch volume collision")
    docker("volume", "create", "--label", "com.anxi-panel.test-owner=" + args.owner, target_volume)
    updates = {
        "SERVER_IMAGE": "alpine:3.20", "SERVER_IMAGE_CANDIDATES": "alpine:3.20",
        "STEAMCMD_IMAGE": args.error_image, "STEAMCMD_IMAGE_CANDIDATES": args.error_image,
        "GAME_DATA_VOLUME": target_volume, "STEAM_INVITE_ENABLED": "false",
    }
    try:
        lines = [line for line in original.decode().splitlines() if line.split("=", 1)[0] not in updates]
        env_path.write_text("\n".join(lines + [key + "=" + value for key, value in updates.items()]) + "\n")
        jobs = []
        for _ in range(2):
            job = api("/api/instances/stardew/install", {
                "steamUsername": "patch-fixture-user", "steamPassword": "patch-fixture-secret",
                "vncPassword": "Patch7!", "imageTag": "3.20",
            }, expected=202)
            job_id = job["jobId"]
            require(job_id not in jobs, "retry reused terminal job")
            jobs.append(job_id)
            deadline = time.monotonic() + 90
            while time.monotonic() < deadline:
                result = api("/api/jobs/" + job_id)["job"]
                if result.get("status") in ("failed", "succeeded", "canceled"):
                    break
                time.sleep(0.5)
            require(result.get("status") == "failed", "controlled install did not fail terminally")
            require("账号或密码错误" in result.get("errorMessage", ""), "actionable install cause lost")
            require("patch-fixture-secret" not in json.dumps(result), "job exposed fixture credential")
            state = api("/api/instances/stardew")
            require(state.get("stateMessage") == result["errorMessage"], "instance lost durable failure cause")
        print("patch E2E: real Docker install failure, durable cause and retry passed", flush=True)
    finally:
        env_path.write_bytes(original)
        created = set(docker("volume", "ls", "--format", "{{.Name}}").splitlines()) - before_volumes
        for volume in sorted(created):
            require(volume == target_volume or "steamcmd" in volume, "unexpected fixture volume identity")
            require(not docker("ps", "-aq", "--filter", "volume=" + volume), "fixture volume still held")
            docker("volume", "rm", volume)


if __name__ == "__main__":
    main()
