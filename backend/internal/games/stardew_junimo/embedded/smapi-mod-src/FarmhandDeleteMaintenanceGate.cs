#if !SAP_CI_BUILD
using System.Text.Json;
using HarmonyLib;
using StardewModdingAPI;
using StardewValley;
using StardewValley.Menus;

namespace StardewAnxiPanel.Control;

internal sealed class FarmhandDeleteMaintenanceGate
{
	private static readonly TimeSpan GuardLease = TimeSpan.FromMinutes(10);
	private string markerPath = "";
	private IMonitor? monitor;
	private FarmhandDeleteMaintenanceMarker? marker;
	private bool markerInvalid;
	private static FarmhandDeleteMaintenanceGate? active;
	private bool joinGuardAvailable;
	private bool releasingJoinGate;
	private SaveGameMenu? panelSaveMenu;
	private int? observedDay;

	public void TrackPanelSaveMenu(SaveGameMenu menu) => panelSaveMenu = menu;

	public void Initialize(string controlDir, IMonitor controlMonitor)
	{
		markerPath = Path.Combine(controlDir, "farmhand-delete-maintenance.json");
		monitor = controlMonitor;
		LoadMarker();
		active = this;
		try
		{
			var method = AccessTools.Method(typeof(Options), nameof(Options.setServerMode), new[] { typeof(string) });
			var harmony = new Harmony("StardewAnxiPanel.Control.FarmhandDelete");
			harmony.Patch(method, prefix: new HarmonyMethod(typeof(FarmhandDeleteMaintenanceGate), nameof(AllowServerModeChange)));
			joinGuardAvailable = Harmony.GetPatchInfo(method)?.Prefixes.Any(patch => patch.owner == harmony.Id) == true;
		}
		catch (Exception ex)
		{
			monitor.Log($"[FarmhandDelete] Join guard unavailable: {ex.Message}", LogLevel.Error);
		}
	}

	private static bool AllowServerModeChange(string __0)
		=> __0 == "offline" || active is null || active.releasingJoinGate || !active.RequiresClosedGate;

	private bool RequiresClosedGate => markerInvalid || marker is not null
		&& (marker.JoinGateClosed || marker.Phase is FarmhandDeleteMaintenanceContract.GuardedPhase or FarmhandDeleteMaintenanceContract.DestructivePhase);

	private void ReopenJoinGate()
	{
		releasingJoinGate = true;
		try { Game1.options.setServerMode("online"); }
		finally { releasingJoinGate = false; }
	}

