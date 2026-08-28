#if !SAP_CI_BUILD
using System.Reflection;
using System.Threading;
using HarmonyLib;
using StardewModdingAPI;
using StardewValley;
using StardewValley.Network;

namespace StardewAnxiPanel.Control;

internal sealed class LoginChatPrivacyPatch
{
    private static IMonitor? monitor;
    private static int parseFailureLogged;
    private bool initialized;

    public bool Available { get; private set; }

    public string Detail { get; private set; } = "Not initialized.";

    public void Initialize(string harmonyID, IMonitor runtimeMonitor)
    {
        if (initialized)
            return;
        initialized = true;
        monitor = runtimeMonitor;
        try
        {
            var target = AccessTools.Method(
                typeof(GameServer),
                "rebroadcastClientMessage",
                new[] { typeof(IncomingMessage), typeof(long) });
            if (target is null)
            {
                Available = false;
                Detail = "GameServer.rebroadcastClientMessage(IncomingMessage, long) was not found.";
                runtimeMonitor.Log($"[LoginChatPrivacy] {Detail}", LogLevel.Error);
                return;
            }
            if (target.DeclaringType != typeof(GameServer) || target.ReturnType != typeof(void) || target.IsStatic)
            {
                Available = false;
                Detail = "GameServer.rebroadcastClientMessage has an unexpected runtime signature.";
                runtimeMonitor.Log($"[LoginChatPrivacy] {Detail}", LogLevel.Error);
                return;
            }

            var harmony = new Harmony(harmonyID + ".LoginChatPrivacy");
            var prefix = typeof(LoginChatPrivacyPatch).GetMethod(
                nameof(BeforeRebroadcastClientMessage),
                BindingFlags.NonPublic | BindingFlags.Static)
                ?? throw new MissingMethodException(nameof(BeforeRebroadcastClientMessage));
            harmony.Patch(target, prefix: new HarmonyMethod(prefix));

            var patchInfo = Harmony.GetPatchInfo(target);
            if (patchInfo?.Prefixes.Any(patch =>
                    string.Equals(patch.owner, harmony.Id, StringComparison.Ordinal)
                    && Equals(patch.PatchMethod, prefix)) != true)
                throw new InvalidOperationException("Harmony did not report the login chat privacy prefix as installed.");

            Available = true;
            Detail = "OK";
            runtimeMonitor.Log(
                "[LoginChatPrivacy] Login credential chat rebroadcast suppression installed.",
                LogLevel.Info);
        }
        catch (Exception ex)
        {
            Available = false;
            Detail = $"Login chat privacy patch failed: {ex.Message}";
            runtimeMonitor.Log($"[LoginChatPrivacy] {Detail}", LogLevel.Error);
        }
    }

    private static bool BeforeRebroadcastClientMessage(IncomingMessage __0)
    {
        if (__0 is null)
            return false;
        if (__0.MessageType != Multiplayer.chatMessage)
            return true;

        if (!LoginChatPrivacyPolicy.TryReadChatText(__0.Reader, out var chatText))
        {
            if (Interlocked.Exchange(ref parseFailureLogged, 1) == 0)
            {
                SafeLog(
                    "[LoginChatPrivacy] A chat packet could not be parsed and was not rebroadcast (fail closed).",
                    LogLevel.Error);
            }
            return false;
        }

        if (!LoginChatPrivacyPolicy.ShouldSuppressRebroadcast(chatText))
            return true;

        return false;
    }

    private static void SafeLog(string message, LogLevel level)
    {
        try
        {
            monitor?.Log(message, level);
        }
        catch
        {
        }
    }

}
#endif
