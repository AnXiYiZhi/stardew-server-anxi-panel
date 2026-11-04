import importlib.util
import ast
import json
from pathlib import Path
import re
import tempfile
import unittest
from types import SimpleNamespace


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("release_patch_features", Path(__file__).with_name("release_patch_features.py"))
PATCH = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PATCH)
MANIFEST = json.loads((ROOT / "backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json").read_text(encoding="utf-8"))


class PatchFixtureTests(unittest.TestCase):
    def test_install_response_is_preserved_before_status_or_json_assertion(self):
        tree = ast.parse((ROOT / "scripts/tests/release_patch_features.py").read_text(encoding="utf-8"))
        main = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "main")
        api = next(node for node in main.body if isinstance(node, ast.FunctionDef) and node.name == "api")
        factory = ast.parse("def make_api():\n api_sequence = 0\n return api\n").body[0]
        factory.body.insert(1, api)
        module = ast.fix_missing_locations(ast.Module(body=[factory], type_ignores=[]))
        for status, raw in [(500, b'{"error":{"code":"prepare_failed","message":"patch-fixture-secret"}}'),
                            (202, b'not-json')]:
            with self.subTest(status=status), tempfile.TemporaryDirectory() as directory:
                calls = []
                def request(path, body):
                    calls.append(path)
                    return status, {}, raw
                scope = dict(PATCH.__dict__)
                scope.update({"args": SimpleNamespace(data_dir=directory), "request": request})
                exec(compile(module, "<actual-api-recorder>", "exec"), scope)
                invoke = scope["make_api"]()
                with self.assertRaises((RuntimeError, json.JSONDecodeError)) as caught:
                    invoke("/api/instances/stardew-3/install", {"steamPassword": "patch-fixture-secret"}, expected=202)
                self.assertEqual(calls, ["/api/instances/stardew-3/install"])
                evidence = Path(directory) / "patch-api-evidence"
                receipt = json.loads((evidence / "0001.private.json").read_text(encoding="utf-8"))
                self.assertEqual(receipt["httpStatus"], status)
                self.assertEqual(bytes.fromhex(receipt["rawHex"]), raw)
                self.assertTrue((evidence / "0001.intent.private.json").is_file())
                self.assertNotIn("patch-fixture-secret", str(caught.exception))

    def test_installation_fixture_only_reads_the_accepted_source_scope(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "instances/stardew"
            source.mkdir(parents=True)
            (source / ".env").write_text("STEAM_INVITE_ENABLED=true\n", encoding="utf-8")
            (source / "docker-compose.yml").write_text("services: {}\n", encoding="utf-8")
            cache = Path(directory) / "session"
            cache.mkdir()
            (cache / "session.json").write_text("fixture-only", encoding="utf-8")
            row = {"Id": "accepted-auth", "Image": "fixed-auth-image", "State": {"StartedAt": "original", "Running": True},
                   "Config": {}, "HostConfig": {}, "Mounts": [{"Type": "volume", "Name": "accepted-cache", "Source": str(cache)}]}
            calls = []
            def docker(*args):
                calls.append(args)
                if args[0] == "ps": return "accepted-auth"
                if args == ("inspect", "accepted-auth"): return json.dumps([row])
                if args == ("volume", "inspect", "accepted-cache"): return json.dumps([{"Mountpoint": str(cache)}])
                raise AssertionError("source preservation probe attempted a mutation")
            original = PATCH.docker
            PATCH.docker = docker
            try:
                before = PATCH.protected_source_snapshot(directory)
                self.assertEqual(before, PATCH.protected_source_snapshot(directory))
                (cache / "session.json").write_text("changed", encoding="utf-8")
                self.assertNotEqual(before, PATCH.protected_source_snapshot(directory))
                self.assertEqual((source / ".env").read_text(encoding="utf-8"), "STEAM_INVITE_ENABLED=true\n")
                self.assertTrue(all(args[0] in {"ps", "inspect", "volume"} for args in calls))
            finally:
                PATCH.docker = original

    def test_both_candidate_runners_export_the_pinned_recommended_server(self):
        image, digest = PATCH.fixture_server_contract(MANIFEST, MANIFEST["server"]["tag"])
        shell = (ROOT / "scripts/release-candidate.sh").read_text(encoding="utf-8")
        windows = (ROOT / "scripts/release-candidate.ps1").read_text(encoding="utf-8")
        self.assertEqual(re.search(r'^junimo_fixture_image="([^"]+)"$', shell, re.MULTILINE)[1], image)
        self.assertEqual(re.search(r'^junimo_fixture_digest="([^"]+)"$', shell, re.MULTILINE)[1], digest)
        self.assertEqual(re.search(r"\$junimoFixtureImage = '([^']+)'", windows)[1], image)
        self.assertEqual(re.search(r"\$junimoFixtureDigest = '([^']+)'", windows)[1], digest)
        self.assertIn('pull_fixture_image "$junimo_fixture_image@$junimo_fixture_digest"', shell)
        self.assertIn('docker save -o "$fixtures_tar" "$previous_ref" registry:2 nginx:alpine alpine:3.20 "$junimo_fixture_image"', shell)
        self.assertIn('Invoke-DockerPullWithTimeout -ImageRef "${junimoFixtureImage}@${junimoFixtureDigest}"', windows)
        self.assertIn('& docker save -o $fixturesTar $previousRef registry:2 nginx:alpine alpine:3.20 $junimoFixtureImage', windows)
        upgrade = (ROOT / "scripts/tests/test_release_candidate_upgrade.sh").read_text(encoding="utf-8")
        self.assertIn('docker tag ' + image + ' "$project/patch-server:' + MANIFEST["server"]["tag"] + '"', upgrade)

    def test_alpine_cannot_impersonate_the_recommended_server(self):
        _, digest = PATCH.fixture_server_contract(MANIFEST, MANIFEST["server"]["tag"])
        with self.assertRaisesRegex(RuntimeError, "fixed runtime digest"):
            PATCH.verify_fixture_server([{"Id": "sha256:" + "0" * 64, "RepoDigests": ["alpine@sha256:" + "0" * 64]}], digest)
        self.assertEqual(PATCH.verify_fixture_server([{"Id": digest, "RepoDigests": []}], digest), digest)
        config_id = "sha256:" + "1" * 64
        self.assertEqual(PATCH.verify_fixture_server([{"Id": config_id, "RepoDigests": ["sdvd/server@" + digest]}], digest), config_id)

    def test_install_intent_only_authorizes_its_original_api_destination(self):
        with tempfile.TemporaryDirectory(prefix="release-patch-fixture-") as directory:
            instance = Path(directory) / "stardew"
            instance.mkdir()
            source = "task-patch-install"
            destination = "stardew_anxi-smapi-update-" + "a" * 24
            (instance / ".env").write_text("GAME_DATA_VOLUME=" + destination + "\n", encoding="utf-8")
            plan_directory = instance / ".local-container/install-game-volumes"
            plan_directory.mkdir(parents=True)
            plan_path = plan_directory / (destination + ".json")
            plan = {"schema": 1, "project": "stardew", "sourceVolume": source, "targetVolume": destination,
                    "image": "task/server:140", "imageId": "fixed-id", "digest": "fixed-digest", "created": True}
            plan_path.write_text(json.dumps(plan), encoding="utf-8")
            self.assertEqual(PATCH.managed_install_destinations(instance, source, "task/server:140", "fixed-id", "fixed-digest", set()), {destination})
            plan["sourceVolume"] = "foreign"
            plan_path.write_text(json.dumps(plan), encoding="utf-8")
            with self.assertRaisesRegex(RuntimeError, "controlled API request"):
                PATCH.managed_install_destinations(instance, source, "task/server:140", "fixed-id", "fixed-digest", set())
            (instance / ".env").write_text("GAME_DATA_VOLUME=../foreign\n", encoding="utf-8")
            with self.assertRaisesRegex(RuntimeError, "unexpected managed"):
                PATCH.managed_install_destinations(instance, source, "task/server:140", "fixed-id", "fixed-digest", {plan_path.name})

    def test_each_retry_destination_requires_an_exact_plan_in_the_source_lineage(self):
        with tempfile.TemporaryDirectory(prefix="release-patch-retry-") as directory:
            instance = Path(directory) / "stardew"
            plans = instance / ".local-container/install-game-volumes"
            plans.mkdir(parents=True)
            first = "stardew_anxi-smapi-update-" + "a" * 24
            second = "stardew_anxi-smapi-update-" + "b" * 24
            (instance / ".env").write_text("GAME_DATA_VOLUME=" + second + "\n", encoding="utf-8")
            for source, target in [("task-source", first), (first, second)]:
                plan = {"schema": 1, "project": "stardew", "sourceVolume": source, "targetVolume": target,
                        "image": "task/server:140", "imageId": "fixed-id", "digest": "fixed-digest", "created": True}
                (plans / (target + ".json")).write_text(json.dumps(plan), encoding="utf-8")
            self.assertEqual(PATCH.managed_install_destinations(instance, "task-source", "task/server:140", "fixed-id", "fixed-digest", set()), {first, second})
            with self.assertRaisesRegex(RuntimeError, "source is outside"):
                PATCH.managed_install_destinations(instance, "foreign-source", "task/server:140", "fixed-id", "fixed-digest", set())

    def test_only_explicit_fixed_digest_fallback_references_are_admitted(self):
        with tempfile.TemporaryDirectory(prefix="release-patch-image-ref-") as directory:
            instance = Path(directory) / "stardew"
            plans = instance / ".local-container/install-game-volumes"
            plans.mkdir(parents=True)
            target = "stardew_anxi-smapi-update-" + "a" * 24
            (instance / ".env").write_text("GAME_DATA_VOLUME=" + target + "\n", encoding="utf-8")
            path = plans / (target + ".json")
            plan = {"schema": 1, "project": "stardew", "sourceVolume": "task-source", "targetVolume": target,
                    "image": "fixed/server:140", "imageId": "fixed-id", "digest": "fixed-digest", "created": True}
            path.write_text(json.dumps(plan), encoding="utf-8")
            admitted = {"task/server:140", "fixed/server:140"}
            self.assertEqual(PATCH.managed_install_destinations(instance, "task-source", "task/server:140", "fixed-id", "fixed-digest", set(), admitted), {target})
            plan["image"] = "unapproved/server:140"
            path.write_text(json.dumps(plan), encoding="utf-8")
            with self.assertRaisesRegex(RuntimeError, "controlled API request"):
                PATCH.managed_install_destinations(instance, "task-source", "task/server:140", "fixed-id", "fixed-digest", set(), admitted)

    def test_complete_same_job_logs_are_read_before_cleanup(self):
        calls = []
        def api(path):
            calls.append(path)
            pages = [[{"id": index} for index in range(1, 501)], [{"id": 501, "message": "ERROR (Invalid Password)"}], []]
            return {"logs": pages[len(calls) - 1]}
        logs = PATCH.collect_job_logs(api, "same-job")
        self.assertEqual(len(logs), 501)
        self.assertEqual(calls, ["/api/jobs/same-job/logs?after=0&limit=1000", "/api/jobs/same-job/logs?after=500&limit=1000", "/api/jobs/same-job/logs?after=501&limit=1000"])

    def test_log_cursor_cannot_repeat_a_previous_page(self):
        def api(path):
            return {"logs": [{"id": 1}] * 1000}
        with self.assertRaisesRegex(RuntimeError, "did not advance"):
            PATCH.collect_job_logs(api, "same-job")

    def test_diagnostic_cause_tokens_do_not_echo_log_payload(self):
        result = PATCH.install_cause_tokens([{"message": "private-payload ERROR (Invalid Password) private-id"}])
        self.assertEqual(result, ["ERROR (Invalid Password)"])

    def test_original_accepted_and_terminal_receipts_cannot_be_overwritten(self):
        with tempfile.TemporaryDirectory(prefix="release-patch-receipt-") as directory:
            root = Path(directory)
            value = {"httpStatus": 202, "jobId": "original-job"}
            receipt = PATCH.record_install_evidence(root, "accepted.private.json", value)
            self.assertEqual(json.loads((root / receipt["file"]).read_text(encoding="utf-8")), value)
            with self.assertRaises(FileExistsError):
                PATCH.record_install_evidence(root, "accepted.private.json", {"jobId": "replacement"})


if __name__ == "__main__":
    unittest.main()
