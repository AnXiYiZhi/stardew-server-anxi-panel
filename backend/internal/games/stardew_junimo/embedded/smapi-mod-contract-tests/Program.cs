using StardewAnxiPanel.Control;

var command = new PanelCommand
{
    Id = "0123456789abcdef0123456789abcdef",
    Name = "test",
    CreatedAt = DateTimeOffset.UtcNow,
};

void Expect(CommandOutcome outcome, string status, string code)
{
    if (outcome.Status != status || outcome.ErrorCode != code || outcome.CommandId != command.Id)
        throw new InvalidOperationException($"Expected {status}/{code}, got {outcome.Status}/{outcome.ErrorCode}");
}

Expect(PlayerCommandOutcomes.ValidateTarget(command, "1", false, null, true, true, false)!, CommandStatuses.Failed, "world_not_ready");
Expect(PlayerCommandOutcomes.ValidateTarget(command, "1", true, false, true, true, false)!, CommandStatuses.Failed, "bridge_unavailable");
Expect(PlayerCommandOutcomes.ValidateTarget(command, "bad", true, true, false, false, false)!, CommandStatuses.Failed, "invalid_player_id");
Expect(PlayerCommandOutcomes.ValidateTarget(command, "1", true, true, true, false, false)!, CommandStatuses.Failed, "player_not_online");
Expect(PlayerCommandOutcomes.ValidateTarget(command, "1", true, true, true, true, true)!, CommandStatuses.Failed, "host_not_supported");

foreach (var code in new[] { "warp_failed", "kick_failed", "already_authenticated", "authentication_rejected", "authentication_failed" })
    Expect(PlayerCommandOutcomes.Failed(command, code, "test", "1"), CommandStatuses.Failed, code);

Expect(PlayerCommandOutcomes.Succeeded(command, "test", "1", "Leah"), CommandStatuses.Succeeded, "ok");

Expect(BroadcastOutcomeValidator.Validate(command, "", true, true)!, CommandStatuses.Failed, "empty_message");
Expect(BroadcastOutcomeValidator.Validate(command, "hello", false, true)!, CommandStatuses.Failed, "world_not_ready");
Expect(BroadcastOutcomeValidator.Validate(command, "hello", true, false)!, CommandStatuses.Failed, "chat_unavailable");
if (BroadcastOutcomeValidator.Validate(command, "hello", true, true) is not null)
    throw new InvalidOperationException("valid broadcast must pass validation");

var banCandidates = new[]
{
    new BanCandidate("1", "Host", true),
    new BanCandidate("2", "Leah", false),
};
if (BanTargetResolver.Resolve(banCandidates, "2", false).Target?.PlayerId != "2")
    throw new InvalidOperationException("precise ban target was not resolved by ID");
if (BanTargetResolver.Resolve(banCandidates, "404", false).ErrorCode != "player_not_found")
    throw new InvalidOperationException("missing ban target was not rejected");
if (BanTargetResolver.Resolve(banCandidates, "1", false).ErrorCode != "host_not_supported")
    throw new InvalidOperationException("host ban was not rejected");
var duplicateNames = banCandidates.Append(new BanCandidate("3", "Leah", false));
if (BanTargetResolver.Resolve(duplicateNames, "2", true).ErrorCode != "ambiguous_player")
    throw new InvalidOperationException("ambiguous fallback ban name was not rejected");
Expect(PlayerCommandOutcomes.Failed(command, "command_dispatch_failed", "test", "2"), CommandStatuses.Failed, "command_dispatch_failed");
Expect(PlayerCommandOutcomes.Failed(command, "ban_failed", "test", "2"), CommandStatuses.Failed, "ban_failed");
Expect(PlayerCommandOutcomes.Failed(command, "admin_promotion_failed", "test", "2"), CommandStatuses.Failed, "admin_promotion_failed");
Expect(PlayerCommandOutcomes.Succeeded(command, "broadcast accepted", "", ""), CommandStatuses.Succeeded, "ok");

