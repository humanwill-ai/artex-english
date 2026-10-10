"""Offline regressions for the opt-in localization evaluation runner.

All provider and credential access is mocked; fixtures are newly invented here.
"""
import contextlib
import copy
from decimal import Decimal
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import urllib.request

spec = importlib.util.spec_from_file_location("localization_eval", Path(__file__).with_name("run_localization_eval.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


def json_fixture():
    return {
        "id": "offline-test", "role": "chat", "output_type": "json", "user": "Review the supplied record only.",
        "expected": {
            "schema": {"type": "object", "additionalProperties": False,
                       "required": ["summary", "evidence", "assessment", "scope_status"],
                       "properties": {"summary": {"type": "string", "minLength": 1},
                                      "evidence": {"type": "array", "items": {"type": "object"}},
                                      "assessment": {"type": "string", "enum": ["supported", "inconclusive"]},
                                      "scope_status": {"type": "string", "enum": ["in_scope", "out_of_scope"]}}},
            "evidence": [{"id": "TEST-01", "raw": "HTTP/1.1 200 OK\r\n상태=준비됨; 结果=示例\r\n"}],
            "assessment": "supported", "scope_status": "in_scope", "exact_literals": ["TEST-01"],
            "language_fields": ["summary"],
        },
    }


def json_value():
    return {"summary": "The saved record reports ready.", "evidence": json_fixture()["expected"]["evidence"],
            "assessment": "supported", "scope_status": "in_scope"}


def reply(content=None, calls=None, model=None, cost="0.001"):
    message = {"role": "assistant", "content": content}
    if calls is not None:
        message["tool_calls"] = calls
    return {"model": model or runner.MODEL, "usage": {"cost": cost},
            "choices": [{"finish_reason": "tool_calls" if calls else "stop", "message": message}]}


def goal_fixture(count=1):
    return {"id": "goal-test", "role": "goals", "output_type": "tool_call", "user": "Describe a final objective.",
            "expected": {"tool_name": "set_goals", "arguments": {"goal_count": count,
                "accept_single_text": count == 1, "require_nonempty_text": True, "vulnclass_empty_or_absent": True},
                "exact_literals": ["TEST-01"], "language_fields": ["text", "goals[].text"], "with_scope": False}}


def goal_reply(arguments):
    return reply(calls=[{"id": "call_test", "type": "function", "function": {
        "name": "set_goals", "arguments": json.dumps(arguments)}}])


class BudgetTests(unittest.TestCase):
    def test_sol_changes_only_model_and_price_limits(self):
        fixture = json_fixture()
        export = {"prompts": {"chat": "Identical original system prompt."}}
        luna = runner.build_request(fixture, export)
        sol = runner.build_request(fixture, export, "openai/gpt-6-sol")
        self.assertEqual(sol["reasoning_effort"], luna["reasoning_effort"])
        comparable = copy.deepcopy(sol)
        comparable["model"] = luna["model"]
        comparable["provider"]["max_price"] = luna["provider"]["max_price"]
        self.assertEqual(comparable, luna)
        raw = runner.encode(sol)
        expected = (Decimal(2 * len(raw) + 4096) * Decimal("4")
                    + Decimal(runner.MAX_OUTPUT) * Decimal("20")) / Decimal(1000000)
        self.assertEqual(runner.reservation(sol), expected)

    def test_pending_failed_and_completed_attempts_all_keep_full_reservation(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "budget.json"
            budget = runner.Budget(path)
            pending = budget.reserve("pending", Decimal("0.30"))
            failed = budget.reserve("failed", Decimal("0.40"))
            completed = budget.reserve("completed", Decimal("0.50"))
            budget.finish(failed, "failed")
            budget.finish(completed, "completed", Decimal("0.01"))
            loaded = runner.Budget(path)
            self.assertEqual(loaded.reserved, Decimal("1.20"))
            self.assertEqual([a["state"] for a in loaded.data["attempts"]], ["pending", "failed", "completed"])
            with self.assertRaises(ValueError):
                loaded.reserve("too-much", runner.CAP - loaded.reserved + Decimal("0.01"))
            self.assertEqual(len(runner.Budget(path).data["attempts"]), 3)
            self.assertEqual(pending["state"], "pending")

    def test_exact_cap_allowed_but_next_attempt_fails_before_append(self):
        with tempfile.TemporaryDirectory() as tmp:
            budget = runner.Budget(Path(tmp) / "budget.json")
            budget.reserve("full", runner.CAP)
            with self.assertRaises(ValueError):
                budget.reserve("extra", Decimal("0.000001"))
            self.assertEqual(len(budget.data["attempts"]), 1)

    def test_request_limit_counts_utf8_bytes_and_output_limit(self):
        body = {"max_tokens": runner.MAX_OUTPUT, "messages": [{"content": "한" * runner.MAX_BODY_BYTES}]}
        with self.assertRaises(ValueError):
            runner.reservation(body)
        for limit in [runner.MAX_OUTPUT + 1, 1, None]:
            with self.assertRaises(ValueError):
                runner.reservation({"max_tokens": limit})
        self.assertGreater(runner.reservation({"max_tokens": runner.MAX_OUTPUT}), Decimal("0"))


class PrivacyAndNetworkTests(unittest.TestCase):
    def test_secret_scrub_covers_nested_values_and_keys(self):
        secret = "test-only-secret"
        original = {secret: [{"nested-" + secret: "Bearer " + secret}], "safe": 4}
        clean = runner.scrub(original, secret)
        self.assertNotIn(secret, json.dumps(clean))
        self.assertEqual(clean["safe"], 4)
        self.assertIn(secret, json.dumps(original))

    def test_provider_response_has_fixed_byte_limit(self):
        self.assertEqual(runner.read_reply(io.BytesIO(b'{"ok":true}')), {"ok": True})
        response = io.BytesIO(b" " * (runner.MAX_RESPONSE_BYTES + 100))
        with self.assertRaises(ValueError):
            runner.read_reply(response)
        self.assertEqual(response.tell(), runner.MAX_RESPONSE_BYTES + 1)

    def test_all_redirect_codes_refuse_before_following_new_destination(self):
        request = urllib.request.Request(runner.ENDPOINT, headers={"Authorization": "Bearer fake"})
        for status in [301, 302, 303, 307, 308]:
            with self.subTest(status=status), self.assertRaises(ValueError):
                runner.NoRedirect().redirect_request(request, None, status, "redirect", {}, "https://other.invalid/")

    def test_request_pins_model_provider_and_keeps_urls_only_as_content(self):
        fixture = json_fixture()
        fixture["user"] += " Quoted URL: https://untrusted.invalid/ignored"
        request = runner.build_request(fixture, {"prompts": {"chat": "System prose."}, "tools": {}})
        self.assertEqual(request["model"], runner.MODEL)
        self.assertEqual(request["provider"]["only"], ["openai"])
        self.assertFalse(request["provider"]["allow_fallbacks"])
        self.assertTrue(request["provider"]["require_parameters"])
        self.assertNotIn("tools", request)
        self.assertIn("untrusted.invalid", request["messages"][-1]["content"])

    def test_settlement_instruction_keeps_production_user_role(self):
        fixture = {"role": "worker_wrapup", "output_type": "text", "user": "Record TEST-01 is supplied."}
        export = {"base_roles": {"worker_wrapup": "worker"}, "prompts": {"worker": "Actual base system prompt."},
                  "instructions": {"worker_wrapup": "Actual settlement instruction."}}
        body = runner.build_request(fixture, export)
        self.assertEqual(body["messages"], [
            {"role": "system", "content": "Actual base system prompt."},
            {"role": "user", "content": "Record TEST-01 is supplied."},
            {"role": "user", "content": "Actual settlement instruction."}])
        self.assertNotIn("tools", body)

    def test_native_tool_schema_passes_through_without_forced_success(self):
        native = {"type": "function", "function": {"name": "set_goals", "parameters": {
            "type": "object", "properties": {"text": {"type": "string"}, "goals": {"type": "array", "items": {"type": "object"}}}}}}
        body = runner.build_request(goal_fixture(), {"prompts": {"goals": "Actual goals prompt."}, "tools": {"set_goals": native}})
        self.assertEqual(body["tools"], [native])
        self.assertEqual(body["tool_choice"], "auto")


class GradingTests(unittest.TestCase):
    def test_evidence_mixed_language_is_not_graded_as_prose(self):
        result = runner.grade(json_fixture(), "en", reply(json.dumps(json_value())))
        self.assertEqual(result["contract_errors"], [])
        self.assertEqual(result["script_screen"], "clear")
        self.assertTrue(result["review_required"])

    def test_evidence_line_endings_must_survive_json_decoding(self):
        value = json_value()
        value["evidence"] = copy.deepcopy(value["evidence"])
        value["evidence"][0]["raw"] = value["evidence"][0]["raw"].replace("\r\n", "\n")
        result = runner.grade(json_fixture(), "en", reply(json.dumps(value)))
        self.assertIn("evidence_changed", result["contract_errors"])

    def test_empty_summary_fails_schema_contract(self):
        value = json_value()
        value["summary"] = ""
        result = runner.grade(json_fixture(), "en", reply(json.dumps(value)))
        self.assertTrue(result["contract_errors"])
        self.assertEqual(result["script_screen"], "empty")

    def test_wrong_enums_missing_fields_and_extra_fields_fail(self):
        value = json_value()
        value["assessment"] = "취약"
        del value["scope_status"]
        value["unrequested"] = "extra"
        errors = runner.grade(json_fixture(), "en", reply(json.dumps(value)))["contract_errors"]
        self.assertTrue(any(":enum" in e for e in errors))
        self.assertTrue(any(":required" in e for e in errors))
        self.assertTrue(any(":extra" in e for e in errors))

    def test_own_prose_language_drift_is_detected_but_not_semantically_judged(self):
        value = json_value()
        value["summary"] = "The saved 상태 record is ready."
        self.assertEqual(runner.grade(json_fixture(), "en", reply(json.dumps(value)))["script_screen"], "drift")
        self.assertEqual(runner.grade(json_fixture(), "ko", reply(json.dumps(json_value())))["script_screen"], "drift")
        value["summary"] = "저장된 기록은 준비 상태입니다."
        self.assertEqual(runner.grade(json_fixture(), "ko", reply(json.dumps(value)))["script_screen"], "clear")

    def test_required_raw_literal_is_exempt_in_plain_wrapup(self):
        fixture = {"output_type": "text", "expected": {"exact_literals": ["TEST-01", "상태=준비됨"], "sentence_count": 1}}
        result = runner.grade(fixture, "en", reply("TEST-01 records 상태=준비됨; current availability remains unknown."))
        self.assertEqual(result["missing_literals"], [])
        self.assertEqual(result["script_screen"], "clear")
        self.assertTrue(result["review_required"])

    def test_valid_single_and_array_goal_forms(self):
        for args in [{"text": "Assess saved TEST-01."}, {"goals": [{"text": "Assess saved TEST-01.", "vulnclass": ""}]}]:
            with self.subTest(args=args):
                result = runner.grade(goal_fixture(), "en", goal_reply(args))
                self.assertEqual(result["contract_errors"], [])

    def test_two_outcomes_cannot_pass_as_one_combined_goal(self):
        args = {"goals": [{"text": "Assess TEST-01 status and maintenance."}]}
        self.assertTrue(runner.grade(goal_fixture(2), "en", goal_reply(args))["contract_errors"])

    def test_nonempty_vulnerability_class_is_invalid_for_benign_goal(self):
        args = {"text": "Assess saved TEST-01.", "vulnclass": "invented-vulnerability"}
        self.assertTrue(runner.grade(goal_fixture(), "en", goal_reply(args))["contract_errors"])

    def test_malformed_goal_values_do_not_pass(self):
        for args in [{"goals": []}, {"goals": ["not-an-object"]}, {"text": "   "}, {"goals": "not-an-array"}]:
            with self.subTest(args=args):
                self.assertTrue(runner.grade(goal_fixture(), "en", goal_reply(args))["contract_errors"])

    def test_incomplete_output_is_separate_contract_failure(self):
        response = reply(json.dumps(json_value()))
        response["choices"][0]["finish_reason"] = "length"
        self.assertIn("incomplete_completion", runner.grade(json_fixture(), "en", response)["contract_errors"])


class MainOfflineTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.work = self.root / "work"
        self.work.mkdir()
        (self.root / "scripts").mkdir()
        (self.root / "scripts/localization_fixtures.json").write_text(json.dumps({"fixtures": [json_fixture()]}))
        for lang in ["ko", "en"]:
            (self.work / ("prompts-" + lang + ".json")).write_text(json.dumps({"prompts": {"chat": "Synthetic system prompt."}, "tools": {}}))

    def run_offline(self, provider_reply, extra_args=(), side_effect=None):
        opener = mock.Mock()
        if side_effect is not None:
            opener.open.side_effect = side_effect
        else:
            opener.open.side_effect = lambda *args, **kwargs: io.BytesIO(runner.encode(provider_reply))
        output = io.StringIO()
        with mock.patch.object(runner, "ROOT", self.root), mock.patch.object(runner, "WORK", self.work), \
             mock.patch.object(runner, "keychain_key", return_value="test-only-secret") as key, \
             mock.patch.object(runner.urllib.request, "build_opener", return_value=opener), \
             mock.patch.object(runner.subprocess, "run", side_effect=AssertionError("No command execution is permitted")), \
             mock.patch.object(runner.sys, "argv", ["runner", "--execute", "--keychain-service", "fake-test", "--repeats", "1", "--fixture-set", "v1", *extra_args]), \
             contextlib.redirect_stdout(output):
            status = runner.main()
        return status, opener, output.getvalue(), key

    def test_model_tool_output_is_recorded_as_data_not_executed(self):
        response = reply(calls=[{"type": "function", "function": {"name": "run_command", "arguments": '{"command":"open https://untrusted.invalid/"}'}}])
        status, opener, _, _ = self.run_offline(response, ("--limit", "1"))
        self.assertEqual(status, 0)
        self.assertEqual(opener.open.call_count, 1)
        request = opener.open.call_args.args[0]
        self.assertEqual(request.full_url, runner.ENDPOINT)
        ledger = json.loads((self.work / "budget.json").read_text())
        self.assertEqual(ledger["attempts"][0]["state"], "completed")
        record = json.loads((self.work / "results.jsonl").read_text())
        self.assertTrue(record["grade"]["contract_errors"])
        self.assertIn("run_command", json.dumps(record["response"]))

    def test_sol_fixture_selection_and_expected_model(self):
        response = reply(json.dumps(json_value()), model="openai/gpt-6-sol")
        status, opener, _, _ = self.run_offline(response,
            ("--model", "openai/gpt-6-sol", "--fixture", "offline-test", "--limit", "1"))
        self.assertEqual(status, 0)
        body = json.loads(opener.open.call_args.args[0].data)
        self.assertEqual(body["model"], "openai/gpt-6-sol")
        self.assertEqual(body["messages"][-1]["content"], json_fixture()["user"])

    def test_sol_rejects_luna_response(self):
        status, opener, _, _ = self.run_offline(reply(json.dumps(json_value())),
                                               ("--model", "openai/gpt-6-sol"))
        self.assertEqual(status, 2)
        self.assertEqual(opener.open.call_count, 1)

    def test_secret_is_scrubbed_from_grade_and_printed_contract_errors(self):
        value = json_value()
        value["summary"] = "The saved record contains test-only-secret."
        value["test-only-secret"] = "unexpected field"
        response = reply(json.dumps(value))
        status, _, output, _ = self.run_offline(response, ("--limit", "1"))
        self.assertEqual(status, 0)
        self.assertNotIn("test-only-secret", output + (self.work / "results.jsonl").read_text())

    def test_wrong_model_stops_without_second_call_and_reservation_stays(self):
        status, opener, _, _ = self.run_offline(reply(json.dumps(json_value()), model="some-other-model"))
        self.assertEqual(status, 2)
        self.assertEqual(opener.open.call_count, 1)
        budget = runner.Budget(self.work / "budget.json")
        self.assertGreater(budget.reserved, 0)
        self.assertEqual(budget.data["attempts"][0]["state"], "failed")

    def test_timeout_reserves_attempt_and_rerun_does_not_retry_it(self):
        status, opener, output, _ = self.run_offline(None, side_effect=TimeoutError("test-only-secret"))
        self.assertEqual(status, 2)
        self.assertEqual(opener.open.call_count, 1)
        before = runner.Budget(self.work / "budget.json")
        first_id = before.data["attempts"][0]["job_id"]
        status, second_opener, _, _ = self.run_offline(reply(json.dumps(json_value())), ("--limit", "1"))
        self.assertEqual(status, 0)
        self.assertEqual(second_opener.open.call_count, 1)
        after = runner.Budget(self.work / "budget.json")
        self.assertEqual(len(after.data["attempts"]), 2)
        self.assertEqual(after.data["attempts"][0]["job_id"], first_id)
        self.assertEqual(after.data["attempts"][0]["state"], "failed")
        self.assertNotIn("test-only-secret", output + (self.work / "results.jsonl").read_text())

    def test_missing_or_unbounded_cost_stops_and_preserves_reservation(self):
        for cost in [None, "NaN", "Infinity", "-0.01", "100"]:
            with self.subTest(cost=cost):
                for filename in ["budget.json", "results.jsonl"]:
                    (self.work / filename).unlink(missing_ok=True)
                status, opener, _, _ = self.run_offline(reply(json.dumps(json_value()), cost=cost))
                self.assertEqual(status, 2)
                self.assertEqual(opener.open.call_count, 1)
                self.assertGreater(runner.Budget(self.work / "budget.json").reserved, 0)

    def test_durable_pending_reservation_exists_before_network(self):
        def inspect_then_timeout(*args, **kwargs):
            attempts = json.loads((self.work / "budget.json").read_text())["attempts"]
            self.assertEqual(attempts[-1]["state"], "pending")
            raise TimeoutError("synthetic timeout")
        status, _, _, _ = self.run_offline(None, side_effect=inspect_then_timeout)
        self.assertEqual(status, 2)

    def test_english_suite_uses_new_export_only_and_shared_budget(self):
        self.run_offline(reply(json.dumps(json_value())), ("--limit", "1"))
        followup = json_fixture()
        followup["id"] = "offline-test-english"
        (self.root / "scripts/localization_fixtures_english.json").write_text(json.dumps({"fixtures": [followup]}))
        directory = self.work / "english-only"
        directory.mkdir()
        (directory / "prompts-en.json").write_text(json.dumps({"prompts": {"chat": "New English core prompt."}, "tools": {}}))
        status, opener, _, _ = self.run_offline(reply(json.dumps(json_value())), ("--fixture-set", "english"))
        self.assertEqual(status, 0)
        self.assertEqual(opener.open.call_count, 1)
        body = json.loads(opener.open.call_args.args[0].data)
        self.assertEqual(body["messages"][0]["content"], "New English core prompt.")
        self.assertEqual(len(runner.Budget(self.work / "budget.json").data["attempts"]), 2)

    def test_progress_diagnostic_uses_english_export_and_existing_ledger(self):
        self.run_offline(reply(json.dumps(json_value())), ("--limit", "1"))
        before = runner.Budget(self.work / "budget.json").reserved
        fixture = json_fixture()
        fixture["id"] = "offline-progress-diagnostic"
        (self.root / "scripts/localization_fixtures_progress.json").write_text(json.dumps({"fixtures": [fixture]}))
        directory = self.work / "english-only"
        directory.mkdir()
        (directory / "prompts-en.json").write_text(json.dumps({"prompts": {"chat": "Same English production prompt."}, "tools": {}}))
        status, opener, _, _ = self.run_offline(reply(json.dumps(json_value())), ("--fixture-set", "progress"))
        self.assertEqual(status, 0)
        self.assertEqual(opener.open.call_count, 1)
        body = json.loads(opener.open.call_args.args[0].data)
        self.assertEqual(body["messages"][0]["content"], "Same English production prompt.")
        budget = runner.Budget(self.work / "budget.json")
        self.assertEqual(len(budget.data["attempts"]), 2)
        self.assertGreater(budget.reserved, before)

    def test_english_suite_rejects_untranslated_prompt_export(self):
        (self.root / "scripts/localization_fixtures_english.json").write_text(json.dumps({"fixtures": [json_fixture()]}))
        directory = self.work / "english-only"
        directory.mkdir()
        (directory / "prompts-en.json").write_text(json.dumps({"prompts": {"chat": "旧版提示"}, "tools": {}}))
        with self.assertRaisesRegex(ValueError, "English-only prompt export"):
            self.run_offline(reply(json.dumps(json_value())), ("--fixture-set", "english"))
        self.assertFalse((self.work / "budget.json").exists())

    def test_versioned_followup_uses_selected_fixture_and_same_budget(self):
        self.run_offline(reply(json.dumps(json_value())), ("--limit", "1"))
        before = runner.Budget(self.work / "budget.json").reserved
        followup = json_fixture()
        followup["id"] = "offline-test-v2"
        followup["user"] = "Assess this explicit positive proposition using supplied records only."
        (self.root / "scripts/localization_fixtures_v2.json").write_text(json.dumps({"fixtures": [followup]}))
        status, opener, _, _ = self.run_offline(reply(json.dumps(json_value())),
                                              ("--fixture-set", "v2", "--limit", "1"))
        self.assertEqual(status, 0)
        body = json.loads(opener.open.call_args.args[0].data)
        self.assertEqual(body["messages"][-1]["content"], followup["user"])
        budget = runner.Budget(self.work / "budget.json")
        self.assertEqual(len(budget.data["attempts"]), 2)
        self.assertGreater(budget.reserved, before)
        records = [json.loads(line) for line in (self.work / "results.jsonl").read_text().splitlines()]
        self.assertEqual(records[-1]["fixture"], followup["id"])


if __name__ == "__main__":
    unittest.main()
