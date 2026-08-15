import { lstat, readFile, readdir } from 'node:fs/promises'
import path from 'node:path'

const iconRoots = ['src/assets/icons', 'src/plugin']

function pathLabel(filename) {
  return filename.replaceAll(path.sep, '/')
}

async function collectSvgFiles(directory, files) {
  let entries
  try {
    entries = await readdir(directory, { withFileTypes: true })
  } catch (error) {
    if (error?.code === 'ENOENT') return
    throw error
  }
  entries.sort((left, right) => left.name.localeCompare(right.name, 'en'))
  for (const entry of entries) {
    const filename = path.join(directory, entry.name)
    const metadata = await lstat(filename)
    if (metadata.isSymbolicLink()) continue
    if (metadata.isDirectory()) await collectSvgFiles(filename, files)
    else if (metadata.isFile() && filename.endsWith('.svg')) files.push(filename)
  }
}

function iconId(rootDir, filename) {
  const relative = pathLabel(path.relative(rootDir, filename))
  const basename = path.basename(filename, '.svg')
  const pluginMatch = relative.match(/^src\/plugin\/([^/]+)\//)
  return pluginMatch ? `${pluginMatch[1]}-${basename}` : basename
}

export function svgToSymbol(source, id, filename) {
  const opening = source.match(/<svg\b([^>]*)>/i)
  const closingIndex = source.toLowerCase().lastIndexOf('</svg>')
  if (!opening || closingIndex < opening.index + opening[0].length) {
    throw new Error(`svg sprite ${pathLabel(filename)}: invalid svg root`)
  }

  let attributes = opening[1]
  const width = attributes.match(/\bwidth\s*=\s*["']([^"']+)["']/i)?.[1]
  const height = attributes.match(/\bheight\s*=\s*["']([^"']+)["']/i)?.[1]
  attributes = attributes
    .replace(/\s+width\s*=\s*["'][^"']+["']/gi, '')
    .replace(/\s+height\s*=\s*["'][^"']+["']/gi, '')
    .replace(/\s+xmlns(?::xlink)?\s*=\s*["'][^"']+["']/gi, '')
    .trim()
  if (!/\bviewBox\s*=/i.test(attributes)) {
    if (!width || !height) throw new Error(`svg sprite ${pathLabel(filename)}: missing viewBox or dimensions`)
    attributes = `${attributes}${attributes ? ' ' : ''}viewBox="0 0 ${width} ${height}"`
  }

  const contentStart = opening.index + opening[0].length
  const content = source.slice(contentStart, closingIndex).trim()
  return `<symbol id="${id}"${attributes ? ` ${attributes}` : ''}>${content}</symbol>`
}

export async function buildSvgSprite({ rootDir }) {
  const root = path.resolve(rootDir)
  const files = []
  for (const iconRoot of iconRoots) await collectSvgFiles(path.join(root, iconRoot), files)
  files.sort((left, right) => pathLabel(left).localeCompare(pathLabel(right), 'en'))

  const symbols = []
  const ids = new Set()
  for (const filename of files) {
    const id = iconId(root, filename)
    if (ids.has(id)) throw new Error(`svg sprite ${pathLabel(filename)}: duplicate icon id ${id}`)
    ids.add(id)
    symbols.push(svgToSymbol(await readFile(filename, 'utf8'), id, path.relative(root, filename)))
  }
  return { count: symbols.length, content: symbols.join('\n') }
}
