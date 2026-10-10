#!/usr/bin/env python3
"""Opt-in synthetic OpenRouter evaluation. Never executes model tool calls.

Prompts come from the offline Go exporter; results and credentials stay outside
tracked source. Network destinations are fixed here, never taken from fixtures
or model output. No automatic retries; every attempt reserves its full bound.
"""

import argparse
import datetime as dt
from decimal import Decimal
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
WORK = ROOT / "work/localization-eval"
MODEL = "openai/gpt-6-luna"
MODEL_IDS = {MODEL, "openai/gpt-6-luna-20260922", "gpt-6-luna", "gpt-6-luna-20260922"}
MODEL_CONFIG = {
    MODEL: {"ids": MODEL_IDS, "prompt_per_million": Decimal("0.20"), "output_per_million": Decimal("1")},
    "openai/gpt-6-sol": {
        "ids": {"openai/gpt-6-sol", "openai/gpt-6-sol-20260922", "gpt-6-sol", "gpt-6-sol-20260922"},
        "prompt_per_million": Decimal("4"), "output_per_million": Decimal("20"),
    },
}
ENDPOINT = "https://openrouter.ai/api/v1/chat/completions"
# Internal USD ceiling remains below the owner's EUR 3 overall authorization.
# Existing attempts retain their full reservations across all fixture sets.
CAP = Decimal("3.00")
MAX_OUTPUT = 1536
MAX_BODY_BYTES = 80000
MAX_RESPONSE_BYTES = 1024 * 1024
# USD/token, matching the enforced provider max_price ceilings below. These
# exceed the catalog price to cover cache-write and regional price differences.
INPUT_CEILING = Decimal("0.0000002")
OUTPUT_CEILING = Decimal("0.000001")


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def atomic_json(path, value):
    temp = path.with_suffix(path.suffix + ".tmp")
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "wb") as out:
        out.write(encode(value))
        out.flush()
        os.fsync(out.fileno())
    os.replace(temp, path)


def reservation(body):
    raw = encode(body)
    if len(raw) > MAX_BODY_BYTES or body.get("max_tokens") != MAX_OUTPUT:
        raise ValueError("Request exceeds the validated size/output limits")
    # Deliberately overestimate byte-tokenized text and JSON/schema framing.
    # Remain far below the model's 272K-token long-context price threshold.
    config = MODEL_CONFIG[body.get("model", MODEL)]
    return (Decimal(2 * len(raw) + 4096) * config["prompt_per_million"]
            + Decimal(MAX_OUTPUT) * config["output_per_million"]) / Decimal(1000000)


class Budget:
    def __init__(self, path):
        self.path = path
        self.data = json.loads(path.read_text()) if path.exists() else {"cap_usd": str(CAP), "attempts": []}
        if Decimal(self.data["cap_usd"]) != CAP:
            raise ValueError("Existing ledger has a different cap")

    @property
    def reserved(self):
        return sum((Decimal(a["reserved_usd"]) for a in self.data["attempts"]), Decimal(0))

    def reserve(self, job_id, bound):
        if not bound.is_finite() or bound <= 0:
            raise ValueError("Invalid reservation")
        if self.reserved + bound > CAP:
            raise ValueError("Conservative budget exhausted; no request sent")
        attempt = {"job_id": job_id, "reserved_usd": str(bound), "state": "pending",
                   "started_utc": dt.datetime.now(dt.timezone.utc).isoformat()}
        self.data["attempts"].append(attempt)
        atomic_json(self.path, self.data)  # Durable before touching the network.
        return attempt

    def finish(self, attempt, state, cost=None):
        attempt["state"] = state
        if cost is not None:
            attempt["reported_cost_usd"] = str(cost)
        atomic_json(self.path, self.data)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("Redirect refused; credential destination is fixed")


def keychain_key(service, account):
    result = subprocess.run(["/usr/bin/security", "find-generic-password", "-s", service,
                             "-a", account, "-w"], capture_output=True, timeout=120)
    if result.returncode or not result.stdout.strip():
        raise ValueError("The requested Keychain entry is unavailable")
    return result.stdout.decode().strip()