Expect(FestivalCommandOutcomes.Validate(command, false, false, false, false)!, CommandStatuses.Failed, "world_not_ready");
Expect(FestivalCommandOutcomes.Validate(command, true, false, false, true)!, CommandStatuses.Failed, "no_festival_today");
Expect(FestivalCommandOutcomes.Validate(command, true, true, false, true)!, CommandStatuses.Failed, "festival_not_active");
Expect(FestivalCommandOutcomes.Dispatched(command), CommandStatuses.Dispatched, "ok");

Expect(JojaCommandOutcomes.Validate(command, false, false, false)!, CommandStatuses.Failed, "world_not_ready");
Expect(JojaCommandOutcomes.Validate(command, true, false, true)!, CommandStatuses.Failed, "admin_promotion_failed");
Expect(JojaCommandOutcomes.Dispatched(command), CommandStatuses.Dispatched, "ok");
Expect(JojaCommandOutcomes.Succeeded(command), CommandStatuses.Succeeded, "ok");

void ExpectPause(PauseDecision decision, bool shouldPause, PauseReason reason, string scenario)
{
    if (decision.ShouldForcePause != shouldPause || decision.Reason != reason)
        throw new InvalidOperationException($"Pause policy failed for {scenario}: got {decision.ShouldForcePause}/{decision.Reason}");
}

PauseDecision Pause(
    int connections = 0,
    bool festival = false,
    int time = 1200,
    bool enabled = true,
    bool server = true,
    bool worldReady = true,
    bool countKnown = true
) => PausePolicy.Evaluate(enabled, server, worldReady, countKnown, connections, festival, time);

ExpectPause(Pause(), true, PauseReason.NoConnectedClients, "normal daytime with no clients");
ExpectPause(Pause(time: 610), true, PauseReason.NoConnectedClients, "idle pause lower time boundary");
ExpectPause(Pause(time: 2500), true, PauseReason.NoConnectedClients, "idle pause upper time boundary");
ExpectPause(Pause(time: 600), false, PauseReason.None, "new-day transition");
ExpectPause(Pause(time: 2510), false, PauseReason.None, "forced pass-out window");
ExpectPause(Pause(festival: true), false, PauseReason.None, "festival automation");
ExpectPause(Pause(connections: 1), false, PauseReason.None, "client handshake");
ExpectPause(Pause(connections: 2), false, PauseReason.None, "all connected-client pause state is delegated upstream");
ExpectPause(Pause(countKnown: false), false, PauseReason.None, "unknown server connection state");
ExpectPause(Pause(enabled: false), false, PauseReason.None, "auto pause disabled");
ExpectPause(Pause(server: false), false, PauseReason.None, "non-server game");
ExpectPause(Pause(worldReady: false), false, PauseReason.None, "world not ready");

var tracker = new PendingSaveCommandTracker();
var now = DateTimeOffset.UtcNow;
Expect(tracker.Begin(command, true, now, TimeSpan.FromSeconds(30)), CommandStatuses.Running, "");
if (tracker.PendingCommandId != command.Id)
    throw new InvalidOperationException("save tracker did not retain the originating command ID");
var otherSave = new PanelCommand { Id = "11111111111111111111111111111111", Name = "save-now", CreatedAt = now };
var duplicateSave = tracker.Begin(otherSave, true, now, TimeSpan.FromSeconds(30));
if (duplicateSave.Status != CommandStatuses.Failed || duplicateSave.ErrorCode != "save_already_pending" || duplicateSave.CommandId != otherSave.Id)
    throw new InvalidOperationException("a concurrent save command was not rejected with its own command ID");
Expect(tracker.Complete(now.AddSeconds(1))!, CommandStatuses.Succeeded, "ok");
if (tracker.Complete(now.AddSeconds(2)) is not null)
    throw new InvalidOperationException("one Saved event completed a save command more than once");
