#if !SAP_CI_BUILD
using System.Reflection;
using StardewModdingAPI;
using StardewValley;

namespace StardewAnxiPanel.Control;

/// <summary>
/// Reflects JunimoServer's public FarmerExtensions.WarpHome(Farmer) helper so
/// the panel can reuse Junimo's own "send this player back to their cabin"
/// logic without adding a compile-time dependency on JunimoServer.
/// </summary>
internal sealed class WarpHomeBridge
{
    private const string ExtensionTypeName = "JunimoServer.Util.FarmerExtensions";
    private const string ServiceAssemblyName = "JunimoServer";

    private bool _available;
    private string _detail = "Not initialized.";
    private MethodInfo? _warpHomeMethod;

    public bool Available => _available;

    public string Detail => _detail;

    public void Initialize(IMonitor monitor)
    {
        try
        {
            var extensionType = FindExtensionType();
            if (extensionType is null)
            {
                _available = false;
                _detail = $"Type '{ExtensionTypeName}' not found in any loaded assembly.";
                monitor.Log($"[WarpHomeBridge] {_detail}", LogLevel.Warn);
                return;
            }

            _warpHomeMethod = extensionType.GetMethod(
                "WarpHome",
                BindingFlags.Public | BindingFlags.Static,
                binder: null,
                types: new[] { typeof(Farmer) },
                modifiers: null);
            if (_warpHomeMethod is null)
            {
                _available = false;
                _detail = "Method 'WarpHome(Farmer)' not found on FarmerExtensions.";
                monitor.Log($"[WarpHomeBridge] {_detail}", LogLevel.Warn);
                return;
            }

            _available = true;
            _detail = "OK";
            monitor.Log("[WarpHomeBridge] Reflection bridge resolved successfully.", LogLevel.Info);
        }
        catch (Exception ex)
        {
            _available = false;
            _detail = $"Reflection init failed: {ex.Message}";
            monitor.Log($"[WarpHomeBridge] Initialize failed: {ex}", LogLevel.Warn);
        }
    }

    private static Type? FindExtensionType()
    {
        foreach (var assembly in AppDomain.CurrentDomain.GetAssemblies())
        {
            if (!string.Equals(assembly.GetName().Name, ServiceAssemblyName, StringComparison.Ordinal))
                continue;

            var type = assembly.GetType(ExtensionTypeName, throwOnError: false);
            if (type is not null)
                return type;
        }
        return null;
    }

    public (bool Success, string Message) WarpHome(Farmer farmer)
    {
        if (!_available || _warpHomeMethod is null)
            return (false, "Warp-home bridge unavailable: " + _detail);

        try
        {
            _warpHomeMethod.Invoke(null, new object[] { farmer });
            return (true, "WarpHome invoked.");
        }
        catch (Exception ex)
        {
            return (false, $"WarpHome invoke failed: {ex.Message}");
        }
    }
}
#endif
