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
	public string GameVersion { get; set; } = "";
	public string ApiVersion { get; set; } = "";
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
	public string TargetSaveId { get; set; } = "";
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

	public static bool CanCustomizeLoadedSave(
		bool creationObserved,
		PendingNewGameMarker? marker,
		InitConfig? init,
		string? loadedSaveId,
		DateTimeOffset now)
	{
		_ = creationObserved;
		var validation = ValidateMarker(marker, init, now);
		if (!validation.Valid)
			return false;
		return !string.IsNullOrWhiteSpace(marker?.TargetSaveId)
			&& !string.IsNullOrWhiteSpace(loadedSaveId)
			&& string.Equals(marker.TargetSaveId, loadedSaveId, StringComparison.Ordinal);
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
    public string PlayerAuthMode { get; set; } = "none";
    public string PlayerAuthConfigRevision { get; set; } = "";
    public bool RolePasswordPatchAvailable { get; set; }
    public string RolePasswordPatchDetail { get; set; } = "";
    public bool WarpHomeBridgeAvailable { get; set; }
    public string WarpHomeBridgeDetail { get; set; } = "";
	public string NewGameTransactionId { get; set; } = "";
	public string RequestedFarmType { get; set; } = "";
	public string ResolvedFarmType { get; set; } = "";
	public bool FarmTypeResolved { get; set; }
	public bool CatalogGenerated { get; set; }
	public string NewGameWarning { get; set; } = "";
	public bool NewGameCreationObserved { get; set; }
	public bool CustomizationApplied { get; set; }
	public bool CustomizationVerified { get; set; }
	public string CustomizationTransactionId { get; set; } = "";
	public string CustomizationSaveId { get; set; } = "";
	public DateTimeOffset? CustomizationVerifiedAt { get; set; }
	public CharacterCustomizationSnapshot? Customization { get; set; }
	public CharacterCustomizationSnapshot? CustomizationAttempt { get; set; }
	public string[] CustomizationMismatches { get; set; } = Array.Empty<string>();
}

public sealed class CharacterCustomizationSnapshot
{
	public string FarmerName { get; set; } = "";
	public string FarmName { get; set; } = "";
	public string FavoriteThing { get; set; } = "";
	public string Gender { get; set; } = "";
	public string PetType { get; set; } = "";
	public string PetBreed { get; set; } = "";
	public int? Skin { get; set; }
	public int? Hair { get; set; }
	public string? Shirt { get; set; }
	public string? Pants { get; set; }
	public int? Accessory { get; set; }
	public RgbColor? EyeColor { get; set; }
	public RgbColor? HairColor { get; set; }
	public RgbColor? PantsColor { get; set; }
	public bool IsCustomized { get; set; }
}

public static class CharacterCustomizationContract
{
	public static CharacterCustomizationSnapshot ExpectedCore(InitConfig config)
	{
		ArgumentNullException.ThrowIfNull(config);
		return new CharacterCustomizationSnapshot
		{
			FarmerName = config.FarmerName ?? "",
			FarmName = config.FarmName ?? "",
			FavoriteThing = string.IsNullOrWhiteSpace(config.FavoriteThing) ? "Anxi" : config.FavoriteThing,
			Gender = string.Equals(config.Gender, "female", StringComparison.OrdinalIgnoreCase) ? "female" : "male",
			PetType = string.Equals(config.PetType, "Dog", StringComparison.OrdinalIgnoreCase) ? "Dog" : "Cat",
			PetBreed = string.IsNullOrWhiteSpace(config.PetBreed) ? "0" : config.PetBreed,
			Skin = config.Skin,
			Hair = config.Hair,
			Shirt = string.IsNullOrWhiteSpace(config.Shirt) ? null : config.Shirt,
			Pants = string.IsNullOrWhiteSpace(config.Pants) ? null : config.Pants,
			Accessory = config.Accessory,
			EyeColor = NormalizeColor(config.EyeColor),
			HairColor = NormalizeColor(config.HairColor),
			PantsColor = NormalizeColor(config.PantsColor),
			IsCustomized = true,
		};
	}

