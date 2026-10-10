---
name: api-recon
description: Use this skill when collecting a website's API endpoints.
---

# API Recon (Frontend API Reconnaissance)

With **authorization**, discover as completely as possible: **backend APIs** (paths, methods, parameters, response bodies), **frontend routes**, and **UI feature triggers** (tabs, dialogs, table actions, etc.).

---

## Boundaries and prohibitions (required reading)

This skill covers **API and parameter-surface reconnaissance only**, not vulnerability discovery or exploitation.

### Task boundaries

| Area | Allowed | Prohibited |
| --- | --- | --- |
| **Objective** | Enumerate paths, methods, parameters, routes, UI triggers | SQLi/XSS/access-control attacks/brute force/vulnerability fuzzing, request-tampering attacks, destructive operations |
| **Authentication** | Hooks and stubs/mocks to bypass **client-side** login gates | Asking users for or guessing credentials; attempting real login form submissions |
| **Runtime** | Hook endpoints without credentials and use mock responses to render the post-login SPA shell | Workflows requiring a real backend session to continue |

### Dynamic analysis without credentials (Phase 3 default)

1. Use `preload.js` / `runtime_harvest.js` to **intercept and stub** login, permission, menu, and other bootstrap endpoints.
2. Return mock bodies for business-query endpoints with **correct structure, successful application status codes, and optionally empty data**.
3. Let the frontend render post-login pages even without a backend or with 401 responses, triggering more XHR/fetch/WebSocket requests.
4. **Empty data, tables, and placeholder UI are expected**; do not switch to real login or vulnerability testing because of them.

Use mocks to expose frontend routes and mount components, **recording outbound requests only**. The purpose is to discover which additional endpoints the frontend requests, rather than obtain real backend data.

### Strict workflow prohibitions

| Prohibited | Alternative |
| --- | --- |
| Before Phase 1 finishes, grep/curl/Read the main `index-*.js` entry to extract API paths | Run `OUTDIR/harvest_static.py` |
| Write a replacement such as `extract_apis.py` | Modify and rerun `OUTDIR/harvest_static.py` |
| Repeat the same failed grep/command two or more times | Change strategy: read tool_logs, modify harvest, consult the reference |
| Skip gates A/B and run the originals in `scripts/` | Copy to OUTDIR and adapt to the target |
| Real username/password, OTP, OAuth, or other authentication | Use stubs/mocks as above |
| Skip stubs and test access-control bypass/injection to obtain real data | Record outbound requests only, within recon scope |
| Irreversible operations such as deletion, sensitive-data export, or bulk writes | The same prohibition applies to coverage clicks |
| Claim all pages/endpoints are found without runtime and dynamic enumeration | Satisfy the completion criteria or state limitations |
| Claim all parameters are known without a trigger matrix and diff | Phase 3b matrix and Phase 5 diff |
| Infer required/optional fields from one runtime sample | Compare multiple samples or infer from validation rules/errors |

---

## Two layers and execution modes

| Layer | Output | Limitations |
| --- | --- | --- |
| **Static** (JS bundles) | Endpoint paths, draft routes, candidate fields at payload construction sites | No HTTP methods; parameters need Phase 1b; runtime-built URLs may be missed |
| **Runtime** (active session) | Methods, bodies, responses, dynamic URLs, WS/SSE; multi-sample diffs enrich parameters | Pages must render to issue requests; one sample cannot establish required/optional fields |

| Mode | Engine | Use |
| --- | --- | --- |
| **depth** | `runtime_harvest.js` (Puppeteer) | API lists, methods/parameters/responses, WS/SSE, reproducible batch runs |
| **coverage** | Browser + `preload.js` | Deeper feature coverage through tabs, dialogs, and tables |
| **both** | depth followed by coverage | Most complete, most time-consuming |

**Parameter method** (no universal script): use harvest/regex for paths, and **expanded context around anchors + UI binding chains + multi-sample diffs + error-based inference** for parameters. See section J of [reference.md](reference.md) for grep recipes.

---

## Completion criteria

Claim reconnaissance is complete only when all applicable items are satisfied:

- [ ] **Static**: Phase 1 harvest produced `api_static.txt`, `routes.txt`, and `js/`.
- [ ] **Runtime**: at least depth or coverage ran; coverage/both requires **working hooks and a dynamic enumeration loop**.
- [ ] **Shell entry**: business paths do not land on `/login` (account for hash routing).
- [ ] **Parameters**: coverage/both completed the trigger matrix and `param_samples.json`; Phase 5 produced `params_merged.json`.
- [ ] **Depth**, if module pages are blank: reconstruct the Phase 4 permission tree and rerun until **module-level APIs**, not only locale/bootstrap, appear.
- [ ] **Delivery**: all Phase 5 outputs exist (see its output table); `insert_assets` writes service and endpoint assets.

---

## Scripts and gates

`scripts/` contains reference templates only. **Do not run unchanged originals and treat them as final results.**

Read first → adapt to the target → write into `OUTDIR` (for example, `recon/`) → record `CHANGES.md`. If unsuitable, rewrite using the methodology and borrow only the structure.

| Gate | When | Reference script → OUTDIR copy | Common adjustments |
| --- | --- | --- | --- |
| **A (static)** | After Phase 0, before the **first** harvest/spider run | `harvest_static.py` / `spider_mpa.py` | Default regex works for most sites; only change endpoint regex, webpack/Vite `publicPath`, or MPA exclude/cookie when manifest/dialect differs |
| **B (runtime)** | After Phase 2, before depth/coverage | `runtime_harvest.js` / `preload.js` + `config.json` | Cookie/localStorage keys, neutralize success values, stubs, login regex, API prefix, hash/history |

**Mandatory SPA order** (not interchangeable; phase order takes precedence over “explore before scripting”):

| Step | Required | Prohibited |
| --- | --- | --- |
| After Phase 0 | Next Bash command: `python3 OUTDIR/harvest_static.py <URL> OUTDIR` | curl/grep/Read of main `index-*.js` entry, typically >500KB |
| Gate A | Copy script → small changes if needed → **run immediately** | Manually extract APIs before deciding whether to harvest |
| Before Phase 1 completes | Validate outputs with `wc -l`; modify harvest and retry 404s | Write extraction scripts; repeatedly grep URLs not downloaded |
| From Phase 1b | Grep only `OUTDIR/js/*.js` | Substitute the main bundle for harvest |

- Correct: copy `harvest_static.py` → optionally adjust regex → **run immediately**.
- Incorrect: curl the main bundle → repeated grep → temporary extraction script → harvest last.
- **MPA**: the next Bash command after Phase 0 is `python3 OUTDIR/spider_mpa.py ...`.

---

## Tool and output constraints

| Constraint | Rule |
| --- | --- |
| Large files | Do not Read/grep `index-*.js` files >100KB into context; batch-process with OUTDIR scripts |
| Grep output | Always use `\| head -20` or `-m 5`; retain only path summaries in conversation, not bundle excerpts |
| Validation | Use `wc -l` and `ls \| wc -l`; do not Read whole directories |
| Initial regex inspection | Optional, at most once, only on a small chunk ≤50KB or HTML; harvest is the authoritative static pass |
| Reference | See [reference.md](reference.md) for recipes/templates/troubleshooting; do not repeat it in full inline |

---

## Execution roadmap

```
Phase 0: classify + OUTDIR
  → Gate A → Phase 1 harvest (run immediately)
  → Phase 1b parameter reverse engineering
  → Phase 2 three authentication gates → config.json
  → Gate B → Phase 3 runtime + parameter matrix
  → Phase 4 permission tree if needed → rerun Phase 3
  → Phase 5 merged report + insert_assets: batch-insert all discovered service and API endpoint assets; omit none
```

Check items in order; **do not enter the next phase before completing the previous item**.

