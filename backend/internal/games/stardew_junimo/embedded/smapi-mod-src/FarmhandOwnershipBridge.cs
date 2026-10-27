using System.Collections;
using System.Globalization;
using System.Reflection;
using System.Text.Json;

namespace StardewAnxiPanel.Control;

public sealed record FarmhandOwnershipProof(bool Required, int ReleasedFarmhands, bool Persisted, string ErrorCode);

// Invoke Junimo's owner lifecycle rather than editing its live map or store.
// The disk read is necessary because MarkReleased deliberately swallows I/O
// failures: its in-memory state alone is not a durable release receipt.
public sealed class FarmhandOwnershipBridge
{
    public const string ServiceTypeName = "JunimoServer.Services.Auth.FarmhandOwnershipService";
    private readonly Type? serviceType;
    private readonly bool required;

    public FarmhandOwnershipBridge(Type? serviceType, bool required)
    {
        this.serviceType = serviceType;
        this.required = required;
    }

    public static FarmhandOwnershipBridge ForLoadedJunimo(bool required)
    {
        var assemblies = AppDomain.CurrentDomain.GetAssemblies()
            .Where(value => value.GetName().Name == "JunimoServer").ToArray();
        if (assemblies.Length > 1)
            return new(null, true);
        var assembly = assemblies.SingleOrDefault();
        var type = assembly?.GetType(ServiceTypeName, throwOnError: false);
        return new(type, required || type is not null);
    }

    public FarmhandOwnershipProof ReleaseAndVerify(IReadOnlyCollection<long> ids, string saveDirectory)
        => Inspect(ids, saveDirectory, release: true);

    public FarmhandOwnershipProof ReadProof(IReadOnlyCollection<long> ids, string saveDirectory)
        => Inspect(ids, saveDirectory, release: false);

    private FarmhandOwnershipProof Inspect(IReadOnlyCollection<long> ids, string saveDirectory, bool release)
    {
        if (!required)
            return new(false, 0, true, "");
        try
        {
            if (serviceType is null || ids.Count != ids.Distinct().Count() || string.IsNullOrWhiteSpace(saveDirectory))
                return Failed("farmhand_ownership_unavailable");
            var instanceField = serviceType.GetField("_instance", BindingFlags.NonPublic | BindingFlags.Static);
            var markReleased = serviceType.GetMethod("MarkReleased", BindingFlags.Public | BindingFlags.Instance, null, new[] { typeof(long) }, null);
            var isReleased = serviceType.GetMethod("IsReleased", BindingFlags.Public | BindingFlags.Instance, null, new[] { typeof(long) }, null);
            var peekStore = serviceType.GetMethod("PeekStore", BindingFlags.Public | BindingFlags.Static, null, new[] { typeof(string) }, null);
            if (instanceField?.FieldType != serviceType
                || instanceField.GetValue(null) is not object instance
                || markReleased?.ReturnType != typeof(void)
                || isReleased?.ReturnType != typeof(bool)
                || peekStore is null || !typeof(IDictionary).IsAssignableFrom(peekStore.ReturnType))
                return Failed("farmhand_ownership_contract_mismatch");

            // Junimo serializes only these fields. Refuse an unknown/corrupt
            // format before it can discard fields during its next write.
            if (!StoreSchemaSupported(saveDirectory))
                return Failed("farmhand_ownership_store_invalid");
            if (release)
                foreach (var id in ids)
                    markReleased.Invoke(instance, new object[] { id });

            if (!StoreSchemaSupported(saveDirectory)
                || peekStore.Invoke(null, new object[] { saveDirectory }) is not IDictionary stored)
                return Failed("farmhand_ownership_unconfirmed");
            var count = 0;
            foreach (var id in ids)
            {
                if (isReleased.Invoke(instance, new object[] { id }) is not true || !stored.Contains(id))
                    continue;
                var record = stored[id];
                if (record is null)
                    continue;
                var type = record.GetType();
                if (type.GetProperty("Platform")?.GetValue(record) is string platform && platform == "released"
                    && type.GetProperty("Id")?.GetValue(record) is string identity && identity.Length == 0
                    && type.GetProperty("Origin")?.GetValue(record) is string origin && origin == "operator")
                    count++;
            }
            return count == ids.Count
                ? new(true, count, true, "")
                : new(true, count, false, "farmhand_ownership_unconfirmed");
        }
        catch
        {
            // Exception strings and store contents can contain platform IDs.
            return Failed("farmhand_ownership_unconfirmed");
        }
    }

    private static bool StoreSchemaSupported(string saveDirectory)
    {
        var path = Path.Combine(saveDirectory, "farmhand-ownership.json");
        if (!File.Exists(path))
            return true;
        if (new FileInfo(path).Length > 1024 * 1024)
            return false;
        using var document = JsonDocument.Parse(File.ReadAllText(path));
        if (document.RootElement.ValueKind != JsonValueKind.Object)
            return false;
        var root = document.RootElement.EnumerateObject().ToArray();
        if (root.Length != 1 || root[0].Name != "Farmhands" || root[0].Value.ValueKind != JsonValueKind.Object)
            return false;
        var ids = new HashSet<long>();
        foreach (var pair in root[0].Value.EnumerateObject())
        {
            if (!long.TryParse(pair.Name, NumberStyles.Integer, CultureInfo.InvariantCulture, out var id)
                || id.ToString(CultureInfo.InvariantCulture) != pair.Name
                || !ids.Add(id) || pair.Value.ValueKind != JsonValueKind.Object)
                return false;
            var keys = new HashSet<string>(StringComparer.Ordinal);
            foreach (var field in pair.Value.EnumerateObject())
            {
                if (!keys.Add(field.Name) || field.Name is not ("Platform" or "Id" or "Origin")
                    || field.Value.ValueKind != JsonValueKind.String)
                    return false;
            }
            if (!keys.Contains("Platform") || !keys.Contains("Id"))
                return false;
        }
        return true;
    }

    private static FarmhandOwnershipProof Failed(string code) => new(true, 0, false, code);
}
