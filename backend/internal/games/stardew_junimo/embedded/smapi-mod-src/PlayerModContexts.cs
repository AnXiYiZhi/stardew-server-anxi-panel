using System.Text;
using System.Text.Json.Serialization;

namespace StardewAnxiPanel.Control;

public static class PlayerModContextStatuses
{
    public const string Reported = "reported";
    public const string Pending = "pending";
    public const string Unavailable = "unavailable";
    public const string Stale = "stale";

    public static bool IsKnown(string? status)
        => status is Reported or Pending or Unavailable or Stale;
}

public sealed class PlayerModContextsFile
{
    public int SchemaVersion { get; set; } = PlayerModContextContract.SchemaVersion;
    public DateTimeOffset UpdatedAt { get; set; }
    public Dictionary<string, PlayerModContext> Players { get; set; } = new(StringComparer.Ordinal);
}

public sealed class PlayerModContext
{
    public string UniqueMultiplayerId { get; set; } = "";
    public bool HasSmapi { get; set; }
    public string GameVersion { get; set; } = "";
    public string ApiVersion { get; set; } = "";

    [JsonIgnore(Condition = JsonIgnoreCondition.Never)]
    public PlayerReportedMod[]? Mods { get; set; }

    public string ContextStatus { get; set; } = PlayerModContextStatuses.Pending;
    public DateTimeOffset? ReportedAt { get; set; }
    public DateTimeOffset UpdatedAt { get; set; }
}

public sealed class PlayerReportedMod
{
    public string UniqueId { get; set; } = "";
    public string Name { get; set; } = "";
    public string Version { get; set; } = "";
}

public static class PlayerModContextContract
{
    public const int SchemaVersion = 1;
    public const int MaxPlayerContexts = 512;
    public const int MaxModsPerPlayer = 1024;
    public const int MaxRawModsInspectedPerPlayer = 2048;
    public const int MaxPlayerIdChars = 32;
    public const int MaxUniqueIdChars = 256;
    public const int MaxNameChars = 256;
    public const int MaxVersionChars = 64;
    public const int MaxContextFileBytes = 4 * 1024 * 1024;

    public static string NormalizePlayerId(string? raw)
    {
        var value = NormalizeText(raw, MaxPlayerIdChars);
        if (value.Length == 0)
            return "";
        var start = value[0] == '-' ? 1 : 0;
        if (start == value.Length)
            return "";
        for (var i = start; i < value.Length; i++)
        {
            if (value[i] is < '0' or > '9')
                return "";
        }
        return value;
    }

    public static string NormalizeText(string? raw, int maxChars)
    {
        if (string.IsNullOrWhiteSpace(raw) || maxChars <= 0)
            return "";

        var builder = new StringBuilder(Math.Min(raw.Length, maxChars));
        foreach (var character in raw.Trim())
        {
            if (char.IsControl(character))
                continue;
            if (builder.Length >= maxChars)
                break;
            builder.Append(character);
        }
        if (builder.Length > 0 && char.IsHighSurrogate(builder[^1]))
            builder.Length--;
        return builder.ToString().Trim();
    }

    public static bool TryNormalizeMods(IEnumerable<PlayerReportedMod> rawMods, out PlayerReportedMod[] mods)
    {
        var unique = new Dictionary<string, PlayerReportedMod>(StringComparer.OrdinalIgnoreCase);
        var inspected = 0;
        foreach (var raw in rawMods)
        {
            if (inspected++ >= MaxRawModsInspectedPerPlayer)
            {
                mods = Array.Empty<PlayerReportedMod>();
                return false;
            }

            var uniqueId = NormalizeText(raw.UniqueId, MaxUniqueIdChars);
            if (uniqueId.Length == 0 || unique.ContainsKey(uniqueId))
                continue;
            if (unique.Count >= MaxModsPerPlayer)
            {
                mods = Array.Empty<PlayerReportedMod>();
                return false;
            }
            unique[uniqueId] = new PlayerReportedMod
            {
                UniqueId = uniqueId,
                Name = NormalizeText(raw.Name, MaxNameChars),
                Version = NormalizeText(raw.Version, MaxVersionChars),
            };
        }
        mods = unique.Values
            .OrderBy(mod => mod.UniqueId, StringComparer.OrdinalIgnoreCase)
            .ThenBy(mod => mod.Version, StringComparer.Ordinal)
            .ToArray();
        return true;
    }

    public static PlayerModContext Pending(string playerId, bool hasSmapi, DateTimeOffset now)
        => new()
        {
            UniqueMultiplayerId = NormalizePlayerId(playerId),
            HasSmapi = hasSmapi,
            Mods = null,
            ContextStatus = PlayerModContextStatuses.Pending,
            UpdatedAt = now,
        };

    public static PlayerModContext Reported(
        string playerId,
        bool hasSmapi,
        string? gameVersion,
        string? apiVersion,
        IEnumerable<PlayerReportedMod>? rawMods,
        DateTimeOffset now)
    {
        if (!hasSmapi || rawMods is null || !TryNormalizeMods(rawMods, out var mods))
            return Unavailable(playerId, hasSmapi, gameVersion, apiVersion, now, reportedAt: now);

        return new PlayerModContext
        {
            UniqueMultiplayerId = NormalizePlayerId(playerId),
            HasSmapi = true,
            GameVersion = NormalizeText(gameVersion, MaxVersionChars),
            ApiVersion = NormalizeText(apiVersion, MaxVersionChars),
            Mods = mods,
            ContextStatus = PlayerModContextStatuses.Reported,
            ReportedAt = now,
            UpdatedAt = now,
        };
    }

