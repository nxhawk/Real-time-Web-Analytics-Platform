// Copies files that are authoritative elsewhere in the repository into docs/public/, so the
// site can serve them without a second copy being edited by hand.
//
// Plain Node with no dependencies, so it behaves the same on Windows, macOS and Linux.

import { copyFile, mkdir, access } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const docsRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')

/** @type {{from: string, to: string, optional?: boolean}[]} */
const assets = [
  // The API contract is the single source of truth and lives at docs/api/openapi.yaml.
  // It is served at /openapi.yaml so the reference page can link to it directly.
  { from: 'api/openapi.yaml', to: 'public/openapi.yaml', optional: true },
]

let copied = 0

for (const asset of assets) {
  const from = resolve(docsRoot, asset.from)
  const to = resolve(docsRoot, asset.to)

  try {
    await access(from)
  } catch {
    if (asset.optional) {
      console.log(`[copy-assets] skipped ${asset.from} (does not exist yet)`)
      continue
    }
    console.error(`[copy-assets] missing required asset: ${asset.from}`)
    process.exit(1)
  }

  await mkdir(dirname(to), { recursive: true })
  await copyFile(from, to)
  console.log(`[copy-assets] ${asset.from} -> ${asset.to}`)
  copied++
}

console.log(`[copy-assets] done, ${copied} file(s) copied`)
