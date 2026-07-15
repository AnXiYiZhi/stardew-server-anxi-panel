#if !SAP_CI_BUILD
using System.Text.Json;
using StardewModdingAPI;
using StardewModdingAPI.Events;
using StardewValley;
using StardewValley.GameData;
using Microsoft.Xna.Framework;
using Microsoft.Xna.Framework.Graphics;

namespace StardewAnxiPanel.Control;

public sealed class ModEntry : Mod
{
    private const int SinglePlayerMenuPausedTimeInterval = -100;
    private static readonly TimeSpan SaveCommandTimeout = TimeSpan.FromMinutes(2);

    private string controlDir = "";
    private string commandDir = "";
    private string commandResultDir = "";
    private InitConfig? initConfig;
    private bool isJunimoRuntime;
    private readonly PasswordProtectionBridge passwordBridge = new();
    private readonly WarpHomeBridge warpHomeBridge = new();
    private readonly PendingSaveCommandTracker pendingSaveCommands = new();
    private bool panelCustomizationApplied;
    private bool singlePlayerMenuPauseApplied;
    private int? singlePlayerMenuPauseSavedInterval;
    private bool pendingNewGameOptions;
	private PendingNewGameMarker? pendingNewGameMarker;
	private FarmCatalogRequest? farmCatalogRequest;
	private FarmTypeResolution? farmTypeResolution;
	private bool catalogGenerated;
	private bool runtimeFarmCatalogReady;
	private const int MaxCatalogImageDataUriChars = 64 * 1024;

    public override void Entry(IModHelper helper)
    {
        controlDir = Environment.GetEnvironmentVariable("SAP_CONTROL_DIR")
            ?? Path.Combine(helper.DirectoryPath, "..", "..", "control");
        controlDir = Path.GetFullPath(controlDir);
        commandDir = Path.Combine(controlDir, "commands");
        commandResultDir = Path.Combine(controlDir, "command-results");
        Directory.CreateDirectory(commandDir);
        Directory.CreateDirectory(commandResultDir);

        helper.Events.GameLoop.GameLaunched += OnGameLaunched;
        helper.Events.GameLoop.SaveCreating += OnSaveCreating;
        helper.Events.GameLoop.SaveLoaded += OnSaveLoaded;
        helper.Events.GameLoop.Saved += OnSaved;
        helper.Events.GameLoop.UpdateTicking += OnUpdateTicking;
        helper.Events.GameLoop.UpdateTicked += OnUpdateTicked;

        helper.ConsoleCommands.Add("sap_status", "Write a Stardew Anxi Panel status snapshot.", (_, _) =>
        {
            WriteStatus("running", "Status requested from SMAPI console.");
        });

        WriteStatus("booting", "SMAPI control mod loaded.");
    }

    private void OnGameLaunched(object? sender, GameLaunchedEventArgs e)
    {
        initConfig = ReadInitConfig();
        isJunimoRuntime = Helper.ModRegistry.IsLoaded("JunimoHost.Server");
        if (isJunimoRuntime)
        {
            passwordBridge.Initialize(Monitor);
            warpHomeBridge.Initialize(Monitor);
        }
		pendingNewGameMarker = ReadJsonFile<PendingNewGameMarker>(PendingNewGamePath(), "pending new-game marker");
		var markerValidation = NewGameControlContract.ValidateMarker(pendingNewGameMarker, initConfig, DateTimeOffset.UtcNow);
		pendingNewGameOptions = markerValidation.Valid;
		if (!markerValidation.Valid && pendingNewGameMarker is not null)
			Monitor.Log($"Pending new-game marker rejected: {markerValidation.ErrorCode}.", LogLevel.Warn);
		farmCatalogRequest = ReadJsonFile<FarmCatalogRequest>(FarmCatalogRequestPath(), "farm catalog request");
		if (!NewGameControlContract.IsFreshCatalogRequest(farmCatalogRequest, DateTimeOffset.UtcNow))
			farmCatalogRequest = null;
        ApplyPendingNewGameWorldOptions();
        ApplyDirectIpNetworkPolicy();
        WritePanelOptions();
        var saveId = initConfig?.SaveId ?? "";
        var farmName = initConfig?.FarmName ?? "";
        var target = saveId == "" ? "not configured" : $"{farmName} ({saveId})";
        WriteStatus("launched", $"Game launched. Target save: {target}", saveId);
    }

    private void OnSaveCreating(object? sender, SaveCreatingEventArgs e)
    {
        ApplyPendingNewGameWorldOptions();
        ApplyPanelCharacterCustomization();
        WriteStatus("save-creating", "Stardew Valley is creating the save requested by JunimoServer.");
    }

    private void OnSaveLoaded(object? sender, SaveLoadedEventArgs e)
    {
        ApplyPanelCharacterCustomization();
        ApplyDirectIpNetworkPolicy();
        var saveFolder = Constants.SaveFolderName;
        WriteStatus("save-loaded", "Save loaded through JunimoServer. Direct IP connections are enabled on UDP port 24642.", saveFolder);
    }

    private void OnSaved(object? sender, SavedEventArgs e)
    {
        if (!Context.IsWorldReady)
            return;

        var saveName = Constants.SaveFolderName;
        if (string.IsNullOrWhiteSpace(saveName))
            saveName = Game1.GetSaveGameName();
        if (string.IsNullOrWhiteSpace(saveName))
            return;

        WriteSaveEvent(saveName);

        var saveOutcome = pendingSaveCommands.Complete(DateTimeOffset.UtcNow);
        if (saveOutcome is not null)
        {
            var resultPath = Path.Combine(commandResultDir, saveOutcome.CommandId + ".json");
            WriteJsonAtomic(resultPath, saveOutcome);
            Monitor.Log($"Save-now command {saveOutcome.CommandId} completed through GameLoop.Saved.", LogLevel.Info);
        }
    }

    private void OnUpdateTicking(object? sender, UpdateTickingEventArgs e)
    {
        try
        {
            ApplySinglePlayerMenuPause();
        }
        catch (Exception ex)
        {
            RecoverSinglePlayerMenuPauseClock();
            Monitor.Log($"Single-player menu pause failed and released the clock: {ex}", LogLevel.Error);
        }
    }

