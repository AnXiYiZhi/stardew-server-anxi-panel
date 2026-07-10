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

    private string controlDir = "";
    private string commandDir = "";
    private InitConfig? initConfig;
    private bool isJunimoRuntime;
    private bool panelCustomizationApplied;
    private bool singlePlayerMenuPauseApplied;
    private int? singlePlayerMenuPauseSavedInterval;
    private bool pendingNewGameOptions;

    public override void Entry(IModHelper helper)
    {
        controlDir = Environment.GetEnvironmentVariable("SAP_CONTROL_DIR")
            ?? Path.Combine(helper.DirectoryPath, "..", "..", "control");
        controlDir = Path.GetFullPath(controlDir);
        commandDir = Path.Combine(controlDir, "commands");
        Directory.CreateDirectory(commandDir);

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
        pendingNewGameOptions = File.Exists(PendingNewGamePath());
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
        ClearPendingNewGameOptions();
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
        WritePlayers();
        ConsumeCommands();
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

        SetFarmType(initConfig.FarmType);
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

    private void ClearPendingNewGameOptions()
    {
        pendingNewGameOptions = false;
        try
        {
            File.Delete(PendingNewGamePath());
        }
        catch (Exception ex)
        {
            Monitor.Log($"Failed to clear pending new-game marker: {ex}", LogLevel.Warn);
        }
    }

    private static void ApplyDirectIpNetworkPolicy()
    {
        if (Game1.options is null)
            return;

        Game1.options.ipConnectionsEnabled = true;
    }

    private static void SetFarmType(string farmType)
    {
        Game1.whichModFarm = null;
        Game1.spawnMonstersAtNight = false;

        switch (farmType.Trim().ToLowerInvariant())
        {
            case "riverland":
                Game1.whichFarm = 1;
                return;
            case "forest":
                Game1.whichFarm = 2;
                return;
            case "hilltop":
            case "hill-top":
            case "hills":
                Game1.whichFarm = 3;
                return;
            case "wilderness":
                Game1.whichFarm = 4;
                Game1.spawnMonstersAtNight = true;
                return;
            case "four_corners":
            case "four-corners":
            case "fourcorners":
                Game1.whichFarm = 5;
                return;
            case "beach":
                Game1.whichFarm = 6;
                return;
            case "meadowlands":
            case "meadowlandsfarm":
                var meadowlands = DataLoader.AdditionalFarms(Game1.content)
                    .FirstOrDefault(farm => string.Equals(farm.Id, "MeadowlandsFarm", StringComparison.OrdinalIgnoreCase));
                if (meadowlands is not null)
                {
                    Game1.whichFarm = 7;
                    Game1.whichModFarm = meadowlands;
                    Game1.spawnMonstersAtNight = meadowlands.SpawnMonstersByDefault;
                    return;
                }
                Game1.whichFarm = 0;
                return;
            default:
                Game1.whichFarm = 0;
                return;
        }
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

    private void WritePanelOptions()
    {
        try
        {
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
                    farmTypes.Add(Option(panelId, label, image));
                }
            }

            var options = new PanelOptions
            {
                Source = "smapi",
                GeneratedAt = DateTimeOffset.UtcNow,
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
            WriteJson(Path.Combine(controlDir, "options.json"), options);
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
        return "data:image/png;base64," + Convert.ToBase64String(stream.ToArray());
    }

    private static OptionItem Option(string id, string label, string image = "", string description = "", string group = "")
    {
        return new OptionItem { Id = id, Label = label, Image = image, Description = description, Group = group };
    }

    private static string PreviewSvg(string text, string bg, string fg)
    {
        var svg = "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 240 132\">"
            + $"<rect width=\"240\" height=\"132\" rx=\"14\" fill=\"{fg}\"/>"
            + $"<path d=\"M0 92c48-18 82 19 128 0s72-6 112 12v28H0z\" fill=\"{bg}\"/>"
            + "<circle cx=\"190\" cy=\"35\" r=\"18\" fill=\"#f7d76b\"/>"
            + $"<text x=\"24\" y=\"76\" font-family=\"Arial,'Microsoft YaHei',sans-serif\" font-size=\"30\" font-weight=\"700\" fill=\"#1f2b1f\">{text}</text>"
            + "</svg>";
        return "data:image/svg+xml;base64," + Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes(svg));
    }

    private void WriteStatus(string state, string message, string? saveId = null)
    {
        var status = new RuntimeStatus
        {
            State = state,
            Message = message,
            SaveId = string.IsNullOrWhiteSpace(saveId) ? initConfig?.SaveId : saveId,
            UpdatedAt = DateTimeOffset.UtcNow,
        };
        WriteJson(Path.Combine(controlDir, "status.json"), status);
    }

    private void WritePlayers()
    {
        var walletMode = Game1.player?.team?.useSeparateWallets.Value == true ? "separate" : "shared";
        var players = Context.IsWorldReady
            ? Game1.getOnlineFarmers().Select(farmer => BuildPlayerInfo(farmer, walletMode)).ToArray()
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

    private static PlayerInfo BuildPlayerInfo(Farmer farmer, string walletMode)
    {
        var location = ResolveFarmerLocation(farmer);
        var tile = farmer.TilePoint;
        return new PlayerInfo
        {
            Name = farmer.Name,
            UniqueMultiplayerId = farmer.UniqueMultiplayerID.ToString(),
            IsHost = farmer.UniqueMultiplayerID == Game1.MasterPlayer.UniqueMultiplayerID,
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
                if (command is not null)
                    HandleCommand(command);
                File.Delete(path);
            }
            catch (Exception ex)
            {
                Monitor.Log($"Failed to consume command {path}: {ex}", LogLevel.Warn);
            }
        }
    }

    private void HandleCommand(PanelCommand command)
    {
        switch (command.Name)
        {
            case "save-now":
                if (Context.IsWorldReady)
                {
                    Game1.saveOnNewDay = true;
                    WriteStatus("command", "Save command accepted.");
                }
                break;
            case "broadcast":
                var message = command.Payload is not null && command.Payload.TryGetValue("message", out var rawMessage)
                    ? rawMessage.GetString()
                    : "";
                SendBroadcastMessage(message ?? "");
                break;
            case "kick":
                var targetId = command.Payload is not null && command.Payload.TryGetValue("uniqueMultiplayerId", out var rawTargetId)
                    ? rawTargetId.GetString()
                    : "";
                KickPlayer(targetId ?? "");
                break;
            case "trigger-event":
                TriggerFestivalEvent();
                break;
            case "enable-joja":
                EnableJojaRoute();
                break;
            case "stop":
                WriteStatus("stopping", "Stop command received. Container stop will be handled by backend later.");
                break;
            default:
                Monitor.Log($"Unknown panel command: {command.Name}", LogLevel.Warn);
                break;
        }
    }

    private void SendBroadcastMessage(string message)
    {
        message = SanitizeChatText(message, 450);
        if (string.IsNullOrWhiteSpace(message))
        {
            WriteStatus("command", "Broadcast command ignored because the message was empty.");
            return;
        }
        if (!Context.IsWorldReady)
        {
            WriteStatus("command", "Broadcast command ignored because the world is not ready.");
            Monitor.Log("Broadcast command ignored because the world is not ready.", LogLevel.Warn);
            return;
        }

        var text = $"[Panel] {message}";
        Helper.Reflection.GetField<Multiplayer>(typeof(Game1), "multiplayer").GetValue()
            .sendChatMessage(DetectChatLanguage(text), text, Multiplayer.AllPlayers);
        WriteStatus("command", "Broadcast command sent.");
        Monitor.Log($"Broadcast command sent: {text}", LogLevel.Info);
    }

    private void KickPlayer(string uniqueMultiplayerId)
    {
        if (!Context.IsWorldReady)
        {
            WriteStatus("command", "Kick command ignored because the world is not ready.");
            return;
        }
        if (!long.TryParse(uniqueMultiplayerId, out var targetId))
        {
            WriteStatus("command", "Kick command ignored because the target player id was invalid.");
            return;
        }

        var target = Game1.getOnlineFarmers().FirstOrDefault(farmer => farmer.UniqueMultiplayerID == targetId);
        if (target is null)
        {
            WriteStatus("command", "Kick command ignored because the target player is not online.");
            return;
        }
        if (target.UniqueMultiplayerID == Game1.MasterPlayer.UniqueMultiplayerID)
        {
            WriteStatus("command", "Kick command ignored because the target player is the host.");
            Monitor.Log("Kick command ignored: cannot kick the host.", LogLevel.Warn);
            return;
        }

        try
        {
            Game1.server?.kick(targetId);
            WriteStatus("command", $"Kick command sent for player {target.Name}.");
            Monitor.Log($"Kick command sent for player {target.Name} ({targetId}).", LogLevel.Info);
        }
        catch (Exception ex)
        {
            WriteStatus("command", "Kick command failed.");
            Monitor.Log($"Kick command failed for player {targetId}: {ex}", LogLevel.Error);
        }
    }

    private void TriggerFestivalEvent()
    {
        if (!Context.IsWorldReady || Game1.chatBox is null)
        {
            WriteStatus("command", "Trigger-event command ignored because the world is not ready.");
            return;
        }

        Game1.chatBox.textBoxEnter("!event");
        WriteStatus("command", "Trigger-event command sent.");
        Monitor.Log("Trigger-event command sent (simulated \"!event\" chat message from the host).", LogLevel.Info);
    }

    private void EnableJojaRoute()
    {
        if (!Context.IsWorldReady || Game1.chatBox is null)
        {
            WriteStatus("command", "Enable-joja command ignored because the world is not ready.");
            return;
        }

        // JunimoServer's "!joja" command requires the sending player to already hold the
        // admin role (see RoleService.IsPlayerAdmin); the panel backend grants the host
        // that role via JunimoServer's own POST /roles/admin before submitting this command.
        Game1.chatBox.textBoxEnter("!joja IRREVERSIBLY_ENABLE_JOJA_RUN");
        WriteStatus("command", "Enable-joja command sent.");
        Monitor.Log("Enable-joja command sent (simulated \"!joja\" chat message from the host).", LogLevel.Info);
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
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllText(path, JsonSerializer.Serialize(value, ContractJson.Options));
        }
        catch (Exception ex)
        {
            Monitor.Log($"Failed to write {path}: {ex}", LogLevel.Warn);
        }
    }

}
#endif