	public static bool CoreEquals(CharacterCustomizationSnapshot? expected, CharacterCustomizationSnapshot? actual)
	{
		if (!RequiredCoreEquals(expected, actual) || expected is null || actual is null)
			return false;
		return expected.Skin == actual.Skin
			&& expected.Hair == actual.Hair
			&& string.Equals(expected.Shirt, actual.Shirt, StringComparison.Ordinal)
			&& string.Equals(expected.Pants, actual.Pants, StringComparison.Ordinal)
			&& expected.Accessory == actual.Accessory
			&& ColorsEqual(expected.EyeColor, actual.EyeColor)
			&& ColorsEqual(expected.HairColor, actual.HairColor)
			&& ColorsEqual(expected.PantsColor, actual.PantsColor)
			&& expected.IsCustomized == actual.IsCustomized;
	}

	public static bool MatchesCore(InitConfig? config, CharacterCustomizationSnapshot? actual)
	{
		return MismatchFields(config, actual).Length == 0;
	}

	public static string[] MismatchFields(InitConfig? config, CharacterCustomizationSnapshot? actual)
	{
		if (config is null)
			return new[] { "config" };
		if (actual is null)
			return new[] { "snapshot" };
		var expected = ExpectedCore(config);
		var mismatches = new List<string>();
		if (!string.Equals(expected.FarmerName, actual.FarmerName, StringComparison.Ordinal)) mismatches.Add("farmerName");
		if (!string.Equals(expected.FarmName, actual.FarmName, StringComparison.Ordinal)) mismatches.Add("farmName");
		if (!string.Equals(expected.FavoriteThing, actual.FavoriteThing, StringComparison.Ordinal)) mismatches.Add("favoriteThing");
		if (!string.Equals(expected.Gender, actual.Gender, StringComparison.Ordinal)) mismatches.Add("gender");
		if (!string.Equals(expected.PetType, actual.PetType, StringComparison.Ordinal)) mismatches.Add("petType");
		if (!string.Equals(expected.PetBreed, actual.PetBreed, StringComparison.Ordinal)) mismatches.Add("petBreed");
		if (expected.Skin.HasValue && expected.Skin != actual.Skin) mismatches.Add("skin");
		if (expected.Hair.HasValue && expected.Hair != actual.Hair) mismatches.Add("hair");
		if (expected.Shirt is not null && !string.Equals(expected.Shirt, actual.Shirt, StringComparison.Ordinal)) mismatches.Add("shirt");
		if (expected.Pants is not null && !string.Equals(expected.Pants, actual.Pants, StringComparison.Ordinal)) mismatches.Add("pants");
		if (expected.Accessory.HasValue && expected.Accessory != actual.Accessory) mismatches.Add("accessory");
		if (expected.EyeColor is not null && !ColorsEqual(expected.EyeColor, actual.EyeColor)) mismatches.Add("eyeColor");
		if (expected.HairColor is not null && !ColorsEqual(expected.HairColor, actual.HairColor)) mismatches.Add("hairColor");
		if (expected.PantsColor is not null && !ColorsEqual(expected.PantsColor, actual.PantsColor)) mismatches.Add("pantsColor");
		if (!actual.IsCustomized) mismatches.Add("isCustomized");
		return mismatches.ToArray();
	}

	private static bool RequiredCoreEquals(CharacterCustomizationSnapshot? expected, CharacterCustomizationSnapshot? actual)
	{
		return expected is not null
			&& actual is not null
			&& string.Equals(expected.FarmerName, actual.FarmerName, StringComparison.Ordinal)
			&& string.Equals(expected.FarmName, actual.FarmName, StringComparison.Ordinal)
			&& string.Equals(expected.FavoriteThing, actual.FavoriteThing, StringComparison.Ordinal)
			&& string.Equals(expected.Gender, actual.Gender, StringComparison.Ordinal)
			&& string.Equals(expected.PetType, actual.PetType, StringComparison.Ordinal)
			&& string.Equals(expected.PetBreed, actual.PetBreed, StringComparison.Ordinal);
	}

	private static RgbColor? NormalizeColor(RgbColor? color)
	{
		return color is null
			? null
			: new RgbColor
			{
				R = Math.Clamp(color.R, 0, 255),
				G = Math.Clamp(color.G, 0, 255),
				B = Math.Clamp(color.B, 0, 255),
			};
	}

