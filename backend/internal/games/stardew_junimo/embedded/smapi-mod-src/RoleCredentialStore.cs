using System.Globalization;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace StardewAnxiPanel.Control;

public sealed class RoleCredentialStore
{
    public const int SchemaVersion = 1;
    public const string FileName = "role-passwords.json";

    private const string MarkerFileName = "role-passwords.initialized";
    private const string LockFileName = ".role-passwords.lock";
    private const int MaxStoreBytes = 1024 * 1024;
    private static readonly TimeSpan LockTimeout = TimeSpan.FromSeconds(5);
    private static readonly TimeSpan StaleLockAge = TimeSpan.FromMinutes(2);
    private readonly string controlDir;
    private readonly string path;

    [DllImport("libc", SetLastError = true)]
    private static extern int chmod(string path, uint mode);

    public RoleCredentialStore(string controlDir)
    {
        this.controlDir = Path.GetFullPath(controlDir);
        path = Path.Combine(this.controlDir, FileName);
    }

    public bool ValidateExisting(out string detail)
    {
        try
        {
            _ = Load();
            detail = "OK";
            return true;
        }
        catch
        {
            detail = "Role credential store is unreadable or invalid.";
            return false;
        }
    }

    public bool TryAuthenticateOrEnroll(
        string saveId,
        long playerId,
        string password,
        byte[] roleKey,
        IReadOnlyDictionary<string, RolePasswordRecord> legacyRoles,
        out string detail)
    {
        detail = "Authentication rejected.";
        if (!ValidSaveId(saveId) || playerId == 0 || roleKey.Length != 32 || !RolePasswordPolicy.IsChatRepresentablePassword(password))
            return false;

        try
        {
            Directory.CreateDirectory(controlDir);
            using var lease = AcquireLock();
            var (store, exists) = Load();
            var migrated = false;
            if (!exists && legacyRoles.Count > 0)
            {
                store.Saves[saveId] = new RoleCredentialSave
                {
                    Roles = legacyRoles.ToDictionary(
                        pair => pair.Key,
                        pair => new RolePasswordRecord
                        {
                            Name = pair.Value.Name,
                            Verifier = pair.Value.Verifier,
                            UpdatedAt = pair.Value.UpdatedAt,
                        },
                        StringComparer.Ordinal),
                };
                Validate(store);
                migrated = true;
            }

            if (!store.Saves.TryGetValue(saveId, out var save))
            {
                save = new RoleCredentialSave { Roles = new(StringComparer.Ordinal) };
                store.Saves[saveId] = save;
            }

            var roleId = playerId.ToString(CultureInfo.InvariantCulture);
            if (save.Roles.TryGetValue(roleId, out var record))
            {
                if (migrated)
                    Write(store);
                var verifier = RolePasswordPolicy.ComputeVerifier(roleKey, roleId, password);
                var matched = RolePasswordPolicy.SecureEquals(verifier, record.Verifier);
                detail = matched ? "Authenticated." : "Authentication rejected.";
                return matched;
            }

            save.Roles[roleId] = new RolePasswordRecord
            {
                Verifier = RolePasswordPolicy.ComputeVerifier(roleKey, roleId, password),
                UpdatedAt = DateTimeOffset.UtcNow.ToString("O", CultureInfo.InvariantCulture),
            };
            Write(store);
            detail = "Credential enrolled and authenticated.";
            return true;
        }
        catch
        {
            detail = "Role credential store is unavailable or invalid.";
            return false;
        }
    }

