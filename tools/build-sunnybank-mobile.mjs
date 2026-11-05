// Read the prototype's generated JSON payload as data; never execute its JavaScript.
import { readFile, writeFile, mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { resolve, dirname } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const input = await readFile(resolve(root, 'prototype/sunnybank-data.js'), 'utf8')
const match = input.match(/(?:^|\n)window\.SUNNYBANK\s*=\s*(\{[\s\S]*\})\s*;?\s*$/)
if (!match) throw new Error('Could not find window.SUNNYBANK JSON assignment')
const data = JSON.parse(match[1])
if (!Array.isArray(data.origin) || !Array.isArray(data.addresses) || !Array.isArray(data.parcels)) throw new Error('Unexpected Sunnybank schema')
const target = resolve(root, 'mobile/src/data/sunnybank.json')
await mkdir(dirname(target), { recursive: true })
await writeFile(target, JSON.stringify(data) + '\n')
console.log(`Wrote ${data.addresses.length} addresses and ${data.parcels.length} parcels to ${target}`)
