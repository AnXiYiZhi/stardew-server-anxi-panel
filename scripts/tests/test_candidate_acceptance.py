import contextlib
import copy
import hashlib
import importlib.util
import io
import json
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("candidate_acceptance", ROOT / "scripts/candidate_acceptance.py")
ACCEPTANCE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ACCEPTANCE)


class CandidateAcceptanceTests(unittest.TestCase):
    def setUp(self):
        self.candidate = {
            "schemaVersion": 1, "version": "0.7.3", "commit": "a" * 40,
            "digest": "sha256:" + "b" * 64, "workflowRunId": "123456",
            "buildDate": "2026-10-27T00:00:00Z", "localImage": "private-local-reference",
        }
        self.acceptance = {
            "schemaVersion": 1, "version": "0.7.3", "commit": "a" * 40,
            "digest": "sha256:" + "b" * 64, "candidateWorkflowRunId": "123456",
            "buildDate": "2026-10-27T00:00:00Z",
            "checks": dict.fromkeys(ACCEPTANCE.required_checks("0.7.3"), True),
            "evidence": {"manifestSha256": "c" * 64, "startedAt": "2026-10-27T00:01:00Z",
                         "completedAt": "2026-10-27T01:00:00Z"},
        }
        self.proof_hash = "d" * 64

    def seal(self, run_id="654321"):
        return ACCEPTANCE.validate_acceptance(self.candidate, self.acceptance, self.proof_hash, run_id)

    def test_requirement_boundary_and_semantic_version_order(self):
        self.assertFalse(ACCEPTANCE.requires("0.7.2"))
        for target in ("0.7.3", "0.7.10", "0.8.0", "1.0.0"):
            with self.subTest(target=target):
                self.assertTrue(ACCEPTANCE.requires(target))
        self.assertEqual(ACCEPTANCE.required_checks("0.7.2"), frozenset())

    def test_exact_stable_version_only(self):
        for value in ("v0.7.3", "0.07.3", "0.7.3-preview", "0.7", " 0.7.3", 73):
            with self.subTest(value=value), self.assertRaises(ValueError):
                ACCEPTANCE.requires(value)

    def test_bridge_seals_only_safe_fields_and_preserves_input(self):
        before = copy.deepcopy(self.acceptance)
        sealed = self.seal()
        self.assertEqual(sealed["proofSha256"], self.proof_hash)
        self.assertEqual(sealed["acceptanceWorkflowRunId"], "654321")
        self.assertEqual(self.acceptance, before)
        self.assertNotIn("localImage", sealed)
        self.assertNotIn("private-local-reference", json.dumps(sealed))

    def test_full_stack_requires_its_real_feature_checks(self):
        self.candidate["version"] = self.acceptance["version"] = "0.8.0"
        with self.assertRaisesRegex(ValueError, "missing required"):
            self.seal()
        self.acceptance["checks"] = dict.fromkeys(ACCEPTANCE.required_checks("0.8.0"), True)
        self.assertIn("validInviteCode", self.seal()["checks"])
        for check in ACCEPTANCE.FULL_STACK_CHECKS:
            with self.subTest(check=check):
                checks = self.acceptance["checks"].copy()
                del self.acceptance["checks"][check]
                with self.assertRaisesRegex(ValueError, "missing required"):
                    self.seal()
                self.acceptance["checks"] = checks

    def test_other_version_check_names_are_allowed_but_still_boolean_true(self):
        self.acceptance["checks"].update(dict.fromkeys(ACCEPTANCE.FULL_STACK_CHECKS, True))
        self.seal()
        self.acceptance["checks"]["validInviteCode"] = False
        with self.assertRaisesRegex(ValueError, "boolean true"):
            self.seal()

    def test_each_candidate_identity_must_match_exactly(self):
        for field in ACCEPTANCE.IDENTITY_FIELDS:
            with self.subTest(field=field):
                original = self.acceptance[field]
                self.acceptance[field] = original + "x"
                with self.assertRaisesRegex(ValueError, "identity mismatch: " + field):
                    self.seal()
                self.acceptance[field] = original

    def test_identity_type_cannot_match_by_numeric_coercion(self):
        self.candidate["workflowRunId"] = self.acceptance["candidateWorkflowRunId"] = 123456
        with self.assertRaisesRegex(ValueError, "identity mismatch"):
            self.seal()

    def test_candidate_and_acceptance_schema_version_are_strict_integers(self):
        for content in (self.candidate, self.acceptance):
            for invalid in (True, "1", 1.0, 2):
                with self.subTest(invalid=invalid):
                    content["schemaVersion"] = invalid
                    with self.assertRaisesRegex(ValueError, "schemaVersion"):
                        self.seal()
            content["schemaVersion"] = 1

    def test_every_required_bridge_check_is_present(self):
        for check in ACCEPTANCE.required_checks("0.7.3"):
            with self.subTest(check=check):
                checks = self.acceptance["checks"].copy()
                del self.acceptance["checks"][check]
                with self.assertRaisesRegex(ValueError, "missing required"):
                    self.seal()
                self.acceptance["checks"] = checks

    def test_check_truthiness_is_not_boolean_evidence(self):
        for invalid in (False, 1, "true", None, [], {}):
            with self.subTest(invalid=invalid):
                self.acceptance["checks"]["webUpgrade"] = invalid
                with self.assertRaisesRegex(ValueError, "boolean true"):
                    self.seal()

    def test_root_check_and_evidence_allowlists_reject_private_fields(self):
        for target in (self.acceptance, self.acceptance["checks"], self.acceptance["evidence"]):
            with self.subTest(target_fields=list(target)):
                target["privateToken"] = "secret-test-value"
                with self.assertRaises(ValueError) as rejected:
                    self.seal()
                self.assertNotIn("secret-test-value", str(rejected.exception))
                del target["privateToken"]

    def test_required_root_and_evidence_fields_cannot_be_omitted(self):
        for target, fields in ((self.acceptance, ACCEPTANCE.REQUIRED_FIELDS),
                               (self.acceptance["evidence"], ACCEPTANCE.EVIDENCE_FIELDS)):
            for field in fields:
                with self.subTest(field=field):
                    original = target.pop(field)
                    with self.assertRaises(ValueError):
                        self.seal()
                    target[field] = original

    def test_digest_commit_manifest_and_run_ids_have_exact_formats(self):
        cases = (("commit", "a" * 39), ("commit", "A" * 40),
                 ("digest", "b" * 64), ("digest", "sha256:" + "b" * 63),
                 ("candidateWorkflowRunId", "0"), ("candidateWorkflowRunId", "0123"))
        for field, invalid in cases:
            with self.subTest(field=field, invalid=invalid):
                original = self.acceptance[field]
                candidate_field = ACCEPTANCE.IDENTITY_FIELDS[field]
                self.candidate[candidate_field] = self.acceptance[field] = invalid
                with self.assertRaises(ValueError):
                    self.seal()
                self.candidate[candidate_field] = self.acceptance[field] = original
        for invalid in ("c" * 63, "C" * 64, "sha256:" + "c" * 64, 123):
            with self.subTest(manifest=invalid):
                self.acceptance["evidence"]["manifestSha256"] = invalid
                with self.assertRaisesRegex(ValueError, "manifestSha256"):
                    self.seal()

    def test_timestamps_are_valid_and_canonical_utc(self):
        for invalid in ("2026-10-27T00:01:00+00:00", "2026-10-27T00:01:00.000Z",
                        "2026-02-30T00:01:00Z", "2026-10-27T24:00:00Z",
                        "2026-10-27T00:01:60Z", "2026-10-27 00:01:00Z", None):
            with self.subTest(invalid=invalid):
                self.acceptance["evidence"]["startedAt"] = invalid
                with self.assertRaisesRegex(ValueError, "UTC timestamp"):
                    self.seal()

    def test_acceptance_cannot_precede_build_or_end_before_start(self):
        for field, invalid in (("startedAt", "2026-10-26T23:59:59Z"),
                               ("completedAt", "2026-10-27T00:00:59Z")):
            with self.subTest(field=field):
                original = self.acceptance["evidence"][field]
                self.acceptance["evidence"][field] = invalid
                with self.assertRaisesRegex(ValueError, "buildDate <= startedAt <= completedAt"):
                    self.seal()
                self.acceptance["evidence"][field] = original

    def test_sealed_proof_revalidates_without_changing_its_stamps(self):
        self.acceptance = self.seal()
        self.assertEqual(self.acceptance, self.seal())
        with self.assertRaisesRegex(ValueError, "acceptanceWorkflowRunId"):
            self.seal(run_id="654322")
        with self.assertRaisesRegex(ValueError, "acceptanceWorkflowRunId"):
            self.seal(run_id=None)
        self.acceptance["proofSha256"] = "e" * 64
        with self.assertRaisesRegex(ValueError, "candidate file bytes"):
            self.seal()

    def test_acceptance_run_stamp_is_optional_but_strict(self):
        self.assertNotIn("acceptanceWorkflowRunId", self.seal(run_id=None))
        for invalid in ("", "0", "01", "abc", 123, True):
            with self.subTest(invalid=invalid), self.assertRaisesRegex(ValueError, "run ID"):
                self.seal(run_id=invalid)

    def test_json_rejects_duplicate_fields_nonfinite_numbers_and_nonobjects(self):
        with tempfile.TemporaryDirectory() as temporary:
            source = Path(temporary) / "proof.json"
            for invalid in ('{"checks":{},"checks":{"webUpgrade":true}}',
                            '{"extra":NaN}', '[]', '"text"'):
                with self.subTest(invalid=invalid):
                    source.write_text(invalid, encoding="utf-8")
                    with self.assertRaises(ValueError):
                        ACCEPTANCE.load_json(source)

    def test_cli_hashes_exact_candidate_bytes_and_supports_read_only_recheck(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            candidate = root / "candidate.json"
            acceptance = root / "input.json"
            output = root / "acceptance.json"
            candidate.write_text(json.dumps(self.candidate) + "\n", encoding="utf-8", newline="\n")
            acceptance.write_text(json.dumps(self.acceptance), encoding="utf-8")
            args = ["--candidate", str(candidate), "--acceptance", str(acceptance),
                    "--acceptance-run-id", "654321"]
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(ACCEPTANCE.main(args + ["--output", str(output)]), 0)
            sealed_bytes = output.read_bytes()
            sealed = json.loads(sealed_bytes)
            self.assertEqual(sealed["proofSha256"], hashlib.sha256(candidate.read_bytes()).hexdigest())
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(ACCEPTANCE.main(["--candidate", str(candidate), "--acceptance", str(output),
                                                  "--acceptance-run-id", "654321"]), 0)
            self.assertEqual(sealed_bytes, output.read_bytes())
            candidate.write_bytes(candidate.read_bytes() + b"\n")
            with self.assertRaisesRegex(ValueError, "candidate file bytes"):
                ACCEPTANCE.main(["--candidate", str(candidate), "--acceptance", str(output),
                                 "--acceptance-run-id", "654321"])

    def test_cli_rejects_replacing_the_candidate_and_failure_leaves_no_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            candidate = root / "candidate.json"
            acceptance = root / "input.json"
            output = root / "acceptance.json"
            candidate.write_text(json.dumps(self.candidate), encoding="utf-8")
            acceptance.write_text(json.dumps(self.acceptance), encoding="utf-8")
            args = ["--candidate", str(candidate), "--acceptance", str(acceptance)]
            with self.assertRaisesRegex(ValueError, "must not replace"):
                ACCEPTANCE.main(args + ["--output", str(candidate)])
            self.acceptance["checks"]["webUpgrade"] = False
            acceptance.write_text(json.dumps(self.acceptance), encoding="utf-8")
            with self.assertRaises(ValueError):
                ACCEPTANCE.main(args + ["--output", str(output)])
            self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
