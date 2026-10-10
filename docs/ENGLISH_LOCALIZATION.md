# English localization boundaries

This document describes the HumanWill English edition’s intended behavior and verification boundaries. Consult [LOCALIZATION_HANDOFF.md](LOCALIZATION_HANDOFF.md) for completed checks and outstanding work.

## English-first scope

The English edition includes application-owned internal instructions. Interface labels, errors, help, reports, notifications, operator output, core agent prompts, tool-description prose, runtime context instructions, and bundled skill prose use English. The interface defaults to `en`; `NEXT_PUBLIC_LOCALE` can select the retained `ko` or `zh` interface catalogs at build time. That switch does not change backend prompts or the English output directive.

Translation preserves behavioral requirements: authorization boundaries, interception decisions, evidence requirements, limits, and tool contracts. English-only development fixtures exercise the normal release path. They are assistant-authored synthetic examples, not tests inherited from upstream. Earlier mixed-language fixtures remain unchanged as optional historical robustness diagnostics; passing them is not a requirement for the English-only release.

## What remains unchanged

- Tool and API names, schema keys/types/enums/defaults, database identifiers, and machine-readable tokens.
- Commands, payloads, paths, URLs, requests, responses, and original external evidence.
- User-written content, customized prompts/settings, and historical findings.
- Source attribution, Git history, copyright notices, licenses, and archived source-language documentation/catalogs.

English-only describes the application-owned experience and supported evaluation scope. It does not filter or rewrite real input. Raw evidence can contain any language and must remain exact; this release does not promise reliable interpretation of every non-English input. Legacy detection strings and old built-in defaults remain only where needed for compatibility and safe upgrades.

## Stored defaults and upgrades

Fresh installations seed English defaults. An existing built-in prompt is migrated only when its current text exactly matches a known original, its version and author/note identify the original system seed, and its complete history contains only that one version. The migration appends an English version and preserves the original history. Previously edited, reseeded, rolled-back, or otherwise unmatched records remain untouched; an operator can review and explicitly reset those to the English defaults.

Reporter and retester defaults require additional checks of their known seeded identities. Built-in tool metadata migrates only when both the description and complete schema match a captured original system default. Only descriptive prose can change; schema contracts, bindings, enablement, and execution settings are preserved. Customized records are not automatically translated. Repeating startup must not create duplicate migrations.

## Verification

| Change | Verification |
| --- | --- |
| Interface catalog | Key parity, ICU argument/branch/tag parity, and browser inspection |
| English internal instructions | Review against original meaning; template/format placeholders and control-flow checks |
| Tool prose | Schema contract comparison excluding only descriptive text |
| Interception explanation | English and supported legacy audit formats; unchanged verdict enums and required fields |
| Stored defaults | Fresh/pristine/custom/history cases, repeatability, and setting preservation with a local database |
| Evidence and reports | Exact raw-content and identifier preservation, rendering/escaping checks |
| Standard model checks | English synthetic fixtures, separate deterministic and qualitative results, recorded prompt/request hashes |

The source-language regression checker uses explicit compatibility exceptions. A character scan alone cannot establish fluency, semantic equivalence, or safe behavior. Model-output checks do not establish full orchestration or security-task performance. Historical model runs predate the English core-prompt translation and must not be presented as validation of the new prompts.

The [contribution guide](../CONTRIBUTING.en.md) and [maintainer guide](../MAINTAINING.en.md) explain the implementation workflow. Publication and release decisions remain separate from implementation.
