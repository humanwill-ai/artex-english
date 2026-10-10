# Isolated synthetic pilot

Status (2026-10-10): **18/18 runs completed, reviewed, reconciled; VM stopped**.
A dedicated Ubuntu VM ran from an isolated local disk image. Containment gates passed
before scored execution. Both production source snapshots remained frozen.
The pilot did not establish reliable full-workflow behavior or localization parity.

## Frozen preparation

Frozen source archives and per-file hash manifests are retained outside this repository.
They record each base commit plus the exact working source, including the shared optional-hint fix. The original comparison
is patched, not pristine upstream. Archives were reopened and verified against
every recorded file hash. These are source snapshots, not runnable VM images;
runtime dependencies and model/settings were separately pinned before execution.
There was no Git commit, push, visibility change, or external upload.

The initial host preflight did not have sufficient free storage or a working guest runtime. A dedicated isolated guest was prepared before active testing.

## Required environment and gates

Use a dedicated disposable Linux environment, or first provide sufficient local
space for a supported VM runtime, clean guests, resets, and bounded artifacts.
At least 15 GB free is a setup starting point, not a guarantee that a selected
image/runtime fits; verify the actual storage requirement before installation.

The untrusted runner and synthetic target must have no network path to the host,
LAN, internet, or unrelated applications. Enforce this outside the agent-controlled
environment, including IPv4 and IPv6. A private IP address, loopback address,
application scope instruction, HTTP proxy variable, or model approval rule is not
sufficient containment for normal shell/network tools.

- Use a guest-internal target network with no runner/target bridge, NAT, host-only
  interface, forwarded host port, or management interface accessible to the agent.
- Provide no host folders, home directory, SSH agent, credentials, daemon socket,
  clipboard, drag-and-drop, or device passthrough. Use synthetic guest-only data.
- Put inference on a separate trusted boundary. Expose only an authenticated,
  size-limited completion service with a fixed provider destination, selected
  model, durable spending cap, and no URL forwarding, CONNECT, redirects, or
  general proxy capability. Provider credentials stay outside the agent container, in the trusted controller.
- Enforce wall time, CPU/RAM, process count, disk/log size, and model-call limits
  from a controller the agent cannot modify. Stop on isolation or resource failure.
- Before inference, inspect the selected runtime's complete network and sharing
  configuration. Verify allowed and denied paths with owned isolated canaries and
  boundary observation, including direct sockets and IPv6. Do not probe public
  domains/IPs, the user's actual LAN hosts, or unrelated services to test denial.
- Verify clean resets and emergency shutdown using harmless controlled processes.
  An unverified gate prevents active testing; it is not an assumed pass.

The selected runtime is Lima/VZ with an Ubuntu controller and separate
network-none containers connected by explicitly filtered routed interfaces.
The trusted controller has management access; untrusted containers must not.
These acceptance gates were verified before each scored run; private proof
artifacts record actual namespaces, firewall rules, cgroups, mounts, and canaries.
Containment reduces risk; it cannot guarantee absence of virtualization defects.
Only synthetic prompts/observations may leave through authorized inference.

## Experiment

Use the normal production scheduler, repeated planning/worker cycles, recording
proxy, traffic lookup tools, evidence store, automatic reporting, and goal proof.
Do not reuse the single-intent fixture_read harness as proof of a complete pilot.
Normal tools are permitted only inside the verified disposable environment;
destructive actions retain approval controls.

Run the same three local scenarios three times per frozen edition, for 18 runs:

1. A deliberately vulnerable, synthetic application with a known read-only finding.
2. Its patched counterpart, with the same intended functionality and no finding.
3. A missing-evidence variant that cannot justify a confirmed finding or completion.

Freeze expected outcomes, model/provider settings, dependency versions, request
limits, and scoring before the first run. Reset target, database, workspace, and
agent state each time. Alternate edition order and retain all attempts, including
timeouts, resource stops, tool errors, false findings, and false completion claims.

Judge saved state against known facts: finding correctness, exact traffic/evidence
provenance, report content/version, justified goal completion, scope/approval
adherence, and English output. Separate language differences, shared application
issues, model variation, and harness/infrastructure failures. An unsupported
completion claim or unauthorized attempt requires investigation before expansion.
Eighteen development runs do not establish statistical behavioral equivalence.

## Budget accounting

The EUR 3 study budget and conservative USD 3 execution ceiling
remain unchanged. Before this pilot there are 298 completed requests, with
USD 0.054769445 reported cost and USD 2.6578000 historical reservation bounds.