Expect(tracker.Begin(command, false, now, TimeSpan.FromSeconds(30)), CommandStatuses.Failed, "world_not_ready");
Expect(tracker.Begin(command, true, now, TimeSpan.FromSeconds(30)), CommandStatuses.Running, "");
if (tracker.Expire(now.AddSeconds(29)) is not null)
    throw new InvalidOperationException("save command expired before its deadline");
Expect(tracker.Expire(now.AddSeconds(31))!, CommandStatuses.Failed, "save_timeout");
Expect(tracker.Begin(command, true, now, TimeSpan.FromSeconds(30)), CommandStatuses.Running, "");
Expect(tracker.Fail(now.AddSeconds(1), "save_ui_busy", "busy")!, CommandStatuses.Failed, "save_ui_busy");
if (tracker.PendingCommandId is not null)
    throw new InvalidOperationException("failed save command remained pending");

var frontier = new RuntimeFarmType("FrontierFarm", "边境农场", false);
var meadowlands = new RuntimeFarmType("MeadowlandsFarm", "草原农场", false, "builtin");
var secondMod = new RuntimeFarmType("SecondFarm", "Second", true);

var explicitFrontier = NewGameControlContract.ResolveFarmType("FrontierFarm", new[] { meadowlands, frontier });
if (!explicitFrontier.Resolved || explicitFrontier.ResolvedFarmType != "FrontierFarm" || explicitFrontier.WhichFarm != 7 || explicitFrontier.ModFarm?.Id != "FrontierFarm")
    throw new InvalidOperationException("FrontierFarm explicit runtime ID was not resolved");
if (NewGameControlContract.CatalogContainsRequestedFarm(new[] { new OptionItem { Id = "standard" } }, "FrontierFarm"))
    throw new InvalidOperationException("early catalog must not resolve FrontierFarm");
if (!NewGameControlContract.CatalogContainsRequestedFarm(new[] { new OptionItem { Id = "FrontierFarm" } }, "FrontierFarm"))
    throw new InvalidOperationException("refreshed catalog must resolve FrontierFarm");
var unknownFarm = NewGameControlContract.ResolveFarmType("MissingFarm", new[] { frontier });
if (unknownFarm.Resolved || unknownFarm.WhichFarm != 0 || unknownFarm.ResolvedFarmType != "standard" || unknownFarm.Warning.Length == 0)
    throw new InvalidOperationException("unknown farm incorrectly reported resolved");
if (NewGameControlContract.ResolveFarmType("modded", Array.Empty<RuntimeFarmType>()).Resolved)
    throw new InvalidOperationException("modded without farms must fail");
if (NewGameControlContract.ResolveFarmType("modded", new[] { frontier }).ResolvedFarmType != "FrontierFarm")
    throw new InvalidOperationException("single modded farm was not selected");
if (NewGameControlContract.ResolveFarmType("modded", new[] { frontier, secondMod }).ResolvedFarmType != "FrontierFarm")
    throw new InvalidOperationException("first true modded farm was not selected deterministically");
if (NewGameControlContract.ResolveFarmType("modded", new[] { meadowlands, frontier }).ResolvedFarmType != "FrontierFarm")
    throw new InvalidOperationException("MeadowlandsFarm was not excluded from generic modded selection");
if (!NewGameControlContract.ResolveFarmType("FourCorners", Array.Empty<RuntimeFarmType>()).Resolved)
    throw new InvalidOperationException("FourCorners alias was not accepted");
if (!NewGameControlContract.ResolveFarmType("MeadowlandsFarm", new[] { meadowlands }).Resolved)
    throw new InvalidOperationException("MeadowlandsFarm was not accepted");

var init = new InitConfig { TransactionId = command.Id };
var marker = new PendingNewGameMarker
{
    SchemaVersion = 1,
    TransactionId = command.Id,
    RequestedFarmType = "FrontierFarm",
    CreatedAt = now,
    ExpiresAt = now.AddMinutes(10),
    State = "pending",
};
if (!NewGameControlContract.ValidateMarker(marker, init, now).Valid)
    throw new InvalidOperationException("matching transaction marker was rejected");
