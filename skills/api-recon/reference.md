# api-recon — Reference

Grep recipes, `config.json` templates, and troubleshooting. Run all grep commands against `js/`. For single-line bundles, optionally use `js-beautify` or `sed 's/}/}\n/g'`; raw grep with a context window is usually sufficient.

## Script notes

Every file in `scripts/` is a **reference template** and must be adapted to the target site before execution. Typical adjustments:

| Script | Common adjustments |
| --- | --- |
| `harvest_static.py` | Endpoint regex, webpack/Vite manifest parsing, microfrontend publicPath, retries/concurrency |
| `runtime_harvest.js` | neutralize field names and success values, stub match rules/body shapes, route sources, WS recording, `waitUntil`/`routeTimeout`/`proxy` |
| `preload.js` | `loginPathRe`, L1 stubs, `neutralize.fields`, `apiPattern`, L3 enablement, `recordDetail`, `observe.*`, `neutralizeVueRouter` |
| `spider_mpa.py` | `--exclude` destructive links, cookie, depth/max, same-domain filtering |
| `extract_route_map.py` | `routeMap` / `routeLink` regex, KEY naming patterns |
| `build_perm_tree.py` | `userRouteAuth` parsing, `ROOTS`/`PREFIX_PARENT` hierarchy heuristics, outer stub field names |
| `config.json` | Central location for all site-specific parameters above |

Put adapted files in the task working directory, such as `recon/`, and report specific changes from the reference scripts.

---

## A. Reverse-engineer the three gates

### A1. Rendering gate: how is logged-in state determined?

```bash
grep -rhoaE '.{0,40}(isLogin|isAuthenticated|loggedIn|hasLogin|requireAuth)\b.{0,80}' js | head
grep -rhoaE 'function (getUser|getToken|getAuth)[0-9]?\([^)]*\)\{.{0,200}' js | head
grep -rhoaE '(localStorage|sessionStorage)\.getItem\("[^"]+"\)' js | sort -u
grep -rhoaE '(Cookies?|cookie)\.(get|load)\("[^"]+"\)' js | sort -u
grep -rhoaE '\batob\(|JSON\.parse\(|jwt|decode' js | head
```

Trace `isLogin = f(getUser())` → `getUser = decode(storage.read(KEY))` to determine the **storage key**, **container** (Cookie vs localStorage), and **encoding**:

| Encoding | Mock value in config |
| --- | --- |
| Plain string / `"1"` / token | `"value": "anything-truthy"` |
| `JSON.parse(x)` | `"value": "json:{\"id\":1,\"username\":\"admin\"}"` |
| `JSON.parse(atob(x))` | `"value": "b64json:{\"id\":1,\"username\":\"admin\"}"` |
| JWT | Unsigned/`alg:none` JWT, or signed with a key in the bundle |
| Encryption (SM2/AES/RSA) | Find hardcoded keys; forge if the rendering gate only needs a decodable blob, otherwise use static fallback |

→ Write to `cookies` / `localStorage`.

### A2. Interceptor gate: what triggers a redirect to /login?

```bash
grep -rhoaE '.{0,60}(interceptors\.response|axios|request\.use).{0,120}' js | head
grep -rhoaE '.{0,40}(response_code|errcode|errno|\bcode\b|\bret\b|\bstatus\b)\s*[=!]==?\s*[\-0-9]{1,4}.{0,60}' js | head -20
grep -rhoaE '.{0,40}(未登录|请重新登录|登录已过期|unauthorized|登录失效|授权|token.{0,10}invalid).{0,40}' js | head
grep -rhoaE '.{0,30}(location\.href|router\.(push|replace)|navigate)\([^)]*login[^)]*\)' js | head
```

Determine the **field name**, **success value** (usually `0` or `200`), and **failure value that triggers redirection**. Verify with a junk session:

```bash
curl -sk -X POST -H 'Cookie: <fakekey>=junk' https://target/api/<protected> -d '{}' -H 'Content-Type: application/json'
```

→ Write to `neutralize.fields` + `neutralize.success`.

### A3. Content gate: where do menus/permissions come from?

```bash
grep -rhoaE '"/api[^"]*(permission|perm|role|menu|acl|resource|nav)[^"]*"' js | sort -u
grep -rhoaE '.{0,30}(menus|permissions|menuList|routeList|authList|role_permissions)\b.{0,120}' js | head
grep -rhoaE 'userRouteAuth|getResultTree|routeMap|routeLink|hasPermission|checkAuth' js | head
grep -rhoaE '([A-Z_][A-Z0-9_]*):\{name:"[^"]*",link:"/[^"]+"\}' js | head
```

