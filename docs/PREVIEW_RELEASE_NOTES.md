# ARTEX English edition v0.1.0-alpha.1 preview notes

**Draft for owner review. No public release has been made.** This candidate is an experimental, source-only English localization of ARTEX for code review and non-destructive verification in a locally isolated synthetic lab. It is not validated for production use or live penetration testing. No prebuilt binaries or container images are part of this candidate.

## What is included

- English is the default interface language. Application-owned messages, core agent instructions, tool descriptions, bundled skill prose, and output-language instructions use English. Korean and Chinese interface catalogs and historical source documents remain available.
- The Go backend, Next.js interface, PostgreSQL storage, agent orchestration, reporting, traffic recording, and existing approval and scope controls are included as source. The original project and Korean edition history remain attributed.
- The updater checks HumanWill releases only. Prebuilt Docker install/update paths stop before deployment; a local image can be built from source.
- The selected code commit built from source on macOS Intel with Go 1.27.1 and Node 26.0.0; TypeScript and 31 static pages completed, and the embedded binary returned help successfully. This is a build smoke check, not a test of agent tasks, database operation, or another platform.

## Known limitations

- English output is requested from the model, not enforced. Generated text, technical evidence, custom prompts, and historical records can contain other languages. This preview makes no claim of task-performance or language parity with either source edition.
- In the latest isolated six-case synthetic comparison, both editions failed the requested positive finding, bound-traffic, current-report, and completed-goal workflow. Both unavailable-service HTTP 503 cases ended marked done with goals met despite missing evidence; they should have remained inconclusive. These defects remain unresolved. The two HTTP 401 negative cases completed with supported evidence and no findings. The repaired experiment launcher worked, but its repair did not fix the application behavior. See the [evaluation detail](MODEL_OUTPUT_EVALUATION.md) and [localization handoff](LOCALIZATION_HANDOFF.md) for scope and earlier results.
- Windows execution, Docker deployment, and the target CI platform matrix are pending separate checks. A local build or synthetic pilot cannot establish safe behavior on a real target.
- The locked frontend dependency tree reports 24 npm advisories, including two critical entries (`next` and transitive `proxy-addr`); the count remains 24 when development dependencies are omitted. No dependency upgrade is included in this candidate. Review the advisories and affected paths before any broader deployment claim.
- The inherited login terms restrict use to local isolated learning and verification and prohibit actual penetration tests and production use. Some older documents discuss authorized live testing. This preview follows the narrower scope described in the [README](../README.md#️-read-first--authorized-use-and-legal-notice); the inherited legal text and attribution remain intact. The maintainers must decide how to reconcile these notices before widening the stated use scope.

## Build and use

Follow the [source-build instructions](../README.md#build-and-run-from-source). Use synthetic data and a lab you built and control. Review the [root AGPL-3.0 license](../LICENSE), the [frontend MIT notice](../web/LICENSE), and the retained [Korean-edition notices](../README.ko-edition.en.md#license-and-disclaimer). This draft does not replace any of them.
