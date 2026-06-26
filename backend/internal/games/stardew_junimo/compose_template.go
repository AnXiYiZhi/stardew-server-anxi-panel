package stardew_junimo

// junimoComposeTemplate is the embedded docker-compose.yml written to new instances.
// It intentionally follows the JunimoServer official service names, volumes, and
// environment variable names because later lifecycle/console operations depend on
// those contracts (for example: `steam-auth`, `server`, and `attach-cli`).
const junimoComposeTemplate = `services:
  steam-auth:
    image: ${STEAM_SERVICE_IMAGE:-` + DefaultSteamServiceImage + `}
    stdin_open: true
    tty: true
    expose:
      - "${STEAM_AUTH_PORT:-3001}"
    environment:
      PORT: "${STEAM_AUTH_PORT:-3001}"
      GAME_DIR: /data/game
      SESSION_DIR: /data/steam-session
      STEAM_USERNAME: "${STEAM_USERNAME}"
      STEAM_PASSWORD: "${STEAM_PASSWORD}"
      STEAM_REFRESH_TOKEN: "${STEAM_REFRESH_TOKEN:-}"
      STEAM_KEEP_LANGUAGES: "${STEAM_KEEP_LANGUAGES:-}"
      STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS: "${STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS:-` + DefaultSteamClientConnectTimeoutSeconds + `}"
      STEAM_CLIENT_CONNECT_RETRIES: "${STEAM_CLIENT_CONNECT_RETRIES:-` + DefaultSteamClientConnectRetries + `}"
      STEAM_AUTH_SESSION_RETRIES: "${STEAM_AUTH_SESSION_RETRIES:-` + DefaultSteamAuthSessionRetries + `}"
      STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS: "${STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS:-` + DefaultSteamAuthSessionRetryDelaySeconds + `}"
    volumes:
      - steam-session:/data/steam-session
      - game-data:/data/game

  server:
    image: sdvd/server:${IMAGE_VERSION:-1.5.0-preview.121}
    stdin_open: true
    tty: true
    depends_on:
      steam-auth:
        condition: service_started
    ports:
      - "${GAME_PORT:-24642}:24642/udp"
      - "${QUERY_PORT:-27015}:27015/udp"
      - "${VNC_PORT:-5800}:5800/tcp"
      - "${API_PORT:-8080}:8080/tcp"
    cap_add:
      - SYS_TIME
    environment:
      STEAM_AUTH_URL: "http://steam-auth:${STEAM_AUTH_PORT:-3001}"
      VNC_PASSWORD: "${VNC_PASSWORD}"
      ALLOW_INSECURE_SETUP: "${ALLOW_INSECURE_SETUP:-false}"
      SERVER_TPS: "${SERVER_TPS:-60}"
      SERVER_FPS: "${SERVER_FPS:-0}"
      SETTINGS_PATH: /data/settings/server-settings.json
      API_ENABLED: "${API_ENABLED:-true}"
      API_PORT: "${API_PORT:-8080}"
      API_KEY: "${API_KEY:-}"
      SERVER_PASSWORD: "${SERVER_PASSWORD:-}"
      MAX_LOGIN_ATTEMPTS: "${MAX_LOGIN_ATTEMPTS:-3}"
      AUTH_TIMEOUT_SECONDS: "${AUTH_TIMEOUT_SECONDS:-120}"
    volumes:
      - game-data:/data/game
      - saves:/config/xdg/config/StardewValley
      - ./.local-container/settings:/data/settings

volumes:
  steam-session:
  game-data:
  saves:
  settings:
`