**Two data layers**, common in enterprise admin interfaces:

| API | Typical payload | Consumer |
| --- | --- | --- |
| `.../role_permissions` | `{ permissions: string[], role_type }` | Route guards and button-level ACL |
| `.../permissions/all` | `tree[{ code, position, children }]` | Sidebar menu rendering |
| `userRouteAuth` in bundle | `{ CODE: { url, name? } }` | code → frontend path |
| `routeMap` in bundle | `{ KEY: { name, link } }` | Alias resolution, such as webpack `o.DASHBOARD` |

Read consuming code to determine how `getResultTree(tree, permissions)` filters and which field `v-if` / `hasAuth(code)` checks.

**Manual mock construction for small sites:** build a permissive payload → `stubs`.

**Full permission-tree reconstruction for large sites with blank sidebars/submodules:** see **section I**.

---

## B. config.json template

```json
{
  "baseUrl": "https://target/",
  "runtimeMode": "both",
  "chromium": "/usr/bin/chromium",

  "cookies": [
    { "name": "auth", "value": "b64json:{\"id\":1,\"username\":\"admin\",\"role\":\"admin\",\"func\":{},\"permissions\":[\"*\"]}" }
  ],
  "localStorage": { "token": "faketoken", "isLogin": "1" },

  "neutralize": {
    "fields": ["response_code", "code", "errno", "ret", "status"],
    "success": 0,
    "flags": { "success": true, "message": "ok" }
  },
  "forward": true,
  "loginUrlPattern": "/login",
  "apiPattern": "/api/|/rest/|/graphql",

  "mockTier": "L1+L2",
  "recordDetail": true,
  "observe": {
    "storageReads": false,
    "cookieReads": false,
    "xhrHeaders": true
  },
  "neutralizeVueRouter": true,
  "stubs": [
    {
      "match": "permissions/all|/menu|role_permissions",
      "body": {
        "response_code": 0, "code": 0,
        "data": {
          "permissions": ["*"],
          "menus": [
            { "name": "dashboard", "path": "/dashboard", "show": true, "children": [] },
            { "name": "alert", "path": "/alert", "show": true, "children": [] }
          ]
        }
      }
    }
  ],

  "explore": {
    "clickTabs": true,
    "clickTables": true,
    "pushStateFallback": true,
    "maxMenuItems": 50
  },

  "routes": ["/dashboard", "/alert", "/asset", "/device", "/report", "/config", "/system"],
  "waitMs": 1500, "perRouteMs": 900, "headless": true,
  "waitUntil": "domcontentloaded",
  "routeTimeout": 12000,
  "proxy": "",

  "captureResponses": true, "recordWs": true, "respMax": 600
}
```

Fields:

- `runtimeMode`: `depth` (Puppeteer), `coverage` (browser MCP), `both`.
- `cookies[].value` prefixes: `b64json:` → base64(JSON); `json:` → raw JSON; no prefix → literal value.
- `forward: true` forwards real requests and rewrites code fields; `false` uses fully offline stubs.
- `mockTier`: preload layers enabled in coverage mode, such as `L1+L2` or `L1+L2+L3`.
- `routes` comes from `routes.txt`; after mocking menus, the harness automatically appends `<a href>` entries.
- `captureResponses` / `recordWs` apply only in depth mode.
- `waitUntil`: use `domcontentloaded` for large SPAs to avoid hangs with `networkidle2`.
- `routeTimeout`: per-route `page.goto` timeout in milliseconds.
- `proxy`: Puppeteer `--proxy-server`; alternatively use `HTTP_PROXY` / `HTTPS_PROXY`.

### B1. Dual-stub template (role_permissions + permissions/all)

```json
"stubs": [
  {
    "match": "role_permissions",
    "body": {
      "response_code": 0,
      "data": {
        "permissions": ["MONITOR", "MONITOR_ALERT", "THREAT", "ASSETS_RISK"],
        "role_type": "SUPER_ADMIN"
      }
    }
  },
  {
    "match": "permissions/all",
    "body": {
      "response_code": 0,
      "data": [
        {
          "code": "MONITOR",
          "position": 1,
          "children": [
            { "code": "MONITOR_ALERT", "position": 1, "children": [] }
          ]
        }
      ]
    }
  }
]
```

Outer field names (`response_code` / `code` / `data`) must match the A2 interceptor gate. `permissions` must cover all leaf codes in the tree.

---

## C. Coverage mode: preload configuration

Edit `CONFIG` at the top of `scripts/preload.js`, or replace it before CDP injection:

```javascript
const CONFIG = {
  loginPathRe: /\/(login|signin)(\/|$|\?)/i,
  mockTier: 'L1+L2',
  forward: true,
  recordDetail: true,
  extractUrlsFromResponse: true,
  neutralizeVueRouter: true,
  observe: { storageReads: false, cookieReads: false, xhrHeaders: true },
  neutralize: { fields: ['response_code', 'code'], success: 0 },
  stubs: [ /* 同 config.json stubs */ ],
  apiPattern: /\/(api|apis|v\d+|dev|internal|graphql)\//i,
};
```

Verify `window.__API_RECON_PRELOAD__ === true` and a stable pathname.

Export recorded results:

```javascript
JSON.stringify({
  apis: [...window.__API_RECON_LOG__],
  detail: window.__API_RECON_DETAIL__,
  routes: [...(window.__API_RECON_ROUTES__ || [])],
  observe: window.__API_RECON_OBSERVE__,
}, null, 2)
```

---

## D. preload / runtime hook capabilities

Built-in browser hooks in preload (coverage) and runtime_harvest (depth):

| Hook | Value for API discovery | Coverage |
| --- | --- | --- |
| fetch / XHR.open | Record request URL/method | Supported: `recordDetail` + `__API_RECON_LOG__` |
| XHR.setRequestHeader | Discover Authorization and other headers | Supported: `observe.xhrHeaders` |
| localStorage/cookie reads | Confirm session key names | Optional: `observe.storageReads/cookieReads` |
| Vue route collection | Complete frontendRoutes | Supported: `__API_RECON_ROUTES__` for loaded routes |
| Neutralize Vue route guards / block login redirects | Render modules to trigger APIs | Supported: `neutralizeVueRouter` + native redirect neutralization |
| React route collection | Enrich routes | Static analysis + clicks; no dedicated hook |
| Block navigation to login paths | Stay on the page for analysis | Login paths only, to avoid blocking business navigation |
| Encryption-library hooks (CryptoJS/SM, etc.) | Encrypted parameters → plaintext API bodies | Not built in: manually hook encryption-function inputs; record conclusions in config |
| Anti-debug bypass | Enable runtime API recording | Not built in: handle manually; static analysis remains available |

---

## E. Endpoint extraction regex (when static results are sparse)

Broaden `extract_endpoints` in `harvest_static.py`, or inspect manually:

```bash
grep -rhoaE '"/[a-z][A-Za-z0-9_/\-]{3,}"' js | sort -u
grep -rhoaE '/api/[a-zA-Z0-9_./-]+' js | sort -u
```

---

## F. Troubleshooting

| Symptom | Cause → action |
| --- | --- |
| Very few static APIs | Endpoint dialect mismatch → broaden regex (section D) |
| Chunk count far below manifest | CSS-only or undeployed chunks; 404s already retried |
| Runtime still shows login | Rendering gate incorrect → recheck A1 key, container, encoding, domain |
| Shell reached, modules blank | Content gate → mock menus (A3); `routes` paths may be wrong |
| Only bootstrap/locale per route | Incomplete permission codes → reconstruct tree in I; check dual `role_permissions` + `permissions/all` stubs |
| Sidebar items exist, subpages blank | Missing intermediate tree nodes or code mismatch with `userRouteAuth` |
| Every API redirects to login | Interceptor gate → verify `neutralize`; extend walk logic for nested fields |
| Zero WS frames | Subscription requires interaction; increase `perRouteMs` |
| Empty response body | Real responses exist only with `forward: true` |
| Chromium missing | Install chromium or set `config.chromium` / `CHROMIUM` |
| Many mocks, still redirected to login | Hooks too late or missing `location.href` setter → document-start + preload |
| All lists empty | Empty L3 arrays are normal; continue through tabs/settings/details |
| Redux action mistaken for route | Filter internal paths containing get/set/change/clear/toggle/upload |
| Vue still redirects to login | Preload not at document-start → change injection timing; or manually clear guards when `neutralizeVueRouter: false` |
| Response URL absent from log | Enable `extractUrlsFromResponse` or manually extract from `__API_RECON_DETAIL__` |
| Authorization header name unknown | Enable `observe.xhrHeaders` or inspect request headers in DevTools |
| Runtime very slow/timeouts | Set `waitUntil: domcontentloaded`, reduce `routeTimeout`, avoid `networkidle2` |
| Proxy connection fails | Check `proxy` / environment variables; use the same proxy port in Puppeteer and curl |

---

## G. Hardened targets

If the server validates the session at each step, with unforgeable signed cookies or server-rendered menus that cannot be stubbed, runtime may stop at the shell. Expected behavior:

- **Static analysis suffices for endpoint enumeration**: module paths are in the code.
- If authorization permits, run the same harness with a **real session**, using `forward: true` without neutralize, to capture real methods/parameters/responses.

---

## H. Per-task checklist

1. Confirm authorization scope.
2. **Read** `scripts/harvest_static.py` → adapt to target → run → review `api_static.txt` and `routes.txt`.
3. **Phase 1b**: expanded path-anchor context + binding layer → `param_candidates.json` (section J).
4. Reverse-engineer A1/A2/A3 → write site-specific `config.json`.
5. **Read and adapt** `runtime_harvest.js` / `preload.js` before execution.
6. `runtimeMode=depth`: `npm install` → run the adapted harvest script.
7. `runtimeMode=coverage/both`: inject adapted preload at document-start → browser MCP dynamic enumeration + **parameter trigger matrix**.
8. Modules do not render → **section I permission-tree reconstruction** → patch stubs → rerun.
9. Multi-sample parameter diffs + error-based inference → `params_merged.json`.
10. Merge → `site_map.json` + `api_merged.txt`; honestly report coverage, gaps, and script changes.

---

## I. Permission-tree reconstruction (deeper Phase 4)

Use when a simple mock such as `menus: [{ path, show: true }]` does not work and submodules still fail to mount.

### I1. Locate the auth module

```bash
grep -l 'userRouteAuth' js/*.js
grep -l 'routeMap\|routeLink' js/*.js
grep -rhoaE 'getResultTree|role_permissions|permissions/all' js | head
```

Record **permission API paths**, **response field names**, and **consuming chunk filenames**.

### I2. Extract routeMap

```bash
python3 scripts/extract_route_map.py recon/js recon/
# 产出 recon/route_map.json
```

If `[!] no routeMap pattern found`, broaden the regex in `extract_route_map.py` or grep manually:

```bash
grep -rhoaE '([A-Z_][A-Z0-9_]*):\{name:"[^"]*",link:"/[^"]+"\}' js | head -20
```

### I3. Build the permission tree and stubs

```bash
python3 scripts/build_perm_tree.py recon/js recon/ --config recon/config.json
```

Script logic:

1. Parse `userRouteAuth={MONITOR:{url:...},...}`, including webpack aliases such as `He=o.DASHBOARD`.
2. Resolve aliases to real paths using `route_map.json`.
3. Infer parents from code prefixes (`MONITOR_ALERT` → `MONITOR`).
4. Output `permissions_tree.json`, `permissions_all_stub.json`, and `role_permissions_stub.json`.
5. With `--config`, update `stubs` and extend `routes` in `config.json` automatically.

**Adapt to the target** at the top of the script:

- `DEFAULT_ROOTS`: top-level module codes.
- `DEFAULT_PREFIX_PARENT`: mapping from `PREFIX_` to parent.
- `DEFAULT_EXTRA_PARENT`: orphan nodes whose parent relation is not based on a prefix.

### I4. Verify stub consistency

```bash
# permissions 数量应 ≈ userRouteAuth 条目数
wc -l recon/perm_codes_all.txt
# routes 应覆盖 route_map 全部 link
python3 -c "import json; m=json.load(open('recon/route_map.json')); r=set(json.load(open('recon/config.json'))['routes']); print('missing', [v['link'] for v in m.values() if v['link'] not in r])"
```

### I5. Rerun runtime and compare

```bash
node recon/runtime_harvest.js recon/config.json
# 对比 forge 前后 runtime_api.json 条数；检查 /attack、/asset 等是否出现模块 API
```

| Before mock construction | After successful mock construction |
| --- | --- |
| Same 3–5 bootstrap requests per route | Different routes trigger different module APIs |
| Only `/api/locale/language` | `/api/web/...` module endpoints appear |
| Single-digit routes in `routes.txt` | 80–110+ `routes` from route_map |

### I6. If it still fails

- **Coverage mode**: click sidebar and tabs; permission-gated requests may require interaction.
- **Stub fields**: compare nesting against the real API (curl + real session).
- **Additional guards**: grep button-level checks such as `hasPermission|checkRole|func.` and extend `role_permissions.permissions`.
- **Static fallback**: module API paths remain in `api_static.txt`; runtime only adds METHOD/body. Retain `param_candidates.json` and recorded samples for parameters.

---

## J. Parameter reverse engineering (Phases 1b / 5b / 5c)

**A methodology, not a universal script.** Find paths with regex; find parameters through expanded anchor context, UI binding chains, multi-sample diffs, and error-based inference.

### J1. Expand anchor context: from paths to payload construction

