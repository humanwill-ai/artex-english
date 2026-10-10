# ARTEX — HumanWill English edition

An English localization of [jiwoochris/artex-ko](https://github.com/jiwoochris/artex-ko), derived from [Autumn-27/ARTEX](https://github.com/Autumn-27/ARTEX).

English · [Korean edition archive](README.ko.md) · [Original Chinese guide](README.zh.md)

ARTEX coordinates LLM agents that plan security investigations, execute tools, record assets and findings, and produce reports. A Go backend serves the Next.js interface and stores application data in PostgreSQL. The framework retains its upstream module identity and history.

This HumanWill edition has an **experimental source-only prerelease**, `v0.1.0-alpha.1`. It has no prebuilt binaries or container images. Build from source and review the [release notes](docs/PREVIEW_RELEASE_NOTES.md) for known limitations and the [localization handoff](docs/LOCALIZATION_HANDOFF.md) for development evidence; neither establishes production readiness.

## English localization

English is the default interface language. This includes navigation, configuration labels, application-owned errors, report headings, notification templates, and operator messages. The agent output-language directive requests English findings, summaries, reports, and chat replies.

Application-owned core instructions, tool descriptions, runtime messages, and bundled skill prose are translated into English while preserving their behavioral requirements and technical contracts. Commands, payloads, URLs, and raw evidence remain exact. Existing customized prompts, user-written content, and historical findings are not automatically translated. Only recognized, untouched built-in defaults receive the English upgrade. Standard synthetic evaluation cases use English; reliable interpretation of other languages is outside the release claim. Translation does not establish identical model behavior or guarantee English output.

The `en`, `ko`, and `zh` interface catalogs remain available. Choose an interface locale at build time:

```bash
cd web
NEXT_PUBLIC_LOCALE=en npm run build:static
```

Use `ko` or `zh` instead for a source-language interface. This selects interface messages and display formatting, not the backend agent output directive, which requests English in this edition. Dates use `en-GB` formatting by default without changing stored timestamps or the browser’s timezone.

See [localization boundaries and validation](docs/ENGLISH_LOCALIZATION.md).

## ⚠️ Read first — authorized use and legal notice

The current HumanWill [responsible-use guidance](web/messages/en.json) supports lawful research, defensive assessment, and authorized penetration testing. For a live assessment, work only on systems you own and have authority to test, or where the appropriate owner has expressly permitted the work within an agreed scope. Agree on targets, methods, timing, and data handling before starting. An ARTEX target entry, scope setting, or approval control does not itself grant permission. Unlawful intrusion, data theft, extortion, and unlawful disruption are unacceptable.

Because this alpha has unresolved finding/report and unavailable-service completion defects, we recommend starting with an isolated synthetic lab and reviewing every generated action, finding, and completion status before relying on it. This is a reliability recommendation, not an additional software-license condition. The login notice is HumanWill's updated guidance; the [Korean-edition notices](README.ko-edition.en.md#license-and-disclaimer) and [defense guide](docs/defense-en.md) are retained as historical source context, not this edition's current use policy. The [AGPL-3.0 license](LICENSE), [frontend MIT notice](web/LICENSE), attribution, and no-warranty text remain in place.

## Build and run from source

Prerequisites: Go **1.26.3 or later** (`go.mod`), Node.js **22 or later** (use a version supporting the project’s test commands), npm, and a reachable PostgreSQL database. The build script also uses `rsync`. Network access is needed to obtain uncached dependencies. External security tools and MCP servers require their own local installation or configuration.

For a checkout you are authorized to access:

```bash
git clone https://github.com/humanwill-ai/artex-english.git
cd artex-english
```

Create `config.json` with a database and role that already exist, or set `ARTEX_PG_DSN` instead:

```json
{
  "database": {
    "host": "127.0.0.1",
    "port": 5432,
    "user": "artex",
    "password": "replace-with-your-local-password",
    "dbname": "artex",
    "sslmode": "disable"
  }
}
```

The example is for a local database. Choose connection and TLS settings appropriate to your database. Application startup creates or migrates its schema and attempts to create a missing database when the configured role has permission. Provide the PostgreSQL service and login role yourself; creating the database explicitly makes setup predictable.

From the repository root:

```bash
ARTEX_OUTPUT=./artex ./build.sh --no-compress
./start.sh
```

The build script installs frontend dependencies, exports the interface, copies it into the embed directory, and builds the binary. If your environment sets `NODE_ENV=production`, install frontend development dependencies with `npm ci --include=dev` in `web/` first and use `ARTEX_SKIP_NPM_CI=1` for the build.

Open `http://localhost:8787`. On first use, set the administrator password at `/setup`. The username is `ARTEX`. Configure an LLM profile in the interface before starting an agent. Provider calls may incur charges; a UI preview does not require a model call.

`start.sh` supervises the executable; Windows packages use `start.bat`. Building a Windows executable and running it on Windows is a separate platform validation step. The interactive `install.sh` also offers local compilation as option **2**. Its prebuilt Docker option is unavailable until an English image is published.

### Configuration

Database settings can be overridden with `ARTEX_PG_DSN`. LLM configuration can be supplied in the interface or through `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` and optional `ARTEX_LLM_PROVIDER`, `ARTEX_LLM_MODEL`, `ARTEX_LLM_BASE_URL`, and `ARTEX_LLM_PROXY` variables.

Worker concurrency, traffic capture, proxies, interception rules, and approvals are managed in the interface. Localization does not disable these controls. The web/API port defaults to `8787`; the recording proxy defaults to `8788`.

### Model selection and output language

English output is requested through a code-owned directive appended after the system prompt body. It is an instruction to the model, not a language filter. Models, custom prompts, output limits, and context can affect results. Validate generated output separately from deterministic interface and prompt-assembly tests; retain technical evidence verbatim even when it is not English.

### Local Docker image

`docker-compose.yml` uses **`humanwill-artex-en:local`** with `pull_policy: never`. Build that image yourself; Compose will not download an upstream application image. The Dockerfile packages a precompiled Linux binary rather than compiling the application inside the image.

For an amd64 image, from the repository root:

```bash
ARTEX_OUTPUT=dist/amd64/artex ./build.sh --target linux/amd64 --no-compress
docker build --platform linux/amd64 --build-arg TARGETARCH=amd64 -t humanwill-artex-en:local .
cp .env.example .env
# Edit .env and replace POSTGRES_PASSWORD before continuing.
docker compose up -d
```

For arm64, use `linux/arm64`, `dist/arm64/artex`, and `TARGETARCH=arm64` consistently. Docker and the external runtime toolchain still require network downloads. Compose starts PostgreSQL, persists it in the `pgdata` volume, and mounts `./data` and `./skills`. These are setup instructions, not a report of a completed Docker validation.

### Updates

The self-updater checks only stable releases from `humanwill-ai/artex-english`; it does not offer this source-only prerelease, and there is **no upstream release fallback**. The prebuilt Docker paths in `install.sh` and `update.sh` stop before deployment. Review incoming source changes and rebuild locally. `update.sh` option **2** offers a source update and rebuild.

GitHub Actions remain disabled for this source-only prerelease. See the [release verification record](docs/PREVIEW_RELEASE_CHECKLIST.md) for completed checks and future gates.

## Development

Install frontend dependencies in `web/` with `npm ci --include=dev`. With database settings configured, run the backend and frontend in separate terminals:

```bash
# Repository root
go run ./cmd/artex -addr :8787 -proxy 127.0.0.1:8788
```

```bash
# Second terminal
cd web
npm run dev -- --port 5173
```

The frontend proxies `/api` to the backend. A UI-only synthetic preview requires neither a database nor an LLM:

```bash
cd web
NEXT_PUBLIC_MOCK=1 npm run dev -- --port 5173
```

The mock preview illustrates the interface; it does not validate backend execution. For commands and test boundaries, see [Contributing](CONTRIBUTING.en.md) and [Maintaining](MAINTAINING.en.md).

## System architecture

- **Planner:** reads task state and creates investigation intents.
- **Workers:** execute one intent at a time and record results.
- **Main/chat agents:** provide the human interaction layer; the reporter writes finding reports.
- **Asset graph:** stores domains, IPs, applications, services, and endpoints across tasks.
- **Investigation graph:** records task goals, intents, facts, findings, and their relationships.
- **Supporting services:** traffic recording, interception and approval controls, notifications, MCP integrations, and skills.

The [inherited architecture walkthrough](README.ko-edition.en.md#system-architecture) explains these components in more detail. Its Korean-edition validation claims do not establish results for this English edition.

## License and attribution

The root [LICENSE](LICENSE) remains **AGPL-3.0**. The existing [web/LICENSE](web/LICENSE) MIT notice, including its copyright attribution, is retained for the frontend material it covers. Other upstream and third-party notices remain in place. This localization does not relicense those materials.

Source lineage: **Autumn-27/ARTEX → jiwoochris/artex-ko → humanwill-ai/artex-english**. The upstream Go module path and dependency identifiers remain unchanged. Repository privacy and GitHub’s fork label do not replace the applicable license terms.

Original language documents and the [historical English guide to the Korean edition](README.ko-edition.en.md) preserve the inherited notices and context. For current preparation status, use [the handoff](docs/LOCALIZATION_HANDOFF.md).
