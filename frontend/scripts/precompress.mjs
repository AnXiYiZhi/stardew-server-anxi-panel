import { readdir, readFile, writeFile } from 'node:fs/promises'
import { gzipSync } from 'node:zlib'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

async function compress(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) await compress(path)
    else if (/\.(?:js|css|html|json|svg)$/.test(entry.name)) {
      await writeFile(`${path}.gz`, gzipSync(await readFile(path), { level: 9 }))
    }
  }
}
await compress(fileURLToPath(new URL('../dist/', import.meta.url)))
