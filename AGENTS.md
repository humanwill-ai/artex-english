# Working on HumanWill ARTEX English localization

This is the HumanWill ARTEX English edition, separate from other research and benchmark repositories.
Read `docs/LOCALIZATION_HANDOFF.md` before implementation and `MAINTAINING.en.md` for the inherited localization boundaries.

## Scope

- Build an English edition of `jiwoochris/artex-ko`, derived from `Autumn-27/ARTEX`.
- Publication, deployment, and release automation require explicit maintainer approval.
- Preserve upstream Git history, authorship, license text, notices, and source identifiers. Do not import private research, evaluation data, model responses, or credentials.
- The English edition includes application-owned internal prompts, tool-description prose, runtime instructions, and standard synthetic fixtures. Translate inherited Chinese/Korean instructions faithfully while preserving their behavioral requirements, authorization/scope controls, tool contracts, commands, payloads, evidence, URLs, and raw external logs.
- Keep historical evaluation fixtures/results intact and label multilingual cases as optional robustness diagnostics, not English-release requirements. Identify newly authored tests and distinguish automated failures from qualitative observations and unconfirmed causes.
- The inherited Korean contribution policy describes the source edition. For this edition, English is the requested user-facing language; retain Korean and Chinese catalogs as references.
- Do not weaken authorization, interception, approval, or scope controls as part of translation.

## Workflow

- Inspect Git status before editing; preserve unrelated work. Use `codex/` branches for implementation.
- Use translation keys for interface text. Preserve message keys, placeholders, ICU formatting, and HTML structure.
- Separate text shown to users from model-facing instructions when one string serves both purposes. Review those cases individually.
- Preserve API values, database identifiers, severity codes, and serialized enums even when their display labels change.
- Record validation and unresolved work in `docs/LOCALIZATION_HANDOFF.md` after substantial sessions.
- Test localization through builds, relevant existing tests, and visual inspection. Use synthetic local data for previews. Do not run remote security tests or paid model calls without task-specific authorization.
- Follow repository review and release procedures before merging, pushing release tags, or publishing artifacts.