def scrub(value, secret):
    if isinstance(value, str):
        return value.replace(secret, "[REDACTED]") if secret else value
    if isinstance(value, list):
        return [scrub(item, secret) for item in value]
    if isinstance(value, dict):
        return {scrub(k, secret): scrub(v, secret) for k, v in value.items()}
    return value


def read_reply(response):
    raw = response.read(MAX_RESPONSE_BYTES + 1)
    if len(raw) > MAX_RESPONSE_BYTES:
        raise ValueError("Provider response exceeds size limit")
    return json.loads(raw)


def schema_errors(value, schema, path="$"):
    errors = []
    kind = schema.get("type")
    types = {"object": dict, "array": list, "string": str, "boolean": bool, "integer": int}
    if kind in types and (not isinstance(value, types[kind]) or (kind == "integer" and isinstance(value, bool))):
        return [path + ":type"]
    if "enum" in schema and value not in schema["enum"]:
        errors.append(path + ":enum")
    if isinstance(value, str) and len(value) < schema.get("minLength", 0):
        errors.append(path + ":minLength")
    if isinstance(value, list) and len(value) < schema.get("minItems", 0):
        errors.append(path + ":minItems")
    if isinstance(value, dict):
        errors += [path + "." + k + ":required" for k in schema.get("required", []) if k not in value]
        for key, child in schema.get("properties", {}).items():
            if key in value:
                errors += schema_errors(value[key], child, path + "." + key)
        if schema.get("additionalProperties") is False:
            errors += [path + "." + k + ":extra" for k in value if k not in schema.get("properties", {})]
    if isinstance(value, list):
        for i, item in enumerate(value):
            errors += schema_errors(item, schema.get("items", {}), f"{path}[{i}]")
    return errors


def script_counts(text):
    import unicodedata
    counts = {"han": 0, "hangul": 0, "letters": 0}
    for char in text:
        name = unicodedata.name(char, "")
        counts["han"] += "CJK" in name or "IDEOGRAPH" in name
        counts["hangul"] += "HANGUL" in name
        counts["letters"] += char.isalpha()
    return counts


def grade(fixture, edition, response):
    expected = fixture["expected"]
    choice = response["choices"][0]
    message = choice["message"]
    content = message.get("content") or ""
    result = {"contract_errors": [], "natural_language": [], "review_required": True}
    if choice.get("finish_reason") not in {"stop", "tool_calls"}:
        result["contract_errors"].append("incomplete_completion")
    value = None
    try:
        if fixture["output_type"] == "json":
            value = json.loads(content)
            result["contract_errors"] += schema_errors(value, expected["schema"])
            if isinstance(value, dict):
                for key in ["assessment", "scope_status"]:
                    if key in expected and value.get(key) != expected[key]:
                        result["contract_errors"].append("expected_" + key)
                if "evidence" in expected and value.get("evidence") != expected["evidence"]:
                    result["contract_errors"].append("evidence_changed")
                result["natural_language"] = [value.get(k, "") for k in expected.get("language_fields", ["summary"])]
            if message.get("tool_calls"):
                result["contract_errors"].append("unexpected_tool_call")
        elif fixture["output_type"] == "tool_call":
            calls = message.get("tool_calls") or []
            if len(calls) != 1 or calls[0].get("function", {}).get("name") != expected["tool_name"]:
                result["contract_errors"].append("expected_tool_call")
            else:
                value = json.loads(calls[0]["function"]["arguments"])
                goals = value.get("goals") or ([value] if "text" in value else [])
                if not goals or any(not isinstance(g.get("text"), str) or not g["text"].strip() for g in goals):
                    result["contract_errors"].append("missing_goal_text")
                rules = expected.get("arguments", {})
                if "goal_count" in rules and len(goals) != rules["goal_count"]:
                    result["contract_errors"].append("goal_count")
                if rules.get("accept_single_text") is False and "goals" not in value:
                    result["contract_errors"].append("expected_goals_array")
                if rules.get("vulnclass_empty_or_absent") and any(g.get("vulnclass") not in {None, ""} for g in goals):
                    result["contract_errors"].append("unexpected_vulnclass")
                result["natural_language"] = [g["text"] for g in goals if isinstance(g, dict) and isinstance(g.get("text"), str)]
        else:
            value = content
            result["natural_language"] = [content]
            if message.get("tool_calls"):
                result["contract_errors"].append("unexpected_tool_call")
    except (ValueError, TypeError, KeyError, AttributeError):
        result["contract_errors"].append("invalid_structured_output")
    searchable = json.dumps(value, ensure_ascii=False) if not isinstance(value, str) else value
    result["missing_literals"] = [x for x in expected.get("exact_literals", []) if x not in (searchable or "")]
    # This is only a drift screen, not proof of English fluency or a refusal judge.
    prose = "\n".join(x for x in result["natural_language"] if isinstance(x, str))
    for literal in expected.get("exact_literals", []):
        prose = prose.replace(literal, "")
    for evidence in expected.get("evidence", []):
        prose = prose.replace(evidence["raw"], "")
    counts = script_counts(prose)
    result["script_counts"] = counts
    result["script_screen"] = ("empty" if counts["letters"] == 0 else
        "drift" if (edition == "en" and (counts["han"] or counts["hangul"])) or
        (edition == "ko" and not counts["hangul"]) else "clear")
    return result


