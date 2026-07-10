using System.Text.Json;
using System.Text.Json.Serialization;

namespace StardewAnxiPanel.Control;

public sealed class InitConfig
{
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
    public string Source { get; set; } = "smapi";
    public DateTimeOffset GeneratedAt { get; set; }
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
}

public sealed class RuntimeStatus
{
    public string State { get; set; } = "";
    public string Message { get; set; } = "";
    public string? SaveId { get; set; }
    public DateTimeOffset UpdatedAt { get; set; }
    public bool PasswordBridgeAvailable { get; set; }
    public string PasswordBridgeDetail { get; set; } = "";
    public bool WarpHomeBridgeAvailable { get; set; }
    public string WarpHomeBridgeDetail { get; set; } = "";
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
    public string Name { get; set; } = "";
    public Dictionary<string, JsonElement>? Payload { get; set; }
    public DateTimeOffset CreatedAt { get; set; }
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