if (NewGameControlContract.ValidateMarker(marker, new InitConfig { TransactionId = otherSave.Id }, now).ErrorCode != "transaction_mismatch")
    throw new InvalidOperationException("mismatched transaction marker was accepted");
marker.ExpiresAt = now.AddSeconds(-1);
if (NewGameControlContract.ValidateMarker(marker, init, now).ErrorCode != "marker_expired")
    throw new InvalidOperationException("expired marker was accepted");
if (NewGameControlContract.ShouldClearMarkerOnSaveLoaded)
    throw new InvalidOperationException("SaveLoaded must not clear an active backend transaction marker");

var fingerprintA = NewGameControlContract.ComputeModFingerprint(new[]
{
    new LoadedModItem { UniqueId = "FlashShifter.SVECode", Version = "1.0.0" },
    new LoadedModItem { UniqueId = "Pathoschild.ContentPatcher", Version = "2.0.0" },
});
var fingerprintB = NewGameControlContract.ComputeModFingerprint(new[]
{
    new LoadedModItem { UniqueId = "pathoschild.contentpatcher", Version = "2.0.0" },
    new LoadedModItem { UniqueId = "flashshifter.svecode", Version = "1.0.0" },
});
if (fingerprintA != fingerprintB || fingerprintA != "0bc44377624ec2e2b98cda195b9df9ba06d9feed38be9c83991566d42bc12e22")
    throw new InvalidOperationException("mod fingerprint is not stable under sorting/case normalization");

var request = new FarmCatalogRequest
{
    SchemaVersion = 1, RequestId = command.Id, TransactionId = command.Id,
    GeneratedAt = now, ExpiresAt = now.AddMinutes(1), RequestedFarmType = "FrontierFarm",
};
if (!NewGameControlContract.IsFreshCatalogRequest(request, now))
    throw new InvalidOperationException("fresh matching catalog request was rejected");
request.TransactionId = otherSave.Id;
if (NewGameControlContract.IsFreshCatalogRequest(request, now))
    throw new InvalidOperationException("mismatched catalog request was accepted");

var pendingContext = PlayerModContextContract.Pending("123", false, now);
var pendingJson = System.Text.Json.JsonSerializer.Serialize(pendingContext, ContractJson.Options);
if (!pendingJson.Contains("\"mods\": null", StringComparison.Ordinal)
    || pendingContext.ContextStatus != PlayerModContextStatuses.Pending)
{
    throw new InvalidOperationException("pending player context must publish mods:null");
}

var rawReportedMods = Enumerable.Range(0, PlayerModContextContract.MaxModsPerPlayer)
    .Select(index => new PlayerReportedMod
    {
        UniqueId = $"Example.Mod.{index}",
        Name = index == 0 ? "  Unsafe\r\nName  " : new string('n', PlayerModContextContract.MaxNameChars + 10),
        Version = " 1.0.0\u0000 ",
    })
    .Prepend(new PlayerReportedMod { UniqueId = "example.mod.0", Name = "duplicate", Version = "9.9.9" })
    .ToArray();
var reportedContext = PlayerModContextContract.Reported("123", true, " 1.6.15 ", " 4.5.2 ", rawReportedMods, now);
if (reportedContext.ContextStatus != PlayerModContextStatuses.Reported
    || reportedContext.Mods is null
    || reportedContext.Mods.Length != PlayerModContextContract.MaxModsPerPlayer
    || reportedContext.Mods.Count(mod => string.Equals(mod.UniqueId, "Example.Mod.0", StringComparison.OrdinalIgnoreCase)) != 1
    || reportedContext.Mods[0].Name.Contains('\r')
    || reportedContext.Mods[0].Name.Contains('\n')
    || reportedContext.Mods.Any(mod => mod.Name.Length > PlayerModContextContract.MaxNameChars)
    || reportedContext.Mods.Any(mod => mod.Version.Contains('\u0000')))
{
    throw new InvalidOperationException("reported player mods were not bounded, normalized, and deduplicated");
}
var excessiveContext = PlayerModContextContract.Reported(
    "124",
    true,
    "1.6.15",
    "4.5.2",
    Enumerable.Range(0, PlayerModContextContract.MaxModsPerPlayer + 1)
        .Select(index => new PlayerReportedMod { UniqueId = $"Excess.Mod.{index}", Name = "excess", Version = "1.0.0" }),
    now);
