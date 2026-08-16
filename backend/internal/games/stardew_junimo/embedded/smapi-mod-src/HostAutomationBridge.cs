#if !SAP_CI_BUILD
using System.Reflection;
using HarmonyLib;
using StardewModdingAPI;
using StardewValley;

namespace StardewAnxiPanel.Control;

internal sealed class HostAutomationBridge
{
    private const string ServiceAssemblyName = "JunimoServer";
    private const string AlwaysOnTypeName = "JunimoServer.Services.AlwaysOn.AlwaysOnServer";

    private static HostAutomationBridge? active;

    private readonly Action stateChanged;
    private IMonitor? monitor;
    private FieldInfo? automationField;
    private FieldInfo? hiddenField;
    private MethodInfo? enableAutoModeMethod;
    private object? service;
    private bool? lastAutomationEnabled;
    private bool? lastHostVisible;
    private bool initialized;

    public HostAutomationBridge(Action stateChanged)
    {
        this.stateChanged = stateChanged;
    }

    public bool Available { get; private set; }

    public string Detail { get; private set; } = "Not initialized.";

    public bool? AutomationEnabled
    {
        get
        {
            try
            {
                return service is not null && automationField?.GetValue(service) is bool value ? value : null;
            }
            catch
            {
                return null;
            }
        }
    }

    public bool? HostVisible
    {
        get
        {
            try
            {
                return hiddenField?.GetValue(null) is bool hidden ? !hidden : null;
            }
            catch
            {
                return null;
            }
        }
    }

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

            automationField = type.GetField("IsAutomating", BindingFlags.Public | BindingFlags.Instance);
            hiddenField = type.GetField("PlayerIsHidden", BindingFlags.Public | BindingFlags.Static);
            enableAutoModeMethod = ExactVoidMethod(type, "EnableAutoMode");
            var toggleAutoMode = ExactVoidMethod(type, "ToggleAutoMode");
            var toggleVisibility = ExactVoidMethod(type, "ToggleVisibility");
            if (automationField?.FieldType != typeof(bool)
                || hiddenField?.FieldType != typeof(bool)
                || enableAutoModeMethod is null
                || toggleAutoMode is null
                || toggleVisibility is null)
            {
                Fail("The expected automation fields or exact no-argument methods were not found.");
                return;
            }

            var automationPostfix = new HarmonyMethod(
                typeof(HostAutomationBridge).GetMethod(nameof(AutomationChangedPostfix), BindingFlags.NonPublic | BindingFlags.Static)
                    ?? throw new MissingMethodException(nameof(AutomationChangedPostfix)));
            var visibilityPostfix = new HarmonyMethod(
                typeof(HostAutomationBridge).GetMethod(nameof(VisibilityChangedPostfix), BindingFlags.NonPublic | BindingFlags.Static)
                    ?? throw new MissingMethodException(nameof(VisibilityChangedPostfix)));
            var harmony = new Harmony(harmonyID + ".HostAutomationBridge");
            harmony.Patch(enableAutoModeMethod, postfix: automationPostfix);
            harmony.Patch(toggleAutoMode, postfix: automationPostfix);
            harmony.Patch(toggleVisibility, postfix: visibilityPostfix);

            if (!HasOwnedPostfix(enableAutoModeMethod, harmony.Id)
                || !HasOwnedPostfix(toggleAutoMode, harmony.Id)
                || !HasOwnedPostfix(toggleVisibility, harmony.Id))
            {
                Fail("Harmony did not report every host-control postfix as installed.");
                return;
            }

            Available = true;
            Detail = "Installed.";
            monitor.Log(
                "[HostAutomationBridge] F9 automation state and F10 visibility are bridged with exact JunimoServer signatures.",
                LogLevel.Info);
        }
        catch (Exception ex)
        {
            Fail($"Bridge initialization failed: {ex.Message}");
        }
    }

    public bool TryEnableAutomation()
    {
        if (!Available || service is null || enableAutoModeMethod is null)
            return false;
        try
        {
            enableAutoModeMethod.Invoke(service, null);
            CaptureAutomation(service);
            return AutomationEnabled == true;
        }
        catch (Exception ex)
        {
            monitor?.Log($"[HostAutomationBridge] Failed to restore automation: {ex}", LogLevel.Error);
            return false;
        }
    }

    public bool SynchronizeVisibility()
    {
        var visible = HostVisible;
        if (!visible.HasValue || !Context.IsWorldReady || Game1.player is null)
            return false;

        var changed = Game1.displayFarmer != visible.Value || Game1.player.hidden.Value == visible.Value;
        Game1.displayFarmer = visible.Value;
        Game1.player.hidden.Value = !visible.Value;
        if (changed || lastHostVisible != visible)
        {
            lastHostVisible = visible;
            stateChanged();
        }
        return true;
    }

    public bool TrySetHostVisible(bool visible)
    {
        if (!Available || hiddenField is null || !Context.IsWorldReady || Game1.player is null)
            return false;

        try
        {
            hiddenField.SetValue(null, !visible);
            return SynchronizeVisibility();
        }
        catch (Exception ex)
        {
            monitor?.Log($"[HostAutomationBridge] Failed to set host visibility atomically: {ex}", LogLevel.Error);
            return false;
        }
    }

    private static void AutomationChangedPostfix(object __instance)
        => active?.CaptureAutomation(__instance);

    private static void VisibilityChangedPostfix(object __instance)
    {
        active?.CaptureAutomation(__instance);
        active?.SynchronizeVisibility();
    }

    private void CaptureAutomation(object instance)
    {
        service = instance;
        var current = AutomationEnabled;
        if (current != lastAutomationEnabled)
        {
            lastAutomationEnabled = current;
            stateChanged();
        }
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

    private static MethodInfo? ExactVoidMethod(Type type, string name)
    {
        var method = type.GetMethod(name, BindingFlags.NonPublic | BindingFlags.Instance, null, Type.EmptyTypes, null);
        return method?.ReturnType == typeof(void) ? method : null;
    }

    private static bool HasOwnedPostfix(MethodBase method, string owner)
        => Harmony.GetPatchInfo(method)?.Postfixes.Any(patch => string.Equals(patch.owner, owner, StringComparison.Ordinal)) == true;

    private void Fail(string detail)
    {
        Available = false;
        Detail = detail;
        monitor?.Log($"[HostAutomationBridge] {detail}", LogLevel.Error);
    }
}
#endif