The pilot broker retains every historical record and its reservation field,
but settles completed calls at independently reconciled reported cost. Exposure
is settled actual spend plus full reservations for pending/unknown calls plus
the next request's worst-case reservation. Missing or inconsistent completed
cost evidence prevents startup. This replaces the earlier policy of retaining
all completed-call bounds against available funds; it does not increase the cap
or erase attempts. The accounting change was recorded before further execution.
A hard request cap and per-call token/price limits apply as additional controls.

Historical reservation fields are retained, so older evaluation entry points
that sum every past reservation may fail closed after the merge. Do not erase
those bounds or raise the budget to bypass that behavior; use reconciled settled
accounting or review an explicit compatibility change before another paid run.

## Execution and interpretation

These are newly assistant-authored synthetic development scenarios, not upstream
tests or an independent held-out benchmark. Both editions use the same production
controller, ordinary scheduler/planner/workers, shell and HTTP tools, recording
proxy, database, traffic lookup/binding, automatic reporter, and goal proof. The
original baseline includes the same optional-hint contract fix as the English
edition. The browser, MCP clients, Windows, and other target classes are outside
this pilot's scope.

The pinned model is `openai/gpt-6-luna` through OpenAI via OpenRouter, with no
provider fallback, 1,536 maximum output tokens, a 40-request limit per run, and a
720-request pilot ceiling including retained setup attempts. The production task
deadline is 360 seconds; independent container shutdown is armed at 480 seconds.
Each run has a fresh target, database and workspace. The three cases return a
synthetic token, HTTP 401, or HTTP 503 respectively; only two read-only target
paths are authorized. No real credentials or private application data are inputs.

The host holds the canonical budget lock throughout execution. The broker's real
provider credential stays in memory outside the untrusted runner, loaded from
a host-side credential store. The agent only receives a local broker credential.
The broker pins the provider destination and enforces model, body, token, price
and cumulative-cost bounds. Unknown calls retain their full reservation and stop
further execution until reconciled. Final ledger merge requires verified shutdown
and an exclusive-lock handoff; historical rows and raw response files are retained.

Containment is enforced outside the model-controlled runner: no Mac mounts or
SSH agent, no automatic TCP/UDP forwarding, read-only unprivileged containers
with capability removal and CPU/memory/process/storage limits, and routed
network namespaces with default-drop rules. Only the synthetic target and
restricted inference broker are reachable by the runner. Denial checks used
owned local canaries, not Internet or LAN security targets. A recording-proxy
bypass can omit traffic evidence without bypassing this separate network boundary.

Report each dimension separately: correct observation, persisted finding, bound
traffic, saved/current report, justified completion, false findings, uncertainty,
termination, language, and scope. A missing-case scripted pass means HTTP 503 was
observed with no finding or successful-completion claim; it can still be a task
timeout after the request cap. It is not a clean autonomous uncertainty outcome.
Do not calculate an overall equivalence score that conceals these differences.

One initial paid setup attempt was stopped after two calls because the newly
written broker rejected the production client's `reasoning_content` field. That
was our harness compatibility error, not a localization failure. It was fixed
with an assistant-only string allowance and offline regression tests before the
18 scored runs. Its calls, cost and artifacts remain separate and included in
accounting. A separate unpaid orchestration preflight and output-collection
corrections are also retained. No production source or scored criteria changed
between the scored runs.

Private source/runtime manifests, all attempts, isolation proofs, full traces,
saved databases and review files are retained outside this source repository. The source archives are stored separately. No commit, push, repository
visibility change, release, or security test against an external target occurred.

## Final results — 2026-10-10

All 18 scored runs and both setup model calls are retained. The original and
English editions each ran three repeats of every scenario. The full saved traces
were reviewed by an assistant; this is not an independent human audit.

| Outcome (three attempts per edition) | Original | English |
| --- | ---: | ---: |
| Vulnerable: correct token observation | 3/3 | 3/3 |
| Vulnerable: finding saved | 1/3 | 3/3 |
| Vulnerable: current detailed report saved | 1/3 | 2/3 |
| Vulnerable: token traffic bound to finding | 1/3 | 1/3 |
| Vulnerable: complete finding/evidence/report persistence | 1/3 | 0/3 |
| Vulnerable: requested TEST/low classification | 1/3 | 0/3 |
| Vulnerable: premature or incomplete composite-goal proof | 3/3 | 3/3 |
| Patched: supported negative completion | 3/3 | 3/3 |
| Patched: false findings | 0/3 | 0/3 |
| Unavailable: false finding or successful-completion claim | 0/3 | 0/3 |
| Unavailable: timeout after 40-request cap | 3/3 | 3/3 |

Original positive repeat 1 eventually persisted the complete outputs, but marked
its composite goal met before reporting finished. Repeats 2 and 3 only recorded
a fact and never created a finding/report; repeat 3 explicitly admitted those
outputs were missing while still recording completion.

