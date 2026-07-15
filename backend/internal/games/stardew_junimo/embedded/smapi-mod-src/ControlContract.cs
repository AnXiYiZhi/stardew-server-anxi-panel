using System.Text.Json;
using System.Text.Json.Serialization;
using System.Security.Cryptography;
using System.Text;

namespace StardewAnxiPanel.Control;

public sealed class InitConfig
{
	public string TransactionId { get; set; } = "";
    public string Mode { get; set; } = "create-or-load";
    public string SaveId { get; set; } = "";
    public string FarmerName { get; set; } = "";
    public string FarmName { get; set; } = "";
    public string FavoriteThing { get; set; } = "Anxi";
    public string Gender { get; set; } = "male";
    public string PetType { get; set; } = "Cat";
    public string PetBreed { get; set; } = "0";
    public int? Skin { get; set; }
    public int? Hair { get; set; }
    public string Shirt { get; set; } = "";
    public string Pants { get; set; } = "";
    public int? Accessory { get; set; }
    public RgbColor? EyeColor { get; set; }
    public RgbColor? HairColor { get; set; }
    public RgbColor? PantsColor { get; set; }
    public string FarmType { get; set; } = "standard";
    public int CabinCount { get; set; }
    public string CabinLayout { get; set; } = "close";
    public string MoneyMode { get; set; } = "shared";
    public int ProfitMargin { get; set; } = 100;
    public bool SkipIntro { get; set; } = true;
    public bool AutoPause { get; set; } = true;
    public bool HideHost { get; set; }
}

public sealed class RgbColor
{
    public int R { get; set; }
    public int G { get; set; }
    public int B { get; set; }
}

public sealed class PanelOptions
{
	public int SchemaVersion { get; set; } = 2;
    public string Source { get; set; } = "smapi";
	public string RequestId { get; set; } = "";
	public string TransactionId { get; set; } = "";
    public DateTimeOffset GeneratedAt { get; set; }
	public string ControlModVersion { get; set; } = "";
	public LoadedModItem[] LoadedMods { get; set; } = Array.Empty<LoadedModItem>();
	public string ModFingerprint { get; set; } = "";
    public OptionItem[] Genders { get; set; } = Array.Empty<OptionItem>();
    public OptionItem[] PetTypes { get; set; } = Array.Empty<OptionItem>();
    public OptionItem[] PetBreeds { get; set; } = Array.Empty<OptionItem>();
    public OptionItem[] CabinCounts { get; set; } = Array.Empty<OptionItem>();
    public OptionItem[] CabinLayouts { get; set; } = Array.Empty<OptionItem>();
    public OptionItem[] ProfitMargins { get; set; } = Array.Empty<OptionItem>();
    public OptionItem[] MoneyModes { get; set; } = Array.Empty<OptionItem>();
    public OptionItem[] FarmTypes { get; set; } = Array.Empty<OptionItem>();
}

public sealed class OptionItem
{
    public string Id { get; set; } = "";
    public string Label { get; set; } = "";
    public string Group { get; set; } = "";
    public string Description { get; set; } = "";
    public string Image { get; set; } = "";
	public string Kind { get; set; } = "builtin";
	public DateTimeOffset GeneratedAt { get; set; }
}

public sealed class LoadedModItem
{
	public string UniqueId { get; set; } = "";
	public string Version { get; set; } = "";
}

public sealed class FarmCatalogRequest
{
	public int SchemaVersion { get; set; } = 1;
	public string RequestId { get; set; } = "";
	public string TransactionId { get; set; } = "";
	public string RequestedFarmType { get; set; } = "";
	public DateTimeOffset GeneratedAt { get; set; }
	public DateTimeOffset ExpiresAt { get; set; }
}

public sealed class PendingNewGameMarker
{
	public int SchemaVersion { get; set; }
	public string TransactionId { get; set; } = "";
	public string RequestedFarmType { get; set; } = "";
	public DateTimeOffset CreatedAt { get; set; }
	public DateTimeOffset ExpiresAt { get; set; }
	public string State { get; set; } = "";
}

public sealed record RuntimeFarmType(string Id, string Label, bool SpawnMonstersByDefault, string Kind = "modded");

public sealed record FarmTypeResolution(
	string RequestedFarmType,
	string ResolvedFarmType,
	bool Resolved,
	int WhichFarm,
	RuntimeFarmType? ModFarm,
	bool SpawnMonstersByDefault,
	string Warning);

public sealed record MarkerValidation(bool Valid, string ErrorCode, PendingNewGameMarker? Marker);

