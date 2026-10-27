namespace StardewAnxiPanel.Control;

public static class HostSleepRetryPolicy
{
    public static bool ShouldRetry(int attempts, int maximum, bool worldReady, bool dayTransitionStarted, bool upstreamSleepRequestManaged)
        => attempts > 0 && attempts < maximum && worldReady && !dayTransitionStarted && !upstreamSleepRequestManaged;
}