	private static bool ColorsEqual(RgbColor? expected, RgbColor? actual)
	{
		return expected is null
			? actual is null
			: actual is not null
				&& expected.R == actual.R
				&& expected.G == actual.G
				&& expected.B == actual.B;
	}
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

public sealed record SaveCommandExpectation(
	bool Valid,
	bool IsTargeted,
	string TransactionId,
	string SaveId,
	string PreSaveAction,
	string PreSaveActionSaveId,
	string ErrorCode);

public sealed record FarmhandBindingSummary(
	int TotalFarmhands,
	int CustomizedFarmhands,
	int BoundFarmhands);

public sealed record FarmhandUnbindDecision(bool CanApply, string ErrorCode, string Message);

public static class FarmhandUnbindContract
{
	public static FarmhandUnbindDecision ValidateBeforeSave(
		SaveCommandExpectation expectation,
		string? actualSaveId,
		bool worldReady,
		bool isServer,
		int onlineFarmhandCount,
		FarmhandBindingSummary? summary)
	{
		if (!string.Equals(expectation.PreSaveAction, SaveCommandContract.UnbindAllFarmhandsAction, StringComparison.Ordinal))
			return new(true, "", "");
		if (!worldReady)
			return new(false, "farmhand_unbind_world_not_ready", "The game world is not ready for farmhand unbinding.");
		if (!isServer)
			return new(false, "farmhand_unbind_not_server", "Farmhand unbinding can only run on the server host.");
		if (!string.Equals(expectation.PreSaveActionSaveId, actualSaveId?.Trim(), StringComparison.Ordinal))
			return new(false, "farmhand_unbind_save_mismatch", "The loaded save does not match the farmhand unbind target.");
		if (onlineFarmhandCount != 0)
			return new(false, "farmhand_unbind_players_connected", "Farmhand unbinding requires every human farmhand to be offline.");
		if (summary is null)
			return new(false, "farmhand_unbind_state_unavailable", "Farmhand binding state is unavailable.");
		if (summary.TotalFarmhands <= 0)
			return new(false, "farmhand_unbind_no_farmhands", "The imported save contains no farmhand roles to unbind.");
		return new(true, "", "");
	}
}

public static class SaveCommandContract
{
	public const string TransactionIdPayloadKey = "transactionId";
	public const string SaveIdPayloadKey = "saveId";
	public const string PreSaveActionPayloadKey = "preSaveAction";
	public const string PreSaveActionSaveIdPayloadKey = "preSaveActionSaveId";
	public const string UnbindAllFarmhandsAction = "unbind-all-farmhands";
	public const string SavedEventName = "GameLoop.Saved";

	public static SaveCommandExpectation ParseExpectation(PanelCommand command)
	{
		ArgumentNullException.ThrowIfNull(command);
		var payload = command.Payload;
		if (payload is null)
			return new(true, false, "", "", "", "", "");

		var hasTransaction = payload.TryGetValue(TransactionIdPayloadKey, out var rawTransaction);
		var hasSave = payload.TryGetValue(SaveIdPayloadKey, out var rawSave);
		var transactionId = ReadString(rawTransaction);
		var saveId = ReadString(rawSave);
		if (hasTransaction != hasSave || (hasTransaction && (transactionId.Length == 0 || saveId.Length == 0)))
			return new(false, false, transactionId, saveId, "", "", "save_target_invalid");

		var hasAction = payload.TryGetValue(PreSaveActionPayloadKey, out var rawAction);
		var hasActionSave = payload.TryGetValue(PreSaveActionSaveIdPayloadKey, out var rawActionSave);
		var action = ReadString(rawAction);
		var actionSaveId = ReadString(rawActionSave);
		if (hasAction != hasActionSave
			|| (hasAction && (!string.Equals(action, UnbindAllFarmhandsAction, StringComparison.Ordinal) || actionSaveId.Length == 0)))
		{
			return new(false, hasTransaction, transactionId, saveId, action, actionSaveId, "save_action_invalid");
		}

		return new(true, hasTransaction, transactionId, saveId, action, actionSaveId, "");
	}

