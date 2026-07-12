#if !SAP_CI_BUILD
using System.Reflection;
using StardewModdingAPI;

namespace StardewAnxiPanel.Control;

/// <summary>
/// Reflects into JunimoServer's internal (non-public, non-contractual)
/// PasswordProtectionService singleton so the panel can authenticate a
/// specific pending player on the admin's behalf. JunimoServer exposes no
/// REST endpoint for this, and simulating the "!login" chat command from the
/// host does not work because JunimoServer authenticates by the message's
/// source FarmerID (the host bypasses auth entirely and cannot act on behalf
/// of another player).
///
/// This is inherently fragile: it depends on JunimoServer's private field
/// name and method signatures, which are not a public contract and can
/// change silently on a JunimoServer upgrade. <see cref="Initialize"/> runs
/// once at startup and records whether the reflection chain resolved, so the
/// panel can disable the "approve" action up front instead of failing
/// silently per click. A successful self-check only means the reflection
/// chain was resolvable at startup - it does not guarantee every future call
/// succeeds or that JunimoServer's internal behavior is unchanged.
/// </summary>
internal sealed class PasswordProtectionBridge
{
    private const string ServiceTypeName = "JunimoServer.Services.PasswordProtection.PasswordProtectionService";
    private const string ServiceAssemblyName = "JunimoServer";

    private bool _available;
    private string _detail = "Not initialized.";

    private FieldInfo? _instanceField;
    private MethodInfo? _tryAuthenticateMethod;
    private MethodInfo? _isPlayerAuthenticatedMethod;
    private PropertyInfo? _resultSuccessProperty;
    private PropertyInfo? _resultMessageProperty;
    private PropertyInfo? _resultShouldKickProperty;

    public bool Available => _available;

    public string Detail => _detail;

    /// <summary>
    /// Resolves the reflection chain into JunimoServer's PasswordProtectionService.
    /// Must be called exactly once, after JunimoHost.Server is confirmed loaded
    /// (so the JunimoServer assembly is guaranteed to already be in the AppDomain).
    /// </summary>
    public void Initialize(IMonitor monitor)
    {
        try
        {
            var serviceType = FindServiceType();
            if (serviceType is null)
            {
                _available = false;
                _detail = $"Type '{ServiceTypeName}' not found in any loaded assembly.";
                monitor.Log($"[PasswordBridge] {_detail}", LogLevel.Warn);
                return;
            }

            _instanceField = serviceType.GetField("_instance", BindingFlags.NonPublic | BindingFlags.Static);
            if (_instanceField is null)
            {
                _available = false;
                _detail = "Static field '_instance' not found on PasswordProtectionService.";
                monitor.Log($"[PasswordBridge] {_detail}", LogLevel.Warn);
                return;
            }

            _tryAuthenticateMethod = serviceType.GetMethod(
                "TryAuthenticate",
                BindingFlags.Public | BindingFlags.Instance,
                binder: null,
                types: new[] { typeof(long), typeof(string) },
                modifiers: null);
            if (_tryAuthenticateMethod is null)
            {
                _available = false;
                _detail = "Method 'TryAuthenticate(long, string)' not found on PasswordProtectionService.";
                monitor.Log($"[PasswordBridge] {_detail}", LogLevel.Warn);
                return;
            }

            _isPlayerAuthenticatedMethod = serviceType.GetMethod(
                "IsPlayerAuthenticated",
                BindingFlags.Public | BindingFlags.Instance,
                binder: null,
                types: new[] { typeof(long) },
                modifiers: null);
            if (_isPlayerAuthenticatedMethod is null)
            {
                _available = false;
                _detail = "Method 'IsPlayerAuthenticated(long)' not found on PasswordProtectionService.";
                monitor.Log($"[PasswordBridge] {_detail}", LogLevel.Warn);
                return;
            }

            var resultType = _tryAuthenticateMethod.ReturnType;
            _resultSuccessProperty = resultType.GetProperty("Success", BindingFlags.Public | BindingFlags.Instance);
            _resultMessageProperty = resultType.GetProperty("Message", BindingFlags.Public | BindingFlags.Instance);
            _resultShouldKickProperty = resultType.GetProperty("ShouldKick", BindingFlags.Public | BindingFlags.Instance);
            if (_resultSuccessProperty is null || _resultMessageProperty is null || _resultShouldKickProperty is null)
            {
                _available = false;
                _detail = $"Properties on '{resultType.FullName}' (Success/Message/ShouldKick) not found as expected.";
                monitor.Log($"[PasswordBridge] {_detail}", LogLevel.Warn);
                return;
            }

            _available = true;
            _detail = "OK";
            monitor.Log("[PasswordBridge] Reflection bridge resolved successfully.", LogLevel.Info);
        }
        catch (Exception ex)
        {
            _available = false;
            _detail = $"Reflection init failed: {ex.Message}";
            monitor.Log($"[PasswordBridge] Initialize failed: {ex}", LogLevel.Warn);
        }
    }

    private static Type? FindServiceType()
    {
        foreach (var assembly in AppDomain.CurrentDomain.GetAssemblies())
        {
            if (!string.Equals(assembly.GetName().Name, ServiceAssemblyName, StringComparison.Ordinal))
                continue;

            var type = assembly.GetType(ServiceTypeName, throwOnError: false);
            if (type is not null)
                return type;
        }
        return null;
    }

    /// <summary>
    /// Authenticates the given player as if they had typed the correct
    /// password themselves. The caller is responsible for providing the
    /// real server password (read directly from the same SERVER_PASSWORD
    /// environment variable JunimoServer itself uses).
    /// </summary>
    public (bool Success, string Message, bool ShouldKick, bool InvocationFailed) TryAuthenticate(long playerId, string password)
    {
        if (!_available || _instanceField is null || _tryAuthenticateMethod is null)
            return (false, "Password bridge unavailable: " + _detail, false, true);

        try
        {
            var instance = _instanceField.GetValue(null);
            if (instance is null)
                return (false, "PasswordProtectionService instance is not ready yet.", false, true);

            var result = _tryAuthenticateMethod.Invoke(instance, new object[] { playerId, password });
            if (result is null)
                return (false, "TryAuthenticate returned no result.", false, true);

            var success = (bool)(_resultSuccessProperty!.GetValue(result) ?? false);
            var message = (string?)_resultMessageProperty!.GetValue(result) ?? "";
            var shouldKick = (bool)(_resultShouldKickProperty!.GetValue(result) ?? false);
            return (success, message, shouldKick, false);
        }
        catch (Exception ex)
        {
            return (false, $"TryAuthenticate invoke failed: {ex.Message}", false, true);
        }
    }

    /// <summary>
    /// Read-only authentication check for a single player. Returns null when
    /// the bridge is unavailable or the query itself fails, so a single bad
    /// lookup does not affect the rest of the players.json snapshot.
    /// </summary>
    public bool? IsPlayerAuthenticated(long playerId)
    {
        if (!_available || _instanceField is null || _isPlayerAuthenticatedMethod is null)
            return null;

        try
        {
            var instance = _instanceField.GetValue(null);
            if (instance is null)
                return null;

            var result = _isPlayerAuthenticatedMethod.Invoke(instance, new object[] { playerId });
            return result as bool?;
        }
        catch
        {
            return null;
        }
    }
}
#endif
