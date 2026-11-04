#!/usr/bin/env python3
"""Exercise patch contracts against a real, isolated candidate Panel."""
import argparse
import gzip
import hashlib
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


def fixture_server_contract(manifest, image_tag):
    server = manifest["server"]
    source = "sdvd/server:" + image_tag
    require(server["tag"] == image_tag and source in server["images"],
            "recommended fixture server is absent from the runtime manifest")
    return source, server["digests"][source]


def verify_fixture_server(records, expected_digest):
    require(isinstance(records, list) and len(records) == 1, "ambiguous fixture server image")
    image = records[0]
    digests = {entry.rsplit("@", 1)[-1] for entry in image.get("RepoDigests", [])}
    require(image.get("Id") == expected_digest or expected_digest in digests,
            "fixture server does not match the fixed runtime digest")
    return image["Id"]


def collect_job_logs(api, job_id):
    logs = []
    after = 0
    for _ in range(10):
        page = api("/api/jobs/" + job_id + "/logs?after=" + str(after) + "&limit=1000")["logs"]
        require(isinstance(page, list), "job logs response changed")
        if not page:
            return logs
        require(all(isinstance(line.get("id"), int) and line["id"] > after for line in page),
                "job log cursor did not advance")
        logs.extend(page)
        after = max(line["id"] for line in page)
    raise RuntimeError("controlled install job log budget exceeded")


def install_plan_names(instance):
    return {path.name for path in (instance / ".local-container" / "install-game-volumes").glob("*.json")}


def managed_install_destinations(instance, source_volume, server_image, image_id, digest, previous_plans, admitted_images=None):
    admitted_images = {server_image} if admitted_images is None else set(admitted_images)
    require(server_image in admitted_images, "selected fixture image is outside the admitted fixed references")
    environment = dict(line.split("=", 1) for line in (instance / ".env").read_text(encoding="utf-8").splitlines()
                       if "=" in line and not line.lstrip().startswith("#"))
    volume = environment.get("GAME_DATA_VOLUME", "")
    plans = {}
    for name in install_plan_names(instance) - previous_plans:
        target = Path(name).stem
        require(re.fullmatch(re.escape(instance.name) + r"_anxi-smapi-update-[0-9a-f]{24}", target),
                "unexpected managed installation destination")
        path = instance / ".local-container" / "install-game-volumes" / name
        require(not path.is_symlink(), "managed installation intent is a symlink")
        plan = json.loads(path.read_text(encoding="utf-8"))
        require(plan.get("schema") == 1 and plan.get("project") == instance.name and
                plan.get("targetVolume") == target and plan.get("image") in admitted_images and
                plan.get("imageId") == image_id and plan.get("digest") == digest and
                isinstance(plan.get("created"), bool),
                "managed installation intent differs from this controlled API request")
        plans[target] = plan
    lineage = {source_volume}
    pending = set(plans)
    while pending:
        admitted = {target for target in pending if plans[target].get("sourceVolume") in lineage}
        require(admitted, "managed installation source is outside this controlled API request")
        lineage.update(admitted)
        pending.difference_update(admitted)
    require(volume in lineage, "unexpected managed installation destination")
    return {target for target, plan in plans.items() if plan["created"]}


def record_install_evidence(directory, name, value):
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    payload = json.dumps(value, ensure_ascii=False, indent=2).encode("utf-8") + b"\n"
    path = directory / name
    with path.open("xb") as handle:
        os.chmod(path, 0o600)
        handle.write(payload)
        handle.flush()
        os.fsync(handle.fileno())
    return {"file": name, "sha256": hashlib.sha256(payload).hexdigest(), "bytes": len(payload)}


