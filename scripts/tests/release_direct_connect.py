#!/usr/bin/env python3
"""Validate driver-owned connection settings on an isolated candidate world."""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.error
import urllib.request
import uuid


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--cookies", required=True)
    parser.add_argument("--data-dir", required=True)
    parser.add_argument("--world-id", required=True)
    args = parser.parse_args()
    require(os.environ.get("ANXI_RELEASE_CANDIDATE_ISOLATED_DOCKER") == "1",
            "direct-connect fixture requires the candidate's isolated Docker daemon")
    require(re.fullmatch(r"stardew-[0-9]+", args.world_id), "expected a disposable world")
    jar = http.cookiejar.MozillaCookieJar(args.cookies)
    jar.load(ignore_discard=True, ignore_expires=True)
    admin = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
    user = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    anonymous = urllib.request.build_opener()

    def api(path, expected=200, body=None, method=None, client=admin):
        data = None if body is None else json.dumps(body).encode("utf-8")
        req = urllib.request.Request(args.url + path, data,
                                     {"Content-Type": "application/json"}, method=method)
        try:
            response = client.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            require(response.status == expected,
                    f"{path}: HTTP {response.status}, expected {expected}")
            raw = response.read()
            return json.loads(raw) if raw else None

    endpoint = "/api/instances/" + args.world_id + "/direct-connect"
    env_path = Path(args.data_dir) / "instances" / args.world_id / ".env"
    original = env_path.read_bytes()
    baseline = api(endpoint)
    default = api("/api/instances/stardew/direct-connect")
    holders = subprocess.check_output([
        "docker", "ps", "-q", "--filter", "label=com.docker.compose.project=" + args.world_id,
    ], text=True, timeout=30).strip()
    require(not holders, "connection fixture world must be stopped")
    api(endpoint, expected=401, client=anonymous)
    api(endpoint, expected=405, method="POST")
    api("/api/instances/missing-direct-fixture/direct-connect", expected=404)
    username = "direct" + uuid.uuid4().hex[:12]
    password = uuid.uuid4().hex
    user_id = None
    try:
        created = api("/api/users", expected=201,
                      body={"username": username, "password": password, "role": "user"})
        user_id = created["user"]["id"]
        api("/api/auth/login", body={"username": username, "password": password}, client=user)
        require(api(endpoint, client=user) == baseline, "ordinary user cannot read connection")
        api("/api/users", expected=403, client=user)
        lines = [line for line in original.decode("utf-8").splitlines()
                 if not line.startswith("GAME_PORT=")]
        for value, expected in ((None, 24642), ("24643", 24643), ("24644", 24644),
                                ("65535", 65535), ("invalid", None), ("65536", None)):
            contents = lines + ([] if value is None else ["GAME_PORT=" + value])
            env_path.write_text("\n".join(contents) + "\n", encoding="utf-8")
            if expected is None:
                result = api(endpoint, expected=500)
                require(result["error"]["code"] == "direct_connect_config_failed",
                        "invalid port did not report a controlled config error")
            else:
                expected_result = {"gamePort": expected, "protocol": "udp"}
                for _ in range(2):
                    require(api(endpoint, client=user) == expected_result,
                            "stopped-world port read or refresh changed")
            require(api("/api/instances/stardew/direct-connect") == default,
                    "changing a second world affected the default world")
    finally:
        env_path.write_bytes(original)
        if user_id is not None:
            api("/api/users/" + str(user_id) + "?hard=true", method="DELETE")
    require(env_path.read_bytes() == original and api(endpoint) == baseline,
            "connection fixture did not restore original configuration")
    require(api("/api/instances/stardew/direct-connect") == default,
            "default world config changed after fixture cleanup")
    print("direct-connect E2E: stopped world, default/custom/invalid ports, refresh, "
          "world isolation, anonymous/ordinary-user permissions and restoration passed", flush=True)


if __name__ == "__main__":
    main()