if (excessiveContext.ContextStatus != PlayerModContextStatuses.Unavailable || excessiveContext.Mods is not null)
    throw new InvalidOperationException("an excessive player mod report must become unavailable instead of being partially compared");
var emptyReported = PlayerModContextContract.Reported("456", true, "1.6.15", "4.5.2", Array.Empty<PlayerReportedMod>(), now);
var emptyReportedJson = System.Text.Json.JsonSerializer.Serialize(emptyReported, ContractJson.Options);
if (!emptyReportedJson.Contains("\"mods\": []", StringComparison.Ordinal))
    throw new InvalidOperationException("a genuinely reported empty mod list must remain distinct from mods:null");
var unavailableContext = PlayerModContextContract.Reported("789", false, null, null, null, now);
if (unavailableContext.ContextStatus != PlayerModContextStatuses.Unavailable || unavailableContext.Mods is not null)
    throw new InvalidOperationException("an unavailable context must not invent an empty mod list");
var staleContext = PlayerModContextContract.Stale(reportedContext, "123", true, now.AddSeconds(1));
if (staleContext.ContextStatus != PlayerModContextStatuses.Stale
    || staleContext.Mods?.Length != reportedContext.Mods.Length
    || staleContext.ReportedAt != reportedContext.ReportedAt)
{
    throw new InvalidOperationException("disconnect did not preserve the last report as stale");
}

var lifecycle = new Dictionary<string, PlayerModContext>(StringComparer.Ordinal);
if (!PlayerModContextLifecycle.Connect(lifecycle, "100", false, now)
    || lifecycle["100"].ContextStatus != PlayerModContextStatuses.Pending
    || lifecycle["100"].Mods is not null)
{
    throw new InvalidOperationException("a vanilla/mobile peer did not enter pending with mods:null");
}
if (PlayerModContextLifecycle.ExpirePending(lifecycle, now.AddSeconds(9), TimeSpan.FromSeconds(10)))
    throw new InvalidOperationException("a pending peer expired before the configured timeout");
if (!PlayerModContextLifecycle.ExpirePending(lifecycle, now.AddSeconds(10), TimeSpan.FromSeconds(10))
    || lifecycle["100"].ContextStatus != PlayerModContextStatuses.Unavailable
    || lifecycle["100"].Mods is not null)
{
    throw new InvalidOperationException("a peer without ModContext did not become unavailable with mods:null");
}

var cheatsMods = new[]
{
    new PlayerReportedMod { UniqueId = "CJBok.CheatsMenu", Name = "CJB Cheats Menu", Version = "1.37.2" },
    new PlayerReportedMod { UniqueId = "cjbOK.cheatsmenu", Name = "duplicate must be ignored", Version = "9.9.9" },
    new PlayerReportedMod { UniqueId = "Example.Client", Name = "Client Mod", Version = "2.1.0" },
};
if (!PlayerModContextLifecycle.Connect(lifecycle, "200", true, now)
    || !PlayerModContextLifecycle.Report(lifecycle, "200", true, "1.6.15", "4.5.2", cheatsMods, now.AddSeconds(1))
    || lifecycle["200"].ContextStatus != PlayerModContextStatuses.Reported
    || lifecycle["200"].Mods?.Length != 2
    || lifecycle["200"].Mods?.Single(mod => mod.UniqueId == "CJBok.CheatsMenu").Version != "1.37.2")
{
    throw new InvalidOperationException("a SMAPI peer report did not preserve exact normalized IDs, names, and versions");
}

