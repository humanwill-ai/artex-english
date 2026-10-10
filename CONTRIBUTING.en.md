# Contributing to the HumanWill English edition

This is the private `humanwill-ai/artex-en` project. Read [AGENTS.md](AGENTS.md), the [localization boundaries](docs/ENGLISH_LOCALIZATION.md), and the [current handoff](docs/LOCALIZATION_HANDOFF.md) before changing it. The [Korean contribution guide](CONTRIBUTING.md) and [its historical English translation](CONTRIBUTING.ko-edition.en.md) describe the source edition.

## Authorized use and legal responsibility

Use synthetic fixtures or an isolated environment you control for development. Do not run remote security tests or paid model calls without task-specific authorization. Keep secrets, real target evidence, and private research out of fixtures, screenshots, logs, and repository content. The inherited [usage notices](README.ko-edition.en.md#license-and-disclaimer) and [code of conduct](CODE_OF_CONDUCT.en.md) remain available.

## Localization policy

Translate application-owned interface text, errors, reports, notifications, and operator messages into English. Preserve message keys, interpolation names, ICU branches and rich-text tags, API fields, enums, database identifiers, commands, paths, URLs, payloads, and raw evidence.

Core Chinese behavioral instructions and source policies stay unchanged in this pass. The output-language directive requests English separately. Review strings that enter both the interface and model context individually. Prefer an additive display field when translating a shared record; do not overwrite its original audit or policy text.

Keep `web/messages/en.json`, `ko.json`, and `zh.json` structurally aligned. Use translation keys in components. Keep existing custom prompts, names, settings, and historical content intact. Deterministic tests can verify assembly and data preservation, but they cannot prove unchanged model behavior.

## Development environment

See [source setup](README.md#build-and-run-from-source) for the database, Go 1.26.3+, Node.js, and local build procedure. Read relevant subsystem design documents before changing behavior, including the [traffic-evidence design](docs/finding-traffic-evidence-en.md) when applicable.

### Backend

From the repository root:

```bash
go build ./...
go vet ./...
go test ./... -count=1
```

Run focused package checks during implementation, then the required broader checks for integration. Record failures and skips accurately. A successful package result with skipped database tests is not a database integration pass.

### Database integration

The inherited CI configuration in [ci.yml](.github/workflows/ci.yml) identifies database-dependent packages: `agent`, `config`, `db`, `evidence`, `llmrec`, and `server`. Each package needs an isolated disposable PostgreSQL database; tests can modify or delete data. Never point them at an application or production database.

With a fresh test database and suitable role already provisioned, set its DSN and run one package, for example:

```bash
ARTEX_PG_DSN='postgres://artex:local-test-password@127.0.0.1:5432/artex_test_agent?sslmode=disable' \
  go test ./agent/ -count=1 -v
```

Repeat for each affected package with its own fresh database and inspect skipped-test output. Workflow definitions describe how checks are intended to run; GitHub Actions are disabled during private preparation.

### Frontend

```bash
cd web
npm ci --include=dev
npm test
npm run check:i18n
npm run build:static
npm run check
```

`check:i18n` checks catalog completeness and formatting contracts. The static build validates the application build and TypeScript integration. Record any existing formatting/lint debt separately instead of reporting the entire check as passed. Use the [mock preview](README.md#development) for browser inspection without a backend or model call.

Check setup, login, dashboard, tasks, assets, findings, traffic, reports, and settings. Include loaded dialogs, empty states, and failure messages; a static initial page alone does not establish translation coverage.

### Documentation and scripts

```bash
python3 -I scripts/check-doc-links.py
```

The documentation checker examines tracked Markdown. Also inspect newly added documents and their local links before staging. For shell edits, run `bash -n` separately for each affected script. Syntax validation does not validate Windows execution, Docker deployment, or database writes.

## Contribution process

1. Inspect the working tree and preserve unrelated changes. Use a `codex/` branch for this localization work.
2. Make a bounded change and add relevant behavioral coverage. Keep translation work separate from speculative refactoring.
3. Record what changed, actual validation results, skipped checks, and remaining work in the handoff.
4. Obtain explicit authorization before committing or pushing implementation changes. Publication, release tags, deployment, and workflow enablement require separate authorization.

Do not send repository data or notifications to upstream projects as part of this workflow. Discuss security findings privately with the HumanWill repository owner; inherited reporting links may point to the Korean project.

## Commit messages

When a commit is authorized, use a concise English description such as `i18n(web): add English task labels`. Preserve upstream authorship and history. No commit or publication is implied by preparing changes locally.

## Contributing detection rules and detection tests

Detection material under [detections/](detections/) remains attributed to its source. Localization should not alter protocol indicators, User-Agents, default ports, audit markers, or rule identifiers. If a functional change affects an indicator, update the corresponding rules and run the relevant suites from [detections/tests/](detections/tests/).

The [historical detection contribution contract](CONTRIBUTING.ko-edition.en.md#contributing-detection-rules-and-detection-tests) documents grounding requirements, reproducible fixtures, and the test runners. Existing CI badges or historical test claims are not validation evidence for a new change.

## License

Retain the root [AGPL-3.0 license](LICENSE), the existing [MIT frontend notice](web/LICENSE), and all applicable third-party attribution. Contributions must respect the applicable license scope; do not replace upstream notices or treat localization as relicensing.
