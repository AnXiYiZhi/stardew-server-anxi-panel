namespace StardewAnxiPanel.Control;

public static class FestivalCommandOutcomes
{
    public static CommandOutcome? Validate(
        PanelCommand command,
        bool worldReady,
        bool festivalToday,
        bool festivalActive,
        bool chatAvailable)
    {
        if (!worldReady)
            return Failed(command, "world_not_ready", "The game world is not ready.");
        if (!festivalToday)
            return Failed(command, "no_festival_today", "There is no festival today.");
        if (!festivalActive)
            return Failed(command, "festival_not_active", "The host is not currently at the festival.");
        if (!chatAvailable)
            return Failed(command, "chat_unavailable", "The game chat system is unavailable.");
        return null;
    }

    public static CommandOutcome Dispatched(PanelCommand command) =>
        Create(command, CommandStatuses.Dispatched, "ok", "The !event command was delivered to JunimoServer; festival completion was not confirmed.");

    public static CommandOutcome Failed(PanelCommand command, string code, string message) =>
        Create(command, CommandStatuses.Failed, code, message);

    private static CommandOutcome Create(PanelCommand command, string status, string code, string message) => new()
    {
        CommandId = command.Id,
        Status = status,
        ErrorCode = code,
        Message = message,
        CreatedAt = command.CreatedAt,
        UpdatedAt = DateTimeOffset.UtcNow,
    };
}

public static class JojaCommandOutcomes
{
    public static CommandOutcome? Validate(
        PanelCommand command,
        bool worldReady,
        bool adminPromoted,
        bool chatAvailable)
    {
        if (!worldReady)
            return Failed(command, "world_not_ready", "The game world is not ready.");
        if (!adminPromoted)
            return Failed(command, "admin_promotion_failed", "JunimoServer admin promotion was not confirmed.");
        if (!chatAvailable)
            return Failed(command, "chat_unavailable", "The game chat system is unavailable.");
        return null;
    }

    public static CommandOutcome Succeeded(PanelCommand command) =>
        Create(command, CommandStatuses.Succeeded, "ok", "The saved game state already confirms the Joja membership route.");

    public static CommandOutcome Dispatched(PanelCommand command) =>
        Create(command, CommandStatuses.Dispatched, "ok", "The irreversible !joja command was delivered to JunimoServer; permanent route state was not yet confirmed.");

    public static CommandOutcome Failed(PanelCommand command, string code, string message) =>
        Create(command, CommandStatuses.Failed, code, message);

    private static CommandOutcome Create(PanelCommand command, string status, string code, string message) => new()
    {
        CommandId = command.Id,
        Status = status,
        ErrorCode = code,
        Message = message,
        CreatedAt = command.CreatedAt,
        UpdatedAt = DateTimeOffset.UtcNow,
    };
}

public sealed class PendingSaveCommandTracker
{
    private PanelCommand? pending;
    private DateTimeOffset deadline;

    public string? PendingCommandId => pending?.Id;

    public CommandOutcome Begin(PanelCommand command, bool worldReady, DateTimeOffset now, TimeSpan timeout)
    {
        if (!worldReady)
            return Create(command, CommandStatuses.Failed, "world_not_ready", "The game world is not ready.", now);
        if (pending is not null)
            return Create(command, CommandStatuses.Failed, "save_already_pending", "Another game save request is already pending.", now);
		var expectation = SaveCommandContract.ParseExpectation(command);
		if (!expectation.Valid)
			return Create(command, CommandStatuses.Failed, expectation.ErrorCode, "The save command target payload is incomplete or invalid.", now);

        pending = command;
        deadline = now.Add(timeout);
        return Create(command, CommandStatuses.Running, "", "The game save request is registered and is waiting for GameLoop.Saved.", now);
    }

    public CommandOutcome? Complete(
        string? verifiedTransactionId,
        string? actualSaveId,
        DateTimeOffset now,
        FarmhandBindingSummary? farmhandBindings = null)
    {
        if (pending is null)
            return null;

        var command = pending;
        pending = null;
        deadline = default;
		var outcome = SaveCommandContract.CompleteSavedEvent(command, actualSaveId, now, farmhandBindings);
		var expectation = SaveCommandContract.ParseExpectation(command);
		if (outcome.Status == CommandStatuses.Succeeded
			&& expectation.IsTargeted
			&& !string.Equals(expectation.TransactionId, verifiedTransactionId, StringComparison.Ordinal))
		{
			outcome.Status = CommandStatuses.Failed;
			outcome.ErrorCode = "save_transaction_mismatch";
			outcome.Message = "GameLoop.Saved completed for a save which was not verified for the command transaction.";
			outcome.Details ??= new Dictionary<string, string>();
			outcome.Details["verifiedTransactionId"] = verifiedTransactionId ?? "";
		}
		return outcome;
    }