def install_cause_tokens(logs):
    messages = "\n".join(str(line.get("message", "")) for line in logs).lower()
    known = ("Junimo .140 installation image does not match its fixed digest", "ERROR (Invalid Password)",
             "prepare image-managed installation volume", "SteamCMD install failed")
    return [token for token in known if token.lower() in messages]


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
        detail = raw.decode(errors="replace").replace("patch-fixture-secret", "[redacted]")
        require(status == expected, f"{path}: HTTP {status}, expected {expected}: {detail[:1000]}")
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
    options = api("/api/instances/stardew/install-options")["imageTagOptions"]
    recommended = [option["tag"] for option in options if option.get("recommended")]
    require(len(recommended) == 1, "ambiguous recommended install version")
    image_tag = recommended[0]
    server_image = args.owner + "/patch-server:" + image_tag
    manifest_path = Path(__file__).resolve().parents[2] / "backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    _, server_digest = fixture_server_contract(manifest, image_tag)
    admitted_images = {server_image} | {image for image in manifest["server"]["images"]
                                       if manifest["server"]["digests"].get(image) == server_digest}
    server_image_id = verify_fixture_server(json.loads(docker("image", "inspect", server_image)), server_digest)
    previous_plans = install_plan_names(instance)
    namespace = args.owner + "-patch-fresh"
    # A fresh .140 source must not exist: an existing source is cloned and fully
    # verified before SteamCMD. The production installer creates its owned target.
    planned_volumes = {namespace + "_steamcmd-login", namespace + "_steamcmd-home",
                       instance.name + "_steamcmd-login", instance.name + "_steamcmd-root-local",
                       instance.name + "_steamcmd-user-local"}
    created_approved = set()
    evidence_dir = Path(args.data_dir) / "patch-install-evidence"
    fixture_env_written = False
    updates = {
        "SERVER_IMAGE": server_image, "SERVER_IMAGE_CANDIDATES": server_image,
        "STEAMCMD_IMAGE": args.error_image, "STEAMCMD_IMAGE_CANDIDATES": args.error_image,
        "GAME_DATA_VOLUME": target_volume, "STEAM_INVITE_ENABLED": "false",
    }
    try:
        record_install_evidence(evidence_dir, "planned-volumes.private.json", {"plannedVolumes": sorted(planned_volumes),
                                "preservedExisting": sorted(planned_volumes & before_volumes), "selectedSourceVolume": target_volume,
                                "sourceVolumeExisted": False, "admittedServerImages": sorted(admitted_images), "serverImageId": server_image_id, "serverDigest": server_digest})
        for index, volume in enumerate(sorted(planned_volumes - before_volumes)):
            created = docker("volume", "create", "--label", "com.anxi-panel.test-owner=" + args.owner, volume)
            record_install_evidence(evidence_dir, "volume-create-" + str(index) + ".private.json",
                                    {"requestedVolume": volume, "returnedVolume": created})
            require(created == volume, "fixture volume create response changed")
            created_approved.add(volume)
            metadata = json.loads(docker("volume", "inspect", volume))
            require(len(metadata) == 1 and metadata[0].get("Labels") == {"com.anxi-panel.test-owner": args.owner},
                    "fixture cache volume ownership changed")
        record_install_evidence(evidence_dir, "approved-volumes.private.json", {"plannedVolumes": sorted(planned_volumes),
                                "createdVolumes": sorted(created_approved), "preservedExisting": sorted(planned_volumes & before_volumes)})
        lines = [line for line in original.decode("utf-8").splitlines() if line.split("=", 1)[0] not in updates]
        env_path.write_text("\n".join(lines + [key + "=" + value for key, value in updates.items()]) + "\n", encoding="utf-8", newline="\n")
        fixture_env_written = True
        jobs = []
        for _ in range(2):
            job = api("/api/instances/stardew/install", {
                "steamUsername": "patch-fixture-user", "steamPassword": "patch-fixture-secret",
                "vncPassword": "Patch7!", "imageTag": image_tag,
            }, expected=202)
            job_id = job["jobId"]
            attempt = len(jobs) + 1
            record_install_evidence(evidence_dir, "attempt-" + str(attempt) + "-accepted.private.json",
                                    {"httpStatus": 202, "jobId": job_id})
            require(job_id not in jobs, "retry reused terminal job")
            jobs.append(job_id)
            deadline = time.monotonic() + 90
            while time.monotonic() < deadline:
                result = api("/api/jobs/" + job_id)["job"]
                if result.get("status") in ("failed", "succeeded", "canceled"):
                    break
                time.sleep(0.5)
            logs = collect_job_logs(api, job_id)
            require("patch-fixture-secret" not in json.dumps({"job": result, "logs": logs}), "job exposed fixture credential")
            receipt = record_install_evidence(evidence_dir, "attempt-" + str(attempt) + "-terminal.private.json",
                                             {"jobId": job_id, "job": result, "logs": logs})
            if result.get("status") != "failed" or "账号或密码错误" not in result.get("errorMessage", ""):
                print(json.dumps({"event": "patch-install-job-diagnostics", "jobIdSHA256": hashlib.sha256(job_id.encode()).hexdigest(),
                                  "status": result.get("status"), "logCount": len(logs), "receipt": receipt,
                                  "causeTokens": install_cause_tokens(logs)},
                                 ensure_ascii=False), flush=True)
            require(result.get("status") == "failed", "controlled install did not fail terminally")
            require("账号或密码错误" in result.get("errorMessage", ""),
                    "actionable install cause lost: " + result.get("errorMessage", ""))
            require("patch-fixture-secret" not in json.dumps(result), "job exposed fixture credential")
            state = api("/api/instances/stardew")
            require(state.get("stateMessage") == result["errorMessage"], "instance lost durable failure cause")
        print("patch E2E: real Docker install failure, durable cause and retry passed", flush=True)
    finally:
        try:
            managed_volumes = managed_install_destinations(instance, target_volume, server_image, server_image_id, server_digest, previous_plans, admitted_images) if fixture_env_written else set()
        finally:
            env_path.write_bytes(original)
        approved = created_approved | managed_volumes
        observed = set(docker("volume", "ls", "--format", "{{.Name}}").splitlines())
        require(observed - before_volumes == approved, "unexpected fixture volume identity")
        record_install_evidence(evidence_dir, "cleanup-approved.private.json", {"createdByExplicitFixtureActions": sorted(created_approved),
                                "boundManagedDestinations": sorted(managed_volumes)})
        for volume in sorted(approved):
            if volume in managed_volumes:
                metadata = json.loads(docker("volume", "inspect", volume))
                require(len(metadata) == 1 and metadata[0].get("Labels") == {
                    "com.anxi-panel.smapi-update-staging": "true", "com.anxi-panel.compose-project": instance.name,
                }, "managed fixture volume ownership changed")
            else:
                metadata = json.loads(docker("volume", "inspect", volume))
                require(len(metadata) == 1 and metadata[0].get("Labels") == {"com.anxi-panel.test-owner": args.owner},
                        "fixture volume ownership changed before cleanup")
            require(not docker("ps", "-aq", "--filter", "volume=" + volume), "fixture volume still held")
            docker("volume", "rm", volume)


if __name__ == "__main__":
    main()