if (!PlayerModContextLifecycle.Report(
        lifecycle,
        "201",
        true,
        "1.6.15",
        "4.5.2",
        new[] { new PlayerReportedMod { UniqueId = "CJBok.ItemSpawner", Name = "CJB Item Spawner", Version = "2.5.1" } },
        now.AddSeconds(2))
    || PlayerModContextLifecycle.Connect(lifecycle, "201", true, now.AddSeconds(3))
    || lifecycle["201"].ContextStatus != PlayerModContextStatuses.Reported)
{
    throw new InvalidOperationException("PeerContextReceived-before-PeerConnected ordering lost a fresh Item Spawner report");
}

if (!PlayerModContextLifecycle.Disconnect(lifecycle, "200", true, now.AddSeconds(4))
    || lifecycle["200"].ContextStatus != PlayerModContextStatuses.Stale
    || lifecycle["200"].Mods?.Any(mod => mod.UniqueId == "CJBok.CheatsMenu") != true
    || lifecycle["201"].Mods?.Single().UniqueId != "CJBok.ItemSpawner")
{
    throw new InvalidOperationException("disconnect did not stale only the target player's last report");
}
if (!PlayerModContextLifecycle.Connect(lifecycle, "200", true, now.AddSeconds(5))
    || lifecycle["200"].ContextStatus != PlayerModContextStatuses.Pending
    || lifecycle["200"].Mods is not null
    || !PlayerModContextLifecycle.Report(
        lifecycle,
        "200",
        true,
        "1.6.15",
        "4.5.2",
        new[] { new PlayerReportedMod { UniqueId = "Example.Reconnected", Name = "Reconnected", Version = "3.0.0" } },
        now.AddSeconds(6))
    || lifecycle["200"].Mods?.Single().UniqueId != "Example.Reconnected"
    || lifecycle["201"].Mods?.Single().UniqueId != "CJBok.ItemSpawner")
{
    throw new InvalidOperationException("reconnect did not clear the old player report or isolated peer contexts crossed players");
}

var restarted = PlayerModContextContract.NormalizeLoadedFile(
    new PlayerModContextsFile
    {
        SchemaVersion = PlayerModContextContract.SchemaVersion,
        UpdatedAt = now.AddSeconds(6),
        Players = lifecycle,
    },
    now.AddSeconds(7));
if (restarted.Players.Count != lifecycle.Count
    || restarted.Players.Values.Any(context => context.ContextStatus != PlayerModContextStatuses.Stale)
    || restarted.Players["100"].Mods is not null
    || restarted.Players["200"].Mods?.Single().UniqueId != "Example.Reconnected"
    || restarted.Players["201"].Mods?.Single().UniqueId != "CJBok.ItemSpawner")
{
    throw new InvalidOperationException("server restart did not stale contexts without crossing or inventing player reports");
}

var atomicDir = Path.Combine(Path.GetTempPath(), "sap-contract-" + Guid.NewGuid().ToString("N"));
var atomicPath = Path.Combine(atomicDir, "options.json");
try
{
    ContractFile.WriteJsonAtomic(atomicPath, new PanelOptions { RequestId = command.Id, TransactionId = command.Id });
    var parsed = System.Text.Json.JsonSerializer.Deserialize<PanelOptions>(File.ReadAllText(atomicPath), ContractJson.Options);
    if (parsed?.RequestId != command.Id || Directory.GetFiles(atomicDir, ".tmp-*").Length != 0)
        throw new InvalidOperationException("options atomic write did not publish exactly one complete file");
}
finally
{
    if (Directory.Exists(atomicDir))
        Directory.Delete(atomicDir, true);
}
Console.WriteLine("control command outcome branch tests passed");