```bash
# 以 Phase 1 已知 path 为锚
grep -n '"/api/user/list"' js/*.js
grep -rhoaE '.{0,120}("/api[^"]+").{0,200}' js | head
grep -rhoaE '(params|data|body|payload)\s*:\s*\{' js | head
grep -rhoaE '(get|post|put|delete|patch)\([^,]+,\s*\{' js | head
```

### J2. Wrappers and transport forms

```bash
# axios / 统一 request
grep -rhoaE '(axios|request)\.(get|post|put|delete|patch)\(' js | head
grep -rhoaE 'interceptors\.(request|response)' js | head

# GraphQL
grep -rhoaE '(query|mutation)\s+\w+|gql`|graphql\(' js | head
grep -rhoaE '\$[a-zA-Z_]+\s*:\s*(Int|String|Boolean|\[)' js | head

# FormData / multipart
grep -rhoaE 'FormData|\.append\(' js | head

# 路径参数
grep -rhoaE 'path:\s*"/[^"]*:[^"]+"' js | head
grep -rhoaE 'useParams|route\.params|\$route\.params' js | head
```

### J3. Validation gate: required fields / formats / enums

```bash
grep -rhoaE '(required|message|pattern|enum|validator)\s*:' js | head
grep -rhoaE 'yup\.|zod\.|async-validator|Form\.Item|a-form-item|el-form-item' js | head
grep -rhoaE 'rules\s*:\s*\[|name:\s*["\'][a-zA-Z_]+["\']' js | head
grep -rhoaE 'label.*value|options\s*:\s*\[' js | head
```

### J4. Binding layer: form → API

```bash
grep -rhoaE 'onFinish|handleSubmit|getFieldsValue|validateFields' js | head
grep -rhoaE '(pick|omit|transform|dayjs|moment)\(' js | head
```

Runtime supplement: DevTools → Network → request → **Initiator** (call stack), then trace upward from `fetch`/`send` to payload construction.

### J5. Encrypted parameters

```bash
grep -rhoaE 'encrypt|decrypt|sign|CryptoJS|sm2|sm3|sm4|RSA|AES' js | head
```

**Do not guess fields from ciphertext.** Hook encryption-function **inputs** to record plaintext payloads before encryption. Write conclusions to `config.json` / `param_candidates.json`.

### J6. Parameter trigger matrix (required in Phase 3)

Record each operation in each module and diff request bodies/queries:

| Operation | Focus |
| --- | --- |
| Initial list screen | Default pagination |
| Search | keyword, filters |
| Advanced filtering | Optional fields |
| Create/edit | Complete entity |
| Batch/export | `ids[]`, `exportType` |
| Sort/paginate | `sortField`, `order` |

Output `param_samples.json`: `[{ "path", "method", "action": "search", "body", "query", "headers" }]`.

### J7. Confidence rules

| Confidence | Condition |
| --- | --- |
| **High** | Static call site and at least two runtime samples agree |
| **Medium** | Static only, or one runtime sample |
| **Low** | Inferred from responses/errors without a second verification |
| **Awaiting trigger** | Field known statically, but UI/permission path not reached |

### J8. Quick scenario guide

| Scenario | Sequence |
| --- | --- |
| REST list page | J1 payload objects → J6 four diffs → J3 rules |
| Create/edit form | J3 Form name → J4 submit chain → runtime submission with deliberately empty fields to inspect 400s |
| GraphQL | J2 variable declarations → record variables for each runtime operation |
| Encrypted body | J5 hook inputs → pre-encryption fields are the actual parameters |

### J9. Mapping to api-recon phases

| api-recon | Parameter reconnaissance |
| --- | --- |
| Phase 1 static | J1 expanded anchor context |
| Phase 2 A2 interceptor | Globally injected fields such as tenantId and sign |
| Phase 3 runtime | J6 trigger matrix + `param_samples.json` |
| Phase 4 permission tree | Different module forms require sufficient permissions to trigger all fields |
| Phase 5 merge | `params_merged.json` + confidence; do not infer required fields from one sample |

### J10. Troubleshooting

| Symptom | Action |
| --- | --- |
| Static field never appears at runtime | Mark “awaiting trigger”; complete permission tree, click advanced filters, try linked-select options |
| Different body shapes for one path | Normal: record separately by `action`; do not force schemas together |
| Mock responses are fake but parameters are needed | Inspect **outbound request** bodies/headers; do not infer from stub responses |
| 400 names a nested field | Check outer `data`/`bizData`/`variables` wrappers |
| GraphQL only shows operation name | Expand `variables` JSON; statically find `$var: Type` |

---
