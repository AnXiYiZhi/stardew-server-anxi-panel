# Stardew new-game assets

This folder is reserved for the small, redistributable-at-build-time image crops used by the new-game screen.

The expected files include farm, pet, gender, character-preview, and cabin-layout PNGs. The current character, gender, and cabin files are direct crops supplied by the maintainer; farm and pet files were exported once from the local Stardew runtime. Commit every image used by the UI here. The running panel must never wait for a player's Steam download or start a temporary game container to generate these assets.

Keep the original game assets scoped to this panel image and do not publish them as a standalone asset pack.