	public FarmhandDeleteMaintenanceDecision CheckReady(string operationId, string expectedSaveId)
	{
		if (!joinGuardAvailable)
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_guard_unavailable", "The world join guard is unavailable; deletion cannot start.");
		if (markerInvalid)
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_invalid", "The existing farmhand deletion maintenance marker is invalid.");
		if (marker is not null && !OwnedBy(operationId))
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "Another farmhand deletion owns the maintenance state.");
		if (marker is not null && OwnedBy(operationId))
		{
			if (RequiresClosedGate && (Game1.options.enableServer || HasConnectedHumans()))
				return FarmhandDeleteMaintenanceDecision.Reject("players_connected", "The maintenance join gate is not closed or players are connected.");
			if (FarmhandDeleteMaintenanceContract.ShouldAutoRelease(marker, DateTimeOffset.UtcNow))
				return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_expired", "The maintenance lease has expired.");
			var cancellation = FarmhandDeleteMaintenanceContract.CancellationDecision(marker);
			if (!cancellation.Allowed)
				return cancellation;
			if (!string.Equals(marker.ExpectedSaveId, expectedSaveId.Trim(), StringComparison.Ordinal))
				return FarmhandDeleteMaintenanceDecision.Reject("active_save_changed", "The loaded save no longer matches the farmhand deletion request.");
		}
		return FarmhandDeleteMaintenanceContract.ValidateReadiness(
			operationId,
			expectedSaveId,
			ActiveSaveId(),
			Context.IsWorldReady,
			Game1.IsServer,
			DayTransitionActive(),
			SleepReadyCount(),
			SettlementMenuActive());
	}

	public FarmhandDeleteMaintenanceDecision Countdown(string operationId, string expectedSaveId, string targetPlayerId)
	{
		if (markerInvalid)
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_invalid", "The existing farmhand deletion maintenance marker is invalid.");
		if (marker is not null && !OwnedBy(operationId))
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "Another farmhand deletion owns the maintenance countdown.");
		if (!long.TryParse(targetPlayerId, out var targetId) || targetId == 0)
			return FarmhandDeleteMaintenanceDecision.Reject("invalid_player_id", "The target player ID is invalid.");
		if (marker is not null)
		{
			var cancellation = FarmhandDeleteMaintenanceContract.CancellationDecision(marker);
			if (!cancellation.Allowed)
				return cancellation;
			if (marker.Phase != FarmhandDeleteMaintenanceContract.CountdownPhase
				|| !string.Equals(marker.ExpectedSaveId, expectedSaveId.Trim(), StringComparison.Ordinal)
				|| !string.Equals(marker.TargetPlayerId, targetId.ToString(System.Globalization.CultureInfo.InvariantCulture), StringComparison.Ordinal))
			{
				return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "The maintenance countdown does not match the deletion request.");
			}
		}

		var readiness = CheckReady(operationId, expectedSaveId);
		if (!readiness.Allowed)
			return readiness;
		if (marker is not null)
			return FarmhandDeleteMaintenanceDecision.Allow("The farmhand deletion maintenance countdown is still active.");

		marker = new FarmhandDeleteMaintenanceMarker
		{
			OperationId = operationId,
			ExpectedSaveId = expectedSaveId.Trim(),
			TargetPlayerId = targetId.ToString(System.Globalization.CultureInfo.InvariantCulture),
			Phase = FarmhandDeleteMaintenanceContract.CountdownPhase,
			CreatedAt = DateTimeOffset.UtcNow,
			ExpiresAt = DateTimeOffset.UtcNow.Add(GuardLease),
		};
		observedDay = Game1.Date.TotalDays;
		try
		{
			ContractFile.WriteJsonAtomic(markerPath, marker);
			return FarmhandDeleteMaintenanceDecision.Allow("The farmhand deletion maintenance countdown is active.");
		}
		catch (Exception ex)
		{
			marker = null;
			monitor?.Log($"[FarmhandDelete] Failed to persist maintenance countdown: {ex}", LogLevel.Error);
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_failed", "The maintenance countdown could not be persisted.");
		}
	}

	public FarmhandDeleteMaintenanceDecision Begin(string operationId, string expectedSaveId, string targetPlayerId, string mode)
	{
		var readiness = CheckReady(operationId, expectedSaveId);
		if (!readiness.Allowed)
			return readiness;
		if (marker is not null && !string.Equals(marker.OperationId, operationId, StringComparison.Ordinal))
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "Another farmhand deletion owns the join gate.");
		if (!long.TryParse(targetPlayerId, out var targetId) || targetId == 0)
			return FarmhandDeleteMaintenanceDecision.Reject("invalid_player_id", "The target player ID is invalid.");
		if (mode is not ("wait" or "maintenance_now"))
			return FarmhandDeleteMaintenanceDecision.Reject("invalid_delete_mode", "The deletion mode is invalid.");
		if (Game1.getOnlineFarmers().Any(player => player.UniqueMultiplayerID == targetId))
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_online", "The target player is online; deletion was canceled.");
		if (mode == "wait" && HasConnectedHumans())
			return FarmhandDeleteMaintenanceDecision.Reject("players_connected", "Players are connected; continue waiting without closing the world.");
		var normalizedTargetId = targetId.ToString(System.Globalization.CultureInfo.InvariantCulture);
		if (marker is not null
			&& (marker.Phase != FarmhandDeleteMaintenanceContract.CountdownPhase
				|| !string.Equals(marker.TargetPlayerId, normalizedTargetId, StringComparison.Ordinal)))
		{
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "The maintenance countdown does not match the deletion request.");
		}

		marker ??= new FarmhandDeleteMaintenanceMarker
		{
			OperationId = operationId,
			ExpectedSaveId = expectedSaveId.Trim(),
			TargetPlayerId = normalizedTargetId,
			CreatedAt = DateTimeOffset.UtcNow,
		};
		marker.Phase = FarmhandDeleteMaintenanceContract.GuardedPhase;
		marker.ExpiresAt = DateTimeOffset.UtcNow.Add(GuardLease);
		marker.CancellationCode = "";
		marker.CancellationMessage = "";
		observedDay = Game1.Date.TotalDays;
		try
		{
			ContractFile.WriteJsonAtomic(markerPath, marker);
			Game1.options.setServerMode("offline");
			marker.JoinGateClosed = true;
			ContractFile.WriteJsonAtomic(markerPath, marker);
			monitor?.Log($"[FarmhandDelete] Join gate closed for operation {operationId}.", LogLevel.Info);
			return FarmhandDeleteMaintenanceDecision.Allow("The world join gate is closed for farmhand deletion maintenance.");
		}
		catch (Exception ex)
		{
			monitor?.Log($"[FarmhandDelete] Failed to close join gate: {ex}", LogLevel.Error);
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_failed", "The world join gate could not be closed.");
		}
	}

	public FarmhandDeleteMaintenanceDecision Seal(string operationId)
	{
		if (!OwnedBy(operationId))
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "The farmhand deletion does not own the join gate.");
		var readiness = CheckReady(operationId, marker!.ExpectedSaveId);
		if (!readiness.Allowed)
			return readiness;
		if (marker.Phase != FarmhandDeleteMaintenanceContract.GuardedPhase || !marker.JoinGateClosed)
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "The farmhand deletion join gate is not ready to cross the destructive boundary.");
		if (Game1.options.enableServer || HasConnectedHumans())
			return FarmhandDeleteMaintenanceDecision.Reject("players_connected", "The world join gate is open or players are still connected.");
		marker!.Phase = FarmhandDeleteMaintenanceContract.DestructivePhase;
		marker.ExpiresAt = null;
		try
		{
			ContractFile.WriteJsonAtomic(markerPath, marker);
			return FarmhandDeleteMaintenanceDecision.Allow("The farmhand deletion crossed its destructive boundary.");
		}
		catch (Exception ex)
		{
			monitor?.Log($"[FarmhandDelete] Failed to seal maintenance marker: {ex}", LogLevel.Error);
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_failed", "The destructive maintenance boundary could not be persisted.");
		}
	}

	public FarmhandDeleteMaintenanceDecision End(string operationId)
	{
		if (markerInvalid)
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_invalid", "The farmhand deletion maintenance marker requires manual recovery.");
		if (marker is null)
			return FarmhandDeleteMaintenanceDecision.Allow("No farmhand deletion maintenance marker remains.");
		var ownedMarker = marker;
		if (!OwnedBy(operationId))
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_conflict", "The farmhand deletion does not own the join gate.");
		try
		{
			if (ownedMarker.JoinGateClosed
				|| ownedMarker.Phase is FarmhandDeleteMaintenanceContract.GuardedPhase or FarmhandDeleteMaintenanceContract.DestructivePhase)
			{
				ReopenJoinGate();
			}
			if (File.Exists(markerPath))
				File.Delete(markerPath);
			marker = null;
			monitor?.Log($"[FarmhandDelete] Join gate reopened for operation {operationId}.", LogLevel.Info);
			return FarmhandDeleteMaintenanceDecision.Allow("The world join gate is open.");
		}
		catch (Exception ex)
		{
			if (ownedMarker.Phase == FarmhandDeleteMaintenanceContract.DestructivePhase)
			{
				try
				{
					Game1.options.setServerMode("offline");
					ownedMarker.JoinGateClosed = true;
					TryPersistMarker();
				}
				catch (Exception restoreEx)
				{
					monitor?.Log($"[FarmhandDelete] Failed to restore the destructive join gate after release failure: {restoreEx}", LogLevel.Error);
				}
			}
			monitor?.Log($"[FarmhandDelete] Failed to reopen join gate: {ex}", LogLevel.Error);
			return FarmhandDeleteMaintenanceDecision.Reject("farmhand_delete_maintenance_release_failed", "The world join gate could not be reopened.");
		}
	}

	public bool OnSaveLoaded()
	{
		panelSaveMenu = null;
		observedDay = Game1.Date.TotalDays;
		LoadMarker();
		if (RequiresClosedGate && !joinGuardAvailable)
		{
			Game1.options.setServerMode("offline");
			Game1.ExitToTitle();
			monitor?.Log("[FarmhandDelete] Cannot resume a guarded world without the join guard; returned to title for recovery.", LogLevel.Error);
			return false;
		}
		if (markerInvalid)
		{
			Game1.options.setServerMode("offline");
			return true;
		}
		if (marker is null)
			return true;
		if (marker.Phase is FarmhandDeleteMaintenanceContract.CountdownPhase or FarmhandDeleteMaintenanceContract.GuardedPhase)
		{
			CancelBeforeDestructiveBoundary("farmhand_delete_canceled", "The world reloaded before deletion; the interrupted operation was canceled.");
			return true;
		}
		if (FarmhandDeleteMaintenanceContract.ShouldAutoRelease(marker, DateTimeOffset.UtcNow))
		{
			_ = End(marker.OperationId);
			return true;
		}
		if (marker.Phase == FarmhandDeleteMaintenanceContract.CanceledPhase)
		{
			if (marker.JoinGateClosed)
			{
				ReopenJoinGate();
				marker.JoinGateClosed = false;
				TryPersistMarker();
			}
			return true;
		}
		if (marker.Phase is FarmhandDeleteMaintenanceContract.GuardedPhase or FarmhandDeleteMaintenanceContract.DestructivePhase)
		{
			Game1.options.setServerMode("offline");
			marker.JoinGateClosed = true;
			TryPersistMarker();
		}
		return true;
	}

	public void OnDayStarted()
	{
		var today = Game1.Date.TotalDays;
		var previous = observedDay;
		observedDay = today;
		// SMAPI also raises DayStarted after a manual save and after loading.
		if (previous is not null && previous.Value != today)
			CancelBeforeDestructiveBoundary("day_transition_in_progress", "The game date changed during farmhand deletion maintenance.");
	}

	public string? VerifiedTransactionIdForSave(string saveId)
	{
		if (markerInvalid || marker is null
			|| marker.Phase is not (FarmhandDeleteMaintenanceContract.GuardedPhase or FarmhandDeleteMaintenanceContract.DestructivePhase)
			|| !string.Equals(marker.ExpectedSaveId, saveId.Trim(), StringComparison.Ordinal))
		{
			return null;
		}
		return marker.OperationId;
	}

	public void Tick()
	{
		if (panelSaveMenu is not null && !ReferenceEquals(Game1.activeClickableMenu, panelSaveMenu))
			panelSaveMenu = null;
		if (marker is null || markerInvalid || !Context.IsWorldReady)
			return;
		if (marker.Phase is FarmhandDeleteMaintenanceContract.CountdownPhase or FarmhandDeleteMaintenanceContract.GuardedPhase)
		{
			if (DayTransitionActive() || SettlementMenuActive())
			{
				CancelBeforeDestructiveBoundary(
					"day_transition_in_progress",
					"Sleep or day settlement started during farmhand deletion maintenance.");
				return;
			}
			if (SleepReadyCount() > 0)
			{
				CancelBeforeDestructiveBoundary(
					"sleep_in_progress",
					"At least one player started sleeping during farmhand deletion maintenance.");
				return;
			}
		}
		if (FarmhandDeleteMaintenanceContract.ShouldAutoRelease(marker, DateTimeOffset.UtcNow))
		{
			monitor?.Log("[FarmhandDelete] Guard lease expired before deletion; reopening the join gate.", LogLevel.Warn);
			_ = End(marker.OperationId);
		}
	}

	private void CancelBeforeDestructiveBoundary(string code, string message)
	{
		if (marker is null || markerInvalid
			|| !FarmhandDeleteMaintenanceContract.CancelBeforeDestructiveBoundary(
				marker, code, message, DateTimeOffset.UtcNow, GuardLease))
		{
			return;
		}
		TryPersistMarker();
		if (marker.JoinGateClosed)
		{
			try
			{
				ReopenJoinGate();
				marker.JoinGateClosed = false;
				TryPersistMarker();
			}
			catch (Exception ex)
			{
				monitor?.Log($"[FarmhandDelete] Failed to reopen the join gate after cancellation: {ex}", LogLevel.Error);
			}
		}
		monitor?.Log($"[FarmhandDelete] Maintenance canceled before deletion: {code}.", LogLevel.Warn);
	}

	private void TryPersistMarker()
	{
		if (marker is null)
			return;
		try
		{
			ContractFile.WriteJsonAtomic(markerPath, marker);
		}
		catch (Exception ex)
		{
			monitor?.Log($"[FarmhandDelete] Failed to update maintenance marker: {ex}", LogLevel.Error);
		}
	}

	private void LoadMarker()
	{
		marker = null;
		markerInvalid = false;
		if (markerPath.Length == 0 || !File.Exists(markerPath))
			return;
		try
		{
			marker = JsonSerializer.Deserialize<FarmhandDeleteMaintenanceMarker>(File.ReadAllText(markerPath), ContractJson.Options);
			markerInvalid = !FarmhandDeleteMaintenanceContract.ValidMarker(marker);
		}
		catch
		{
			markerInvalid = true;
		}
	}

	private bool OwnedBy(string operationId)
		=> marker is not null && string.Equals(marker.OperationId, operationId, StringComparison.Ordinal);

	private static string ActiveSaveId()
	{
		var saveId = Constants.SaveFolderName;
		return string.IsNullOrWhiteSpace(saveId) ? Game1.GetSaveGameName() ?? "" : saveId;
	}

	private static bool DayTransitionActive()
		=> Game1.newDay || (Game1.newDaySync is not null && Game1.newDaySync.hasInstance() && !Game1.newDaySync.hasFinished());

	private static int SleepReadyCount()
	{
		try
		{
			return Game1.netReady?.GetNumberReady("sleep") ?? 0;
		}
		catch
		{
			return 1;
		}
	}

	private bool SettlementMenuActive()
		=> Game1.activeClickableMenu is ShippingMenu
			|| (Game1.activeClickableMenu is SaveGameMenu && !ReferenceEquals(Game1.activeClickableMenu, panelSaveMenu));

	private static bool HasConnectedHumans()
		=> (Game1.server?.connectionsCount ?? (Game1.options.enableServer ? -1 : 0)) != 0
			|| Game1.getOnlineFarmers().Any(player => player.UniqueMultiplayerID != Game1.MasterPlayer.UniqueMultiplayerID);
}
#endif