	public static CommandOutcome CompleteSavedEvent(
		PanelCommand command,
		string? actualSaveId,
		DateTimeOffset now,
		FarmhandBindingSummary? farmhandBindings = null)
	{
		ArgumentNullException.ThrowIfNull(command);
		var expectation = ParseExpectation(command);
		var actual = actualSaveId?.Trim() ?? "";
		var details = BuildDetails(expectation, actual);

		if (!expectation.Valid)
			return Outcome(command, CommandStatuses.Failed, expectation.ErrorCode,
				"The save command target payload is incomplete or invalid.", now, details);
		if (actual.Length == 0)
			return Outcome(command, CommandStatuses.Failed, "save_identity_missing",
				"GameLoop.Saved did not expose the saved game identity.", now, details);
		if (expectation.IsTargeted && !string.Equals(expectation.SaveId, actual, StringComparison.Ordinal))
			return Outcome(command, CommandStatuses.Failed, "save_target_mismatch",
				"GameLoop.Saved completed for a different save than the command target.", now, details);
		if (expectation.PreSaveAction.Length > 0
			&& !string.Equals(expectation.PreSaveActionSaveId, actual, StringComparison.Ordinal))
		{
			return Outcome(command, CommandStatuses.Failed, "farmhand_unbind_save_mismatch",
				"GameLoop.Saved completed for a different save than the farmhand unbind target.", now, details);
		}
		if (string.Equals(expectation.PreSaveAction, UnbindAllFarmhandsAction, StringComparison.Ordinal))
		{
			if (farmhandBindings is null)
				return Outcome(command, CommandStatuses.Failed, "farmhand_unbind_unverified",
					"GameLoop.Saved completed but farmhand binding state could not be verified.", now, details);
			details["farmhandCount"] = farmhandBindings.TotalFarmhands.ToString(System.Globalization.CultureInfo.InvariantCulture);
			details["customizedFarmhandCount"] = farmhandBindings.CustomizedFarmhands.ToString(System.Globalization.CultureInfo.InvariantCulture);
			details["boundFarmhandCount"] = farmhandBindings.BoundFarmhands.ToString(System.Globalization.CultureInfo.InvariantCulture);
			if (farmhandBindings.TotalFarmhands <= 0 || farmhandBindings.BoundFarmhands != 0)
				return Outcome(command, CommandStatuses.Failed, "farmhand_unbind_incomplete",
					"GameLoop.Saved completed but at least one farmhand role remains bound.", now, details);
		}

		return Outcome(command, CommandStatuses.Succeeded, "ok",
			expectation.IsTargeted
				? "GameLoop.Saved confirmed that the requested target save completed."
				: "GameLoop.Saved confirmed that the requested game save completed.",
			now, details);
	}

	private static string ReadString(JsonElement value)
		=> value.ValueKind == JsonValueKind.String ? value.GetString()?.Trim() ?? "" : "";

	private static Dictionary<string, string> BuildDetails(SaveCommandExpectation expectation, string actualSaveId)
	{
		var details = new Dictionary<string, string>(StringComparer.Ordinal)
		{
			["event"] = SavedEventName,
			["saveId"] = actualSaveId,
		};
		if (expectation.TransactionId.Length > 0)
			details["transactionId"] = expectation.TransactionId;
		if (expectation.SaveId.Length > 0)
			details["expectedSaveId"] = expectation.SaveId;
		if (expectation.PreSaveAction.Length > 0)
			details["preSaveAction"] = expectation.PreSaveAction;
		if (expectation.PreSaveActionSaveId.Length > 0)
			details["preSaveActionSaveId"] = expectation.PreSaveActionSaveId;
		return details;
	}

	private static CommandOutcome Outcome(
		PanelCommand command,
		string status,
		string errorCode,
		string message,
		DateTimeOffset now,
		Dictionary<string, string> details) => new()
	{
		CommandId = command.Id,
		Status = status,
		ErrorCode = errorCode,
		Message = message,
		CreatedAt = command.CreatedAt,
		UpdatedAt = now,
		Details = details,
	};
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
