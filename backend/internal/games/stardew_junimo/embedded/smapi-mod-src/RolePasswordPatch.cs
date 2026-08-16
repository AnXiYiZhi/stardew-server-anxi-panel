#if !SAP_CI_BUILD
using System.Reflection;
using HarmonyLib;
using StardewModdingAPI;

namespace StardewAnxiPanel.Control;

internal sealed class RolePasswordPatch
{
    private static RolePasswordPolicy policy = RolePasswordPolicy.Parse(null, null, null, null, null);
    private static Func<string> activeSaveId = () => "";

    public bool Available { get; private set; } = true;

    public string Detail { get; private set; } = "Not initialized.";

    public void Initialize(
        string harmonyID,
        MethodInfo? tryAuthenticateMethod,
        RolePasswordPolicy loadedPolicy,
        Func<string> activeSaveIdProvider,
        IMonitor monitor)
    {
        policy = loadedPolicy;
        activeSaveId = activeSaveIdProvider;
        if (!loadedPolicy.RequiresPatch)
        {
            Available = true;
            Detail = "Not required for the configured mode.";
            return;
        }
        if (tryAuthenticateMethod is null)
        {
            Available = false;
            Detail = "TryAuthenticate method is unavailable; role passwords fail closed.";
            monitor.Log($"[RolePassword] {Detail}", LogLevel.Warn);
            return;
        }
        try
        {
            var harmony = new Harmony(harmonyID + ".RolePassword");
            var prefix = typeof(RolePasswordPatch).GetMethod(nameof(BeforeTryAuthenticate), BindingFlags.NonPublic | BindingFlags.Static)
                ?? throw new MissingMethodException(nameof(BeforeTryAuthenticate));
            harmony.Patch(tryAuthenticateMethod, prefix: new HarmonyMethod(prefix));
            var patchInfo = Harmony.GetPatchInfo(tryAuthenticateMethod);
            if (patchInfo?.Prefixes.Any(patch => string.Equals(patch.owner, harmony.Id, StringComparison.Ordinal)) != true)
                throw new InvalidOperationException("Harmony did not report the role password prefix as installed.");
            Available = loadedPolicy.Valid;
            Detail = loadedPolicy.Valid ? "OK" : loadedPolicy.Detail;
            monitor.Log(
                loadedPolicy.Valid
                    ? "[RolePassword] Role-specific authentication patch installed."
                    : $"[RolePassword] Patch installed in fail-closed mode: {loadedPolicy.Detail}",
                loadedPolicy.Valid ? LogLevel.Info : LogLevel.Warn);
        }
        catch (Exception ex)
        {
            Available = false;
            Detail = $"Role password patch failed: {ex.Message}";
            monitor.Log($"[RolePassword] {Detail}", LogLevel.Warn);
        }
    }

    private static void BeforeTryAuthenticate(long __0, ref string __1)
    {
        try
        {
            __1 = policy.RewritePassword(activeSaveId(), __0, __1);
        }
        catch
        {
            __1 = RolePasswordPolicy.InvalidPasswordSentinel;
        }
    }
}
#endif
