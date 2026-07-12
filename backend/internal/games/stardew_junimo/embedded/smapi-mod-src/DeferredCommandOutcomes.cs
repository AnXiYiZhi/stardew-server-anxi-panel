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

        pending = command;
        deadline = now.Add(timeout);
        return Create(command, CommandStatuses.Running, "", "The game save request is registered and is waiting for GameLoop.Saved.", now);
    }

    public CommandOutcome? Complete(DateTimeOffset now)
    {
        if (pending is null)
            return null;

        var command = pending;
        pending = null;
        deadline = default;
        return Create(command, CommandStatuses.Succeeded, "ok", "GameLoop.Saved confirmed that the requested game save completed.", now);
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