    private (RoleCredentialStorePayload Store, bool Exists) Load()
    {
        var markerExists = ReadMarker();
        byte[] raw;
        try
        {
            using var input = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read);
            if (input.Length <= 0 || input.Length > MaxStoreBytes)
                throw new InvalidDataException("Role credential store has an invalid size.");
            raw = new byte[(int)input.Length];
            var offset = 0;
            while (offset < raw.Length)
            {
                var read = input.Read(raw, offset, raw.Length - offset);
                if (read == 0)
                    throw new EndOfStreamException("Role credential store ended before its declared length.");
                offset += read;
            }
        }
        catch (FileNotFoundException)
        {
            if (markerExists)
                throw new InvalidDataException("Role credential store is missing after initialization.");
            return (NewStore(), false);
        }
        catch (DirectoryNotFoundException)
        {
            if (markerExists)
                throw new InvalidDataException("Role credential store is missing after initialization.");
            return (NewStore(), false);
        }
        var store = JsonSerializer.Deserialize<RoleCredentialStorePayload>(raw, ContractJson.Options)
            ?? throw new InvalidDataException("Role credential store is empty.");
        Validate(store);
        if (!markerExists)
            throw new InvalidDataException("Role credential store exists without its initialization marker.");
        return (store, true);
    }

    private static RoleCredentialStorePayload NewStore()
        => new()
        {
            SchemaVersion = SchemaVersion,
            Saves = new(StringComparer.Ordinal),
        };

    private static void Validate(RoleCredentialStorePayload store)
    {
        if (store.SchemaVersion != SchemaVersion || store.Saves is null)
            throw new InvalidDataException("Role credential store schema is missing or unsupported.");
        foreach (var savePair in store.Saves)
        {
            if (!ValidSaveId(savePair.Key) || savePair.Value?.Roles is null)
                throw new InvalidDataException("Role credential store contains an invalid save record.");
            foreach (var rolePair in savePair.Value.Roles)
            {
                if (!RolePasswordPolicy.IsCanonicalRoleId(rolePair.Key)
                    || rolePair.Value is null
                    || rolePair.Value.Verifier.Length != 43
                    || RolePasswordPolicy.Base64UrlDecode(rolePair.Value.Verifier).Length != 32)
                {
                    throw new InvalidDataException("Role credential store contains an invalid role record.");
                }
            }
        }
    }

    private void Write(RoleCredentialStorePayload store)
    {
        Validate(store);
        var raw = JsonSerializer.SerializeToUtf8Bytes(store, ContractJson.Options);
        if (raw.Length > MaxStoreBytes)
            throw new InvalidDataException("Role credential store exceeds its size limit.");
        Directory.CreateDirectory(controlDir);
        var tempPath = Path.Combine(controlDir, FileName + ".tmp-" + Guid.NewGuid().ToString("N"));
        try
        {
            using (var output = new FileStream(
                tempPath,
                FileMode.CreateNew,
                FileAccess.Write,
                FileShare.None,
                4096,
                FileOptions.WriteThrough))
            {
                output.Write(raw);
                output.WriteByte((byte)'\n');
                output.Flush(flushToDisk: true);
            }
            RestrictToOwner(tempPath);
            // A durable marker without a store fails closed. The reverse order
            // could make a crash look like a never-initialized store and reopen
            // first-login enrollment for existing roles.
            WriteMarker();
            File.Move(tempPath, path, overwrite: true);
        }
        finally
        {
            try
            {
                if (File.Exists(tempPath))
                    File.Delete(tempPath);
            }
            catch
            {
                // A failed cleanup must not replace the primary store error.
            }
        }
    }

    private bool ReadMarker()
    {
        var markerPath = Path.Combine(controlDir, MarkerFileName);
        try
        {
            return string.Equals(File.ReadAllText(markerPath, Encoding.ASCII), "1\n", StringComparison.Ordinal)
                ? true
                : throw new InvalidDataException("Role credential store marker is invalid.");
        }
        catch (FileNotFoundException)
        {
            return false;
        }
        catch (DirectoryNotFoundException)
        {
            return false;
        }
    }

    private void WriteMarker()
    {
        if (ReadMarker())
            return;
        var markerPath = Path.Combine(controlDir, MarkerFileName);
        try
        {
            using (var marker = new FileStream(
                markerPath,
                FileMode.CreateNew,
                FileAccess.Write,
                FileShare.None,
                4096,
                FileOptions.WriteThrough))
            {
                marker.Write(Encoding.ASCII.GetBytes("1\n"));
                marker.Flush(flushToDisk: true);
            }
        }
        catch (IOException)
        {
            if (!ReadMarker())
                throw;
            return;
        }
        RestrictToOwner(markerPath);
    }

    private RoleCredentialStoreLease AcquireLock()
    {
        var lockPath = Path.Combine(controlDir, LockFileName);
        var owner = Convert.ToBase64String(RandomNumberGenerator.GetBytes(24)).TrimEnd('=').Replace('+', '-').Replace('/', '_');
        var deadline = DateTimeOffset.UtcNow + LockTimeout;
        while (true)
        {
            try
            {
                using (var file = new FileStream(
                    lockPath,
                    FileMode.CreateNew,
                    FileAccess.Write,
                    FileShare.None,
                    4096,
                    FileOptions.WriteThrough))
                {
                    var rawOwner = Encoding.ASCII.GetBytes(owner);
                    file.Write(rawOwner);
                    file.Flush(flushToDisk: true);
                }
                RestrictToOwner(lockPath);
                return new RoleCredentialStoreLease(lockPath, owner);
            }
            catch (IOException)
            {
                BreakStaleLock(lockPath);
                if (DateTimeOffset.UtcNow >= deadline)
                    throw new TimeoutException("Timed out waiting for the role credential store lock.");
                Thread.Sleep(25);
            }
        }
    }

    private static void BreakStaleLock(string lockPath)
    {
        try
        {
            if (!File.Exists(lockPath) || DateTime.UtcNow - File.GetLastWriteTimeUtc(lockPath) <= StaleLockAge)
                return;
            var stalePath = lockPath + ".stale-" + Guid.NewGuid().ToString("N");
            File.Move(lockPath, stalePath);
            File.Delete(stalePath);
        }
        catch
        {
            // Another process still owns the lock or won the stale-lock race.
        }
    }

    private static void RestrictToOwner(string filePath)
    {
        if (!OperatingSystem.IsWindows() && chmod(filePath, 0x180) != 0)
            throw new IOException("Could not restrict the role credential file permissions.");
    }

    private static bool ValidSaveId(string saveId)
    {
        if (string.IsNullOrWhiteSpace(saveId) || !string.Equals(saveId, saveId.Trim(), StringComparison.Ordinal))
            return false;
        var count = 0;
        foreach (var rune in saveId.EnumerateRunes())
        {
            if (Rune.IsControl(rune) || ++count > 512)
                return false;
        }
        return true;
    }
}

internal sealed class RoleCredentialStoreLease : IDisposable
{
    private readonly string path;
    private readonly string owner;
    private bool disposed;

    public RoleCredentialStoreLease(string path, string owner)
    {
        this.path = path;
        this.owner = owner;
    }

    public void Dispose()
    {
        if (disposed)
            return;
        disposed = true;
        try
        {
            if (File.Exists(path) && string.Equals(File.ReadAllText(path, Encoding.ASCII), owner, StringComparison.Ordinal))
                File.Delete(path);
        }
        catch
        {
            // A stale-lock recovery race must not delete another owner's lock.
        }
    }
}

public sealed class RoleCredentialStorePayload
{
    public int SchemaVersion { get; set; }
    public Dictionary<string, RoleCredentialSave> Saves { get; set; } = null!;
}

public sealed class RoleCredentialSave
{
    public Dictionary<string, RolePasswordRecord> Roles { get; set; } = null!;
}
