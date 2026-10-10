# Maintaining the HumanWill English edition

The current project is an English localization of `jiwoochris/artex-ko`, itself derived from `Autumn-27/ARTEX`. The [Korean maintainer guide](MAINTAINING.md) and [its historical English translation](MAINTAINING.ko-edition.en.md) remain available. Their release procedures and validation claims describe the source edition, not this English edition.

## Source lineage and remotes

The imported Korean source revision is `7333fa47b9bdd89b4973992b2e15d9c393539b8c`. Upstream history, authorship, module identifiers, and licenses are retained. Use the [handoff](docs/LOCALIZATION_HANDOFF.md) as the record of subsequent changes.

Expected checkout remotes:

- `origin`: `humanwill-ai/artex-english`.
- `upstream`: `jiwoochris/artex-ko`, with its push URL disabled in the prepared checkout.
- Original project lineage: `Autumn-27/ARTEX`; its availability is not assumed.

Inspect `git remote -v` on a new checkout. Do not point this project at the private research repository or copy research data into it.

## Upstream review

Fetch and inspect changes before deciding which to integrate. Separate functional changes, application display text, model-facing instructions, protocol identifiers, and third-party materials. Preserve provenance and licenses when applying a change. Do not automatically merge, push, or adopt a new upstream prompt policy as part of translation.

Record the source revision and the exact scope imported in the handoff. The old Korean-edition tracking base in the historical maintainer guide is not the current HumanWill import base.

## Translation maintenance

English lives in `web/messages/en.json`; Korean and Chinese catalogs remain selectable interface locales. Preserve identical message structure and interpolation/ICU contracts across all three. English is the default; `NEXT_PUBLIC_LOCALE=ko` or `zh` changes the interface at build time. The active login guidance and acknowledgement were updated together in all three catalogs. Former source notices remain in labeled historical guides, and the boundary manifest retains original and reviewed catalog hashes.

Application-owned behavioral instructions and runtime context use English. Preserve their meaning and all authorization/scope restrictions. Keep customized prompts, commands, evidence, and wire values unchanged. Preserve original audit content and policy decisions; legacy language matchers remain compatibility data rather than English output. Any migration of stored defaults must recognize an exact pristine default and preserve customized records and version history.

When backend message text changes, search for consumers that compare, parse, prefix-match, or branch on the old wording. Prefer stable machine-readable values; when retaining string-based compatibility, cover old and new messages in tests. In particular, approval summaries, skill-overwrite errors, archive errors, and mention tokens have parsing consumers.

## Validation and reporting

Use the commands and database isolation guidance in [Contributing](CONTRIBUTING.en.md#development-environment). Run focused checks while changing code and broader checks during integration. Keep these results separate:

- Catalog keys, ICU syntax, arguments, and rich-text tags.
- Builds and functional tests.
- PostgreSQL integration checks, including skipped tests.
- Browser inspection with synthetic data.
- Windows and Docker runtime checks.
- Actual model-output evaluation and security-task performance.

Standard synthetic workflow fixtures use English. Retain earlier multilingual fixtures and scores as separately labeled robustness diagnostics; multilingual interpretation is not a release guarantee. Record whether each issue comes from an upstream test, a newly authored fixture, or an actual use case, and whether it is a machine-check failure or a qualitative observation.

A passing catalog check or build does not establish that all runtime screens are English. A model-output language check does not establish unchanged task performance. Paid model calls and remote testing require task-specific authorization.

Record failures honestly, including dependency availability, machine resource limits, or pre-existing lint debt. Do not copy upstream badge status or historical benchmark claims into current validation results.

## Updates, containers, and releases

The `v0.1.0-alpha.1` HumanWill release is source-only, with no prebuilt application image. The updater targets stable releases from `humanwill-ai/artex-english` only and must not fall back to upstream binaries. The prebuilt Docker options in the install/update scripts fail before deployment; source builds remain available.

Compose uses `humanwill-artex-en:local` with `pull_policy: never`. Follow the [local image instructions](README.md#local-docker-image) to prepare the Linux binary and build that image. Preserve the distinction between a locally built image and a published artifact.

Review workflow triggers, repository destinations, Docker namespaces, secrets, package contents, license notices, and permissions before enabling release automation. Do not enable workflows, push release tags, publish images, or change repository visibility merely to test a localization change.

When checking GitHub state, specify `humanwill-ai/artex-english` explicitly so upstream results are not mistaken for this repository’s results. No GitHub action is needed for local documentation or translation validation.

## Historical materials

The retained Korean and Chinese guides, source-language screenshots, and `*.ko-edition.en.md` documents preserve source context. They may mention old upstream images, old release workflows, Korean-only output, or past availability checks. Use the current README and handoff for the English edition’s setup and status.

Preserve all original copyright and license notices. The root AGPL-3.0 license and existing frontend MIT notice remain in their respective files.
