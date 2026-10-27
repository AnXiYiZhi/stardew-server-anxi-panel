using System.Text.Json;
using StardewAnxiPanel.Control;

internal static class FarmhandOwnershipBridgeTests
{
    public static void Run()
    {
        var command = new PanelCommand
        {
            Id = "ownership-proof", Name = "save-now", CreatedAt = DateTimeOffset.UtcNow,
            Payload = new()
            {
                ["preSaveAction"] = JsonSerializer.SerializeToElement(SaveCommandContract.UnbindAllFarmhandsAction),
                ["preSaveActionSaveId"] = JsonSerializer.SerializeToElement("Owned_1"),
            },
        };
        var saved = SaveCommandContract.CompleteSavedEvent(command, "Owned_1", DateTimeOffset.UtcNow,
            new FarmhandBindingSummary(2, 1, 0, true, 2, true));
        Assert(saved.Status == CommandStatuses.Succeeded
            && saved.Details?["farmhandOwnershipPersisted"] == "true", "durable anonymous ownership receipt");
        foreach (var proof in new[] { new FarmhandBindingSummary(2, 1, 0, true, 2, false), new FarmhandBindingSummary(2, 1, 0, true, 1, true) })
        {
            var rejected = SaveCommandContract.CompleteSavedEvent(command, "Owned_1", DateTimeOffset.UtcNow, proof);
            Assert(rejected.Status == CommandStatuses.Failed && rejected.ErrorCode == "farmhand_ownership_unconfirmed", "Saved is insufficient without ownership proof");
        }
        var root = Path.Combine(Path.GetTempPath(), "ownership-contract-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(root);
        try
        {
            var ids = new long[] { 42, -43 };
            var legacy = new FarmhandOwnershipBridge(null, false).ReleaseAndVerify(ids, root);
            Assert(!legacy.Required && legacy.Persisted, "legacy no-op");
            Assert(!new FarmhandOwnershipBridge(null, true).ReleaseAndVerify(ids, root).Persisted, "required missing service");
            FakeOwnership.Reset(root);
            var bridge = new FarmhandOwnershipBridge(typeof(FakeOwnership), true);
            var released = bridge.ReleaseAndVerify(ids, root);
            Assert(released.Persisted && released.ReleasedFarmhands == 2, "live and disk release");
            Assert(bridge.ReadProof(ids, root).Persisted && FakeOwnership.ReleaseCalls == 2, "read proof must not release again");
            var disk = FakeOwnership.PeekStore(root);
            disk[42].Platform = "steam";
            File.WriteAllText(Path.Combine(root, "farmhand-ownership.json"), JsonSerializer.Serialize(new { Farmhands=disk }));
            Assert(!bridge.ReadProof(ids, root).Persisted, "reclaimed disk record");
            FakeOwnership.Reset(root);
            FakeOwnership.FailWrites = true;
            Assert(!bridge.ReleaseAndVerify(ids, root).Persisted, "swallowed disk write failure");
            FakeOwnership.Reset(root);
            var path = Path.Combine(root, "farmhand-ownership.json");
            foreach (var json in new[] { "{", "{\"Farmhands\":{},\"Future\":true}", "{\"Farmhands\":{\"42\":{\"Platform\":\"steam\",\"Id\":\"redacted\",\"Future\":1}}}" })
            {
                File.WriteAllText(path, json);
                Assert(!bridge.ReleaseAndVerify(ids, root).Persisted && FakeOwnership.ReleaseCalls == 0, "invalid/unknown store before mutation");
                Assert(File.ReadAllText(path) == json, "refused store preserved");
            }
            FakeOwnership.Reset(root);
            var duplicate = bridge.ReleaseAndVerify(new long[] { 42, 42 }, root);
            Assert(!duplicate.Persisted && FakeOwnership.ReleaseCalls == 0, "duplicate role identity");
        }
        finally
        {
            Directory.Delete(root, true);
        }
    }

    private static void Assert(bool condition, string message)
    {
        if (!condition) throw new InvalidOperationException("ownership bridge: " + message);
    }

    internal sealed class Owner
    {
        public string Platform { get; set; } = "released";
        public string Id { get; set; } = "";
        public string Origin { get; set; } = "operator";
    }

    internal sealed class FakeOwnership
    {
#pragma warning disable CS0414 // Exact reflected singleton contract.
        private static FakeOwnership _instance = new();
#pragma warning restore CS0414
        private static string directory = "";
        private static readonly Dictionary<long, Owner> records = new();
        public static bool FailWrites;
        public static int ReleaseCalls;
        public static void Reset(string path)
        {
            directory = path;
            records.Clear();
            FailWrites = false;
            ReleaseCalls = 0;
            File.Delete(Path.Combine(path, "farmhand-ownership.json"));
        }
        public void MarkReleased(long id)
        {
            ReleaseCalls++;
            records[id] = new();
            if (!FailWrites) Flush();
        }
        public bool IsReleased(long id) => records.TryGetValue(id, out var value) && value.Platform == "released";
        public static Dictionary<long, Owner> PeekStore(string path)
        {
            var file = Path.Combine(path, "farmhand-ownership.json");
            if (!File.Exists(file)) return new();
            using var json = JsonDocument.Parse(File.ReadAllText(file));
            return JsonSerializer.Deserialize<Dictionary<long, Owner>>(json.RootElement.GetProperty("Farmhands").GetRawText())!;
        }
        public static void Flush() => File.WriteAllText(Path.Combine(directory, "farmhand-ownership.json"), JsonSerializer.Serialize(new { Farmhands=records }));
    }
}