    private void OnUpdateTicked(object? sender, UpdateTickedEventArgs e)
    {
        if (!e.IsMultipleOf(120))
            return;

        ApplyPendingNewGameWorldOptions();
        ApplyDirectIpNetworkPolicy();
		if (farmCatalogRequest is not null && !runtimeFarmCatalogReady)
			WritePanelOptions();
        WritePlayers();
        ConsumeCommands();

        var timedOutSave = pendingSaveCommands.Expire(DateTimeOffset.UtcNow);
        if (timedOutSave is not null)
        {
            var resultPath = Path.Combine(commandResultDir, timedOutSave.CommandId + ".json");
            WriteJsonAtomic(resultPath, timedOutSave);
            Monitor.Log($"Save-now command {timedOutSave.CommandId} timed out waiting for GameLoop.Saved.", LogLevel.Warn);
        }
    }

    private void ApplySinglePlayerMenuPause()
    {
        if (!isJunimoRuntime || initConfig?.AutoPause == false || !Context.IsWorldReady)
        {
            ClearSinglePlayerMenuPause();
            return;
        }

        var masterPlayerId = Game1.MasterPlayer?.UniqueMultiplayerID ?? Game1.player.UniqueMultiplayerID;
        var humanPlayers = Game1.getOnlineFarmers()
            .Where(farmer => farmer.UniqueMultiplayerID != masterPlayerId)
            .ToArray();
        var shouldPause = ShouldPauseForHumanPlayers(humanPlayers);

        if (shouldPause)
        {
            HoldSinglePlayerMenuPauseClock();
            return;
        }

        ClearSinglePlayerMenuPause();
    }

    private void HoldSinglePlayerMenuPauseClock()
    {
        if (Game1.gameTimeInterval >= 0 && singlePlayerMenuPauseSavedInterval is null)
            singlePlayerMenuPauseSavedInterval = Game1.gameTimeInterval;

        Game1.gameTimeInterval = SinglePlayerMenuPausedTimeInterval;
        singlePlayerMenuPauseApplied = true;
    }

    private void ClearSinglePlayerMenuPause()
    {
        if (!singlePlayerMenuPauseApplied && Game1.gameTimeInterval != SinglePlayerMenuPausedTimeInterval)
            return;

        if (Game1.gameTimeInterval == SinglePlayerMenuPausedTimeInterval)
            Game1.gameTimeInterval = Math.Max(0, singlePlayerMenuPauseSavedInterval ?? 0);

        if (Game1.netWorldState.Value.IsTimePaused)
            Game1.netWorldState.Value.IsTimePaused = false;
        if (Game1.netWorldState.Value.IsPaused)
            Game1.netWorldState.Value.IsPaused = false;
        Game1.isTimePaused = false;
        if (Game1.pauseTime > 0f)
            Game1.pauseTime = 0f;
        singlePlayerMenuPauseSavedInterval = null;
        singlePlayerMenuPauseApplied = false;
    }

    private void RecoverSinglePlayerMenuPauseClock()
    {
        if (Game1.gameTimeInterval == SinglePlayerMenuPausedTimeInterval)
            Game1.gameTimeInterval = Math.Max(0, singlePlayerMenuPauseSavedInterval ?? 0);

        singlePlayerMenuPauseSavedInterval = null;
        singlePlayerMenuPauseApplied = false;
    }

    private static bool FarmerRequestsMenuPause(Farmer farmer)
    {
        return farmer.hasMenuOpen.Value || farmer.requestingTimePause.Value;
    }

    private static bool ShouldPauseForHumanPlayers(IReadOnlyCollection<Farmer> humanPlayers)
    {
        if (humanPlayers.Count == 0)
            return false;

        return humanPlayers.All(FarmerRequestsMenuPause);
    }

    private void ApplyPanelCharacterCustomization()
    {
        if (panelCustomizationApplied || initConfig is null || !Context.IsWorldReady)
            return;
        if (!IsPanelNewGameMode(initConfig.Mode))
            return;
        if (!string.Equals(Game1.player.farmName.Value, initConfig.FarmName, StringComparison.OrdinalIgnoreCase))
            return;

        var cfg = initConfig;
        Game1.player.Name = cfg.FarmerName;
        Game1.player.displayName = cfg.FarmerName;
        Game1.player.favoriteThing.Value = string.IsNullOrWhiteSpace(cfg.FavoriteThing) ? "Anxi" : cfg.FavoriteThing;
        Game1.player.changeGender(!string.Equals(cfg.Gender, "female", StringComparison.OrdinalIgnoreCase));
        Game1.player.whichPetType = string.Equals(cfg.PetType, "Dog", StringComparison.OrdinalIgnoreCase) ? "Dog" : "Cat";
        Game1.player.whichPetBreed = string.IsNullOrWhiteSpace(cfg.PetBreed) ? "0" : cfg.PetBreed;
        if (cfg.Skin.HasValue)
            Game1.player.changeSkinColor(cfg.Skin.Value, force: true);
        if (cfg.Hair.HasValue)
            Game1.player.changeHairStyle(cfg.Hair.Value);
        if (!string.IsNullOrWhiteSpace(cfg.Shirt))
            Game1.player.changeShirt(cfg.Shirt);
        if (!string.IsNullOrWhiteSpace(cfg.Pants))
            Game1.player.changePantStyle(cfg.Pants);
        if (cfg.Accessory.HasValue)
            Game1.player.changeAccessory(cfg.Accessory.Value);
        if (cfg.EyeColor is not null)
            Game1.player.changeEyeColor(ToColor(cfg.EyeColor));
        if (cfg.HairColor is not null)
            Game1.player.changeHairColor(ToColor(cfg.HairColor));
        if (cfg.PantsColor is not null)
            Game1.player.changePantsColor(ToColor(cfg.PantsColor));
        Game1.player.isCustomized.Value = true;
        Game1.player.ConvertClothingOverrideToClothesItems();
        panelCustomizationApplied = true;
        Game1.saveOnNewDay = true;
        WriteStatus("save-customized", "Panel character customization applied to the JunimoServer world.", Constants.SaveFolderName);
    }

    private static Color ToColor(RgbColor color)
    {
        return new Color(
            Math.Clamp(color.R, 0, 255),
            Math.Clamp(color.G, 0, 255),
            Math.Clamp(color.B, 0, 255));
    }

    private static float ProfitMarginToModifier(int profitMargin)
    {
        return profitMargin switch
        {
            25 => 0.25f,
            50 => 0.5f,
            75 => 0.75f,
            _ => 1f,
        };
    }

