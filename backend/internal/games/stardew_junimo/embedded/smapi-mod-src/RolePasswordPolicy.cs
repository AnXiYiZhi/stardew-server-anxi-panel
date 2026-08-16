using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace StardewAnxiPanel.Control;

public sealed class RolePasswordPolicy
{
    public const string NoneMode = "none";
    public const string GlobalMode = "global";
    public const string RoleMode = "role";
    public const string InvalidPasswordSentinel = "\0sap-role-auth-invalid";

    private const int SchemaVersion = 1;
    private const int KeyLength = 32;
    private readonly byte[] roleKey;
    private readonly Dictionary<string, RolePasswordRecord> roles;
    private readonly string serverPassword;
    private readonly RoleCredentialStore? credentialStore;

    private RolePasswordPolicy(
        string mode,
        string revision,
        byte[] roleKey,
        Dictionary<string, RolePasswordRecord> roles,
        string serverPassword,
        bool valid,
        string detail,
        RoleCredentialStore? credentialStore = null)
    {
        Mode = mode;
        Revision = revision;
        this.roleKey = roleKey;
        this.roles = roles;
        this.serverPassword = serverPassword;
        this.credentialStore = credentialStore;
        Valid = valid;
        Detail = detail;
    }

    public string Mode { get; }

    public string Revision { get; }

    public bool Valid { get; }

    public string Detail { get; }

    public bool RequiresPatch => string.Equals(Mode, RoleMode, StringComparison.Ordinal);

    public string InternalGuard => serverPassword;

    public static RolePasswordPolicy LoadFromEnvironment(string controlDir)
    {
        return Parse(
            Environment.GetEnvironmentVariable("SAP_PLAYER_AUTH_MODE"),
            Environment.GetEnvironmentVariable("SAP_PLAYER_AUTH_REVISION"),
            Environment.GetEnvironmentVariable("SAP_ROLE_AUTH_KEY"),
            Environment.GetEnvironmentVariable("SAP_ROLE_PASSWORDS_B64"),
            Environment.GetEnvironmentVariable("SERVER_PASSWORD"))
            .UseCredentialStore(controlDir);
    }

    public static RolePasswordPolicy Parse(
        string? rawMode,
        string? rawRevision,
        string? encodedKey,
        string? encodedPayload,
        string? serverPassword)
    {
        var password = serverPassword ?? "";
        var mode = (rawMode ?? "").Trim().ToLowerInvariant();
        if (mode.Length == 0)
            mode = password.Length == 0 ? NoneMode : GlobalMode;
        var revision = (rawRevision ?? "").Trim();
        if (revision.Length == 0)
            revision = "legacy-" + mode;

        if (mode is not (NoneMode or GlobalMode or RoleMode))
            return Invalid(mode, revision, password, "Unsupported player authentication mode.");
        if (mode != RoleMode)
            return new RolePasswordPolicy(mode, revision, Array.Empty<byte>(), new(), password, true, "OK");

        try
        {
            var key = Base64UrlDecode(encodedKey ?? "");
            if (key.Length != KeyLength)
                return Invalid(mode, revision, password, "Role authentication key is missing or invalid.");
            RolePasswordPayload payload;
            if (string.IsNullOrWhiteSpace(encodedPayload))
            {
                payload = new RolePasswordPayload { SchemaVersion = SchemaVersion, Roles = new(StringComparer.Ordinal) };
            }
            else
            {
                var payloadBytes = Base64UrlDecode(encodedPayload);
                payload = JsonSerializer.Deserialize<RolePasswordPayload>(payloadBytes, ContractJson.Options)
                    ?? throw new InvalidDataException("Role password configuration is empty.");
            }
            if (payload.SchemaVersion != SchemaVersion || payload.Roles is null)
                return Invalid(mode, revision, password, "Role password configuration is missing or unsupported.");
            foreach (var pair in payload.Roles)
            {
                if (!IsCanonicalRoleId(pair.Key)
                    || pair.Value.Verifier.Length != 43
                    || Base64UrlDecode(pair.Value.Verifier).Length != 32)
                {
                    return Invalid(mode, revision, password, "Role password configuration contains an invalid record.");
                }
            }
            var expectedGuard = DeriveInternalGuard(key);
            if (!SecureEquals(password, expectedGuard))
                return Invalid(mode, revision, password, "Role authentication guard does not match SERVER_PASSWORD.");
            return new RolePasswordPolicy(mode, revision, key, payload.Roles, password, true, "OK");
        }
        catch (Exception ex)
        {
            return Invalid(mode, revision, password, "Role password configuration could not be parsed: " + ex.Message);
        }
    }

