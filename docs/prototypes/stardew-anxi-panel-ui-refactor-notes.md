# Stardew Anxi Panel UI Refactor Prototype Notes

Date: 2026-06-27

## Purpose

This prototype supports the post-MVP UI / interaction refactor. It is not a replacement for the older product prototype; it is a more implementation-focused blueprint for the next frontend iteration.

Prototype file:

- `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html`

Related spec:

- `docs/frontend-ui-interaction-refactor.md`

Figma draft:

- `https://www.figma.com/design/GHadKWWdw2jWxgPXgY7fdM`

## Design Thesis

The MVP already proves the backend and frontend feature set. The next version should make the panel feel like a reliable operations console rather than a long debug page with a pixel skin.

The prototype therefore prioritizes:

- daily start/stop/copy invite code workflow,
- active save visibility before start,
- one primary action per state,
- right-side latest job and health visibility,
- clear admin-only troubleshooting and security surfaces,
- a tighter version of the existing farm visual identity.

## Screen States Represented

The HTML prototype shows:

1. **Overview / Running Dashboard**
   - left role-aware navigation,
   - top status bar,
   - primary server control area,
   - active save preflight,
   - invite code,
   - latest job and health rail.

2. **Install Wizard**
   - credentials/version step,
   - image pull,
   - Steam Guard decision,
   - game files ready.

3. **Saves and Mods Maintenance**
   - active save row,
   - table-like save list,
   - restart-required mod banner,
   - maintenance action grouping.

4. **Troubleshoot / Security**
   - diagnostics,
   - support bundle,
   - Docker/Compose state,
   - audit/user management separation.

## Visual Direction

The prototype keeps the Stardew-inspired identity without using official assets:

- parchment surfaces,
- wood rails,
- compact farm status cards,
- green/gold/sky semantic colors,
- small pixel accents,
- operational density.

Compared with the current CSS, the V2 direction reduces:

- huge login-era hero typography after dashboard entry,
- decorative shadows on every card,
- 14-24px border radii for routine controls,
- thick border treatment on every surface,
- unrelated per-section color worlds.

## Implementation Mapping

Suggested component mapping:

```text
AppShell
  LeftNav
  TopStatusBar
  RouteSurface
  OpsRail

games/stardew
  OverviewPage
  InstallWizard
  SavesPage
  ModsPage
  ConsolePage
  TroubleshootPage
```

Existing logic can be reused:

- `InstallSection` state handling moves into `InstallWizard`.
- `LifecycleSection` becomes part of `OverviewPage`.
- `SavesSection`, `ModsSection`, and `ConsoleSection` become route/page content.
- current advanced region splits into `TroubleshootPage` and `SecurityPage`.

## Known Prototype Limits

- The HTML file is static and does not call APIs.
- The layout intentionally shows idealized data.
- It does not cover every Steam auth edge case visually; the spec covers those states.
- Mobile behavior is indicated by responsive CSS but should be validated again during real implementation.