    private static bool IsPanelNewGameMode(string mode)
    {
        return string.Equals(mode, "panel-newgame", StringComparison.OrdinalIgnoreCase)
            || string.Equals(mode, "native-create", StringComparison.OrdinalIgnoreCase);
    }

    private void ApplyPendingNewGameWorldOptions()
    {
        if (initConfig is null || Context.IsWorldReady)
            return;
        if (!pendingNewGameOptions)
            return;
        if (!IsPanelNewGameMode(initConfig.Mode))
            return;

		farmTypeResolution = SetFarmType(initConfig.FarmType);
		if (!farmTypeResolution.Resolved)
			Monitor.Log(farmTypeResolution.Warning, LogLevel.Warn);
        Game1.startingCabins = Math.Clamp(initConfig.CabinCount, 0, 7);
        Game1.cabinsSeparate = string.Equals(initConfig.CabinLayout, "separate", StringComparison.OrdinalIgnoreCase);
        Game1.multiplayerMode = Game1.multiplayerServer;
        if (Game1.player is not null)
        {
            Game1.player.difficultyModifier = ProfitMarginToModifier(initConfig.ProfitMargin);
            Game1.player.team.useSeparateWallets.Value = string.Equals(initConfig.MoneyMode, "separate", StringComparison.OrdinalIgnoreCase);
        }
    }

    private string PendingNewGamePath()
    {
        return Path.Combine(controlDir, "new-game-pending");
    }

	private string FarmCatalogRequestPath()
    {
		return Path.Combine(controlDir, "farm-catalog-request.json");
    }

    private static void ApplyDirectIpNetworkPolicy()
    {
        if (Game1.options is null)
            return;

        Game1.options.ipConnectionsEnabled = true;
    }

	private static FarmTypeResolution SetFarmType(string farmType)
    {
		var actualFarms = DataLoader.AdditionalFarms(Game1.content).ToArray();
		var runtimeFarms = actualFarms
			.Select(farm => new RuntimeFarmType(
				farm.Id ?? "",
				string.IsNullOrWhiteSpace(farm.Id) ? "Modded Farm" : farm.Id,
				farm.SpawnMonstersByDefault,
				string.Equals(farm.Id, "MeadowlandsFarm", StringComparison.OrdinalIgnoreCase) ? "builtin" : "modded"))
			.ToArray();
		var resolution = NewGameControlContract.ResolveFarmType(farmType, runtimeFarms);
        Game1.whichModFarm = null;
		Game1.whichFarm = resolution.WhichFarm;
		Game1.spawnMonstersAtNight = resolution.SpawnMonstersByDefault;
		if (resolution.ModFarm is not null)
			Game1.whichModFarm = actualFarms.FirstOrDefault(farm => string.Equals(farm.Id, resolution.ModFarm.Id, StringComparison.OrdinalIgnoreCase));
		return resolution;
    }

    private InitConfig? ReadInitConfig()
    {
        var path = Path.Combine(controlDir, "server-init.json");
        if (!File.Exists(path))
            return null;

        try
        {
            var json = File.ReadAllText(path);
            return JsonSerializer.Deserialize<InitConfig>(json, ContractJson.Options);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Failed to read init config: {ex}", LogLevel.Warn);
            return null;
        }
    }

	private T? ReadJsonFile<T>(string path, string description) where T : class
	{
		if (!File.Exists(path))
			return null;
		try
		{
			return JsonSerializer.Deserialize<T>(File.ReadAllText(path), ContractJson.Options);
		}
		catch (Exception ex)
		{
			Monitor.Log($"Failed to read {description}: {ex}", LogLevel.Warn);
			return null;
		}
	}

    private void WritePanelOptions()
    {
        try
        {
			var generatedAt = DateTimeOffset.UtcNow;
            var farmTypes = new List<OptionItem>
            {
                Option("standard", "标准农场", CropTextureDataUri(Game1.mouseCursors, new Rectangle(0, 324, 22, 20), 4) ?? PreviewSvg("标准", "#79a75f", "#d6edbd")),
                Option("riverland", "河边农场", CropTextureDataUri(Game1.mouseCursors, new Rectangle(22, 324, 22, 20), 4) ?? PreviewSvg("河边", "#4d99b5", "#c8edf3")),
                Option("forest", "森林农场", CropTextureDataUri(Game1.mouseCursors, new Rectangle(44, 324, 22, 20), 4) ?? PreviewSvg("森林", "#4d7f48", "#cfe6b8")),
                Option("hilltop", "山顶农场", CropTextureDataUri(Game1.mouseCursors, new Rectangle(66, 324, 22, 20), 4) ?? PreviewSvg("山顶", "#8f7860", "#e6d1b4")),
                Option("wilderness", "荒野农场", CropTextureDataUri(Game1.mouseCursors, new Rectangle(88, 324, 22, 20), 4) ?? PreviewSvg("荒野", "#695784", "#d8cbe8")),
                Option("four_corners", "四角农场", CropTextureDataUri(Game1.mouseCursors, new Rectangle(0, 345, 22, 20), 4) ?? PreviewSvg("四角", "#b38b3b", "#ecd99b")),
                Option("beach", "海滩农场", CropTextureDataUri(Game1.mouseCursors, new Rectangle(22, 345, 22, 20), 4) ?? PreviewSvg("海滩", "#d8ad5a", "#f4ddb0")),
            };

            foreach (var farm in DataLoader.AdditionalFarms(Game1.content))
            {
                var id = string.IsNullOrWhiteSpace(farm.Id) ? "meadowlands" : farm.Id;
                var label = string.Equals(id, "MeadowlandsFarm", StringComparison.OrdinalIgnoreCase) ? "草原农场" : id;
                var panelId = string.Equals(id, "MeadowlandsFarm", StringComparison.OrdinalIgnoreCase) ? "meadowlands" : id;
                if (farmTypes.All(item => !string.Equals(item.Id, panelId, StringComparison.OrdinalIgnoreCase)))
                {
                    var image = FarmTypeImage(farm) ?? PreviewSvg(label.Replace("农场", ""), "#7daa55", "#dcefb7");
					var kind = string.Equals(id, "MeadowlandsFarm", StringComparison.OrdinalIgnoreCase) ? "builtin" : "modded";
					farmTypes.Add(Option(panelId, label, image, kind: kind, generatedAt: generatedAt));
                }
            }
			var loadedMods = NewGameControlContract.SortLoadedMods(Helper.ModRegistry.GetAll().Select(mod => new LoadedModItem
			{
				UniqueId = mod.Manifest.UniqueID,
				Version = mod.Manifest.Version.ToString(),
			}));
			runtimeFarmCatalogReady = farmCatalogRequest is null ||
				NewGameControlContract.CatalogContainsRequestedFarm(farmTypes, farmCatalogRequest.RequestedFarmType);

            var options = new PanelOptions
            {
				SchemaVersion = 2,
				Source = "smapi-runtime",
				RequestId = farmCatalogRequest?.RequestId ?? "",
				TransactionId = farmCatalogRequest?.TransactionId ?? "",
				GeneratedAt = generatedAt,
				ControlModVersion = ModManifest.Version.ToString(),
				LoadedMods = loadedMods,
				ModFingerprint = NewGameControlContract.ComputeModFingerprint(loadedMods),
                Genders = new[] { Option("male", "男"), Option("female", "女") },
                PetTypes = new[] {
                    Option("Cat", "猫", FirstPetBreedImage("Cat") ?? PreviewSvg("猫", "#d08d3c", "#f5d19a")),
                    Option("Dog", "狗", FirstPetBreedImage("Dog") ?? PreviewSvg("狗", "#8d6b4f", "#e4c6a5")),
                },
                PetBreeds = BuildPetBreedOptions(),
                CabinCounts = Enumerable.Range(0, 8).Select(count => Option(count.ToString(), $"{count + 1} 人")).ToArray(),
                CabinLayouts = new[] { Option("close", "靠近", "", "联机小屋靠近农舍。"), Option("separate", "分散", "", "联机小屋分布在地图上。") },
                ProfitMargins = new[] { Option("100", "100%"), Option("75", "75%"), Option("50", "50%"), Option("25", "25%") },
                MoneyModes = new[] { Option("shared", "共享资金"), Option("separate", "分开资金") },
                FarmTypes = farmTypes.ToArray(),
            };
			catalogGenerated = WriteJsonAtomic(Path.Combine(controlDir, "options.json"), options);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Failed to write panel options: {ex}", LogLevel.Warn);
        }
    }

