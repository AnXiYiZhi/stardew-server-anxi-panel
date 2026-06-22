# Stardew Anxi Panel

`stardew-server-anxi-panel` is a Stardew Valley dedicated server web management panel built around [JunimoServer](https://stardew-valley-dedicated-server.github.io/server/).

The goal is to let users run one Docker image, open a browser, initialize an admin account, install the Stardew server, complete Steam authentication, choose a save, start the server, view the invite code, monitor status, manage saves and mods, send server commands, and manage panel users.

> Current status: **Milestone 1: Backend Foundation**. The backend now includes configuration loading, SQLite initialization with a minimal migration runner, enhanced health checks, basic structured logging, and unified JSON error responses. Docker control, user auth, Junimo installation, Steam Auth, saves, mods, and console features are planned but not implemented yet.

## GitHub Description

English:

```text
A Stardew Valley dedicated server web panel powered by JunimoServer, Go, React, SQLite, and Docker Compose.
```

中文：

```text
基于 JunimoServer 的星露谷物语专用服务器 Web 管理面板，使用 Go、React、SQLite 和 Docker Compose 构建。
```

## What It Will Do

The intended user flow is:

1. Run the Anxi Panel Docker image.
2. The backend automatically prepares the JunimoServer working directory and config files.
3. Open the panel in a browser.
4. Create the first admin account.
5. Click **Install Game**.
6. Enter Steam username, Steam password, and VNC password.
7. The backend writes `.env`, pulls Junimo images, and runs Steam Auth.
8. Steam Guard prompts are displayed in the frontend, while the backend completes the PTY interaction.
9. Click **Start Server** after installation.
10. Choose a save: upload, select existing, or create new.
11. The backend runs `docker compose up -d`.
12. The backend uses `attach-cli` to fetch the invite code and displays it in the panel.
13. Manage server status, commands, chat announcements, saves, mods, and panel users from the web UI.

## Architecture

Planned stack:

- Backend: Go
- Frontend: React + TypeScript + Vite
- Database: SQLite
- Runtime control: Docker Socket + Docker Compose V2
- Game integration: GameDriver-style abstraction
- First driver: Stardew Valley via JunimoServer

High-level flow:

```text
React Frontend
  -> Go API
  -> jobs/state machine
  -> games/stardew_junimo driver
  -> Docker Compose / mounted files / attach-cli / Junimo HTTP status
  -> JunimoServer containers
```

The panel does not replace JunimoServer. It wraps JunimoServer's official Docker workflow in a safer, visible, browser-based management experience.

## Repository Layout

```text
stardew-server-anxi-panel
├─ backend              Go API service
├─ frontend             React + TypeScript frontend
├─ docs
│  ├─ architecture.md   Architecture decisions
│  ├─ handoff-roadmap.md
│  └─ prototypes        Product prototype and notes
└─ README.md
```

## Backend Development

The backend lives in `backend/`.

```bash
cd backend
go test ./...
go run ./cmd/panel
```

Default listen address:

```text
:8090
```

Override with:

```bash
PANEL_ADDR=:8091 go run ./cmd/panel
```

Backend configuration:

| Variable | Default | Purpose |
| --- | --- | --- |
| `PANEL_ADDR` | `:8090` | HTTP listen address. |
| `PANEL_DATA_DIR` | `/data` | Panel data directory, created on startup. |
| `PANEL_DB_PATH` | `$PANEL_DATA_DIR/panel.db` | SQLite database path, created on startup. |
| `PANEL_SECRET` | empty | Reserved for future auth/session features. |
| `PANEL_VERSION` | `dev` | Version string returned by `/health`. |

Health check:

```text
GET /health
```

Example response:

```json
{
  "status": "ok",
  "service": "stardew-anxi-panel",
  "version": "dev",
  "database": {
    "status": "ok"
  }
}
```

## Frontend Development

The frontend lives in `frontend/`.

```bash
cd frontend
npm install
npm run dev
```

Common scripts:

```bash
npm run build
npm run preview
```

The current frontend is only a basic placeholder for Milestone 0.

## Current Milestone

Milestone 1 includes:

- Go backend skeleton from Milestone 0
- Environment-based backend configuration
- SQLite database creation and connection
- Minimal embedded migration runner
- Enhanced `/health` endpoint with version and database status
- Basic structured logging
- Unified JSON error responses
- React + TypeScript + Vite frontend skeleton
- Initial documentation

Milestone 1 does **not** include:

- Docker / Compose control logic
- Admin initialization and login
- SQLite migrations
- Junimo working directory preparation
- Steam Auth interaction
- Server start/stop/restart
- Invite code fetching
- Save management
- Mod management
- Console commands
- Panel user management

## Documentation

Read these before continuing development:

- [Architecture](docs/architecture.md)
- [Handoff Roadmap](docs/handoff-roadmap.md)
- [Prototype Notes](docs/prototypes/stardew-anxi-panel-prototype-notes.md)
- [Product Prototype HTML](docs/prototypes/stardew-anxi-panel-product-prototype.html)

## Design Direction

The planned UI uses a Stardew-inspired pixel farm style: wooden frames, parchment panels, bold borders, inventory-like navigation, and high-density server management information.

The prototype is located in:

```text
docs/prototypes/
```

## Important Boundary

All Stardew/Junimo-specific logic should live behind the `games/stardew_junimo` driver.

Do not place save, mod, or console behavior in top-level generic modules. The top-level backend should provide generic infrastructure only: auth, Docker command wrapper, jobs, storage, web API, and game driver registry.

## License

License has not been chosen yet.