    public RolePasswordPolicy UseCredentialStore(string controlDir)
    {
        if (!RequiresPatch || !Valid)
            return this;
        var store = new RoleCredentialStore(controlDir);
        if (!store.ValidateExisting(out var detail))
            return Invalid(Mode, Revision, serverPassword, detail);
        return new RolePasswordPolicy(Mode, Revision, roleKey, roles, serverPassword, true, "OK", store);
    }

    public string RewritePassword(long playerId, string? providedPassword)
    {
        var provided = providedPassword ?? "";
        if (!RequiresPatch)
            return provided;

        if (!Valid)
            return InvalidPasswordSentinel;

        // The Panel approval command supplies the real internal guard. Keeping
        // this exact bypass means role mode still supports explicit admin approval.
        if (SecureEquals(provided, serverPassword))
            return provided;

        var roleId = playerId.ToString(CultureInfo.InvariantCulture);
        if (!roles.TryGetValue(roleId, out var record))
            return InvalidPasswordSentinel;
        var verifier = ComputeVerifier(roleKey, roleId, provided);
        return SecureEquals(verifier, record.Verifier) ? serverPassword : InvalidPasswordSentinel;
    }

    public string RewritePassword(string? saveId, long playerId, string? providedPassword)
    {
        var provided = providedPassword ?? "";
        if (!RequiresPatch)
            return provided;
        if (!Valid)
            return InvalidPasswordSentinel;
        if (credentialStore is not null && !credentialStore.ValidateExisting(out _))
            return InvalidPasswordSentinel;
        if (SecureEquals(provided, serverPassword))
            return provided;
        if (credentialStore is null)
            return RewritePassword(playerId, provided);
        return credentialStore.TryAuthenticateOrEnroll(saveId ?? "", playerId, provided, roleKey, roles, out _)
            ? serverPassword
            : InvalidPasswordSentinel;
    }

    public static string ComputeVerifier(byte[] key, string roleId, string password)
    {
        using var hmac = new HMACSHA256(key);
        var payload = Encoding.UTF8.GetBytes("sap-role-password-v1\0" + roleId + "\0" + password);
        return Base64UrlEncode(hmac.ComputeHash(payload));
    }

    public static string DeriveInternalGuard(byte[] key)
    {
        using var hmac = new HMACSHA256(key);
        return "sap_" + Base64UrlEncode(hmac.ComputeHash(Encoding.UTF8.GetBytes("sap-junimo-internal-password-v1")));
    }

    internal static bool IsCanonicalRoleId(string roleId)
    {
        return long.TryParse(roleId, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out var parsed)
            && parsed != 0
            && string.Equals(roleId, parsed.ToString(CultureInfo.InvariantCulture), StringComparison.Ordinal);
    }

    internal static bool IsChatRepresentablePassword(string password)
    {
        if (password.StartsWith(' ')
            || password.EndsWith(' ')
            || password.Contains("  ", StringComparison.Ordinal))
        {
            return false;
        }

        var count = 0;
        foreach (var rune in password.EnumerateRunes())
        {
            if (Rune.IsControl(rune) || ++count > 128)
                return false;
        }
        return count > 0;
    }

    private static RolePasswordPolicy Invalid(string mode, string revision, string serverPassword, string detail)
        => new(mode, revision, Array.Empty<byte>(), new(), serverPassword, false, detail);

    internal static string Base64UrlEncode(byte[] value)
        => Convert.ToBase64String(value).TrimEnd('=').Replace('+', '-').Replace('/', '_');

    internal static byte[] Base64UrlDecode(string value)
    {
        var normalized = value.Replace('-', '+').Replace('_', '/');
        normalized += (normalized.Length % 4) switch
        {
            2 => "==",
            3 => "=",
            0 => "",
            _ => throw new FormatException("Invalid base64url value."),
        };
        return Convert.FromBase64String(normalized);
    }

    internal static bool SecureEquals(string left, string right)
    {
        var leftBytes = Encoding.UTF8.GetBytes(left);
        var rightBytes = Encoding.UTF8.GetBytes(right);
        return leftBytes.Length == rightBytes.Length
            && CryptographicOperations.FixedTimeEquals(leftBytes, rightBytes);
    }
}

public sealed class RolePasswordPayload
{
    public int SchemaVersion { get; set; }
    public Dictionary<string, RolePasswordRecord> Roles { get; set; } = new(StringComparer.Ordinal);
}

public sealed class RolePasswordRecord
{
    public string Name { get; set; } = "";
    public string Verifier { get; set; } = "";
    public string UpdatedAt { get; set; } = "";
}
