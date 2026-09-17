// Reproducible derivatives; preserve artist source PNGs. Pass a sharp module
// path as argv[2] when using the bundled workspace dependency runtime.
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import { stat } from 'node:fs/promises'
const require = createRequire(import.meta.url)
const sharp = require(process.argv[2] || 'sharp')
const root = fileURLToPath(new URL('../public/assets/stardew/ui/', import.meta.url))
const assets = [
  ['buttons/button_wood_green', 543],
  ['buttons/button_wood_honey', 543],
  ['buttons/button_wood_red', 543],
  ['panels/right_card_health_9slice', 351],
  ['panels/right_card_progress_9slice', 564],
  ['panels/right_card_recent_9slice', 512],
  ['panels/right_rail_shell_middle_tile_seamless', 320],
  ['panels/right_rail_shell_top_line_image2', 512],
  ['panels/right_rail_shell_bottom', 512],
  ['panels/panel_side_rail_bottom_image2', 384],
  ['icons/icon_right_rail_health_heart_image2', 96],
  ['icons/icon_right_rail_in_progress_clock_image2', 96],
  ['icons/icon_right_rail_recent_tasks_clipboard_image2', 96],
  ['backgrounds/background_login_farm_generated', 1672],
]
let before = 0, after = 0
for (const [name, width] of assets) {
  const source = `${root}${name}.png`
  const target = `${root}${name}.optimized.webp`
  await sharp(source).resize({ width, kernel: 'nearest', withoutEnlargement: true }).webp({ lossless: true, effort: 6 }).toFile(target)
  const sourceBytes = (await stat(source)).size, targetBytes = (await stat(target)).size
  before += sourceBytes; after += targetBytes
  console.log(`${name}: ${sourceBytes} -> ${targetBytes}`)
}
console.log(`UI derivatives: ${before} -> ${after} bytes (${(100 - after / before * 100).toFixed(1)}% smaller)`)
