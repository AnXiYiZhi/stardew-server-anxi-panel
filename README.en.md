# Stardew Anxi Panel

[中文](README.md)

`stardew-server-anxi-panel` is currently a Stardew Valley dedicated server web management panel built around [JunimoServer](https://stardew-valley-dedicated-server.github.io/server/).

The immediate goal is to let users run one Anxi Panel Docker image, open a browser, initialize an admin account, install the Stardew server, complete Steam authentication, choose a save, start the server, view the invite code, monitor status, manage saves and mods, send server commands, and manage panel users.

The long-term goal is a multi-game server panel: a global panel shows every game server instance, and selecting one game opens that game's dedicated management panel. Stardew + JunimoServer is the first game implementation. Minecraft, Don't Starve Together, Terraria, Palworld, and other games should be added as separate game modules and drivers later.

The first production-ready version should use **Single Game Mode** by default: after login, users go directly to the Stardew panel. The global game list stays hidden until a second game panel exists. Internally, the app should still use `instances + driver_id + GameDriver`.

> Current status: **Milestone 4: Jobs and State Machine complete**. Milestones 0, 1, 2, 3, and 4 are complete. The backend includes configuration loading, SQLite initialization, embedded migrations, unified JSON errors, setup/admin auth, login/session, admin/user roles, admin user management, a generic Docker / Docker Compose CLI control layer, persisted jobs/job_logs, Stardew single-instance state, and SSE job log streaming. The frontend supports setup, login, a basic dashboard, user management, Docker status checks, Stardew instance state, and a jobs center. Junimo installation, Steam Auth, server lifecycle, saves, mods, and console features are still planned but not implemented yet.

## GitHub Description

```text
A Stardew Valley dedicated server web panel powered by JunimoServer, Go, React, SQLite, and Docker Compose.
```

## What It Will Do

The intended user flow is:

1. Run the Anxi Panel Docker image.
2. The backend automatically prepares the JunimoServer working directory and config files.
3. Open the panel in a browser.
4. Create the first admin account.
5. Click **Install Game**.
6. Enter Steam username, Steam password, and VNC password.
7. The backend writes `.env`, directly pulls the JunimoServer-related container images, and runs Steam Auth.
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

Long-term product layers:

```text
Global Panel
  -> Game Instance List
  -> Game-specific Frontend Module
  -> GameDriver
  -> Game Server Containers
```

First implementation flow:

```text
React Frontend
  -> Go API
  -> jobs/state machine
  -> games/stardew_junimo driver
  -> Docker Compose / mounted files / attach-cli / Junimo HTTP status
  -> JunimoServer containers
```

The panel does not replace JunimoServer. It wraps JunimoServer's official Docker workflow in a safer, visible, browser-based management experience.

Current display mode:

```text
PANEL_MODE=single
/ -> /instances/stardew
```

Future multi-game mode:

```text
PANEL_MODE=multi
/ -> global game instance list
/instances/stardew -> Stardew panel
/instances/minecraft -> Minecraft panel
```

Future games should not be added as branches inside Stardew pages. They should be added as their own frontend game module and backend driver:

```text
frontend/src/games/stardew        + backend/internal/games/stardew_junimo
frontend/src/games/minecraft      + backend/internal/games/minecraft
frontend/src/games/dst            + backend/internal/games/dont_starve_together
frontend/src/games/terraria       + backend/internal/games/terraria
frontend/src/games/palworld       + backend/internal/games/palworld
```

## Repository Layout

```text
stardew-server-anxi-panel
├─ backend              Go API service
├─ frontend             React + TypeScript frontend
├─ docs
│  ├─ architecture.md   Architecture decisions
│  ├─ handoff-roadmap.md
│  └─ prototypes        Product prototype and notes
├─ LICENSE
├─ README.en.md
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
| `PANEL_MODE` | `single` | Product display mode. `single` goes directly to the default game panel; `multi` shows the global game list. |
| `DEFAULT_INSTANCE_ID` | `stardew` | Default instance used in Single Game Mode. |
| `DEFAULT_DRIVER_ID` | `stardew_junimo` | Driver used by the first default instance. |

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

## Jobs / State API

Jobs and instance state APIs require login. Creating test jobs is admin-only.

Implemented endpoints:

```text
GET  /api/jobs
GET  /api/jobs/:id
GET  /api/jobs/:id/logs?after=0&limit=200
GET  /api/jobs/:id/stream
POST /api/jobs/:id/cancel
POST /api/jobs/test
POST /api/jobs/test-fail
GET  /api/instances/stardew/state
```

Notes:

- `jobs`, `job_logs`, and `instance_state` are persisted in SQLite.
- Job statuses are `queued`, `running`, `succeeded`, `failed`, and `canceled`.
- `GET /api/jobs/:id/stream` uses SSE and sends a `finished` event when the job completes.
- `POST /api/jobs/test` creates a simulated successful job that writes logs for about 5 seconds.
- `POST /api/jobs/test-fail` creates a simulated failing job and saves the failure message.
- `POST /api/jobs/:id/cancel` currently returns 501 `not_implemented`.
- Ordinary users cannot create test jobs.

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

The current frontend includes setup, login, a basic dashboard, user management, Docker status checks, Stardew instance state, a jobs center, job detail, and live job logs.

## Current Milestone

Milestone 0 includes:

- Go backend skeleton
- React + TypeScript + Vite frontend skeleton
- Initial directory structure
- Basic `/health`
- Initial documentation

Milestone 1 includes:

- Go backend skeleton
- Environment-based backend configuration
- SQLite database creation and connection
- Minimal embedded migration runner
- Enhanced `/health` endpoint with version and database status
- Basic structured logging
- Unified JSON error responses

Milestone 2 includes:

- Auth SQLite migrations
- Setup/admin initialization
- Argon2id password hashing
- HttpOnly Cookie sessions
- Login, logout, and current user APIs
- admin/user roles
- admin-only user management

Milestone 3 includes:

- Generic Docker / Compose CLI control layer
- Structured command results
- Command timeout and output limits
- Sensitive output redaction
- admin-only Docker status APIs
- Docker status area in the frontend

Milestone 4 includes:

- `jobs`, `job_logs`, and `instance_state` migrations
- Generic Job Manager
- Simulated long-running jobs
- SSE job log stream
- Stardew single-instance state API
- Frontend jobs center

Not implemented yet:

- Junimo working directory preparation
- Steam Auth interaction
- Server start/stop/restart
- Invite code fetching
- Save management
- Mod management
- Console commands

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

The frontend should follow the same boundary: the global panel owns instance lists, login, users, jobs, and global status; Stardew-specific Steam Guard, invite code, and farm settings belong in the Stardew game module. Future Minecraft RCON, whitelist, OP, and world-management UI should belong in a Minecraft game module.

Milestones 0-4 do not need to be rewritten. Any temporary Stardew single-instance paths should be folded into `instances + driver_id + GameDriver registry` in Milestone 5. Milestone 8 should not force the global panel to be visible yet; it should implement Single Game Mode by routing login directly to the Stardew game module, then enable Multi Game Mode when a second game panel exists.

## License And Third-Party Notice

This project is released under the MIT License. See [LICENSE](LICENSE).

This project directly pulls and runs JunimoServer container images to provide Stardew Valley dedicated server functionality. JunimoServer is an independent third-party project. Its upstream repository is [stardew-valley-dedicated-server/server](https://github.com/stardew-valley-dedicated-server/server), and its upstream license is the [MIT License](https://github.com/stardew-valley-dedicated-server/server/blob/master/LICENSE). The JunimoServer container images, their bundled components, and their dependencies remain governed by the upstream project and their respective third-party licenses. This repository does not claim ownership of JunimoServer, Stardew Valley, Steam, or any related trademarks, game content, assets, or services.

Users are responsible for ensuring that they have the legal authorization required to run a Stardew Valley server and for complying with the licenses, terms of service, and usage rules of JunimoServer, Stardew Valley, Steam, and all related third-party components.
