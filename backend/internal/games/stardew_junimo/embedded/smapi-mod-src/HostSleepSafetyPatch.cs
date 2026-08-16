#if !SAP_CI_BUILD
using System.Reflection;
using HarmonyLib;
using StardewModdingAPI;
using StardewValley;
using StardewValley.Locations;

namespace StardewAnxiPanel.Control;

internal sealed class HostSleepSafetyPatch
{
    private const string ServiceAssemblyName = "JunimoServer";
    private const string AlwaysOnTypeName = "JunimoServer.Services.AlwaysOn.AlwaysOnServer";

    private static HostSleepSafetyPatch? active;

    private readonly Func<FarmHouse, bool> validateBed;
    private IMonitor? monitor;
    private FieldInfo? warpingSleepField;
    private bool initialized;
    private bool sleepBlocked;
    private bool sleepCommitted;
    private bool bedValidatedForCurrentCall;
    private int sleepAttempts;

    private const int MaxSleepAttempts = 4;

    public HostSleepSafetyPatch(Func<FarmHouse, bool> validateBed)
    {
        this.validateBed = validateBed;
    }

    public bool Available { get; private set; }

    public string Detail { get; private set; } = "Not initialized.";

    public void Initialize(string harmonyID, IMonitor monitor)
    {
        if (initialized)
            return;
        initialized = true;
        this.monitor = monitor;
        active = this;

        try
        {
            var type = FindAlwaysOnType();
            if (type is null)
            {
                Fail($"Type '{AlwaysOnTypeName}' was not found in the loaded JunimoServer assembly.");
                return;
            }

            var handleAutoSleep = type.GetMethod(
                "HandleAutoSleep",
                BindingFlags.NonPublic | BindingFlags.Instance,
                null,
                Type.EmptyTypes,
                null);
            var hostSleepInBed = type.GetMethod(
                "HostSleepInBed",
                BindingFlags.NonPublic | BindingFlags.Instance,
                null,
                new[] { typeof(FarmHouse) },
                null);
            warpingSleepField = type.GetField("_warpingSleep", BindingFlags.NonPublic | BindingFlags.Instance);
            if (handleAutoSleep?.ReturnType != typeof(void)
                || hostSleepInBed?.ReturnType != typeof(void)
                || warpingSleepField?.FieldType != typeof(bool))
            {
                Fail("The exact sleep methods or _warpingSleep field were not found.");
                return;
            }

            var guardPrefix = new HarmonyMethod(
                typeof(HostSleepSafetyPatch).GetMethod(nameof(GuardAutoSleepPrefix), BindingFlags.NonPublic | BindingFlags.Static)
                    ?? throw new MissingMethodException(nameof(GuardAutoSleepPrefix)));
            var bedPrefix = new HarmonyMethod(
                typeof(HostSleepSafetyPatch).GetMethod(nameof(ValidateBedPrefix), BindingFlags.NonPublic | BindingFlags.Static)
                    ?? throw new MissingMethodException(nameof(ValidateBedPrefix)));
            var bedPostfix = new HarmonyMethod(
                typeof(HostSleepSafetyPatch).GetMethod(nameof(LatchSuccessfulSleepPostfix), BindingFlags.NonPublic | BindingFlags.Static)
                    ?? throw new MissingMethodException(nameof(LatchSuccessfulSleepPostfix)));
            var harmony = new Harmony(harmonyID + ".HostSleepSafety");
            harmony.Patch(handleAutoSleep, prefix: guardPrefix);
            harmony.Patch(hostSleepInBed, prefix: bedPrefix, postfix: bedPostfix);

            if (!HasOwnedPrefix(handleAutoSleep, harmony.Id)
                || !HasOwnedPrefix(hostSleepInBed, harmony.Id)
                || !HasOwnedPostfix(hostSleepInBed, harmony.Id))
            {
                Fail("Harmony did not report the sleep guard, bed validator, and one-shot latch as installed.");
                return;
            }

            Available = true;
            Detail = "Installed; valid-bed sleep is capped at four game-thread actions per day and missing-bed fallback is limited to one attempt per fault episode.";
            monitor.Log($"[HostSleepSafety] {Detail}", LogLevel.Info);
        }
        catch (Exception ex)
        {
            Fail($"Patch initialization failed: {ex.Message}");
        }
    }

    public void NotifyBedHealthy()
        => sleepBlocked = false;

    public void Reset()
    {
        sleepBlocked = false;
        sleepCommitted = false;
        bedValidatedForCurrentCall = false;
        sleepAttempts = 0;
    }

