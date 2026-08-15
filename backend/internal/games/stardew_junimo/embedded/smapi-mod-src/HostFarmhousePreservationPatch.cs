#if !SAP_CI_BUILD
using System.Reflection;
using HarmonyLib;
using StardewModdingAPI;

namespace StardewAnxiPanel.Control;

/// <summary>
/// Prevents JunimoServer's transitional #346 self-heal from unconditionally
/// resetting a valid imported host farmhouse to level zero on every save load.
///
/// JunimoServer removed the old farmhand-to-host upgrade mirroring activity
/// which caused #346. Its remaining public reset method is resolved by exact
/// assembly, type, method, return type, and parameter shape before patching so
/// a future upstream rename or signature change fails visibly instead of
/// patching an unrelated method.
/// </summary>
internal sealed class HostFarmhousePreservationPatch
{
    private const string ServiceAssemblyName = "JunimoServer";
    private const string GuardTypeName = "JunimoServer.Services.AlwaysOn.HostFarmhouseUpgradeGuard";
    private const string ResetMethodName = "ResetHostFarmhouseToLevelZero";

    private bool initialized;

    public bool Available { get; private set; }

    public string Detail { get; private set; } = "Not initialized.";

    public void Initialize(string harmonyID, IMonitor monitor)
    {
        if (initialized)
            return;
        initialized = true;

        try
        {
            var guardType = FindGuardType();
            if (guardType is null)
            {
                Fail($"Type '{GuardTypeName}' was not found in the loaded JunimoServer assembly.", monitor);
                return;
            }

            var resetMethod = guardType.GetMethod(
                ResetMethodName,
                BindingFlags.Public | BindingFlags.Static,
                binder: null,
                types: Type.EmptyTypes,
                modifiers: null);
            if (resetMethod is null || resetMethod.ReturnType != typeof(void))
            {
                Fail($"Method '{GuardTypeName}.{ResetMethodName}()' with a void return type was not found.", monitor);
                return;
            }

            var prefix = typeof(HostFarmhousePreservationPatch).GetMethod(
                nameof(SkipHostFarmhouseReset),
                BindingFlags.NonPublic | BindingFlags.Static)
                ?? throw new MissingMethodException(nameof(SkipHostFarmhouseReset));
            var harmony = new Harmony(harmonyID + ".HostFarmhousePreservation");
            harmony.Patch(resetMethod, prefix: new HarmonyMethod(prefix));

            var patchInfo = Harmony.GetPatchInfo(resetMethod);
            if (patchInfo?.Prefixes.Any(patch => string.Equals(patch.owner, harmony.Id, StringComparison.Ordinal)) != true)
            {
                Fail("Harmony did not report the host farmhouse preservation prefix as installed.", monitor);
                return;
            }

            Available = true;
            Detail = "Installed.";
            monitor.Log(
                "[HostFarmhousePreservation] JunimoServer's load-time host farmhouse level-zero reset is disabled; the save's host house level and interior are preserved.",
                LogLevel.Info);
        }
        catch (Exception ex)
        {
            Fail($"Patch initialization failed: {ex.Message}", monitor);
        }
    }

    private static Type? FindGuardType()
    {
        foreach (var assembly in AppDomain.CurrentDomain.GetAssemblies())
        {
            if (!string.Equals(assembly.GetName().Name, ServiceAssemblyName, StringComparison.Ordinal))
                continue;

            return assembly.GetType(GuardTypeName, throwOnError: false);
        }
        return null;
    }

    // Harmony prefixes returning false skip the original method.
    private static bool SkipHostFarmhouseReset() => false;

    private void Fail(string detail, IMonitor monitor)
    {
        Available = false;
        Detail = detail;
        monitor.Log($"[HostFarmhousePreservation] {detail}", LogLevel.Error);
    }
}
#endif
