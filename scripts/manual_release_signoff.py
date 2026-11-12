"""Seal a repository owner's approval of an existing immutable candidate."""

import argparse
import hashlib
import re
import subprocess
import sys

from scripts.candidate_acceptance import (
    COMMIT, RUN_ID, SHA256, load_json, required_checks, utc_timestamp, write_json,
)


IDENTITY_FIELDS = {
    "version": "version",
    "commit": "commit",
    "candidateWorkflowRunId": "workflowRunId",
    "digest": "digest",
}
SIGNOFF_FIELDS = IDENTITY_FIELDS.keys() | {"evidenceSha256", "checksWithoutCandidateEvidence"}
RELEASE_CONTROL_PATHS = {
    b".agents/error-notebook.md",
    b".github/workflows/release.yml",
    b"AGENTS.md",
    b"docs/09-image-build.md",
    b"scripts/manual_release_signoff.py",
    b"scripts/tests/test_candidate_acceptance.py",
}
ACTOR = re.compile(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})\Z")


def validate_signoff(candidate, signoff, proof_sha256, actor, run_id, release_head):
    if type(candidate.get("schemaVersion")) is not int or candidate["schemaVersion"] != 1:
        raise ValueError("candidate schemaVersion must be 1")
    if set(signoff) != SIGNOFF_FIELDS:
        raise ValueError("manual sign-off fields do not match the schema")
    for field, candidate_field in IDENTITY_FIELDS.items():
        if not isinstance(signoff[field], str) or signoff[field] != candidate.get(candidate_field):
            raise ValueError("manual sign-off identity mismatch: " + field)
    checks = required_checks(signoff["version"])
    if (not COMMIT.fullmatch(signoff["commit"]) or not RUN_ID.fullmatch(signoff["candidateWorkflowRunId"])
            or not signoff["digest"].startswith("sha256:") or not SHA256.fullmatch(signoff["digest"][7:])):
        raise ValueError("manual sign-off candidate identifiers are malformed")
    evidence = signoff["evidenceSha256"]
    if not isinstance(evidence, str) or not SHA256.fullmatch(evidence):
        raise ValueError("evidenceSha256 must be lowercase SHA256 hexadecimal")
    missing = signoff["checksWithoutCandidateEvidence"]
    if (not isinstance(missing, list) or any(not isinstance(check, str) or check not in checks for check in missing)
            or len(missing) != len(set(missing))):
        raise ValueError("checksWithoutCandidateEvidence must list distinct checks for this version")
    if not ACTOR.fullmatch(actor) or not RUN_ID.fullmatch(run_id) or not COMMIT.fullmatch(release_head):
        raise ValueError("manual sign-off workflow identity is malformed")
    build_date = candidate.get("buildDate")
    utc_timestamp(build_date, "buildDate")
    return {
        **signoff,
        "schemaVersion": 1,
        "mode": "manual_signoff",
        "buildDate": build_date,
        "candidateProofSha256": proof_sha256,
        "approvedBy": actor,
        "releaseWorkflowRunId": run_id,
        "releaseControlCommit": release_head,
    }


def verify_release_source(commit, release_head):
    if commit == release_head:
        return
    ancestry = subprocess.run(["git", "merge-base", "--is-ancestor", commit, release_head], check=False)
    if ancestry.returncode:
        raise ValueError("the approved candidate must be an ancestor of current main")
    changed = subprocess.check_output(["git", "diff", "--name-only", "--no-renames", "-z", commit, release_head, "--"])
    if set(changed.split(b"\0")) - RELEASE_CONTROL_PATHS - {b""}:
        raise ValueError("current main has changes outside the release-control files")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", required=True)
    parser.add_argument("--signoff", required=True)
    parser.add_argument("--actor", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--release-head", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    candidate_raw, candidate = load_json(args.candidate)
    _, signoff = load_json(args.signoff)
    sealed = validate_signoff(candidate, signoff, hashlib.sha256(candidate_raw).hexdigest(),
                              args.actor, args.run_id, args.release_head)
    verify_release_source(sealed["commit"], args.release_head)
    write_json(args.output, sealed)


if __name__ == "__main__":
    try:
        main()
    except ValueError as error:
        sys.exit("manual sign-off rejected: " + str(error))
    except (OSError, subprocess.CalledProcessError):
        sys.exit("manual sign-off rejected: could not verify candidate source or evidence files")