    private OptionItem[] BuildPetBreedOptions()
    {
        var breeds = new List<OptionItem>();
        foreach (var pet in Game1.petData)
        {
            var index = 1;
            foreach (var breed in pet.Value.Breeds)
            {
                if (!breed.CanBeChosenAtStart)
                    continue;

                var label = pet.Key == "Dog" ? $"狗 {index}" : $"猫 {index}";
                var fallback = pet.Key == "Dog"
                    ? PreviewSvg(label, "#8d6b4f", "#e4c6a5")
                    : PreviewSvg(label, "#d08d3c", "#f5d19a");
                breeds.Add(Option(breed.Id, label, BreedImage(breed) ?? fallback, "", pet.Key));
                index++;
            }
        }
        return breeds.ToArray();
    }

    private string? FirstPetBreedImage(string petType)
    {
        if (!Game1.petData.TryGetValue(petType, out var data))
            return null;
        foreach (var breed in data.Breeds)
        {
            if (breed.CanBeChosenAtStart)
                return BreedImage(breed);
        }
        return null;
    }

    private string? BreedImage(StardewValley.GameData.Pets.PetBreed breed)
    {
        if (string.IsNullOrWhiteSpace(breed.IconTexture))
            return null;
        try
        {
            var texture = Game1.content.Load<Texture2D>(breed.IconTexture);
            return CropTextureDataUri(texture, breed.IconSourceRect, 4);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Could not export pet breed image {breed.Id}: {ex.Message}", LogLevel.Trace);
            return null;
        }
    }

    private string? FarmTypeImage(ModFarmType farm)
    {
        if (string.IsNullOrWhiteSpace(farm.IconTexture))
            return null;
        try
        {
            var texture = Game1.content.Load<Texture2D>(farm.IconTexture);
            return CropTextureDataUri(texture, new Rectangle(0, 0, 22, 20), 4);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Could not export farm image {farm.Id}: {ex.Message}", LogLevel.Trace);
            return null;
        }
    }

    private string? CropTextureDataUri(Texture2D texture, Rectangle source, int scale)
    {
        if (source.Width <= 0 || source.Height <= 0)
            return null;

        source = Rectangle.Intersect(source, new Rectangle(0, 0, texture.Width, texture.Height));
        if (source.Width <= 0 || source.Height <= 0)
            return null;

        scale = Math.Max(1, scale);
        var full = new Color[texture.Width * texture.Height];
        texture.GetData(full);

        var width = source.Width * scale;
        var height = source.Height * scale;
        var cropped = new Color[width * height];
        for (var y = 0; y < source.Height; y++)
        {
            for (var x = 0; x < source.Width; x++)
            {
                var color = full[(source.Y + y) * texture.Width + source.X + x];
                for (var sy = 0; sy < scale; sy++)
                {
                    for (var sx = 0; sx < scale; sx++)
                    {
                        cropped[(y * scale + sy) * width + x * scale + sx] = color;
                    }
                }
            }
        }

        using var output = new Texture2D(Game1.graphics.GraphicsDevice, width, height);
        output.SetData(cropped);
        using var stream = new MemoryStream();
        output.SaveAsPng(stream, width, height);
		return BoundCatalogImage("data:image/png;base64," + Convert.ToBase64String(stream.ToArray()));
    }

	private static OptionItem Option(string id, string label, string image = "", string description = "", string group = "", string kind = "builtin", DateTimeOffset? generatedAt = null)
    {
		return new OptionItem { Id = id, Label = label, Image = BoundCatalogImage(image), Description = description, Group = group, Kind = kind, GeneratedAt = generatedAt ?? DateTimeOffset.UtcNow };
    }

	private static string BoundCatalogImage(string? image)
	{
		return !string.IsNullOrEmpty(image) && image.Length <= MaxCatalogImageDataUriChars ? image : "";
	}

