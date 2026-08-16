namespace StardewAnxiPanel.Control;

public enum HostControlMode
{
    Unknown,
    Automation,
    Manual,
}

public readonly record struct ManualControlDecision(
    HostControlMode Mode,
    bool ShouldRestoreAutomation,
    DateTimeOffset? LeaseExpiresAt);

public static class ManualControlPolicy
{
    public static ManualControlDecision Evaluate(
        bool automationKnown,
        bool automationEnabled,
        bool connectionCountKnown,
        int connectionCount,
        DateTimeOffset? lastManualInputAt,
        DateTimeOffset now,
        TimeSpan unattendedLease)
    {
        if (!automationKnown)
            return new(HostControlMode.Unknown, false, null);
        if (automationEnabled)
            return new(HostControlMode.Automation, false, null);

        var leaseExpiresAt = lastManualInputAt?.Add(unattendedLease);
        var unattended = connectionCountKnown && connectionCount <= 0;
        var expired = unattended && leaseExpiresAt.HasValue && now >= leaseExpiresAt.Value;
        return new(HostControlMode.Manual, expired, leaseExpiresAt);
    }
}

public static class HostVisibilityContract
{
    public static bool IsConsistent(bool hostVisible, bool displayFarmer, bool farmerHidden)
        => displayFarmer == hostVisible && farmerHidden == !hostVisible;
}

public static class HostBedContract
{
    public const string Single = "Single";
    public const string Double = "Double";

    public static string ExpectedBedType(int houseUpgradeLevel)
        => houseUpgradeLevel switch
        {
            0 => Single,
            1 or 2 or 3 => Double,
            _ => "",
        };

    public static bool HasUnambiguousMapBedPosition(int houseUpgradeLevel, int defaultBedPositionCount)
        => ExpectedBedType(houseUpgradeLevel).Length > 0 && defaultBedPositionCount == 1;
}
