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
Console.WriteLine("control command outcome branch tests passed");
