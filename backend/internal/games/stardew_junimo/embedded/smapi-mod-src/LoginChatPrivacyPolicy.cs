namespace StardewAnxiPanel.Control;

public static class LoginChatPrivacyPolicy
{
    private const string LoginCommand = "!login";

    public static bool TryReadChatText(BinaryReader? reader, out string chatText)
    {
        chatText = "";
        Stream? stream = null;
        long originalPosition = 0;
        var originalPositionCaptured = false;
        var parsed = false;
        var restored = false;
        try
        {
            stream = reader?.BaseStream;
            if (reader is null || stream is null || !stream.CanSeek)
                return false;

            originalPosition = stream.Position;
            originalPositionCaptured = true;
            stream.Position = 0;
            reader.ReadInt64();
            reader.ReadInt16();
            chatText = reader.ReadString();
            parsed = true;
        }
        catch
        {
            chatText = "";
        }
        finally
        {
            if (originalPositionCaptured)
            {
                try
                {
                    stream!.Position = originalPosition;
                    restored = true;
                }
                catch
                {
                    chatText = "";
                }
            }
        }
        return parsed && restored;
    }

    public static bool ShouldSuppressRebroadcast(string? message)
    {
        if (string.IsNullOrEmpty(message))
            return false;

        var commandStart = 0;
        while (commandStart < message.Length && char.IsWhiteSpace(message[commandStart]))
            commandStart++;

        if (message.Length - commandStart < LoginCommand.Length
            || !message.AsSpan(commandStart, LoginCommand.Length)
                .Equals(LoginCommand.AsSpan(), StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }

        var commandEnd = commandStart + LoginCommand.Length;
        return commandEnd == message.Length || char.IsWhiteSpace(message[commandEnd]);
    }
}