public static class NewGameControlContract
{
	public static bool ShouldClearMarkerOnSaveLoaded => false;

	public static bool CatalogContainsRequestedFarm(IEnumerable<OptionItem> farms, string requestedFarmType)
	{
		var requested = requestedFarmType?.Trim() ?? "";
		return requested.Length == 0 || farms.Any(farm => string.Equals(farm.Id, requested, StringComparison.OrdinalIgnoreCase));
	}
	public static string ComputeModFingerprint(IEnumerable<LoadedModItem> mods)
	{
		var canonical = mods
			.Where(mod => !string.IsNullOrWhiteSpace(mod.UniqueId))
			.Select(mod => $"{mod.UniqueId.Trim().ToLowerInvariant()}@{mod.Version.Trim()}")
			.OrderBy(value => value, StringComparer.Ordinal)
			.ToArray();
		var payload = Encoding.UTF8.GetBytes(string.Join("\n", canonical) + (canonical.Length > 0 ? "\n" : ""));
		return Convert.ToHexString(SHA256.HashData(payload)).ToLowerInvariant();
	}

	public static LoadedModItem[] SortLoadedMods(IEnumerable<LoadedModItem> mods)
	{
		return mods
			.Where(mod => !string.IsNullOrWhiteSpace(mod.UniqueId))
			.Select(mod => new LoadedModItem { UniqueId = mod.UniqueId.Trim(), Version = mod.Version.Trim() })
			.OrderBy(mod => mod.UniqueId, StringComparer.OrdinalIgnoreCase)
			.ThenBy(mod => mod.Version, StringComparer.Ordinal)
			.ToArray();
	}

	public static bool IsFreshCatalogRequest(FarmCatalogRequest? request, DateTimeOffset now)
	{
		return request is not null
			&& request.SchemaVersion == 1
			&& request.RequestId.Length > 0
			&& string.Equals(request.RequestId, request.TransactionId, StringComparison.Ordinal)
			&& request.ExpiresAt > now;
	}

	public static MarkerValidation ValidateMarker(PendingNewGameMarker? marker, InitConfig? init, DateTimeOffset now)
	{
		if (marker is null)
			return new(false, "marker_missing", null);
		if (marker.SchemaVersion != 1)
			return new(false, "marker_schema_unsupported", marker);
		if (init is null || string.IsNullOrWhiteSpace(init.TransactionId))
			return new(false, "init_transaction_missing", marker);
		if (!string.Equals(marker.TransactionId, init.TransactionId, StringComparison.Ordinal))
			return new(false, "transaction_mismatch", marker);
		if (marker.ExpiresAt <= now)
			return new(false, "marker_expired", marker);
		if (!string.Equals(marker.State, "pending", StringComparison.OrdinalIgnoreCase))
			return new(false, "marker_not_pending", marker);
		return new(true, "", marker);
	}

	public static FarmTypeResolution ResolveFarmType(string? requested, IReadOnlyList<RuntimeFarmType> additionalFarms)
	{
		var raw = requested?.Trim() ?? "";
		var normalized = raw.ToLowerInvariant();
		return normalized switch
		{
			"standard" => Builtin(raw, "standard", 0),
			"riverland" => Builtin(raw, "riverland", 1),
			"forest" => Builtin(raw, "forest", 2),
			"hilltop" or "hill-top" or "hills" => Builtin(raw, "hilltop", 3),
			"wilderness" => Builtin(raw, "wilderness", 4, true),
			"four_corners" or "four-corners" or "fourcorners" => Builtin(raw, "fourcorners", 5),
			"beach" => Builtin(raw, "beach", 6),
			"meadowlands" or "meadowlandsfarm" => ResolveAdditional(raw, "MeadowlandsFarm", additionalFarms, true),
			"modded" => ResolveFirstModded(raw, additionalFarms),
			_ => ResolveAdditional(raw, raw, additionalFarms, false),
		};
	}

	private static FarmTypeResolution Builtin(string requested, string resolved, int whichFarm, bool monsters = false)
		=> new(requested, resolved, true, whichFarm, null, monsters, "");

	private static FarmTypeResolution ResolveAdditional(string requested, string id, IReadOnlyList<RuntimeFarmType> farms, bool meadowlands)
	{
		var farm = farms.FirstOrDefault(candidate => string.Equals(candidate.Id, id, StringComparison.OrdinalIgnoreCase));
		if (farm is null || (!meadowlands && string.Equals(farm.Id, "MeadowlandsFarm", StringComparison.OrdinalIgnoreCase)))
			return Unknown(requested, $"FarmType '{requested}' is not loaded in Data/AdditionalFarms.");
		return new(requested, meadowlands ? "meadowlands" : farm.Id, true, 7, farm, farm.SpawnMonstersByDefault, "");
	}