def build_request(fixture, export, model=MODEL):
    role = fixture["role"]
    base_role = export.get("base_roles", {}).get(role, role)
    prompt = export["prompts"][base_role]
    config = MODEL_CONFIG[model]
    body = {"model": model, "messages": [{"role": "system", "content": prompt}],
            "max_tokens": MAX_OUTPUT, "stream": False, "reasoning_effort": "none",
            "provider": {"only": ["openai"], "allow_fallbacks": False, "require_parameters": True,
                         "max_price": {"prompt": float(config["prompt_per_million"]),
                                       "completion": float(config["output_per_million"]), "request": 0}}}
    body["messages"].extend(fixture.get("transcript", []))
    body["messages"].append({"role": "user", "content": fixture["user"]})
    if role in export.get("base_roles", {}):
        # Production settlement instructions arrive as a user turn. Do not
        # promote them into the system prompt merely for this evaluation.
        body["messages"].append({"role": "user", "content": export["instructions"][role]})
    if fixture["output_type"] == "tool_call":
        name = fixture["expected"]["tool_name"]
        body["tools"] = [export["tools"][name]]
        body["tool_choice"] = "auto"  # Observe selection; do not force a passing call.
    return body


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--execute", action="store_true", help="Opt in to billed model requests")
    parser.add_argument("--keychain-service")
    parser.add_argument("--keychain-account", default="api-key")
    parser.add_argument("--model", choices=sorted(MODEL_CONFIG), default=MODEL)
    parser.add_argument("--fixture", help="Run one exact fixture ID without changing its contents")
    parser.add_argument("--fixture-set", choices=["v1", "v2", "english", "progress"], default="english",
                        help="English-only standard suite (default), historical v1/v2, or post-hoc progress diagnosis")
    parser.add_argument("--repeats", type=int, default=3)
    parser.add_argument("--limit", type=int, default=0, help="Limit new requests, for an initial smoke run")
    args = parser.parse_args()
    if args.repeats not in {1, 2, 3} or args.limit < 0:
        parser.error("Use one to three repeats and a nonnegative limit")
    WORK.mkdir(parents=True, exist_ok=True)
    WORK.chmod(0o700)
    fixture_name = {"v1": "localization_fixtures.json", "v2": "localization_fixtures_v2.json",
                    "english": "localization_fixtures_english.json",
                    "progress": "localization_fixtures_progress.json"}[args.fixture_set]
    fixtures = json.loads((ROOT / "scripts" / fixture_name).read_text())["fixtures"]
    if args.fixture:
        fixtures = [f for f in fixtures if f["id"] == args.fixture]
        if not fixtures:
            parser.error("Requested fixture ID is not in the selected fixture set")
    english_only = args.fixture_set in {"english", "progress"}
    languages = ["en"] if english_only else ["ko", "en"]
    export_dir = WORK / "english-only" if english_only else WORK
    exports = {lang: json.loads((export_dir / f"prompts-{lang}.json").read_text()) for lang in languages}
    if english_only:
        counts = script_counts(json.dumps(exports, ensure_ascii=False))
        if counts["han"] or counts["hangul"]:
            raise ValueError("English-only evaluation requires an English-only prompt export")
    jobs = []
    for repeat in range(args.repeats):
        for fixture in fixtures:
            for lang in languages if repeat % 2 == 0 else list(reversed(languages)):
                body = build_request(fixture, exports[lang], args.model)
                identity = {"fixture": fixture["id"], "edition": lang, "repeat": repeat,
                            "request_sha256": hashlib.sha256(encode(body)).hexdigest()}
                jobs.append((identity, fixture, body, reservation(body)))
    bound = sum((job[3] for job in jobs), Decimal(0))
    print(json.dumps({"model": args.model, "planned_requests": len(jobs), "maximum_reserved_usd": str(bound),
                      "cap_usd": str(CAP), "execute": args.execute}), flush=True)
    if bound > CAP:
        raise ValueError("Whole planned run exceeds conservative budget; reduce repeats")
    if not args.execute:
        return 0
    if not args.keychain_service:
        parser.error("--keychain-service is required for execution")
    with (WORK / "run.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        budget = Budget(WORK / "budget.json")
        secret = keychain_key(args.keychain_service, args.keychain_account)
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        records_path = WORK / "results.jsonl"
        done = {a["job_id"] for a in budget.data["attempts"]}
        sent = 0
        for identity, fixture, body, bound in jobs:
            job_id = hashlib.sha256(encode(identity)).hexdigest()
            if job_id in done:
                continue  # Unknown/failed attempts are not retried implicitly.
            if args.limit and sent >= args.limit:
                break
            attempt = budget.reserve(job_id, bound)
            record = {**identity, "job_id": job_id, "reserved_usd": str(bound)}
            try:
                request = urllib.request.Request(ENDPOINT, data=encode(body), method="POST",
                    headers={"Authorization": "Bearer " + secret, "Content-Type": "application/json",
                             "X-OpenRouter-Title": "ARTEX synthetic localization evaluation"})
                with opener.open(request, timeout=75) as response:
                    reply = read_reply(response)
                cost = Decimal(str(reply.get("usage", {}).get("cost")))
                if not cost.is_finite() or cost < 0 or cost > bound:
                    raise ValueError("Provider usage exceeds reservation or is invalid")
                record["response"] = scrub(reply, secret)
                if reply.get("model") not in MODEL_CONFIG[args.model]["ids"]:
                    raise ValueError("Returned model differs from the requested model")
                record["grade"] = grade(fixture, identity["edition"], record["response"])
                record["state"] = "completed"
                budget.finish(attempt, "completed", cost)
            except Exception as exc:
                # Do not print provider bodies, credential subprocess output, or
                # arbitrary exception text. A failed request retains its bound.
                record["state"] = "failed"
                record["error_type"] = type(exc).__name__
                if isinstance(exc, urllib.error.HTTPError):
                    record["http_status"] = exc.code
                budget.finish(attempt, "failed")
            fd = os.open(records_path, os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
            with os.fdopen(fd, "ab") as out:
                out.write(encode(record) + b"\n")
                out.flush()
                os.fsync(out.fileno())
            sent += 1
            print(json.dumps({**identity, "state": record["state"], "reserved_total_usd": str(budget.reserved),
                              "checks": record.get("grade", {}).get("contract_errors", []),
                              "script_screen": record.get("grade", {}).get("script_screen")}), flush=True)
            if record["state"] != "completed":
                return 2  # Stop on unknown costs, API errors, or unexpected models.
        return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        print("Evaluation stopped: " + type(exc).__name__, file=sys.stderr)
        sys.exit(2)