    private static string PreviewSvg(string text, string bg, string fg)
    {
        var svg = "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 240 132\">"
            + $"<rect width=\"240\" height=\"132\" rx=\"14\" fill=\"{fg}\"/>"
            + $"<path d=\"M0 92c48-18 82 19 128 0s72-6 112 12v28H0z\" fill=\"{bg}\"/>"
            + "<circle cx=\"190\" cy=\"35\" r=\"18\" fill=\"#f7d76b\"/>"
            + $"<text x=\"24\" y=\"76\" font-family=\"Arial,'Microsoft YaHei',sans-serif\" font-size=\"30\" font-weight=\"700\" fill=\"#1f2b1f\">{text}</text>"
            + "</svg>";
		return BoundCatalogImage("data:image/svg+xml;base64," + Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes(svg)));
    }

    private void WriteStatus(string state, string message, string? saveId = null)
    {
        var status = new RuntimeStatus
        {
            State = state,
            Message = message,
            SaveId = string.IsNullOrWhiteSpace(saveId) ? initConfig?.SaveId : saveId,
            UpdatedAt = DateTimeOffset.UtcNow,
            PasswordBridgeAvailable = passwordBridge.Available,
            PasswordBridgeDetail = passwordBridge.Detail,
            WarpHomeBridgeAvailable = warpHomeBridge.Available,
            WarpHomeBridgeDetail = warpHomeBridge.Detail,
			NewGameTransactionId = initConfig?.TransactionId ?? "",
			RequestedFarmType = initConfig?.FarmType ?? "",
			ResolvedFarmType = farmTypeResolution?.ResolvedFarmType ?? "",
			FarmTypeResolved = farmTypeResolution?.Resolved ?? false,
			CatalogGenerated = catalogGenerated,
			NewGameWarning = farmTypeResolution?.Warning ?? "",
        };
        WriteJson(Path.Combine(controlDir, "status.json"), status);
    }

    private void WritePlayers()
    {
        var walletMode = Game1.player?.team?.useSeparateWallets.Value == true ? "separate" : "shared";
        var players = Context.IsWorldReady
            ? Game1.getOnlineFarmers().Select(farmer => BuildPlayerInfo(farmer, walletMode, passwordBridge)).ToArray()
            : Array.Empty<PlayerInfo>();

        WriteJson(Path.Combine(controlDir, "players.json"), new PlayersFile
        {
            UpdatedAt = DateTimeOffset.UtcNow,
            SaveId = Context.IsWorldReady ? Game1.GetSaveGameName() : "",
            Players = players,
        });
    }

    private void WriteSaveEvent(string saveName)
    {
        try
        {
            var eventDir = Path.Combine(controlDir, "save-events");
            Directory.CreateDirectory(eventDir);
            var path = Path.Combine(eventDir, $"{DateTimeOffset.UtcNow:yyyyMMddHHmmssfff}-{Guid.NewGuid():N}.json");
            WriteJson(path, new SaveEventFile
            {
                Type = "saved",
                SaveName = saveName,
                CreatedAt = DateTimeOffset.UtcNow,
            });
        }
        catch (Exception ex)
        {
            Monitor.Log($"Failed to write save event: {ex}", LogLevel.Warn);
        }
    }

    private static PlayerInfo BuildPlayerInfo(Farmer farmer, string walletMode, PasswordProtectionBridge bridge)
    {
        var location = ResolveFarmerLocation(farmer);
        var tile = farmer.TilePoint;
        var isHost = farmer.UniqueMultiplayerID == Game1.MasterPlayer.UniqueMultiplayerID;
        return new PlayerInfo
        {
            Name = farmer.Name,
            UniqueMultiplayerId = farmer.UniqueMultiplayerID.ToString(),
            IsHost = isHost,
            Location = location.Name,
            LocationName = location.Name,
            LocationDisplayName = location.DisplayName,
            TileX = tile.X,
            TileY = tile.Y,
            PixelX = (int)farmer.Position.X,
            PixelY = (int)farmer.Position.Y,
            Money = farmer.Money,
            FarmIncome = farmer.totalMoneyEarned,
            PersonalIncome = farmer.stats.Get("individualMoneyEarned"),
            TotalMoneyEarned = farmer.totalMoneyEarned,
            WalletMode = walletMode,
            // Host always bypasses JunimoServer's password protection entirely,
            // so it is never worth reflecting into IsPlayerAuthenticated for it.
            IsAuthenticated = isHost ? true : bridge.IsPlayerAuthenticated(farmer.UniqueMultiplayerID),
        };
    }

    private static FarmerLocationSnapshot ResolveFarmerLocation(Farmer farmer)
    {
        GameLocation? resolved = null;
        try
        {
            Utility.ForEachLocation(location =>
            {
                if (location.farmers.Any(f => f.UniqueMultiplayerID == farmer.UniqueMultiplayerID))
                {
                    resolved = location;
                    return false;
                }
                return true;
            });
        }
        catch
        {
            // Best-effort only; fall back to Farmer.currentLocation below.
        }

        resolved ??= farmer.currentLocation;
        var name = resolved?.NameOrUniqueName ?? resolved?.Name ?? "";
        var displayName = resolved?.DisplayName ?? name;
        return new FarmerLocationSnapshot(name, displayName);
    }

    private sealed record FarmerLocationSnapshot(string Name, string DisplayName);

    private void ConsumeCommands()
    {
        foreach (var path in Directory.GetFiles(commandDir, "*.json").OrderBy(p => p))
        {
            try
            {
                var json = File.ReadAllText(path);
                var command = JsonSerializer.Deserialize<PanelCommand>(json, ContractJson.Options);
                if (command is null)
                    continue;

                // Old panel commands had no ID or result protocol. Preserve their
                // fire-and-forget behavior for rolling upgrades.
                if (string.IsNullOrWhiteSpace(command.Id))
                {
                    HandleCommand(command);
                    File.Delete(path);
                    continue;
                }

                var resultPath = Path.Combine(commandResultDir, command.Id + ".json");
                if (File.Exists(resultPath))
                {
                    File.Delete(path);
                    continue;
                }

				var running = NewOutcome(command, CommandStatuses.Running, "", "Command execution started.");
				if (!WriteJsonAtomic(resultPath, running))
					continue;

                CommandOutcome outcome;
                try
                {
                    outcome = HandleCommand(command);
                }
                catch (Exception ex)
                {
                    outcome = NewOutcome(command, CommandStatuses.Failed, "command_handler_failed", ex.Message);
                    Monitor.Log($"Panel command {command.Id} failed: {ex}", LogLevel.Warn);
                }

                // Never delete the command before its durable result exists. If the
                // process dies after execution but before this write, the ambiguity is
                // intentionally surfaced as unknown and is never auto-retried by panel.
                if (WriteJsonAtomic(resultPath, outcome))
                    File.Delete(path);
            }
            catch (Exception ex)
            {
                Monitor.Log($"Failed to consume command {path}: {ex}", LogLevel.Warn);
            }
        }
    }

    private CommandOutcome HandleCommand(PanelCommand command)
    {
        switch (command.Name)
        {
            case "save-now":
                var saveOutcome = pendingSaveCommands.Begin(command, Context.IsWorldReady, DateTimeOffset.UtcNow, SaveCommandTimeout);
                if (saveOutcome.Status == CommandStatuses.Running)
                {
                    Game1.saveOnNewDay = true;
                    Monitor.Log($"Save-now command {command.Id} registered; waiting for GameLoop.Saved.", LogLevel.Info);
                }
                return saveOutcome;
            case "broadcast":
                var message = command.Payload is not null && command.Payload.TryGetValue("message", out var rawMessage)
                    ? rawMessage.GetString()
                    : "";
                return SendBroadcastMessage(command, message ?? "");
            case "kick":
                var targetId = command.Payload is not null && command.Payload.TryGetValue("uniqueMultiplayerId", out var rawTargetId)
                    ? rawTargetId.GetString()
                    : "";
                return KickPlayer(command, targetId ?? "");
            case "warp-home":
                var warpHomeTargetId = command.Payload is not null && command.Payload.TryGetValue("uniqueMultiplayerId", out var rawWarpHomeTargetId)
                    ? rawWarpHomeTargetId.GetString()
                    : "";
                return WarpPlayerHome(command, warpHomeTargetId ?? "");
            case "approve-auth":
                var approveTargetId = command.Payload is not null && command.Payload.TryGetValue("uniqueMultiplayerId", out var rawApproveTargetId)
                    ? rawApproveTargetId.GetString()
                    : "";
                return ApproveAuth(command, approveTargetId ?? "");
            case "trigger-event":
                return TriggerFestivalEvent(command);
            case "enable-joja":
                var jojaAdminPromoted = command.Payload is not null && command.Payload.TryGetValue("adminPromoted", out var rawJojaAdminPromoted)
                    && (rawJojaAdminPromoted.ValueKind == JsonValueKind.True || string.Equals(rawJojaAdminPromoted.GetString(), "true", StringComparison.OrdinalIgnoreCase));
                return EnableJojaRoute(command, jojaAdminPromoted);
            case "ban":
                var banName = command.Payload is not null && command.Payload.TryGetValue("name", out var rawBanName)
                    ? rawBanName.GetString()
                    : "";
                var banTargetId = command.Payload is not null && command.Payload.TryGetValue("uniqueMultiplayerId", out var rawBanTargetId)
                    ? rawBanTargetId.GetString()
                    : "";
                var adminPromoted = command.Payload is not null && command.Payload.TryGetValue("adminPromoted", out var rawAdminPromoted)
                    && (rawAdminPromoted.ValueKind == JsonValueKind.True || string.Equals(rawAdminPromoted.GetString(), "true", StringComparison.OrdinalIgnoreCase));
                return BanPlayer(command, banTargetId ?? "", banName ?? "", adminPromoted);
            case "stop":
                WriteStatus("stopping", "Stop command received. Container stop will be handled by backend later.");
                break;
            default:
                Monitor.Log($"Unknown panel command: {command.Name}", LogLevel.Warn);
                return NewOutcome(command, CommandStatuses.Failed, "command_unknown", "Unknown panel command.");
        }
        return NewOutcome(command, CommandStatuses.Dispatched, "", "Command dispatched to the game loop.");
    }

    private static CommandOutcome NewOutcome(PanelCommand command, string status, string errorCode, string message)
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

    private static CommandOutcome PlayerCommandSucceeded(PanelCommand command, string message, string playerId, string playerName)
    {
        return PlayerCommandOutcomes.Succeeded(command, message, playerId, playerName);
    }

    private static CommandOutcome PlayerCommandFailed(PanelCommand command, string errorCode, string message, string playerId = "")
    {
        return PlayerCommandOutcomes.Failed(command, errorCode, message, playerId);
    }

    private CommandOutcome SendBroadcastMessage(PanelCommand command, string message)
    {
        message = SanitizeChatText(message, 450);
        var validation = BroadcastOutcomeValidator.Validate(command, message, Context.IsWorldReady, chatAvailable: true);
        if (validation is not null)
            return validation;

        try
        {
            var chatSystem = Helper.Reflection.GetField<Multiplayer>(typeof(Game1), "multiplayer").GetValue();
            validation = BroadcastOutcomeValidator.Validate(command, message, worldReady: true, chatAvailable: chatSystem is not null);
            if (validation is not null)
                return validation;

            var text = $"[Panel] {message}";
            chatSystem!.sendChatMessage(DetectChatLanguage(text), text, Multiplayer.AllPlayers);
            Monitor.Log($"Broadcast command handed to the game chat system: {text}", LogLevel.Info);
            return NewOutcome(command, CommandStatuses.Succeeded, "ok", "Message accepted by the game chat system; delivery to every client is not guaranteed.");
        }
        catch (Exception ex)
        {
            Monitor.Log($"Broadcast command failed: {ex}", LogLevel.Error);
            return PlayerCommandFailed(command, "broadcast_failed", "The game chat system rejected the broadcast.");
        }
    }

    private CommandOutcome KickPlayer(PanelCommand command, string uniqueMultiplayerId)
    {
        if (!Context.IsWorldReady)
        {
            return PlayerCommandFailed(command, "world_not_ready", "The game world is not ready.", uniqueMultiplayerId);
        }
        if (!long.TryParse(uniqueMultiplayerId, out var targetId))
        {
            return PlayerCommandFailed(command, "invalid_player_id", "The target player ID is invalid.");
        }

        var target = Game1.getOnlineFarmers().FirstOrDefault(farmer => farmer.UniqueMultiplayerID == targetId);
        if (target is null)
        {
            return PlayerCommandFailed(command, "player_not_online", "The target player is not online.", uniqueMultiplayerId);
        }
        if (target.UniqueMultiplayerID == Game1.MasterPlayer.UniqueMultiplayerID)
        {
            Monitor.Log("Kick command ignored: cannot kick the host.", LogLevel.Warn);
            return PlayerCommandFailed(command, "host_not_supported", "The server host cannot be kicked.", uniqueMultiplayerId);
        }

        try
        {
            if (Game1.server is null)
                return PlayerCommandFailed(command, "kick_failed", "The multiplayer server is unavailable.", uniqueMultiplayerId);
            Game1.server.kick(targetId);
            Monitor.Log($"Kick command sent for player {target.Name} ({targetId}).", LogLevel.Info);
            return PlayerCommandSucceeded(command, $"Player {target.Name} was kicked.", uniqueMultiplayerId, target.Name);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Kick command failed for player {targetId}: {ex}", LogLevel.Error);
            return PlayerCommandFailed(command, "kick_failed", "The game rejected the kick operation.", uniqueMultiplayerId);
        }
    }

    private CommandOutcome WarpPlayerHome(PanelCommand command, string uniqueMultiplayerId)
    {
        if (!Context.IsWorldReady)
        {
            return PlayerCommandFailed(command, "world_not_ready", "The game world is not ready.", uniqueMultiplayerId);
        }
        if (!warpHomeBridge.Available)
        {
            Monitor.Log($"Warp-home command ignored: warp-home bridge unavailable ({warpHomeBridge.Detail}).", LogLevel.Warn);
            return PlayerCommandFailed(command, "bridge_unavailable", "The warp-home bridge is unavailable.", uniqueMultiplayerId);
        }
        if (!long.TryParse(uniqueMultiplayerId, out var targetId))
        {
            return PlayerCommandFailed(command, "invalid_player_id", "The target player ID is invalid.");
        }

        var target = Game1.getOnlineFarmers().FirstOrDefault(farmer => farmer.UniqueMultiplayerID == targetId);
        if (target is null)
        {
            return PlayerCommandFailed(command, "player_not_online", "The target player is not online.", uniqueMultiplayerId);
        }
        if (target.UniqueMultiplayerID == Game1.MasterPlayer.UniqueMultiplayerID)
        {
            Monitor.Log("Warp-home command ignored: host has no farmhand cabin to warp to.", LogLevel.Warn);
            return PlayerCommandFailed(command, "host_not_supported", "The server host cannot be warped home.", uniqueMultiplayerId);
        }

        var (success, message) = warpHomeBridge.WarpHome(target);
        if (success)
        {
            Monitor.Log($"Warp-home command succeeded for player {target.Name} ({targetId}).", LogLevel.Info);
            return PlayerCommandSucceeded(command, $"Player {target.Name} was warped home.", uniqueMultiplayerId, target.Name);
        }
        else
        {
            Monitor.Log($"Warp-home command failed for player {target.Name} ({targetId}): {message}", LogLevel.Warn);
            return PlayerCommandFailed(command, "warp_failed", "The game rejected the warp-home operation.", uniqueMultiplayerId);
        }
    }

    private CommandOutcome ApproveAuth(PanelCommand command, string uniqueMultiplayerId)
    {
        if (!Context.IsWorldReady)
        {
            return PlayerCommandFailed(command, "world_not_ready", "The game world is not ready.", uniqueMultiplayerId);
        }
        if (!passwordBridge.Available)
        {
            Monitor.Log($"Approve-auth command ignored: password bridge unavailable ({passwordBridge.Detail}).", LogLevel.Warn);
            return PlayerCommandFailed(command, "bridge_unavailable", "The password bridge is unavailable.", uniqueMultiplayerId);
        }
        if (!long.TryParse(uniqueMultiplayerId, out var targetId))
        {
            return PlayerCommandFailed(command, "invalid_player_id", "The target player ID is invalid.");
        }

        var target = Game1.getOnlineFarmers().FirstOrDefault(farmer => farmer.UniqueMultiplayerID == targetId);
        if (target is null)
        {
            return PlayerCommandFailed(command, "player_not_online", "The target player is not online.", uniqueMultiplayerId);
        }
        if (target.UniqueMultiplayerID == Game1.MasterPlayer.UniqueMultiplayerID)
        {
            Monitor.Log("Approve-auth command ignored: host bypasses authentication already.", LogLevel.Warn);
            return PlayerCommandFailed(command, "host_not_supported", "The server host does not require authentication approval.", uniqueMultiplayerId);
        }

        try
        {
            if (passwordBridge.IsPlayerAuthenticated(targetId) == true)
                return PlayerCommandFailed(command, "already_authenticated", "The target player is already authenticated.", uniqueMultiplayerId);

            var password = Environment.GetEnvironmentVariable("SERVER_PASSWORD") ?? "";
            var (success, message, _, invocationFailed) = passwordBridge.TryAuthenticate(targetId, password);
            if (success)
            {
                Monitor.Log($"Approve-auth command succeeded for player {target.Name} ({targetId}).", LogLevel.Info);
                return PlayerCommandSucceeded(command, $"Authentication was approved for player {target.Name}.", uniqueMultiplayerId, target.Name);
            }

            Monitor.Log($"Approve-auth command failed for player {target.Name} ({targetId}): {message}", LogLevel.Warn);
            return PlayerCommandFailed(
                command,
                invocationFailed ? "authentication_failed" : "authentication_rejected",
                invocationFailed ? "The authentication service failed." : "The authentication service rejected the approval.",
                uniqueMultiplayerId);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Approve-auth command failed for player {targetId}: {ex}", LogLevel.Error);
            return PlayerCommandFailed(command, "authentication_failed", "The authentication service failed.", uniqueMultiplayerId);
        }
    }

    private CommandOutcome TriggerFestivalEvent(PanelCommand command)
    {
        var festivalToday = Context.IsWorldReady && Utility.isFestivalDay(Game1.dayOfMonth, Game1.season);
        var festivalActive = Context.IsWorldReady && Game1.CurrentEvent is { isFestival: true };
        var validation = FestivalCommandOutcomes.Validate(command, Context.IsWorldReady, festivalToday, festivalActive, Game1.chatBox is not null);
        if (validation is not null)
            return validation;

        try
        {
            Game1.chatBox!.textBoxEnter("!event");
            Monitor.Log("Trigger-event command dispatched (simulated \"!event\" chat message from the host); final festival effect is not confirmed.", LogLevel.Info);
            return FestivalCommandOutcomes.Dispatched(command);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Trigger-event command dispatch failed: {ex}", LogLevel.Error);
            return FestivalCommandOutcomes.Failed(command, "command_dispatch_failed", "The !event command could not be delivered to JunimoServer.");
        }
    }

    private CommandOutcome EnableJojaRoute(PanelCommand command, bool adminPromoted)
    {
        var validation = JojaCommandOutcomes.Validate(command, Context.IsWorldReady, adminPromoted, Game1.chatBox is not null);
        if (validation is not null)
            return validation;

        // JojaMember is durable save data and therefore a stronger signal than
        // JunimoServer's in-memory AlwaysOnConfig toggle. Only that durable state
        // is allowed to produce succeeded; a successful chat dispatch alone is not.
        if (Game1.MasterPlayer.mailReceived.Contains("JojaMember"))
            return JojaCommandOutcomes.Succeeded(command);

        // JunimoServer's "!joja" command requires the sending player to already hold the
        // admin role (see RoleService.IsPlayerAdmin); the panel backend grants the host
        // that role via JunimoServer's own POST /roles/admin before submitting this command.
        try
        {
            Game1.chatBox!.textBoxEnter("!joja IRREVERSIBLY_ENABLE_JOJA_RUN");
            Monitor.Log("Enable-joja command dispatched; permanent saved-game route state is not yet confirmed.", LogLevel.Info);
            return JojaCommandOutcomes.Dispatched(command);
        }
        catch (Exception ex)
        {
            Monitor.Log($"Enable-joja command dispatch failed: {ex}", LogLevel.Error);
            return JojaCommandOutcomes.Failed(command, "command_dispatch_failed", "The !joja command could not be delivered to JunimoServer.");
        }
    }

    private CommandOutcome BanPlayer(PanelCommand command, string uniqueMultiplayerId, string name, bool adminPromoted)
    {
        if (!Context.IsWorldReady)
            return PlayerCommandFailed(command, "world_not_ready", "The game world is not ready.", uniqueMultiplayerId);
        if (!adminPromoted)
            return PlayerCommandFailed(command, "admin_promotion_failed", "JunimoServer admin promotion was not confirmed.", uniqueMultiplayerId);

        name = SanitizeChatText(name, 60);
        var candidates = Game1.getAllFarmers()
            .Select(farmer => new BanCandidate(farmer.UniqueMultiplayerID.ToString(), farmer.Name, farmer.UniqueMultiplayerID == Game1.MasterPlayer.UniqueMultiplayerID))
            .ToArray();
        var directBanAvailable = Game1.server is not null;
        var resolution = BanTargetResolver.Resolve(candidates, uniqueMultiplayerId, requireUniqueNameForFallback: !directBanAvailable);
        if (!resolution.Success)
        {
            var message = resolution.ErrorCode switch
            {
                "ambiguous_player" => "Multiple players share the fallback ban name.",
                "host_not_supported" => "The server host cannot be banned.",
                _ => "The target player was not found.",
            };
            return PlayerCommandFailed(command, resolution.ErrorCode, message, uniqueMultiplayerId);
        }

        var target = resolution.Target!;
        if (directBanAvailable && long.TryParse(target.PlayerId, out var targetId))
        {
            try
            {
                Game1.server!.ban(targetId);
                Monitor.Log($"Ban invoked for player {target.Name} ({target.PlayerId}).", LogLevel.Info);
                return PlayerCommandSucceeded(command, $"Player {target.Name} was banned.", target.PlayerId, target.Name);
            }
            catch (Exception ex)
            {
                Monitor.Log($"Ban failed for player {target.Name} ({target.PlayerId}): {ex}", LogLevel.Error);
                return PlayerCommandFailed(command, "ban_failed", "The game server rejected the ban operation.", target.PlayerId);
            }
        }

        if (Game1.chatBox is null || string.IsNullOrWhiteSpace(target.Name))
            return PlayerCommandFailed(command, "command_dispatch_failed", "JunimoServer ban command dispatch is unavailable.", target.PlayerId);
        try
        {
            Game1.chatBox.textBoxEnter($"!ban {target.Name}");
            Monitor.Log($"Ban command dispatched to JunimoServer for player {target.Name} ({target.PlayerId}).", LogLevel.Info);
            return NewOutcome(command, CommandStatuses.Dispatched, "ok", "Ban command dispatched to JunimoServer; final processing could not be confirmed.");
        }
        catch (Exception ex)
        {
            Monitor.Log($"Ban command dispatch failed for player {target.Name} ({target.PlayerId}): {ex}", LogLevel.Error);
            return PlayerCommandFailed(command, "command_dispatch_failed", "JunimoServer ban command dispatch failed.", target.PlayerId);
        }
    }

    private static string SanitizeChatText(string input, int maxLength)
    {
        if (string.IsNullOrEmpty(input))
            return "";

        var sanitized = new string(input.Where(c => !char.IsControl(c) || c == ' ').ToArray()).Trim();
        while (sanitized.Contains("  "))
            sanitized = sanitized.Replace("  ", " ");
        if (sanitized.Length > maxLength)
            sanitized = sanitized[..maxLength];
        return sanitized;
    }

    private static LocalizedContentManager.LanguageCode DetectChatLanguage(string text)
    {
        var hasCyrillic = false;
        var hasHangul = false;
        var hasKana = false;
        var hasHan = false;
        var hasThai = false;

        foreach (var c in text)
        {
            if (c >= 0x0400 && c <= 0x04FF)
                hasCyrillic = true;
            else if ((c >= 0xAC00 && c <= 0xD7A3) || (c >= 0x1100 && c <= 0x11FF))
                hasHangul = true;
            else if ((c >= 0x3040 && c <= 0x309F) || (c >= 0x30A0 && c <= 0x30FF))
                hasKana = true;
            else if (c >= 0x4E00 && c <= 0x9FFF)
                hasHan = true;
            else if (c >= 0x0E00 && c <= 0x0E7F)
                hasThai = true;
        }

        if (hasKana)
            return LocalizedContentManager.LanguageCode.ja;
        if (hasHangul)
            return LocalizedContentManager.LanguageCode.ko;
        if (hasCyrillic)
            return LocalizedContentManager.LanguageCode.ru;
        if (hasThai)
            return LocalizedContentManager.LanguageCode.th;
        if (hasHan)
            return LocalizedContentManager.LanguageCode.zh;
        return LocalizedContentManager.LanguageCode.en;
    }

    private void WriteJson(string path, object value)
    {
		WriteJsonAtomic(path, value);
    }

    private bool WriteJsonAtomic(string path, object value)
    {
        var tempPath = "";
        try
        {
			ContractFile.WriteJsonAtomic(path, value);
            return true;
        }
        catch (Exception ex)
        {
            Monitor.Log($"Failed to atomically write {path}: {ex}", LogLevel.Warn);
            return false;
        }
        finally
        {
            if (tempPath.Length > 0 && File.Exists(tempPath))
                File.Delete(tempPath);
        }
    }

}
#endif