    public CommandOutcome? Expire(DateTimeOffset now)
    {
        if (pending is null || now < deadline)
            return null;

        var command = pending;
        pending = null;
        deadline = default;
        return Create(command, CommandStatuses.Failed, "save_timeout", "The game save did not complete before the confirmation timeout.", now);
    }

    public CommandOutcome? Fail(DateTimeOffset now, string code, string message)
    {
		if (pending is null)
			return null;
		var command = pending;
		pending = null;
		deadline = default;
		return Create(command, CommandStatuses.Failed, code, message, now);
    }

    private static CommandOutcome Create(PanelCommand command, string status, string code, string message, DateTimeOffset now) => new()
    {
        CommandId = command.Id,
        Status = status,
        ErrorCode = code,
        Message = message,
        CreatedAt = command.CreatedAt,
        UpdatedAt = now,
    };
}

public sealed class PendingSaveCommandJournal
{
    public const int CurrentSchemaVersion = 1;

    public int SchemaVersion { get; set; } = CurrentSchemaVersion;
    public PanelCommand Command { get; set; } = new();
    public DateTimeOffset UpdatedAt { get; set; }
}

public sealed record SaveCommandRecoveryDecision(bool CanResume, bool TerminalFailure, string ErrorCode);

public static class SaveCommandRecoveryContract
{
    public static bool Matches(PendingSaveCommandJournal? journal, PanelCommand command)
    {
        if (journal is null || journal.SchemaVersion != PendingSaveCommandJournal.CurrentSchemaVersion)
            return false;
        if (!string.Equals(journal.Command.Id, command.Id, StringComparison.Ordinal)
            || !string.Equals(journal.Command.Name, command.Name, StringComparison.Ordinal))
            return false;

        var persisted = SaveCommandContract.ParseExpectation(journal.Command);
        var incoming = SaveCommandContract.ParseExpectation(command);
		return persisted.Valid == incoming.Valid
			&& persisted.IsTargeted == incoming.IsTargeted
			&& string.Equals(persisted.TransactionId, incoming.TransactionId, StringComparison.Ordinal)
			&& string.Equals(persisted.SaveId, incoming.SaveId, StringComparison.Ordinal)
			&& string.Equals(persisted.PreSaveAction, incoming.PreSaveAction, StringComparison.Ordinal)
			&& string.Equals(persisted.PreSaveActionSaveId, incoming.PreSaveActionSaveId, StringComparison.Ordinal);
    }

    public static SaveCommandRecoveryDecision Evaluate(
        PanelCommand command,
        bool worldReady,
        string? actualSaveId,
        string? verifiedTransactionId,
        string? verifiedSaveId)
    {
        if (string.IsNullOrWhiteSpace(command.Id) || !string.Equals(command.Name, "save-now", StringComparison.Ordinal))
            return new(false, true, "save_journal_invalid");
        var expectation = SaveCommandContract.ParseExpectation(command);
        if (!expectation.Valid)
            return new(false, true, expectation.ErrorCode);
		if (!worldReady)
			return new(false, false, "world_not_ready");
		if (expectation.PreSaveAction.Length > 0
			&& !string.Equals(expectation.PreSaveActionSaveId, actualSaveId?.Trim(), StringComparison.Ordinal))
		{
			return new(false, false, "save_target_not_ready");
		}
		if (!expectation.IsTargeted)
            return new(true, false, "");

        if (!string.Equals(expectation.SaveId, actualSaveId?.Trim(), StringComparison.Ordinal)
            || !string.Equals(expectation.SaveId, verifiedSaveId?.Trim(), StringComparison.Ordinal)
            || !string.Equals(expectation.TransactionId, verifiedTransactionId?.Trim(), StringComparison.Ordinal))
            return new(false, false, "save_target_not_ready");
        return new(true, false, "");
    }
}