    private static bool GuardAutoSleepPrefix(object __instance)
    {
        if (active is null)
            return true;
        if (active.sleepBlocked)
            return false;
        if (!active.sleepCommitted)
            return true;

        active.ContinueCommittedSleep(__instance);
        return false;
    }

    private static bool ValidateBedPrefix(object __instance, [HarmonyArgument(0)] FarmHouse farmHouse)
    {
        if (active is null)
            return false;
        if (active.validateBed(farmHouse))
        {
            active.sleepBlocked = false;
            active.bedValidatedForCurrentCall = true;
            return true;
        }

        active.bedValidatedForCurrentCall = false;
        active.sleepBlocked = true;
        active.sleepCommitted = false;
        try
        {
            active.warpingSleepField?.SetValue(__instance, false);
        }
        catch (Exception ex)
        {
            active.monitor?.Log($"[HostSleepSafety] Could not clear the failed sleep-warp latch: {ex.Message}", LogLevel.Error);
        }
        active.monitor?.Log(
            "[HostSleepSafety] errorCode=host_bed_missing action=sleep_blocked fallbackAttempts=1; automatic day forcing is disabled for this fault episode.",
            LogLevel.Error);
        return false;
    }

    private static void LatchSuccessfulSleepPostfix(object __instance)
    {
        if (active is null || !active.bedValidatedForCurrentCall)
            return;

        active.bedValidatedForCurrentCall = false;
        active.sleepCommitted = true;
        active.sleepAttempts = 1;
        try
        {
            // JunimoServer clears this at the end of HostSleepInBed, which lets
            // HandleAutoSleep repeat once per update until the ready check fires.
            // Keep the upstream latch armed while the bounded silent retries
            // converge the ready dialog; Reset() releases it at the next
            // DayStarted/SaveLoaded boundary.
            active.warpingSleepField?.SetValue(__instance, true);
        }
        catch (Exception ex)
        {
            active.monitor?.Log($"[HostSleepSafety] Could not retain the successful sleep latch: {ex.Message}", LogLevel.Error);
        }
    }

    private void ContinueCommittedSleep(object instance)
    {
        if (sleepAttempts >= MaxSleepAttempts || !Context.IsWorldReady)
            return;
        if (Game1.currentLocation is not FarmHouse farmHouse || !validateBed(farmHouse))
        {
            sleepCommitted = false;
            sleepBlocked = true;
            try
            {
                warpingSleepField?.SetValue(instance, false);
            }
            catch (Exception ex)
            {
                monitor?.Log($"[HostSleepSafety] Could not clear the failed sleep-warp latch: {ex.Message}", LogLevel.Error);
            }
            monitor?.Log(
                $"[HostSleepSafety] errorCode=host_bed_missing action=sleep_retry_blocked attempts={sleepAttempts}; automatic day forcing is disabled for this fault episode.",
                LogLevel.Error);
            return;
        }

        Game1.player.position.Set(Utility.PointToVector2(farmHouse.GetPlayerBedSpot()) * 64f);
        farmHouse.answerDialogueAction("Sleep_Yes", null);
        sleepAttempts++;
        try
        {
            warpingSleepField?.SetValue(instance, true);
        }
        catch (Exception ex)
        {
            monitor?.Log($"[HostSleepSafety] Could not retain the bounded sleep-retry latch: {ex.Message}", LogLevel.Error);
        }
        monitor?.Log(
            $"[HostSleepSafety] action=sleep_retry attempt={sleepAttempts} maxAttempts={MaxSleepAttempts} bedSource=main_farmhouse",
            LogLevel.Trace);
    }

    private static Type? FindAlwaysOnType()
    {
        foreach (var assembly in AppDomain.CurrentDomain.GetAssemblies())
        {
            if (string.Equals(assembly.GetName().Name, ServiceAssemblyName, StringComparison.Ordinal))
                return assembly.GetType(AlwaysOnTypeName, throwOnError: false);
        }
        return null;
    }

    private static bool HasOwnedPrefix(MethodBase method, string owner)
        => Harmony.GetPatchInfo(method)?.Prefixes.Any(patch => string.Equals(patch.owner, owner, StringComparison.Ordinal)) == true;

    private static bool HasOwnedPostfix(MethodBase method, string owner)
        => Harmony.GetPatchInfo(method)?.Postfixes.Any(patch => string.Equals(patch.owner, owner, StringComparison.Ordinal)) == true;

    private void Fail(string detail)
    {
        Available = false;
        Detail = detail;
        monitor?.Log($"[HostSleepSafety] {detail}", LogLevel.Error);
    }
}
#endif
