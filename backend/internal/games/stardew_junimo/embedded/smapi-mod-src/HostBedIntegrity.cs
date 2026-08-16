#if !SAP_CI_BUILD
using Microsoft.Xna.Framework;
using StardewModdingAPI;
using StardewValley;
using StardewValley.Locations;
using StardewValley.Objects;

namespace StardewAnxiPanel.Control;

internal sealed class HostBedIntegrity
{
    private static readonly TimeSpan ErrorLogInterval = TimeSpan.FromSeconds(30);

    private readonly IMonitor monitor;
    private DateTimeOffset lastErrorLogAt = DateTimeOffset.MinValue;
    private bool repairedThisLoad;

    public HostBedIntegrity(IMonitor monitor)
    {
        this.monitor = monitor;
    }

    public HostBedStatus Snapshot { get; private set; } = HostBedStatus.Unavailable();

    public void BeginSaveLoad()
    {
        repairedThisLoad = false;
        Snapshot = HostBedStatus.Unavailable();
    }

    public HostBedStatus Ensure()
    {
        var now = DateTimeOffset.UtcNow;
        if (!Context.IsWorldReady || !Game1.IsServer || Game1.MasterPlayer is null)
            return Set(HostBedStatus.Unavailable(now));

        if (Game1.getLocationFromName("FarmHouse") is not FarmHouse farmHouse
            || !string.Equals(farmHouse.NameOrUniqueName, "FarmHouse", StringComparison.Ordinal)
            || !ReferenceEquals(farmHouse.owner, Game1.MasterPlayer))
        {
            return Fail(now, -1, "host_farmhouse_unavailable", "The exact master FarmHouse could not be resolved safely.");
        }

        var houseUpgradeLevel = farmHouse.upgradeLevel;
        var expectedBedType = HostBedContract.ExpectedBedType(houseUpgradeLevel);
        if (expectedBedType.Length == 0)
            return Fail(now, houseUpgradeLevel, "host_bed_layout_unresolved", "The host house upgrade level is outside the supported game range.");

        var existing = farmHouse.GetPlayerBed();
        if (existing is not null)
            return Set(Healthy(now, farmHouse, existing, expectedBedType, repaired: repairedThisLoad));

        var backLayer = farmHouse.Map?.GetLayer("Back");
        if (backLayer is null)
            return Fail(now, houseUpgradeLevel, "host_bed_layout_unresolved", "The loaded FarmHouse map has no Back layer.");

        var positions = new List<Vector2>();
        for (var x = 0; x < backLayer.LayerWidth; x++)
        {
            for (var y = 0; y < backLayer.LayerHeight; y++)
            {
                if (farmHouse.doesTileHaveProperty(x, y, "DefaultBedPosition", "Back") is not null)
                    positions.Add(new Vector2(x, y));
            }
        }

        if (!HostBedContract.HasUnambiguousMapBedPosition(houseUpgradeLevel, positions.Count))
        {
            return Fail(
                now,
                houseUpgradeLevel,
                "host_bed_layout_unresolved",
                $"The loaded FarmHouse map exposes {positions.Count} DefaultBedPosition tiles; exactly one is required.");
        }

        var bedItemID = houseUpgradeLevel == 0
            ? BedFurniture.DEFAULT_BED_INDEX
            : Utility.GetDoubleWideVersionOfBed(BedFurniture.DEFAULT_BED_INDEX);
        var created = new BedFurniture(bedItemID, positions[0]);
        var furnitureCountBefore = farmHouse.furniture.Count;

        try
        {
            farmHouse.furniture.Add(created);
            var actual = farmHouse.GetPlayerBed();
            var expectedType = houseUpgradeLevel == 0
                ? BedFurniture.BedType.Single
                : BedFurniture.BedType.Double;
            if (!ReferenceEquals(actual, created)
                || created.bedType != expectedType
                || farmHouse.furniture.Count != furnitureCountBefore + 1
                || farmHouse.GetPlayerBedSpot() != created.GetBedSpot())
            {
                RemoveCreatedBed(farmHouse, created);
                return Fail(now, houseUpgradeLevel, "host_bed_repair_failed", "The game did not read back the map-derived host bed consistently.", repairAttempted: true);
            }

            monitor.Log(
                $"[HostBedIntegrity] repaired=true houseUpgradeLevel={houseUpgradeLevel} bedType={created.bedType} "
                    + $"tile=({(int)positions[0].X},{(int)positions[0].Y}) source=FarmHouse.Back.DefaultBedPosition",
                LogLevel.Warn);
            repairedThisLoad = true;
            return Set(Healthy(now, farmHouse, created, expectedBedType, repaired: true));
        }
        catch (Exception ex)
        {
            RemoveCreatedBed(farmHouse, created);
            return Fail(now, houseUpgradeLevel, "host_bed_repair_failed", $"The map-derived bed could not be added safely: {ex.Message}", repairAttempted: true);
        }
    }

    private static HostBedStatus Healthy(
        DateTimeOffset now,
        FarmHouse farmHouse,
        BedFurniture bed,
        string expectedBedType,
        bool repaired)
    {
        var tile = bed.TileLocation;
        var bedSpot = bed.GetBedSpot();
        return new HostBedStatus
        {
            State = repaired ? "repaired" : "healthy",
            Healthy = true,
            HouseUpgradeLevel = farmHouse.upgradeLevel,
            ExpectedBedType = expectedBedType,
            ActualBedType = bed.bedType.ToString(),
            BedItemId = bed.ItemId,
            BedTileX = (int)tile.X,
            BedTileY = (int)tile.Y,
            PlayerBedSpotX = bedSpot.X,
            PlayerBedSpotY = bedSpot.Y,
            FurnitureCount = farmHouse.furniture.Count,
            BedCount = farmHouse.furniture.OfType<BedFurniture>().Count(),
            LayoutSource = "FarmHouse.Back.DefaultBedPosition",
            RepairAttempted = repaired,
            Repaired = repaired,
            CheckedAt = now,
        };
    }

    private HostBedStatus Fail(
        DateTimeOffset now,
        int houseUpgradeLevel,
        string failureReason,
        string detail,
        bool repairAttempted = false)
    {
        var status = new HostBedStatus
        {
            State = "missing",
            Healthy = false,
            ErrorCode = "host_bed_missing",
            FailureReason = failureReason,
            Detail = detail,
            HouseUpgradeLevel = houseUpgradeLevel,
            ExpectedBedType = HostBedContract.ExpectedBedType(houseUpgradeLevel),
            RepairAttempted = repairAttempted,
            CheckedAt = now,
        };
        if (now - lastErrorLogAt >= ErrorLogInterval)
        {
            lastErrorLogAt = now;
            monitor.Log(
                $"[HostBedIntegrity] errorCode={status.ErrorCode} failureReason={failureReason} "
                    + $"houseUpgradeLevel={houseUpgradeLevel} detail={detail}",
                LogLevel.Error);
        }
        return Set(status);
    }

    private HostBedStatus Set(HostBedStatus status)
    {
        Snapshot = status;
        return status;
    }

    private static void RemoveCreatedBed(FarmHouse farmHouse, BedFurniture created)
    {
        try
        {
            var guid = farmHouse.furniture.GuidOf(created);
            if (guid != Guid.Empty)
                farmHouse.furniture.Remove(guid);
        }
        catch
        {
            // The caller reports a failed repair. Never touch unrelated furniture.
        }
    }
}
#endif
