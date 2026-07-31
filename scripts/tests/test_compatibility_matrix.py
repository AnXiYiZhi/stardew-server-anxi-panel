import copy
import hashlib
import importlib.util
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("compatibility_matrix", ROOT / "scripts" / "compatibility_matrix.py")
MATRIX = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MATRIX)


class FakeRangeResponse:
    def __init__(self, url, body, start, end, total, *, redirect_url=None, status=206):
        self.status = status
        self._url = redirect_url or url
        self._body = body
        self.headers = {
            "Content-Range": f"bytes {start}-{end}/{total}",
            "Content-Length": str(len(body)),
        }

    def __enter__(self):
        return self

    def __exit__(self, _type, _value, _traceback):
        return False

    def geturl(self):
        return self._url

    def read(self, size=-1):
        return self._body[:size]


class CompatibilityMatrixTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.base = MATRIX.load(ROOT / "backend" / "internal" / "games" / "stardew_junimo" / "config" / "runtime_stack_manifest.json")

    def test_embedded_recommended_is_valid(self):
        MATRIX.validate(self.base)
        self.assertEqual("recommended", self.base["status"])
        self.assertEqual("required", self.base["runtimeUpdatePolicy"])

    def test_update_policy_is_explicit(self):
        for policy in (None, "silent", ""):
            value = copy.deepcopy(self.base)
            value["runtimeUpdatePolicy"] = policy
            with self.assertRaisesRegex(MATRIX.MatrixError, "runtimeUpdatePolicy"):
                MATRIX.validate(value)

    def test_exact_component_digests_are_required(self):
        value = copy.deepcopy(self.base)
        value["server"]["digests"] = {}
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)

    def test_image_aliases_must_share_one_digest(self):
        value = copy.deepcopy(self.base)
        value["server"]["digests"][value["server"]["images"][1]] = "sha256:" + "f" * 64
        with self.assertRaisesRegex(MATRIX.MatrixError, "share one canonical digest"):
            MATRIX.validate(value)

    def test_required_remote_image_policy(self):
        self.assertTrue(MATRIX.required_remote_image("sdvd/server:1.2.3"))
        self.assertTrue(MATRIX.required_remote_image("ghcr.io/anxiyizhi/example:1.2.3"))
        self.assertTrue(MATRIX.required_remote_image("crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/example:1.2.3"))
        self.assertFalse(MATRIX.required_remote_image("docker.1ms.run/sdvd/server:1.2.3"))

    def test_latest_and_incomplete_auth_source_are_rejected(self):
        value = copy.deepcopy(self.base)
        value["steamAuth"]["tag"] = "latest"
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)
        value = copy.deepcopy(self.base)
        value["steamAuth"]["sourceRevision"] = "short"
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)

    def test_candidate_and_tested_statuses_are_not_part_of_release_flow(self):
        for status in ("candidate", "tested", "discovered"):
            value = copy.deepcopy(self.base)
            value["status"] = status
            with self.assertRaisesRegex(MATRIX.MatrixError, "status is invalid"):
                MATRIX.validate(value)

    def test_smapi_accelerators_and_hosts_are_exact(self):
        value = copy.deepcopy(self.base)
        value["smapi"]["urls"][0] = "https://mirror.example/smapi.zip"
        with self.assertRaisesRegex(MATRIX.MatrixError, "reviewed accelerator order"):
            MATRIX.validate(value)
        value = copy.deepcopy(self.base)
        value["smapi"]["trustedHosts"][0] = "mirror.example"
        with self.assertRaisesRegex(MATRIX.MatrixError, "reviewed accelerator hosts"):
            MATRIX.validate(value)

    def test_withdrawn_requires_fallback(self):
        value = copy.deepcopy(self.base)
        value["status"] = "withdrawn"
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.validate(value)
        value["withdrawal"] = {"reason": "critical regression", "fallbackStackVersion": "previous-exact-stack"}
        MATRIX.validate(value)

    def test_release_digest_parser_and_panel_minimum(self):
        digest = "sha256:" + "c" * 64
        self.assertEqual(digest, MATRIX.digest_from_imagetools(f"Name: example/image:1\nDigest: {digest}\n"))
        with self.assertRaises(MATRIX.MatrixError):
            MATRIX.digest_from_imagetools("Digest: sha256:short")
        self.assertTrue(MATRIX.panel_version_satisfies("v0.1.14", "0.1.14"))
        self.assertFalse(MATRIX.panel_version_satisfies("0.1.13", "0.1.14"))

    def test_smapi_download_resumes_completed_chunks_across_sources(self):
        archive = b"abcdefghij"
        smapi = self.smapi_fixture(archive)
        calls = []

        def open_url(request, timeout):
            self.assertEqual(MATRIX.SMAPI_REQUEST_TIMEOUT_SECONDS, timeout)
            start, end = self.request_range(request)
            calls.append((request.full_url, start, end))
            if request.full_url == smapi["urls"][0] and start == 4:
                raise OSError("connection reset")
            return FakeRangeResponse(request.full_url, archive[start : end + 1], start, end, len(archive))

        original_chunk_bytes = MATRIX.SMAPI_CHUNK_BYTES
        MATRIX.SMAPI_CHUNK_BYTES = 4
        try:
            MATRIX.download_smapi_archive(smapi, open_url=open_url, sleep=lambda _: None)
        finally:
            MATRIX.SMAPI_CHUNK_BYTES = original_chunk_bytes

        self.assertEqual(
            [
                (smapi["urls"][0], 0, 3),
                (smapi["urls"][0], 4, 7),
                (smapi["urls"][1], 4, 7),
                (smapi["urls"][1], 8, 9),
            ],
            calls,
        )

    def test_smapi_download_retries_truncated_chunk_without_hashing_it(self):
        archive = b"verified"
        smapi = self.smapi_fixture(archive)

        def open_url(request, timeout):
            self.assertEqual(MATRIX.SMAPI_REQUEST_TIMEOUT_SECONDS, timeout)
            start, end = self.request_range(request)
            body = archive[start : end + 1]
            if request.full_url == smapi["urls"][0]:
                body = body[:-1]
            return FakeRangeResponse(request.full_url, body, start, end, len(archive))

        MATRIX.download_smapi_archive(smapi, open_url=open_url, sleep=lambda _: None)

    def test_smapi_download_rejects_untrusted_redirect_and_uses_next_source(self):
        archive = b"trusted"
        smapi = self.smapi_fixture(archive)
        calls = []

        def open_url(request, timeout):
            self.assertEqual(MATRIX.SMAPI_REQUEST_TIMEOUT_SECONDS, timeout)
            start, end = self.request_range(request)
            calls.append(request.full_url)
            redirect = "https://evil.example/archive.zip" if request.full_url == smapi["urls"][0] else None
            return FakeRangeResponse(request.full_url, archive, start, end, len(archive), redirect_url=redirect)

        MATRIX.download_smapi_archive(smapi, open_url=open_url, sleep=lambda _: None)
        self.assertEqual(smapi["urls"], calls)

    def test_smapi_download_exhausts_bounded_rounds(self):
        archive = b"unavailable"
        smapi = self.smapi_fixture(archive)
        calls = []

        def open_url(request, timeout):
            self.assertEqual(MATRIX.SMAPI_REQUEST_TIMEOUT_SECONDS, timeout)
            calls.append(request.full_url)
            raise OSError("offline")

        with self.assertRaisesRegex(MATRIX.MatrixError, "failed at byte 0"):
            MATRIX.download_smapi_archive(smapi, open_url=open_url, sleep=lambda _: None, retry_rounds=2)
        self.assertEqual(4, len(calls))

    def test_smapi_download_rejects_final_hash_mismatch(self):
        archive = b"tampered"
        smapi = self.smapi_fixture(b"expected")

        def open_url(request, timeout):
            self.assertEqual(MATRIX.SMAPI_REQUEST_TIMEOUT_SECONDS, timeout)
            start, end = self.request_range(request)
            return FakeRangeResponse(request.full_url, archive, start, end, len(archive))

        smapi["archiveBytes"] = len(archive)
        with self.assertRaisesRegex(MATRIX.MatrixError, "SHA256 mismatch"):
            MATRIX.download_smapi_archive(smapi, open_url=open_url, sleep=lambda _: None)

    def test_smapi_download_restarts_fresh_after_integrity_mismatch(self):
        archive = b"expected"
        smapi = self.smapi_fixture(archive)
        calls = []

        def open_url(request, timeout):
            self.assertEqual(MATRIX.SMAPI_REQUEST_TIMEOUT_SECONDS, timeout)
            start, end = self.request_range(request)
            calls.append(request.full_url)
            body = b"tampered" if request.full_url == smapi["urls"][0] else archive
            return FakeRangeResponse(request.full_url, body[start : end + 1], start, end, len(archive))

        MATRIX.download_smapi_archive(smapi, open_url=open_url, sleep=lambda _: None)
        self.assertEqual(smapi["urls"], calls)

    def test_required_image_inspection_retries_transient_failure(self):
        digest = "sha256:" + "a" * 64
        results = [
            subprocess.CompletedProcess([], 1, "", "TLS timeout"),
            subprocess.CompletedProcess([], 0, f"Digest: {digest}\n", ""),
        ]
        sleeps = []

        self.assertTrue(
            MATRIX.inspect_remote_image(
                "example/image:1",
                digest,
                required=True,
                run=lambda *_args, **_kwargs: results.pop(0),
                sleep=sleeps.append,
            )
        )
        self.assertEqual([1], sleeps)

    def test_image_digest_mismatch_is_not_retried(self):
        expected = "sha256:" + "a" * 64
        actual = "sha256:" + "b" * 64
        calls = []

        def run(*_args, **_kwargs):
            calls.append(True)
            return subprocess.CompletedProcess([], 0, f"Digest: {actual}\n", "")

        with self.assertRaisesRegex(MATRIX.MatrixError, "tag/digest mismatch"):
            MATRIX.inspect_remote_image("example/image:1", expected, required=True, run=run, sleep=lambda _: None)
        self.assertEqual(1, len(calls))

    def test_traceability_fetch_retries_but_stays_bounded(self):
        results = [
            subprocess.CompletedProcess([], 1, "", "network reset"),
            subprocess.CompletedProcess([], 1, "", "network reset"),
        ]
        with self.assertRaisesRegex(MATRIX.MatrixError, "after 2 attempts"):
            MATRIX.run_traceability_command(
                ["git", "fetch"],
                run=lambda *_args, **_kwargs: results.pop(0),
                sleep=lambda _: None,
                retry_rounds=2,
            )

    @staticmethod
    def smapi_fixture(archive):
        return {
            "urls": ["https://mirror-one.example/archive.zip", "https://mirror-two.example/archive.zip"],
            "trustedHosts": ["mirror-one.example", "mirror-two.example"],
            "archiveBytes": len(archive),
            "sha256": hashlib.sha256(archive).hexdigest(),
        }

    @staticmethod
    def request_range(request):
        value = request.get_header("Range").removeprefix("bytes=")
        start, end = value.split("-", 1)
        return int(start), int(end)


if __name__ == "__main__":
    unittest.main()
