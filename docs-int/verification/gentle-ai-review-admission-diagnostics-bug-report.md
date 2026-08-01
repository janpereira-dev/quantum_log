# Bug Report Draft: Incomplete Review Admission Has No Payload Diagnostics

## Summary

Frozen M4 review lineage `review-eafa95efa16a8d47` cannot admit any of four reviewer captures for candidate `sha256:eafa95efa16a8d47751a8678aaa8e54f9bee6734d314b65ed999586ebb0214c7`. Each capture was repeatedly rejected as `incomplete`, including after three OpenCode restarts. Current inspection surfaces no rejected payload or admission diagnostic, preventing distinction between malformed reviewer output and a native admission failure.

## Environment And Evidence

- Gentle AI review flow: live OpenCode reviewer prompts verified against v2.2.2 strict result schema.
- Review lineage: `review-eafa95efa16a8d47`.
- Candidate target: `sha256:eafa95efa16a8d47751a8678aaa8e54f9bee6734d314b65ed999586ebb0214c7`.
- Four reviewer captures: repeatedly rejected as `incomplete`.
- OpenCode restarts: three.
- Plugin behavior: raw result is preserved under a `rinc1_*` identifier, but no retrieval path is available.
- Diagnostic behavior: `review inspect-authority` exposes no rejected-payload details or admission-failure diagnostics.

## Expected Behavior

When a reviewer result is rejected as `incomplete`, authorized local inspection should expose enough sanitized diagnostics to identify whether:

1. Reviewer payload was malformed or failed schema validation.
2. Payload was valid but failed native admission for another reason.

At minimum, diagnostics should identify rejection stage, machine-readable reason, schema-validation errors when applicable, and a retrievable sanitized rejected-result artifact keyed by its `rinc1_*` identifier.

## Reproduction Steps

1. Start or resume frozen review lineage `review-eafa95efa16a8d47` for target `sha256:eafa95efa16a8d47751a8678aaa8e54f9bee6734d314b65ed999586ebb0214c7`.
2. Submit each of four reviewer captures using live OpenCode reviewer prompts conforming to v2.2.2 strict result schema.
3. Observe each capture rejected as `incomplete`.
4. Restart OpenCode and repeat capture attempts; this occurred across three restarts.
5. Run `review inspect-authority`.
6. Observe no rejected raw payload, `rinc1_*` retrieval path, rejection stage, or native admission diagnostic.

## Actual Behavior

Review state reports `incomplete`, while available inspection does not reveal why. Although plugin handling preserves a raw result under `rinc1_*`, operators cannot retrieve it. This leaves no evidence path to determine whether remediation belongs in reviewer prompt/payload generation or native result admission.

## Impact

High-risk frozen review cannot progress with confidence. Retrying reviewer captures can repeat without producing new diagnostic evidence, while manually declaring success would be unsafe. Teams cannot isolate fault ownership, prepare a minimal fix, or validate that a retry addresses root cause.

## Proposed Fix Direction

Add an authorized, local-only rejected-result inspection path. Preserve privacy controls and avoid exposing secrets or full unredacted content. Return sanitized structured fields such as result identifier, review lineage, admission stage, rejection code, schema errors, timestamp, and actionable next step. Extend `review inspect-authority` or add a dedicated inspect command to retrieve the matching sanitized `rinc1_*` artifact.

## Scope And Non-Goals

Scope: diagnostic visibility for rejected reviewer results in this lineage class.

Non-goals: changing reviewer prompts, weakening v2.2.2 schema validation, admitting rejected captures, altering M4 code or review state, retrying reviewers automatically, or exposing raw sensitive reviewer content.
