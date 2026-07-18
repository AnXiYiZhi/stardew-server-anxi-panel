namespace StardewAnxiPanel.Control;

public enum PauseReason
{
    None,
    NoConnectedClients,
    AllGameplayPlayersRequestedPause,
}

public readonly record struct PauseDecision(bool ShouldForcePause, PauseReason Reason);

public static class PausePolicy
{
    public static PauseDecision Evaluate(
        bool enabled,
        bool isServer,
        bool worldReady,
        bool connectionCountKnown,
        int connectionCount,
        bool isFestivalDay,
        int timeOfDay,
        int gameplayPlayerCount,
        int pauseRequestCount)
    {
        if (!enabled || !isServer || !worldReady)
            return new(false, PauseReason.None);

        if (
            connectionCountKnown
            && connectionCount <= 0
            && !isFestivalDay
            && timeOfDay is >= 610 and <= 2500
        )
        {
            return new(true, PauseReason.NoConnectedClients);
        }

        if (
            connectionCountKnown
            && connectionCount == gameplayPlayerCount
            && gameplayPlayerCount > 0
            && pauseRequestCount >= gameplayPlayerCount
        )
        {
            return new(true, PauseReason.AllGameplayPlayersRequestedPause);
        }

        return new(false, PauseReason.None);
    }
}
