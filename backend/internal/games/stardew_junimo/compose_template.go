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
    image: ${SERVER_IMAGE:-` + DefaultServerImage + `}
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
      SAP_CONTROL_DIR: /data/control
    volumes:
      - game-data:/data/game
      - ./.local-container/saves:/config/xdg/config/StardewValley
      - ./.local-container/settings:/data/settings
      - ./.local-container/control:/data/control
      - ./.local-container/cont-env/APP_NAME:/etc/cont-env.d/APP_NAME:ro
      - ./.local-container/cont-env/DBUS_SESSION_BUS_ADDRESS:/etc/cont-env.d/DBUS_SESSION_BUS_ADDRESS:ro
      - ./.local-container/cont-env/DOCKER_IMAGE_PLATFORM:/etc/cont-env.d/DOCKER_IMAGE_PLATFORM:ro
      - ./.local-container/cont-env/GTK_A11Y:/etc/cont-env.d/GTK_A11Y:ro
      - ./.local-container/cont-env/NO_AT_BRIDGE:/etc/cont-env.d/NO_AT_BRIDGE:ro
      - ./.local-container/cont-env/TAKE_CONFIG_OWNERSHIP:/etc/cont-env.d/TAKE_CONFIG_OWNERSHIP:ro
      - ./.local-container/cont-env/XDG_CACHE_HOME:/etc/cont-env.d/XDG_CACHE_HOME:ro
      - ./.local-container/cont-env/XDG_CONFIG_HOME:/etc/cont-env.d/XDG_CONFIG_HOME:ro
      - ./.local-container/cont-env/XDG_DATA_HOME:/etc/cont-env.d/XDG_DATA_HOME:ro
      - ./.local-container/cont-env/XDG_RUNTIME_DIR:/etc/cont-env.d/XDG_RUNTIME_DIR:ro
      - ./.local-container/cont-env/XDG_STATE_HOME:/etc/cont-env.d/XDG_STATE_HOME:ro
      - ./.local-container/cont-groups/cinit/id:/etc/cont-groups.d/cinit/id:ro
      - ./.local-container/cont-groups/nogroup/id:/etc/cont-groups.d/nogroup/id:ro
      - ./.local-container/cont-groups/root/id:/etc/cont-groups.d/root/id:ro
      - ./.local-container/cont-groups/shadow/id:/etc/cont-groups.d/shadow/id:ro
      - ./.local-container/cont-groups/staff/id:/etc/cont-groups.d/staff/id:ro
      - ./.local-container/cont-users/_apt/gid:/etc/cont-users.d/_apt/gid:ro
      - ./.local-container/cont-users/_apt/home:/etc/cont-users.d/_apt/home:ro
      - ./.local-container/cont-users/_apt/id:/etc/cont-users.d/_apt/id:ro
      - ./.local-container/cont-users/root/gid:/etc/cont-users.d/root/gid:ro
      - ./.local-container/cont-users/root/grps:/etc/cont-users.d/root/grps:ro
      - ./.local-container/cont-users/root/home:/etc/cont-users.d/root/home:ro
      - ./.local-container/cont-users/root/id:/etc/cont-users.d/root/id:ro
      - ./.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control
      - ./.local-container/mods:/data/Mods

volumes:
  steam-session:
  game-data:
`
