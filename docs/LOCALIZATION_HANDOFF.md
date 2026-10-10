# ARTEX English edition: implementation and validation record

This public-facing record describes the source edition in this candidate and its experimental source-only preview. It is a summary of completed work and observed limitations, not a claim that security workflows are reliable on real targets.

## Provenance and scope

The English edition derives from `jiwoochris/artex-ko` at `7333fa47b9bdd89b4973992b2e15d9c393539b8c`, itself derived from `Autumn-27/ARTEX`. The imported upstream history, authorship, module identifiers, root AGPL-3.0 text, and frontend MIT notice are retained. English is the default interface language; the Korean and Chinese catalogs remain selectable. Their current login guidance was updated together while the earlier source notices remain in archived documents. Application-owned prompts, tool descriptions, runtime instructions, server messages and synthetic examples were translated while retaining authorization and scope controls, tool contracts, commands, payloads, and recorded evidence.

The edition also includes a read-only task export and structured recording provenance for independent offline analysis. The recording path is explicitly partial; it does not guarantee complete capture or model/tool correlation for every source. Exporting a task does not invoke the normal archive lifecycle. This source feature runs without an external analysis service.

## Completed local validation

The interface catalogs passed key, ICU, argument, rich-text, and untranslated-application-text checks. Frontend production and synthetic-mock builds, TypeScript checks, frontend tests, Go build and vet, and the isolated PostgreSQL suites passed during implementation. The source edition was also compiled with an embedded frontend and passed a basic `-h` startup check at that earlier stage. Some local checks used macOS Intel, Go 1.27.1, Node 26.0.0 and PostgreSQL 14.2 or 16; they do not establish the repository CI matrix on Go 1.26 and Node 22. Windows execution and Docker installation remain unverified. See the preview notes for the current clean-source build result and dependency advisories.

Synthetic model-output checks used newly authored cases and offline grading. The earlier prompts and results remain historical diagnostics rather than a release gate or a broad security score. Across the completed synthetic comparison and follow-ups, 947 model calls were recorded at USD 0.272153795 reported inference cost. Raw responses, per-output reviews, ledgers and full environment artifacts are retained outside this source repository. The fixture sets and offline harness remain in `scripts/` so their contracts can be inspected without making model calls. Details and interpretation are in [MODEL_OUTPUT_EVALUATION.md](MODEL_OUTPUT_EVALUATION.md).

A separate isolated production-path pilot ran 18 synthetic scenarios/repeats across the English and patched source editions. Both editions observed a test disclosure in all positive cases, but neither reliably completed the requested finding, bound traffic and current-report workflow. The patched 401 cases completed without false findings. The unavailable 503 cases timed out in the 18-run pilot. A later six-case follow-up confirmed both editions still failed the positive workflow and incorrectly marked unavailable cases done with goals met. These failures remain unresolved; one sample per follow-up scenario cannot establish localization causation or performance parity. See [PRIVATE_PILOT.md](PRIVATE_PILOT.md) and [MODEL_OUTPUT_EVALUATION.md](MODEL_OUTPUT_EVALUATION.md).

A later recording/export integration test used an isolated PostgreSQL instance, a scripted local provider and a synthetic fixture-read tool. It exported a 14-event task, which an offline analyzer imported and displayed. This establishes that one synthetic path works; recording coverage remains partial.

## Preview boundary and next work

This source-only alpha supports lawful research and authorized assessment within the appropriate owner’s agreed scope. Because the application has unresolved workflow defects, begin with a locally isolated synthetic lab and review generated actions, evidence, reports, and completion status. The active HumanWill responsible-use guidance is described in the [README](../README.md); earlier source-edition notices remain archived for attribution and comparison.

Priorities before broader release are the unresolved completion/evidence workflow, dependency advisories, target CI versions, Windows and Docker validation, and review of the updated responsible-use guidance. GitHub release automation remains disabled for this source-only prerelease; the release workflow's tag trigger, image destination, permissions, secrets and package contents require separate review before activation.