    public static PlayerModContext Unavailable(
        string playerId,
        bool hasSmapi,
        string? gameVersion,
        string? apiVersion,
        DateTimeOffset now,
        DateTimeOffset? reportedAt = null)
        => new()
        {
            UniqueMultiplayerId = NormalizePlayerId(playerId),
            HasSmapi = hasSmapi,
            GameVersion = NormalizeText(gameVersion, MaxVersionChars),
            ApiVersion = NormalizeText(apiVersion, MaxVersionChars),
            Mods = null,
            ContextStatus = PlayerModContextStatuses.Unavailable,
            ReportedAt = reportedAt,
            UpdatedAt = now,
        };

    public static PlayerModContext Stale(PlayerModContext? previous, string playerId, bool hasSmapi, DateTimeOffset now)
    {
        var normalizedId = NormalizePlayerId(playerId);
        var preservedMods = Array.Empty<PlayerReportedMod>();
        var canPreserveReport = previous is not null
            && previous.Mods is not null
            && previous.ContextStatus is PlayerModContextStatuses.Reported or PlayerModContextStatuses.Stale
            && TryNormalizeMods(previous.Mods, out preservedMods);
        return new PlayerModContext
        {
            UniqueMultiplayerId = normalizedId,
            HasSmapi = previous?.HasSmapi ?? hasSmapi,
            GameVersion = NormalizeText(previous?.GameVersion, MaxVersionChars),
            ApiVersion = NormalizeText(previous?.ApiVersion, MaxVersionChars),
            Mods = canPreserveReport ? preservedMods : null,
            ContextStatus = PlayerModContextStatuses.Stale,
            ReportedAt = previous?.ReportedAt,
            UpdatedAt = now,
        };
    }

    public static PlayerModContextsFile NormalizeLoadedFile(PlayerModContextsFile? raw, DateTimeOffset now)
    {
        var normalized = new PlayerModContextsFile { UpdatedAt = now };
        if (raw?.SchemaVersion != SchemaVersion || raw.Players is null)
            return normalized;

        foreach (var entry in raw.Players.Values
                     .OrderByDescending(context => context.UpdatedAt)
                     .Take(MaxPlayerContexts))
        {
            var playerId = NormalizePlayerId(entry.UniqueMultiplayerId);
            if (playerId.Length == 0 || normalized.Players.ContainsKey(playerId))
                continue;
            normalized.Players[playerId] = Stale(entry, playerId, entry.HasSmapi, now);
        }
        return normalized;
    }

    public static void PruneOldest(IDictionary<string, PlayerModContext> contexts)
    {
        if (contexts.Count <= MaxPlayerContexts)
            return;
        foreach (var playerId in contexts.Values
                     .OrderBy(context => context.ContextStatus == PlayerModContextStatuses.Stale ? 0 : 1)
                     .ThenBy(context => context.UpdatedAt)
                     .Select(context => context.UniqueMultiplayerId)
                     .Take(contexts.Count - MaxPlayerContexts)
                     .ToArray())
        {
            contexts.Remove(playerId);
        }
    }
}

public static class PlayerModContextLifecycle
{
    public static bool Connect(
        IDictionary<string, PlayerModContext> contexts,
        string playerId,
        bool hasSmapi,
        DateTimeOffset now)
    {
        var normalizedId = PlayerModContextContract.NormalizePlayerId(playerId);
        if (normalizedId.Length == 0)
            return false;
        if (contexts.TryGetValue(normalizedId, out var current)
            && current.ContextStatus == PlayerModContextStatuses.Reported)
        {
            // SMAPI may publish PeerContextReceived before PeerConnected. Keep the
            // fresh report instead of replacing it with a pending placeholder.
            return false;
        }

        contexts[normalizedId] = PlayerModContextContract.Pending(normalizedId, hasSmapi, now);
        return true;
    }

    public static bool Report(
        IDictionary<string, PlayerModContext> contexts,
        string playerId,
        bool hasSmapi,
        string? gameVersion,
        string? apiVersion,
        IEnumerable<PlayerReportedMod>? rawMods,
        DateTimeOffset now)
    {
        var context = PlayerModContextContract.Reported(
            playerId,
            hasSmapi,
            gameVersion,
            apiVersion,
            rawMods,
            now);
        if (context.UniqueMultiplayerId.Length == 0)
            return false;
        contexts[context.UniqueMultiplayerId] = context;
        return true;
    }

    public static bool Disconnect(
        IDictionary<string, PlayerModContext> contexts,
        string playerId,
        bool hasSmapi,
        DateTimeOffset now)
    {
        var normalizedId = PlayerModContextContract.NormalizePlayerId(playerId);
        if (normalizedId.Length == 0)
            return false;
        contexts.TryGetValue(normalizedId, out var previous);
        contexts[normalizedId] = PlayerModContextContract.Stale(previous, normalizedId, hasSmapi, now);
        return true;
    }

    public static bool ExpirePending(
        IDictionary<string, PlayerModContext> contexts,
        DateTimeOffset now,
        TimeSpan timeout)
    {
        var changed = false;
        foreach (var entry in contexts.ToArray())
        {
            var context = entry.Value;
            if (context.ContextStatus != PlayerModContextStatuses.Pending
                || now - context.UpdatedAt < timeout)
            {
                continue;
            }
            contexts[entry.Key] = PlayerModContextContract.Unavailable(
                entry.Key,
                context.HasSmapi,
                context.GameVersion,
                context.ApiVersion,
                now,
                context.ReportedAt);
            changed = true;
        }
        return changed;
    }
}
