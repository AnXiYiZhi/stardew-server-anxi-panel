namespace StardewAnxiPanel.Control;

public static class PlayerCommandOutcomes
{
    public static CommandOutcome Succeeded(PanelCommand command, string message, string playerId, string playerName)
    {
        var outcome = Create(command, CommandStatuses.Succeeded, "ok", message);
        outcome.Details = new Dictionary<string, string>
        {
            ["playerId"] = playerId,
            ["playerName"] = playerName,
        };
        return outcome;
    }

    public static CommandOutcome Failed(PanelCommand command, string errorCode, string message, string playerId = "")
    {
        var outcome = Create(command, CommandStatuses.Failed, errorCode, message);
        if (!string.IsNullOrWhiteSpace(playerId))
            outcome.Details = new Dictionary<string, string> { ["playerId"] = playerId };
        return outcome;
    }

    public static CommandOutcome? ValidateTarget(
        PanelCommand command,
        string playerId,
        bool worldReady,
        bool? bridgeAvailable,
        bool playerIdValid,
        bool playerOnline,
        bool isHost)
    {
        if (!worldReady)
            return Failed(command, "world_not_ready", "The game world is not ready.", playerId);
        if (bridgeAvailable == false)
            return Failed(command, "bridge_unavailable", "The required control bridge is unavailable.", playerId);
        if (!playerIdValid)
            return Failed(command, "invalid_player_id", "The target player ID is invalid.");
        if (!playerOnline)
            return Failed(command, "player_not_online", "The target player is not online.", playerId);
        if (isHost)
            return Failed(command, "host_not_supported", "The server host is not a supported target.", playerId);
        return null;
    }

    private static CommandOutcome Create(PanelCommand command, string status, string errorCode, string message)
    {
        return new CommandOutcome
        {
            CommandId = command.Id,
            Status = status,
            ErrorCode = errorCode,
            Message = message,
            CreatedAt = command.CreatedAt,
            UpdatedAt = DateTimeOffset.UtcNow,
        };
    }
}

public sealed record BanCandidate(string PlayerId, string Name, bool IsHost);

public sealed record BanTargetResolution(BanCandidate? Target, string ErrorCode)
{
    public bool Success => Target is not null && string.IsNullOrEmpty(ErrorCode);
}

public static class BanTargetResolver
{
    public static BanTargetResolution Resolve(
        IEnumerable<BanCandidate> candidates,
        string uniqueMultiplayerId,
        bool requireUniqueNameForFallback)
    {
        var all = candidates.ToArray();
        var idMatches = all.Where(candidate => candidate.PlayerId == uniqueMultiplayerId).ToArray();
        if (idMatches.Length == 0)
            return new BanTargetResolution(null, "player_not_found");
        if (idMatches.Length > 1)
            return new BanTargetResolution(null, "ambiguous_player");

        var target = idMatches[0];
        if (target.IsHost)
            return new BanTargetResolution(null, "host_not_supported");
        if (requireUniqueNameForFallback && all.Count(candidate => candidate.Name == target.Name) > 1)
            return new BanTargetResolution(null, "ambiguous_player");
        return new BanTargetResolution(target, "");
    }
}

public static class BroadcastOutcomeValidator
{
    public static CommandOutcome? Validate(PanelCommand command, string message, bool worldReady, bool chatAvailable)
    {
        if (string.IsNullOrWhiteSpace(message))
            return PlayerCommandOutcomes.Failed(command, "empty_message", "The broadcast message is empty.");
        if (!worldReady)
            return PlayerCommandOutcomes.Failed(command, "world_not_ready", "The game world is not ready.");
        if (!chatAvailable)
            return PlayerCommandOutcomes.Failed(command, "chat_unavailable", "The game chat system is unavailable.");
        return null;
    }
}