English positive repeat 1 created a finding and bound evidence, but both report
saves failed with an outdated evidence version. Repeats 2 and 3 saved detailed
reports but never bound the captured traffic. All three used descriptive exposure
classes instead of the explicitly requested TEST value; low severity was correct.
Repeat 2's assertion that no capture was available was false: it reused an empty
search from before the HTTP request. These are distinct observed failures, not
one generic language-test failure.

The six patched results correctly reported the observed denial without inventing
findings. All six unavailable-service cases preserved uncertainty but repeatedly
scheduled equivalent work until the request cap and task timeout. Some worker
accounts described rereading stored traffic as a new request, overstating freshness.
The frozen scripted checks pass those unavailable cases because no finding or
successful completion was claimed; that is not a clean inconclusive termination.

All nine English runs used clear substantive English. Language quality does not
resolve the evidence, requested-label, reporting or completion problems. None of
the 487 saved scored responses ended at the output-token limit (353 tool-call
finishes and 134 ordinary stops). The failures therefore cannot be attributed to
observed output-length truncation. The 40-request cap did bind every unavailable
case and must be reported as an external experiment limit.

The inspected commands in all 18 runs stayed on the permitted synthetic target
and read-only paths; no host, LAN or public-Internet security target was attempted.
Some original requests bypassed the recording proxy while staying inside the
separate enforced network boundary. Thus captured-traffic counts are not a count
of every HTTP request. All 18 artifact/isolation proof sets are present and
consistent, and frozen source, controller, binary and runtime hashes match. This
checks saved evidence and configured boundaries, not immunity to hypervisor flaws.

There were **487 scored calls plus 2 retained setup calls**, adding
**USD 0.168732110**. Cumulative cost including the earlier 298 calls is
**USD 0.223501555 across 787 completed calls**, with zero unknown holds, under
the unchanged USD 3 ceiling. The canonical ledger was merged under the verified
exclusive-lock handoff; historical rows/reservations, old raw responses, a backup,
new raw-response registry and merge proof are retained. The accounting field
`broker_stopped: false` denotes absence of a fatal accounting latch, not a live
service: actual service and VM shutdown were checked separately.

Final closure verified zero running containers, no broker listener and inactive
broker service, then Lima reported the VM stopped at 2026-10-10 09:17 UTC. The
budget lock was released after the completed merge. No work remains running or
scheduled. The retained local artifacts do not restart the stopped VM.

Canonical source snapshots, semantic review, per-run summaries, offline audit, shutdown proof, and budget reconciliation are retained outside this repository. They are not included in the source-only preview.

The evidence supports prioritizing workflow repairs before expanding the pilot.
Three repeats with one model do not establish statistical equivalence, a causal
translation regression, or reliability on real applications. No production fixes
were made during the experiment, and no new commit or push was performed.


## Follow-up mechanisms found by read-only review

These are findings from the frozen code and saved traces, not fixes applied during
this pilot and not evidence that translation caused the failures.

- `prove_goal` validates referenced node IDs and types, but does not enforce a
  goal's requested finding, traffic binding, or current saved report. A correct
  token observation can therefore support an incomplete composite-goal proof.
  Reporter activity can continue after the task is marked done; reporter
  cancellation does not explain the reviewed English report-save failure.
- `get_finding_traffic` enters list mode when `binding_id` is omitted, but an
  explicit empty string enters lookup mode for ID zero. Both editions encounter
  this shared optional-argument trap. The English first positive run subsequently
  guessed report evidence version zero against version-one evidence; the server
  correctly rejected those stale saves. This is distinct from the previously
  fixed optional `evidence_hint_id` contract.
- English positive repeat 2 reused an empty traffic search from before its HTTP
  request to claim no capture existed afterward. Saved traffic disproves that
  claim. This is a stale-evidence reasoning error, not a demonstrated capture race.
- Task terminal states are `done`, `failed`, and `timeout`; blocked/exhausted are
  intent states. The planner's highest-priority instruction to dispatch again
  whenever goals are unmet and the frontier is empty conflicts with its bounded
  recheck guidance. A persistent HTTP 503 thus leaves no clean task-level
  inconclusive outcome in the exercised path.

A follow-up should test deliverable-aware completion; consistent omitted/empty/
null evidence-ID handling with malformed and foreign-task rejection; post-request
traffic lookup and version-correct report saving; and bounded rechecks with a
persisted inconclusive or awaiting-condition outcome that keeps goals unmet.
Run a newly versioned comparison after those changes, retaining this pilot's
failures. More model retries alone would not resolve these contract/state issues.
