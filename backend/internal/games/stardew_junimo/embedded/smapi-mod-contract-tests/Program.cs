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
var genericTrackedSave = tracker.Complete("frozen-tx-must-not-leak", "Current_1", now.AddSeconds(1))!;
Expect(genericTrackedSave, CommandStatuses.Succeeded, "ok");
if (genericTrackedSave.Details is null || genericTrackedSave.Details.ContainsKey("transactionId"))
    throw new InvalidOperationException("generic tracker completion fabricated a transaction ID");
if (tracker.Complete("", "Current_1", now.AddSeconds(2)) is not null)
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

var targetedSaveCommand = new PanelCommand
{
    Id = command.Id,
    Name = "save-now",
    CreatedAt = now,
    Payload = new Dictionary<string, System.Text.Json.JsonElement>
    {
        [SaveCommandContract.TransactionIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("tx-1"),
        [SaveCommandContract.SaveIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("Target_1"),
    },
};
var saveExpectation = SaveCommandContract.ParseExpectation(targetedSaveCommand);
if (!saveExpectation.Valid || !saveExpectation.IsTargeted
    || saveExpectation.TransactionId != "tx-1" || saveExpectation.SaveId != "Target_1")
{
    throw new InvalidOperationException("targeted save command payload was not parsed exactly");
}
var targetedSaveSuccess = SaveCommandContract.CompleteSavedEvent(targetedSaveCommand, "Target_1", now.AddSeconds(1));
Expect(targetedSaveSuccess, CommandStatuses.Succeeded, "ok");
if (targetedSaveSuccess.Details is null
    || targetedSaveSuccess.Details.GetValueOrDefault("event") != SaveCommandContract.SavedEventName
    || targetedSaveSuccess.Details.GetValueOrDefault("saveId") != "Target_1"
    || targetedSaveSuccess.Details.GetValueOrDefault("expectedSaveId") != "Target_1"
    || targetedSaveSuccess.Details.GetValueOrDefault("transactionId") != "tx-1")
{
    throw new InvalidOperationException("targeted save success details did not preserve the expected target");
}
var targetedSaveMismatch = SaveCommandContract.CompleteSavedEvent(targetedSaveCommand, "Old_1", now.AddSeconds(1));
Expect(targetedSaveMismatch, CommandStatuses.Failed, "save_target_mismatch");
if (targetedSaveMismatch.Details is null
    || targetedSaveMismatch.Details.GetValueOrDefault("saveId") != "Old_1"
    || targetedSaveMismatch.Details.GetValueOrDefault("expectedSaveId") != "Target_1"
    || targetedSaveMismatch.Details.GetValueOrDefault("transactionId") != "tx-1")
{
    throw new InvalidOperationException("save target mismatch details did not report expected and actual identities");
}
var targetedTracker = new PendingSaveCommandTracker();
Expect(targetedTracker.Begin(targetedSaveCommand, true, now, TimeSpan.FromSeconds(30)), CommandStatuses.Running, "");
Expect(targetedTracker.Complete("tx-1", "Target_1", now.AddSeconds(1))!, CommandStatuses.Succeeded, "ok");
Expect(targetedTracker.Begin(targetedSaveCommand, true, now, TimeSpan.FromSeconds(30)), CommandStatuses.Running, "");
Expect(targetedTracker.Complete("", "Old_1", now.AddSeconds(1))!, CommandStatuses.Failed, "save_target_mismatch");
if (targetedTracker.PendingCommandId is not null)
    throw new InvalidOperationException("a mismatched Saved event left the targeted save command pending");
Expect(targetedTracker.Begin(targetedSaveCommand, true, now, TimeSpan.FromSeconds(30)), CommandStatuses.Running, "");
var transactionMismatch = targetedTracker.Complete("other-tx", "Target_1", now.AddSeconds(1))!;
Expect(transactionMismatch, CommandStatuses.Failed, "save_transaction_mismatch");
if (transactionMismatch.Details is null
    || transactionMismatch.Details.GetValueOrDefault("transactionId") != "tx-1"
    || transactionMismatch.Details.GetValueOrDefault("verifiedTransactionId") != "other-tx")
{
    throw new InvalidOperationException("save transaction mismatch details did not preserve expected and verified identities");
}
var genericSaveCommand = new PanelCommand { Id = command.Id, Name = "save-now", CreatedAt = now };
var genericSaveSuccess = SaveCommandContract.CompleteSavedEvent(genericSaveCommand, "Current_1", now.AddSeconds(1));
Expect(genericSaveSuccess, CommandStatuses.Succeeded, "ok");
if (genericSaveSuccess.Details is null
    || genericSaveSuccess.Details.GetValueOrDefault("saveId") != "Current_1"
    || genericSaveSuccess.Details.ContainsKey("transactionId")
    || genericSaveSuccess.Details.ContainsKey("expectedSaveId"))
{
    throw new InvalidOperationException("an unbound save command received stale target details");
}
var unbindSaveCommand = new PanelCommand
{
    Id = command.Id,
    Name = "save-now",
    CreatedAt = now,
    Payload = new Dictionary<string, System.Text.Json.JsonElement>
    {
        [SaveCommandContract.PreSaveActionPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement(SaveCommandContract.UnbindAllFarmhandsAction),
        [SaveCommandContract.PreSaveActionSaveIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("Imported_1"),
    },
};
var unbindExpectation = SaveCommandContract.ParseExpectation(unbindSaveCommand);
if (!unbindExpectation.Valid || unbindExpectation.IsTargeted
    || unbindExpectation.PreSaveAction != SaveCommandContract.UnbindAllFarmhandsAction
    || unbindExpectation.PreSaveActionSaveId != "Imported_1")
{
    throw new InvalidOperationException("farmhand unbind save action was not parsed exactly");
}
var unbindReady = FarmhandUnbindContract.ValidateBeforeSave(
    unbindExpectation, "Imported_1", true, true, 0, new FarmhandBindingSummary(3, 3, 3));
if (!unbindReady.CanApply)
    throw new InvalidOperationException($"valid farmhand unbind was rejected: {unbindReady.ErrorCode}");
var unbindPlayersConnected = FarmhandUnbindContract.ValidateBeforeSave(
    unbindExpectation, "Imported_1", true, true, 1, new FarmhandBindingSummary(3, 3, 3));
if (unbindPlayersConnected.CanApply || unbindPlayersConnected.ErrorCode != "farmhand_unbind_players_connected")
    throw new InvalidOperationException("farmhand unbind accepted an online human farmhand");
var unbindWrongSave = FarmhandUnbindContract.ValidateBeforeSave(
    unbindExpectation, "Other_1", true, true, 0, new FarmhandBindingSummary(3, 3, 3));
if (unbindWrongSave.CanApply || unbindWrongSave.ErrorCode != "farmhand_unbind_save_mismatch")
    throw new InvalidOperationException("farmhand unbind accepted the wrong loaded save");
Expect(SaveCommandContract.CompleteSavedEvent(
        unbindSaveCommand, "Imported_1", now.AddSeconds(1), new FarmhandBindingSummary(3, 3, 0)),
    CommandStatuses.Succeeded, "ok");
Expect(SaveCommandContract.CompleteSavedEvent(
        unbindSaveCommand, "Imported_1", now.AddSeconds(1), new FarmhandBindingSummary(3, 3, 1)),
    CommandStatuses.Failed, "farmhand_unbind_incomplete");
Expect(SaveCommandContract.CompleteSavedEvent(unbindSaveCommand, "Imported_1", now.AddSeconds(1)),
    CommandStatuses.Failed, "farmhand_unbind_unverified");
Expect(SaveCommandContract.CompleteSavedEvent(
        unbindSaveCommand, "Other_1", now.AddSeconds(1), new FarmhandBindingSummary(3, 3, 0)),
    CommandStatuses.Failed, "farmhand_unbind_save_mismatch");
var incompleteSaveCommand = new PanelCommand
{
    Id = command.Id,
    Name = "save-now",
    CreatedAt = now,
    Payload = new Dictionary<string, System.Text.Json.JsonElement>
    {
        [SaveCommandContract.TransactionIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("tx-1"),
    },
};
Expect(SaveCommandContract.CompleteSavedEvent(incompleteSaveCommand, "Target_1", now.AddSeconds(1)),
    CommandStatuses.Failed, "save_target_invalid");
var invalidTargetTracker = new PendingSaveCommandTracker();
Expect(invalidTargetTracker.Begin(incompleteSaveCommand, true, now, TimeSpan.FromSeconds(30)),
    CommandStatuses.Failed, "save_target_invalid");
if (invalidTargetTracker.PendingCommandId is not null)
    throw new InvalidOperationException("an invalid targeted save command became pending");
var wrongTypeSaveCommand = new PanelCommand
{
    Id = command.Id,
    Name = "save-now",
    CreatedAt = now,
    Payload = new Dictionary<string, System.Text.Json.JsonElement>
    {
        [SaveCommandContract.TransactionIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("tx-1"),
        [SaveCommandContract.SaveIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement(1),
    },
};
Expect(SaveCommandContract.CompleteSavedEvent(wrongTypeSaveCommand, "Target_1", now.AddSeconds(1)),
    CommandStatuses.Failed, "save_target_invalid");

var saveJournal = new PendingSaveCommandJournal { Command = targetedSaveCommand, UpdatedAt = now };
if (!SaveCommandRecoveryContract.Matches(saveJournal, targetedSaveCommand))
    throw new InvalidOperationException("durable save journal did not match its original command");
var differentTarget = new PanelCommand
{
    Id = targetedSaveCommand.Id,
    Name = targetedSaveCommand.Name,
    CreatedAt = targetedSaveCommand.CreatedAt,
    Payload = new Dictionary<string, System.Text.Json.JsonElement>
    {
        [SaveCommandContract.TransactionIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("tx-1"),
        [SaveCommandContract.SaveIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("Other_1"),
    },
};
if (SaveCommandRecoveryContract.Matches(saveJournal, differentTarget))
    throw new InvalidOperationException("durable save journal accepted a different target");
var unbindJournal = new PendingSaveCommandJournal { Command = unbindSaveCommand, UpdatedAt = now };
if (!SaveCommandRecoveryContract.Matches(unbindJournal, unbindSaveCommand))
    throw new InvalidOperationException("durable save journal did not preserve the farmhand unbind action");
var differentActionTarget = new PanelCommand
{
    Id = unbindSaveCommand.Id,
    Name = unbindSaveCommand.Name,
    CreatedAt = unbindSaveCommand.CreatedAt,
    Payload = new Dictionary<string, System.Text.Json.JsonElement>
    {
        [SaveCommandContract.PreSaveActionPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement(SaveCommandContract.UnbindAllFarmhandsAction),
        [SaveCommandContract.PreSaveActionSaveIdPayloadKey] = System.Text.Json.JsonSerializer.SerializeToElement("Other_1"),
    },
};
if (SaveCommandRecoveryContract.Matches(unbindJournal, differentActionTarget))
    throw new InvalidOperationException("durable save journal accepted a different farmhand unbind target");
var unbindWaitingResume = SaveCommandRecoveryContract.Evaluate(unbindSaveCommand, true, "Other_1", null, null);
if (unbindWaitingResume.CanResume || unbindWaitingResume.TerminalFailure || unbindWaitingResume.ErrorCode != "save_target_not_ready")
    throw new InvalidOperationException("farmhand unbind recovery did not wait for its exact save");
var unbindReadyResume = SaveCommandRecoveryContract.Evaluate(unbindSaveCommand, true, "Imported_1", null, null);
if (!unbindReadyResume.CanResume || unbindReadyResume.TerminalFailure)
    throw new InvalidOperationException("farmhand unbind recovery rejected its exact save");
var waitingResume = SaveCommandRecoveryContract.Evaluate(targetedSaveCommand, true, "Old_1", "tx-1", "Old_1");
if (waitingResume.CanResume || waitingResume.TerminalFailure || waitingResume.ErrorCode != "save_target_not_ready")
    throw new InvalidOperationException("targeted save recovery did not wait for the exact verified world");
var readyResume = SaveCommandRecoveryContract.Evaluate(targetedSaveCommand, true, "Target_1", "tx-1", "Target_1");
if (!readyResume.CanResume || readyResume.TerminalFailure)
    throw new InvalidOperationException("targeted save recovery rejected the exact verified world");
var restartTracker = new PendingSaveCommandTracker();
Expect(restartTracker.Begin(saveJournal.Command, true, now.AddMinutes(5), TimeSpan.FromSeconds(30)), CommandStatuses.Running, "");
Expect(restartTracker.Complete("tx-1", "Target_1", now.AddMinutes(5).AddSeconds(1))!, CommandStatuses.Succeeded, "ok");

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
if (NewGameControlContract.CanCustomizeLoadedSave(true, marker, init, "new-save", now))
    throw new InvalidOperationException("SaveCreating evidence bypassed a missing exact target");
marker.TargetSaveId = "new-save";
if (!NewGameControlContract.CanCustomizeLoadedSave(true, marker, init, "new-save", now))
    throw new InvalidOperationException("an exact target was rejected when SaveCreating evidence was present");
if (!NewGameControlContract.CanCustomizeLoadedSave(false, marker, init, "new-save", now))
    throw new InvalidOperationException("an exact recovery target was rejected");
if (NewGameControlContract.CanCustomizeLoadedSave(true, marker, init, "other-save", now))
    throw new InvalidOperationException("SaveCreating evidence allowed a non-target loaded save");
if (NewGameControlContract.CanCustomizeLoadedSave(false, marker, init, "other-save", now))
    throw new InvalidOperationException("a non-target recovery save was allowed to receive customization");
if (NewGameControlContract.CanCustomizeLoadedSave(true, marker, init, "new-save ", now))
    throw new InvalidOperationException("a non-exact target identity was accepted");
if (NewGameControlContract.ValidateMarker(marker, new InitConfig { TransactionId = otherSave.Id }, now).ErrorCode != "transaction_mismatch")
    throw new InvalidOperationException("mismatched transaction marker was accepted");
marker.ExpiresAt = now.AddSeconds(-1);
if (NewGameControlContract.ValidateMarker(marker, init, now).ErrorCode != "marker_expired")
    throw new InvalidOperationException("expired marker was accepted");
if (NewGameControlContract.ShouldClearMarkerOnSaveLoaded)
    throw new InvalidOperationException("SaveLoaded must not clear an active backend transaction marker");

var customizationConfig = new InitConfig
{
    FarmerName = "Leah",
    FarmName = "Blue Farm",
    FavoriteThing = "",
    Gender = "FEMALE",
    PetType = "dog",
    PetBreed = "",
	Skin = 3,
	Hair = 14,
	Shirt = "1001",
	Pants = "1002",
	Accessory = 7,
	EyeColor = new RgbColor { R = 11, G = 22, B = 33 },
	HairColor = new RgbColor { R = 44, G = 55, B = 66 },
	PantsColor = new RgbColor { R = 77, G = 88, B = 99 },
};
CharacterCustomizationSnapshot CoreSnapshot(
    string farmerName = "Leah",
    string farmName = "Blue Farm",
    string favoriteThing = "Anxi",
    string gender = "female",
    string petType = "Dog",
    string petBreed = "0",
	int? skin = 3,
	int? hair = 14,
	string? shirt = "1001",
	string? pants = "1002",
	int? accessory = 7,
	int eyeR = 11,
	int eyeG = 22,
	int eyeB = 33,
	int hairR = 44,
	int hairG = 55,
	int hairB = 66,
	int pantsR = 77,
	int pantsG = 88,
	int pantsB = 99,
	bool isCustomized = true) => new()
{
    FarmerName = farmerName,
    FarmName = farmName,
    FavoriteThing = favoriteThing,
    Gender = gender,
    PetType = petType,
    PetBreed = petBreed,
	Skin = skin,
	Hair = hair,
	Shirt = shirt,
	Pants = pants,
	Accessory = accessory,
	EyeColor = new RgbColor { R = eyeR, G = eyeG, B = eyeB },
	HairColor = new RgbColor { R = hairR, G = hairG, B = hairB },
	PantsColor = new RgbColor { R = pantsR, G = pantsG, B = pantsB },
	IsCustomized = isCustomized,
};
var expectedCore = CharacterCustomizationContract.ExpectedCore(customizationConfig);
if (!CharacterCustomizationContract.CoreEquals(expectedCore, CoreSnapshot()))
    throw new InvalidOperationException("normalized expected character core did not match an equal snapshot");
if (!CharacterCustomizationContract.MatchesCore(customizationConfig, CoreSnapshot()))
    throw new InvalidOperationException("character core did not match its init config");
var shirtMismatchFields = CharacterCustomizationContract.MismatchFields(customizationConfig, CoreSnapshot(shirt: "different"));
if (shirtMismatchFields.Length != 1 || shirtMismatchFields[0] != "shirt")
	throw new InvalidOperationException("character mismatch diagnostics did not identify the exact field");
var clampedColors = CharacterCustomizationContract.ExpectedCore(new InitConfig
{
	EyeColor = new RgbColor { R = -1, G = 256, B = 12 },
	HairColor = new RgbColor { R = 300, G = 13, B = -2 },
	PantsColor = new RgbColor { R = 14, G = -3, B = 400 },
});
if (clampedColors.EyeColor is not { R: 0, G: 255, B: 12 }
	|| clampedColors.HairColor is not { R: 255, G: 13, B: 0 }
	|| clampedColors.PantsColor is not { R: 14, G: 0, B: 255 })
{
	throw new InvalidOperationException("expected appearance colors did not match ToColor's 0..255 channel clamp");
}
foreach (var mismatch in new[]
{
    CoreSnapshot(farmerName: "Other"),
    CoreSnapshot(farmName: "Other Farm"),
    CoreSnapshot(favoriteThing: "Other"),
    CoreSnapshot(gender: "male"),
    CoreSnapshot(petType: "Cat"),
    CoreSnapshot(petBreed: "1"),
	CoreSnapshot(skin: 4),
	CoreSnapshot(hair: 15),
	CoreSnapshot(shirt: "1003"),
	CoreSnapshot(pants: "1004"),
	CoreSnapshot(accessory: 8),
	CoreSnapshot(eyeR: 12),
	CoreSnapshot(eyeG: 23),
	CoreSnapshot(eyeB: 34),
	CoreSnapshot(hairR: 45),
	CoreSnapshot(hairG: 56),
	CoreSnapshot(hairB: 67),
	CoreSnapshot(pantsR: 78),
	CoreSnapshot(pantsG: 89),
	CoreSnapshot(pantsB: 100),
	CoreSnapshot(isCustomized: false),
})
{
    if (CharacterCustomizationContract.CoreEquals(expectedCore, mismatch)
		|| CharacterCustomizationContract.MatchesCore(customizationConfig, mismatch))
	{
		throw new InvalidOperationException("a configured character identity or appearance mismatch was accepted");
	}
}
if (CharacterCustomizationContract.CoreEquals(null, CoreSnapshot())
    || CharacterCustomizationContract.CoreEquals(expectedCore, null)
    || CharacterCustomizationContract.MatchesCore(null, CoreSnapshot()))
{
    throw new InvalidOperationException("a missing character core/config was accepted");
}
var sparseCustomizationConfig = new InitConfig
{
	FarmerName = "Leah",
	FarmName = "Blue Farm",
	FavoriteThing = "",
	Gender = "FEMALE",
	PetType = "dog",
	PetBreed = "",
};
if (!CharacterCustomizationContract.MatchesCore(sparseCustomizationConfig, CoreSnapshot())
	|| CharacterCustomizationContract.MatchesCore(sparseCustomizationConfig, CoreSnapshot(isCustomized: false)))
{
	throw new InvalidOperationException("unspecified appearance fields were not optional or isCustomized was not enforced");
}
var serializedCustomizationStatus = System.Text.Json.JsonSerializer.Serialize(
	new RuntimeStatus { Customization = CoreSnapshot() }, ContractJson.Options);
using (var customizationDocument = System.Text.Json.JsonDocument.Parse(serializedCustomizationStatus))
{
	var customizationJSON = customizationDocument.RootElement.GetProperty("customization");
	if (customizationJSON.GetProperty("skin").GetInt32() != 3
		|| customizationJSON.GetProperty("hair").GetInt32() != 14
		|| customizationJSON.GetProperty("shirt").GetString() != "1001"
		|| customizationJSON.GetProperty("pants").GetString() != "1002"
		|| customizationJSON.GetProperty("accessory").GetInt32() != 7
		|| customizationJSON.GetProperty("eyeColor").GetProperty("r").GetInt32() != 11
		|| customizationJSON.GetProperty("hairColor").GetProperty("g").GetInt32() != 55
		|| customizationJSON.GetProperty("pantsColor").GetProperty("b").GetInt32() != 99
		|| !customizationJSON.GetProperty("isCustomized").GetBoolean())
	{
		throw new InvalidOperationException("customization status JSON did not expose the complete camelCase appearance snapshot");
	}
}

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
