package agent

import (
	"context"
	"errors"
	"fmt"
)

// AbortCause names why an agent run's context was cancelled. Every cancellation
// site should attach one so the activity trace can report the real initiator.
type AbortCause struct {
	Code  string
	Short string
	Text  string
}

func (c *AbortCause) Error() string { return c.Text }

func cause(code, short, text string) *AbortCause {
	return &AbortCause{Code: code, Short: short, Text: text}
}

// Causef builds a cause that includes runtime-specific detail.
func Causef(code, short, format string, args ...any) *AbortCause {
	return &AbortCause{Code: code, Short: short, Text: fmt.Sprintf(format, args...)}
}

var (
	// Task-level execution context.
	AbortPausedByUser = cause("paused_by_user", "The user paused the task",
		"The user paused the task through the task control API (POST /api/tasks/{id}/control, action=pause). This planner/worker run was deliberately canceled; running intents return to frontier(open), are claimed again after the task resumes, and run from the beginning.")
	AbortPausedByOrchestrator = cause("paused_by_orchestrator", "The orchestration agent paused the task",
		"The orchestration agent called pause_task to pause this task. This planner/worker run was deliberately canceled; running intents return to frontier(open) and execute again after resuming.")
	AbortTaskDeleted = cause("task_deleted", "The task was deleted",
		"The task is being deleted (DELETE /api/tasks/{id}). The deletion barrier canceled its running planner, workers, and main agent; results from this run will no longer be used.")
	AbortPausedOnReload = cause("paused_on_reload", "Backend restored the task pause",
		"At startup, the backend restored the task pause from persisted database state. This run was canceled; normally no agent is running during restoration.")
	AbortGoalMet = cause("goal_met", "Planner determined all goals were met",
		"The planner determined that the task goals were met and set the task to done, then canceled remaining workers. These intents are marked stopped, not failed.")
	AbortSettleDrainTimeout = cause("settle_drain_timeout", "The task timeout wrap-up wait expired",
		"After the task reached timeout, it waited for running workers to finish gracefully, but the 90-second drain grace period was insufficient, so they were forcibly canceled. Intents are marked exhausted; facts and assets already written during wrap-up are retained.")

	// Per-work context.
	AbortKilledByPlanner = cause("killed_by_planner", "The planner terminated this intent",
		"The planner called kill_work to terminate this intent, usually because the direction was off track or no longer useful. The intent is marked stopped and will not be automatically claimed again.")
	AbortWorkPausedByUser = cause("work_paused_by_user", "The user paused this worker intent",
		"The user paused the running worker. This call was canceled and the intent becomes paused; all registered intents, facts, findings, and activity records are retained, and execution restarts from the beginning after resuming.")
	AbortWorkCancelledByUser = cause("work_cancelled_by_user", "The user deleted this worker intent",
		"The user deleted the running worker. This call was canceled; after the worker leaves the write section, the server handles the intent according to the chosen deletion mode: soft deletion only marks it deleted and retains all outputs, while hard deletion cascades to remove this intent and downstream nodes supported solely by it.")
	AbortWorkFinished = cause("work_finished", "Worker finished and released context",
		"The worker finished normally, and the engine releases its context resources in detachWork. This is not a run interruption; if it appears in an interruption message, cancellation raced with settlement events.")
	AbortPausedRaceGuard = cause("paused_race_guard", "New run rejected: task paused",
		"While the task is paused, the engine refuses to issue a new execution context, preventing a race between claiming and pausing from starting a worker. Already claimed intents return to frontier.")

	// Main Agent and standalone conversation contexts.
	AbortChatStoppedByUser = cause("chat_stopped_by_user", "The user stopped this conversation turn",
		"The user clicked Stop, deliberately stopping this main-agent or chat-agent run. Existing activity records are retained, and the user can send another message.")
	AbortChatPausedWithTask = cause("chat_paused_with_task", "Task pause stopped main-agent chat",
		"When the user paused the task, the running main-agent conversation was canceled as well. Existing activity records are retained; resuming the task does not automatically replay this message.")
	AbortChatTurnFinished = cause("chat_turn_finished", "Turn finished and released context",
		"This conversation turn finished normally, and the server is releasing its context resources. This is not a run interruption; if it appears in an interruption message, cancellation raced with settlement events.")

	// Process-level and per-run hard backstop.
	AbortShutdown = cause("shutdown", "The backend process is shutting down",
		"The backend process received SIGINT or SIGTERM and is restarting, updating, or shutting down. All running agents are canceled; remaining running intents are reset to open and rerun after restart.")
	AbortRunHardTimeout = cause("run_hard_timeout", "Run hard-timeout fallback triggered",
		"This run exceeded its soft wall-clock budget and additional grace period, indicating that a model request or tool has not returned for a long time and normal wrap-up at turn boundaries could not run. Check the last tool call that had not returned before interruption.")
)

// AbortReason resolves the named cause attached to a cancelled run context.
func AbortReason(ctx context.Context) (code, short, text string, ok bool) {
	c := context.Cause(ctx)
	if c == nil {
		return "", "", "", false
	}
	var ac *AbortCause
	if errors.As(c, &ac) {
		return ac.Code, ac.Short, ac.Text, true
	}
	switch {
	case errors.Is(c, context.DeadlineExceeded):
		return "deadline_exceeded", "Upstream context reached its deadline",
			"The upstream context reached its deadline, but its creator did not attach a named reason via WithTimeoutCause: " + c.Error(), true
	case errors.Is(c, context.Canceled):
		return "canceled_no_cause", "No named cancellation reason attached",
			"The upstream context was canceled, but the caller did not attach a named reason via context.WithCancelCause. Register the reason in agent/cancelcause.go and wire it into that cancellation point.", true
	default:
		return "other", firstLine(c.Error(), 80), c.Error(), true
	}
}
