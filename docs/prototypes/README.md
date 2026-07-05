# Prototype Artifact Index

Full prototype screenshots and extraction workspaces were moved out of the main repository on 2026-07-06 to reduce clone size and search noise. Keep this directory as a lightweight pointer only.

## Kept In Repo

- `overview-design-baseline-2026-06-30.png` - downscaled overview design baseline from the original image2 prototype pass.
- `overview-current-baseline-2026-07-04.png` - downscaled overview implementation baseline from the current-frontend-code capture pass.

## External Artifacts

- `stardew-page-prototypes-image2-2026-06-30` - full page prototype screenshots and extraction sources. Store as a Release artifact, object-storage folder, or design repository export.
- `stardew-current-frontend-code-image2-2026-07-04` - full current implementation screenshots used for visual comparison. Store beside the prototype artifact above.
- `assets/ui-extracted` - historical extraction workspace. Regenerate with `scripts/extract-ui-assets.py` only when a design task explicitly needs it.

Runtime code must not reference `docs/prototypes`. Production assets belong under `frontend/public/assets/...`.