1. [ ] **Phase 0**: inspect SPA/MPA and create `OUTDIR` → [Phase 0](#phase-0--classification).
2. [ ] **Gate A + Phase 1**: copy script → **immediate** harvest → validate with `wc -l` → [Phase 1](#phase-1--static-analysis).
3. [ ] **Phase 1b**: expanded anchor context + binding layer → `param_candidates.json` → [Phase 1b](#phase-1b--parameter-reverse-engineering).
4. [ ] **Phase 2**: three authentication gates → `config.json` → [Phase 2](#phase-2--three-authentication-gates).
5. [ ] **Gate B**: adapt runtime scripts → [Phase 3](#phase-3--runtime).
6. [ ] **Phase 3**: depth / coverage / both; confirm shell entry; parameter trigger matrix → `param_samples.json`.
7. [ ] **Phase 4**, if needed: permission tree → patch stubs → rerun Phase 3 → [Phase 4](#phase-4--permission-tree-reconstruction).
8. [ ] **Phase 5**: merge outputs + report + `insert_assets` → [Phase 5](#phase-5--merge-and-report).

---

## Phase 0 — Classification

Fetch entry HTML and **create `OUTDIR`**; do not modify the skill's `scripts/`:

- **SPA**: empty shell + `<div id=app>` + chunks → Phases 1–5.
- **MPA**: SSR + `<form>`, without endpoint bundles → after gate A:

```bash
python3 recon/spider_mpa.py <BASE_URL> <OUTDIR> [--cookie "session=..."] [--max 300] [--depth 5] [--exclude "logout|delete|destroy"]
```

Produces `forms.txt`, `links.txt`, and `api_inline.txt`. If a SPA yields approximately zero forms, switch to Phase 1.

---

## Phase 1 — Static analysis

Follow [Scripts and gates](#scripts-and-gates) and [Tool and output constraints](#tool-and-output-constraints).

```bash
python3 recon/harvest_static.py <BASE_URL> <OUTDIR>
```

Harvest parses HTML scripts → webpack/Vite manifests → downloads all lazy chunks → produces `js/`, `api_static.txt`, `routes.txt`, and `chunkmap.txt`.

```bash
wc -l OUTDIR/api_static.txt OUTDIR/routes.txt
ls OUTDIR/js | wc -l
```

- Compare chunk count with the manifest. For 404s, modify harvest and retry; do not curl chunks individually.
- Too few entries in `api_static.txt`: broaden the endpoint regex in OUTDIR and rerun (see reference).

### Phase 1b — Parameter reverse engineering

Paths come from Phase 1; parameter fields require separate reconnaissance. Follow the grep rules in [Tool and output constraints](#tool-and-output-constraints).

**Completion standard:** for important endpoints, identify field name, transport location, inferred type, required status, sample value, and confidence.

#### 1b.0 — Transport forms

| Form | Parameter location | Static inspection priority |
| --- | --- | --- |
| REST JSON | body + query | `(params\|data\|body)\s*:\s*\{` near path anchors |
| GraphQL | `variables` | gql templates, `$page: Int` |
| Traditional form | urlencoded | `<form>`, `FormData` |
| File upload | multipart | `FormData.append` |
| Path parameters | `/user/:id` | Route tables + `useParams` / `$route.params` |
| Encryption/signing | Wrapped in `sign`/`data` | Hook encryption-function inputs (reference section D) |

Record each endpoint's `transport: query|json|form|graphql|encrypted`.

#### 1b.1 — Expand context around anchors

Use known paths as anchors and expand the window to find payload construction objects:

```bash
grep -n '"/api/user/list"' OUTDIR/js/*.js | head -20
grep -rhoaE '.{0,120}("/api[^"]+").{0,200}' OUTDIR/js/*.js | head -20
grep -rhoaE '(params|data|body|payload)\s*:\s*\{' OUTDIR/js/*.js | head -20
```

| Wrapper | Parameter clues |
| --- | --- |
| axios instance | `data` / `params` |
| Shared request wrapper | Interceptors inject global fields |
| OpenAPI client | Generated method signatures |
| React Query / SWR | Second hook argument |
| Vue composable | Composable inputs |

Type clues: `yup`/`zod`/rules, `Form.Item name=`, embedded Swagger.

→ `param_candidates.json`: `{ path, fields[], source: "static-callsite", confidence }`.

#### 1b.2 — Binding layer

```
Form field → onFinish/handleSubmit → transform → API payload
```

| Binding source | Method |
| --- | --- |
| Form submission | Follow submit → transform → API |
| Table search | `getFieldsValue()` → `params` |
| Route | `:id` / `?tab=` |
| Interceptor | Global `tenantId`, pagination, sign |
| Enum select | `options` → API enum values |

Trace upward from `fetch`/`XHR.send` in the DevTools call stack to payload construction.

#### 1b.3 — Three payload questions (distinct from Phase 2 authentication gates)

| Question | Answer needed |
| --- | --- |
| **Construction** | Where payload is built and transformed |
| **Validation** | required, pattern, enum |
| **Transport** | path / query / body / multipart / headers |

While inspecting the Phase 2 interceptor gate, also read global injected fields such as Authorization, `X-Tenant-Id`, and sign.

#### 1b.4 — Link to Phase 3

Static/binding analysis produces candidate fields. **Required/optional status and conditional dependencies** need the Phase 3 trigger matrix, diffs, and Phase 5 error-based inference.

---

## Phase 2 — Three authentication gates

Grep within `OUTDIR/js/`, limiting output with `head`, then write `config.json` (recipes in reference):

| Gate | Question | Keywords |
| --- | --- | --- |
| **Rendering** | How is logged-in state determined? | `isLogin`, `getToken`, Cookie/localStorage |
| **Interceptor** | What triggers a redirect to `/login`? | `response_code`, `errno`, axios interceptor |
| **Content** | Where do menus/permissions come from? | `menu`, `permission`, `role`, `acl`, `routes` |

Do not treat a localStorage key name as a credential; confirm its meaning from chunks/request chains.

**Exit condition: gate B.** Put conclusions in `config.json` and adapt `OUTDIR/runtime_harvest.js` / `preload.js`.

### Phase 2b — API observation (optional)

Use the OUTDIR copy of `preload.js` to confirm session key names, Authorization, and nested API URLs:

| Setting | Output |
| --- | --- |
| `recordDetail: true` | `__API_RECON_DETAIL__` |
| `observe.xhrHeaders: true` | Header observations |
| `extractUrlsFromResponse: true` | Nested APIs in responses |
| `observe.storageReads/cookieReads: true` | Values to populate config |
| `neutralizeVueRouter: true` | `__API_RECON_ROUTES__` |

Export each coverage round: `__API_RECON_LOG__`, `__API_RECON_DETAIL__`, `__API_RECON_ROUTES__`, `__API_RECON_OBSERVE__`.

---

## Phase 3 — Runtime

Gate B must be complete. Follow [Boundaries and prohibitions](#boundaries-and-prohibitions-required-reading) and the credential-free mock strategy.

Set `"runtimeMode": "depth" | "coverage" | "both"` in `config.json` (template in reference).

### Hooks and stubs (shared by depth and coverage)

| Layer | Scope | Purpose |
| --- | --- | --- |
| L1 exact | Auth/permission/bootstrap stubs | Pass initial-screen authentication |
| L2 negative-result correction | All JSON responses | Change unauthenticated codes to success |
| L3 fallback | `/api` and similar paths not matched by L1 | Empty success body to render UI |

- **depth**: fake auth + `forward` application-code rewriting + `stubs`; traverse `routes` with hash/history; produce `runtime_api.json`.
- **coverage**: inject `preload.js` at **document-start** via CDP `addScriptToEvaluateOnNewDocument` or a userscript.

Verify that `window.__API_RECON_PRELOAD__` exists and business paths do not return to `/login`.

```bash
cd recon && npm install
node runtime_harvest.js config.json
```

### 3b — Dynamic coverage enumeration (required)

1. Main navigation/sidebar: click each item and wait 1–3 seconds for network activity.
2. Tabs: `role=tab`, `.ant-tabs-tab`.
3. Tables: first-row view/edit/details.
4. Toolbar: export, filter, create; **avoid irreversible deletion**.
5. Merge APIs/routes upon entering each module.
6. SPA: controlled `pushState` for uncovered paths in `routes.txt`; prohibited for MPA.

**Parameter trigger matrix (required):** record each operation type for every module and **diff multiple samples**:

| Operation | Typical added parameters |
| --- | --- |
| Initial list screen | Pagination + default filters |
| Search click | keyword, filter |
| Advanced filters | More optional fields |
| Create/edit | Full entity |
| Batch/export/sort | `ids[]`, `exportType`, `sortField` |

**Outbound bodies/headers remain real under stubs**; use the request as evidence. Record to `scan_raw.json`, `param_samples.json`, and `api_detail.json`.

- **Vue**: `neutralizeVueRouter: true` + document-start preload.
- **React**: `routes.txt` + sidebar clicks + `pushState`.
- **both**: 3a depth first, then 3b coverage.

---

## Phase 4 — Permission tree reconstruction

**Trigger:** blank module pages or only bootstrap (such as locale) on every route means the content gate has not been passed.

| Symptom | Meaning |
| --- | --- |
| Shell entry succeeds | Rendering and interceptor gates passed |
| Missing sidebar items/blank clicks | Incomplete stub shape or permission codes |
| Same few APIs on every route | `v-if permission` failed |
| Far fewer routes in `routes.txt` than bundles | Complete routes from the auth module |

```bash
grep -rhoaE '"/api[^"]*(permission|perm|role|menu|acl)[^"]*"' OUTDIR/js/*.js | sort -u | head -30
grep -rhoaE 'userRouteAuth|getResultTree|routeMap|routeLink|menuList|authList' OUTDIR/js/*.js | head -20
```

Typical chain: `role_permissions` (flat codes) + `permissions/all` (tree) → `getResultTree` → `userRouteAuth[CODE].url`.

```bash
python3 recon/extract_route_map.py recon/js recon/
python3 recon/build_perm_tree.py recon/js recon/ --config recon/config.json
```

Intermediate outputs: `route_map.json`, `userRouteAuth.json`, `permissions_tree.json`, `*_stub.json`, `perm_codes_all.txt`.

Check stubs: outer `response_code` matches the interceptor gate; flat codes align with the tree; `routes` covers every link in `route_map`.

After updating `config.json`, **rerun Phase 3**. For large SPAs, adjust `waitUntil`, `routeTimeout`, and `perRouteMs` (reference sections A3/I).

---

## Phase 5 — Merge and report

### Outputs

| File | Phase | Contents |
| --- | --- | --- |
| `js/`, `api_static.txt`, `routes.txt`, `chunkmap.txt` | 1 | Static bundles and paths |
| `param_candidates.json` | 1b | Static parameter field candidates |
| `config.json` | 2 | Three gates + runtime configuration |
| `runtime_api.json` | 3a | Detailed depth recording, including WS/SSE |
| `param_samples.json`, `scan_raw.json`, `api_detail.json` | 3b | Multiple samples, click log, details |
| `route_map.json` and related files | 4 | Permission-tree intermediate files, if run |
| `params_merged.json` | 5 | Merged parameter fields + confidence |
| `api_merged.txt` | 5 | `METHOD /path [params] [static\|runtime\|both]` |
| `site_map.json` | 5 | Routes, APIs, parameters, features, limitations |
| **insert_assets** | 5 | Write all service and endpoint assets to the inventory |

### 5b — Merge parameters

Diff `param_samples.json`; **there is no universal merge script**. See reference J7 for confidence rules (high/medium/low/awaiting trigger).

### 5c — Infer from errors

Within authorization scope, incomplete requests may be used to read 400 responses (**parameter reconnaissance, not vulnerability testing**), such as `field 'x' is required` or enum errors. Account for `data` wrappers, `variables`, and pre-encryption `bizData`.

Report runtimeMode, static/runtime API counts, parameter confidence, uncovered modules, and a summary of `CHANGES.md` relative to the reference scripts.

Suggested `site_map.json` structure:

```json
{
  "site": "https://example.com",
  "runtimeMode": "both",
  "appType": "vue-spa",
  "routeGuardStrategy": ["nav-neutralize", "L1-auth", "L2-patch", "forward"],
  "apisFromStatic": [],
  "apisFromRuntime": [],
  "apis": [],
  "params": [{ "method": "POST", "path": "/api/user/list", "transport": "json", "fields": [] }],
  "frontendRoutes": [],
  "routesVerifiedByClick": [],
  "featuresTriggered": [],
  "limitations": ""
}
```

See [reference.md](reference.md) for more fields and grep recipes.

---

## General notes

- **Framework-independent**: the same method applies to webpack/Vite/Angular lazy loading.
- **Transport**: REST/JSON, GraphQL, WebSocket, SSE; gRPC-web is outside scope.
- **SSR**: client fetches can be recorded; RSC/Server Actions cannot be enumerated completely.
- **Blind spots**: JSVMP, WASM, strict HMAC/mTLS validation → static analysis and stated limitations.
- **Parameter blind spots**: conditional dependencies, hidden parameters, WASM payload construction → mark “awaiting trigger” or “unreachable”.
- **Static fallback**: endpoints remain enumerable statically when runtime is blocked.

---

## Additional resources

- Grep recipes, `config.json` templates, troubleshooting, hooks, parameter reverse engineering in section J, and site_map templates: **[reference.md](reference.md)**.
- Reference script paths appear in the [Scripts and gates](#scripts-and-gates) table.
