#!/usr/bin/env python3
"""Validate and seal the privacy-safe acceptance of one immutable candidate."""

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path


COMMON_CHECKS = frozenset({
    "isolatedDocker", "noProductionData", "previousWebCheck",
    "previousWebDryRun", "administratorConfirm", "disconnectReconnect",
    "webUpgrade", "unhealthyRollback", "sqlitePreserved",
    "initializationUsersPreserved", "nonTargetResourcesPreserved",
    "changedFeatureDockerE2E", "postUpgradeFeatureAcceptance", "panelRestart",
    "resourcesCleaned",
})
BRIDGE_CHECKS = frozenset({
    "bridgePersisted", "legacySees073Only", "globalUpdateCenter",
})
FULL_STACK_CHECKS = frozenset({
    "prepared073", "scopeConfirmed", "server140", "formalAuth140",
    "validInviteCode", "saveAndFarmersPreserved", "modsPreserved",
    "backupRestore", "archiveExport", "authSessionPreserved",
    "negativeMissingBridge",
})
ALL_CHECKS = COMMON_CHECKS | BRIDGE_CHECKS | FULL_STACK_CHECKS
IDENTITY_FIELDS = {
    "version": "version", "commit": "commit", "digest": "digest",
    "candidateWorkflowRunId": "workflowRunId", "buildDate": "buildDate",
}
REQUIRED_FIELDS = frozenset({"schemaVersion", "checks", "evidence"}) | IDENTITY_FIELDS.keys()
OPTIONAL_FIELDS = frozenset({"acceptanceWorkflowRunId", "proofSha256"})
EVIDENCE_FIELDS = frozenset({"manifestSha256", "startedAt", "completedAt"})
VERSION = re.compile(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z")
SHA256 = re.compile(r"[0-9a-f]{64}\Z")
COMMIT = re.compile(r"[0-9a-f]{40}\Z")
RUN_ID = re.compile(r"[1-9][0-9]*\Z")
UTC_TIMESTAMP = re.compile(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z\Z")


def version(value):
    match = VERSION.fullmatch(value) if isinstance(value, str) else None
    if match is None:
        raise ValueError("version must be an exact stable version")
    return tuple(map(int, match.groups()))


def requires(value):
    return version(value) >= (0, 7, 3)


def required_checks(value):
    parsed = version(value)
    if parsed < (0, 7, 3):
        return frozenset()
    return COMMON_CHECKS | (BRIDGE_CHECKS if parsed < (0, 8, 0) else FULL_STACK_CHECKS)


def utc_timestamp(value, field):
    if not isinstance(value, str) or UTC_TIMESTAMP.fullmatch(value) is None:
        raise ValueError(field + " must be a canonical UTC timestamp")
    try:
        return datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    except ValueError as error:
        raise ValueError(field + " must be a valid UTC timestamp") from error


def _object_without_duplicates(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON field")
        result[key] = value
    return result


def _invalid_constant(_value):
    raise ValueError("non-finite JSON number")


def load_json(path):
    raw = Path(path).read_bytes()
    if len(raw) > 128 * 1024:
        raise ValueError("acceptance or candidate JSON exceeds the size limit")
    try:
        parsed = json.loads(raw.decode("utf-8"), object_pairs_hook=_object_without_duplicates,
                            parse_constant=_invalid_constant)
    except (UnicodeError, json.JSONDecodeError) as error:
        raise ValueError("input must be valid UTF-8 JSON") from error
    if not isinstance(parsed, dict):
        raise ValueError("JSON root must be an object")
    return raw, parsed


def validate_acceptance(candidate, acceptance, proof_sha256, acceptance_run_id=None):
    """Return a sealed copy; never copy paths or other candidate fields into it."""
    if not isinstance(candidate, dict) or type(candidate.get("schemaVersion")) is not int or candidate["schemaVersion"] != 1:
        raise ValueError("candidate schemaVersion must be 1")
    if not isinstance(acceptance, dict):
        raise ValueError("acceptance root must be an object")
    if set(acceptance) - REQUIRED_FIELDS - OPTIONAL_FIELDS:
        raise ValueError("unknown acceptance root field")
    if REQUIRED_FIELDS - set(acceptance):
        raise ValueError("missing acceptance root field")
    if type(acceptance["schemaVersion"]) is not int or acceptance["schemaVersion"] != 1:
        raise ValueError("acceptance schemaVersion must be 1")
    if not isinstance(proof_sha256, str) or SHA256.fullmatch(proof_sha256) is None:
        raise ValueError("candidate proof SHA256 must be lowercase hexadecimal")

    for field, candidate_field in IDENTITY_FIELDS.items():
        value = acceptance[field]
        if not isinstance(value, str) or value != candidate.get(candidate_field):
            raise ValueError("acceptance identity mismatch: " + field)
    version(acceptance["version"])
    if COMMIT.fullmatch(acceptance["commit"]) is None:
        raise ValueError("commit must be a full lowercase SHA1")
    if not acceptance["digest"].startswith("sha256:") or SHA256.fullmatch(acceptance["digest"][7:]) is None:
        raise ValueError("digest must be an exact SHA256 digest")
    if RUN_ID.fullmatch(acceptance["candidateWorkflowRunId"]) is None:
        raise ValueError("candidateWorkflowRunId must be a positive numeric string")
    built_at = utc_timestamp(acceptance["buildDate"], "buildDate")

    checks = acceptance["checks"]
    if not isinstance(checks, dict) or set(checks) - ALL_CHECKS:
        raise ValueError("checks must contain only supported check names")
    if required_checks(acceptance["version"]) - set(checks):
        raise ValueError("missing required acceptance check")
    if any(value is not True for value in checks.values()):
        raise ValueError("each acceptance check must be boolean true")

    evidence = acceptance["evidence"]
    if not isinstance(evidence, dict) or set(evidence) != EVIDENCE_FIELDS:
        raise ValueError("evidence must contain exactly manifestSha256, startedAt and completedAt")
    manifest = evidence["manifestSha256"]
    if not isinstance(manifest, str) or SHA256.fullmatch(manifest) is None:
        raise ValueError("manifestSha256 must be lowercase SHA256 hexadecimal")
    started_at = utc_timestamp(evidence["startedAt"], "evidence.startedAt")
    completed_at = utc_timestamp(evidence["completedAt"], "evidence.completedAt")
    if not built_at <= started_at <= completed_at:
        raise ValueError("acceptance times must follow buildDate <= startedAt <= completedAt")

    if acceptance_run_id is not None and (not isinstance(acceptance_run_id, str) or RUN_ID.fullmatch(acceptance_run_id) is None):
        raise ValueError("acceptance run ID must be a positive numeric string")
    if "acceptanceWorkflowRunId" in acceptance:
        stamp = acceptance["acceptanceWorkflowRunId"]
        if not isinstance(stamp, str) or RUN_ID.fullmatch(stamp) is None or stamp != acceptance_run_id:
            raise ValueError("acceptanceWorkflowRunId must match the supplied acceptance run ID")
    if "proofSha256" in acceptance and acceptance["proofSha256"] != proof_sha256:
        raise ValueError("proofSha256 does not match the candidate file bytes")

    sealed = dict(acceptance)
    sealed["proofSha256"] = proof_sha256
    if acceptance_run_id is not None:
        sealed["acceptanceWorkflowRunId"] = acceptance_run_id
    return sealed


def write_json(path, content):
    destination = Path(path)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", newline="\n",
                                         prefix=".acceptance-", suffix=".json",
                                         dir=destination.parent, delete=False) as handle:
            temporary = Path(handle.name)
            json.dump(content, handle, ensure_ascii=False, indent=2, sort_keys=True)
            handle.write("\n")
        os.replace(temporary, destination)
        temporary = None
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate")
    parser.add_argument("--acceptance")
    parser.add_argument("--output", help="write the sealed proof; omit for read-only verification")
    parser.add_argument("--acceptance-run-id")
    parser.add_argument("--required", metavar="VERSION", help="print whether this version requires acceptance")
    args = parser.parse_args(argv)
    if args.required is not None:
        if any((args.candidate, args.acceptance, args.output, args.acceptance_run_id)):
            parser.error("--required cannot be combined with proof arguments")
        print("true" if requires(args.required) else "false")
        return 0
    if not args.candidate or not args.acceptance:
        parser.error("--candidate and --acceptance are required")
    if args.output and Path(args.output).resolve() == Path(args.candidate).resolve():
        raise ValueError("output must not replace the candidate proof")
    candidate_raw, candidate = load_json(args.candidate)
    _, acceptance = load_json(args.acceptance)
    sealed = validate_acceptance(candidate, acceptance, hashlib.sha256(candidate_raw).hexdigest(),
                                 args.acceptance_run_id)
    if args.output:
        write_json(args.output, sealed)
    print("candidate acceptance verified")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except ValueError as error:
        print("candidate acceptance rejected: " + str(error), file=sys.stderr)
        sys.exit(1)
    except OSError:
        # A filesystem error may include a private local path.
        print("candidate acceptance rejected: proof file I/O failed", file=sys.stderr)
        sys.exit(1)