	private static FarmTypeResolution ResolveFirstModded(string requested, IReadOnlyList<RuntimeFarmType> farms)
	{
		var farm = farms.FirstOrDefault(candidate => !string.Equals(candidate.Id, "MeadowlandsFarm", StringComparison.OrdinalIgnoreCase));
		return farm is null
			? Unknown(requested, "No modded farm is loaded in Data/AdditionalFarms.")
			: new(requested, farm.Id, true, 7, farm, farm.SpawnMonstersByDefault, "");
	}

	private static FarmTypeResolution Unknown(string requested, string warning)
		=> new(requested, "standard", false, 0, null, false, warning);
}

public static class ContractFile
{
	public static void WriteJsonAtomic(string path, object value)
	{
		var directory = Path.GetDirectoryName(path) ?? throw new InvalidOperationException("target directory is missing");
		Directory.CreateDirectory(directory);
		var tempPath = Path.Combine(directory, $".tmp-{Guid.NewGuid():N}");
		try
		{
			File.WriteAllText(tempPath, JsonSerializer.Serialize(value, ContractJson.Options));
			File.Move(tempPath, path, true);
		}
		finally
		{
			if (File.Exists(tempPath))
				File.Delete(tempPath);
		}
	}
}

public sealed class RuntimeStatus
{
	public int CommandResultVersion { get; set; } = 1;
    public string State { get; set; } = "";
    public string Message { get; set; } = "";
    public string? SaveId { get; set; }
    public DateTimeOffset UpdatedAt { get; set; }
    public bool PasswordBridgeAvailable { get; set; }
    public string PasswordBridgeDetail { get; set; } = "";
    public bool WarpHomeBridgeAvailable { get; set; }
    public string WarpHomeBridgeDetail { get; set; } = "";
	public string NewGameTransactionId { get; set; } = "";
	public string RequestedFarmType { get; set; } = "";
	public string ResolvedFarmType { get; set; } = "";
	public bool FarmTypeResolved { get; set; }
	public bool CatalogGenerated { get; set; }
	public string NewGameWarning { get; set; } = "";
}

public sealed class PlayersFile
{
    public DateTimeOffset UpdatedAt { get; set; }
    public string SaveId { get; set; } = "";
    public PlayerInfo[] Players { get; set; } = Array.Empty<PlayerInfo>();
}

public sealed class SaveEventFile
{
    public string Type { get; set; } = "";
    public string SaveName { get; set; } = "";
    public DateTimeOffset CreatedAt { get; set; }
}

public sealed class PlayerInfo
{
    public string Name { get; set; } = "";
    public string UniqueMultiplayerId { get; set; } = "";
    public bool IsHost { get; set; }
    public string Location { get; set; } = "";
    public string LocationName { get; set; } = "";
    public string LocationDisplayName { get; set; } = "";
    public int? TileX { get; set; }
    public int? TileY { get; set; }
    public int? PixelX { get; set; }
    public int? PixelY { get; set; }
    public int Money { get; set; }
    public long FarmIncome { get; set; }
    public long PersonalIncome { get; set; }
    public long TotalMoneyEarned { get; set; }
    public string WalletMode { get; set; } = "";
    public bool? IsAuthenticated { get; set; }
}

public sealed class PanelCommand
{
	public string Id { get; set; } = "";
    public string Name { get; set; } = "";
    public Dictionary<string, JsonElement>? Payload { get; set; }
    public DateTimeOffset CreatedAt { get; set; }
}

public static class CommandStatuses
{
    public const string Queued = "queued";
    public const string Running = "running";
    public const string Succeeded = "succeeded";
    public const string Failed = "failed";
    public const string Dispatched = "dispatched";
    public const string Expired = "expired";
    public const string Unknown = "unknown";
}

public sealed class CommandOutcome
{
    public string CommandId { get; set; } = "";
    public string Status { get; set; } = CommandStatuses.Unknown;
    public string ErrorCode { get; set; } = "";
    public string Message { get; set; } = "";
    public DateTimeOffset CreatedAt { get; set; }
    public DateTimeOffset UpdatedAt { get; set; }
    public Dictionary<string, string>? Details { get; set; }
}

public static class ContractJson
{
    public static readonly JsonSerializerOptions Options = new()
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        WriteIndented = true,
    };
}
