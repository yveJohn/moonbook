import { parse as parseJavaScript } from '@babel/parser'
import { parse as parseSfc } from '@vue/compiler-sfc'
import { randomUUID } from 'node:crypto'
import { lstat, readFile, readdir, rename, unlink, writeFile } from 'node:fs/promises'
import path from 'node:path'

const sourceRoots = ['src/view', 'src/plugin']

function pathLabel(filename) {
  return filename.replaceAll(path.sep, '/')
}

function pathMapError(filename, message, cause) {
  return new Error(`path map ${pathLabel(filename)}: ${message}`, cause ? { cause } : undefined)
}

function parserPlugins(lang = '') {
  const plugins = ['importAttributes', 'topLevelAwait']
  if (lang === 'ts' || lang === 'tsx') plugins.push('typescript')
  if (lang === 'jsx' || lang === 'tsx') plugins.push('jsx')
  return plugins
}

function scriptNames(block, filename) {
  if (!block?.content?.trim()) return []

  let ast
  try {
    ast = parseJavaScript(block.content, {
      sourceType: 'module',
      plugins: parserPlugins(block.lang)
    })
  } catch (error) {
    throw pathMapError(filename, 'failed to parse component script', error)
  }

  const names = []
  for (const statement of ast.program.body) {
    const expression = statement.type === 'ExpressionStatement' ? statement.expression : null
    if (
      expression?.type !== 'CallExpression' ||
      expression.callee?.type !== 'Identifier' ||
      expression.callee.name !== 'defineOptions'
    ) {
      continue
    }

    const options = expression.arguments[0]
    if (!options || options.type !== 'ObjectExpression') continue
    for (const property of options.properties) {
      if (property.type !== 'ObjectProperty' || property.computed) continue
      const key = property.key.type === 'Identifier' ? property.key.name : property.key.value
      if (key !== 'name') continue
      if (property.value.type !== 'StringLiteral') {
        throw pathMapError(filename, 'defineOptions.name must be a static string')
      }
      if (!property.value.value.trim()) {
        throw pathMapError(filename, 'defineOptions.name must not be empty')
      }
      names.push(property.value.value.trim())
    }
  }
  return names
}

function fallbackName(filename) {
  const basename = path.basename(filename, '.vue')
  return basename
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('')
}

export function componentNameFromSource(source, filename) {
  const parsed = parseSfc(source, { filename })
  if (parsed.errors.length) {
    throw pathMapError(filename, 'failed to parse Vue SFC', parsed.errors[0])
  }

  const names = [
    ...scriptNames(parsed.descriptor.script, filename),
    ...scriptNames(parsed.descriptor.scriptSetup, filename)
  ]
  if (names.length > 1) {
    throw pathMapError(filename, 'multiple defineOptions.name declarations are not allowed')
  }
  return names[0] ?? fallbackName(filename)
}

async function collectVueFiles(directory, files) {
  let entries
  try {
    entries = await readdir(directory, { withFileTypes: true })
  } catch (error) {
    throw pathMapError(directory, 'failed to read source directory', error)
  }

  entries.sort((left, right) => left.name.localeCompare(right.name, 'en'))
  for (const entry of entries) {
    const filename = path.join(directory, entry.name)
    const metadata = await lstat(filename)
    if (metadata.isSymbolicLink()) continue
    if (metadata.isDirectory()) {
      await collectVueFiles(filename, files)
    } else if (metadata.isFile() && filename.endsWith('.vue')) {
      files.push(filename)
    }
  }
}

export async function buildPathMap({ rootDir }) {
  const root = path.resolve(rootDir)
  const files = []
  for (const sourceRoot of sourceRoots) {
    await collectVueFiles(path.join(root, sourceRoot), files)
  }

  const entries = []
  for (const filename of files) {
    let source
    try {
      source = await readFile(filename, 'utf8')
    } catch (error) {
      throw pathMapError(filename, 'failed to read Vue component', error)
    }
    const relative = path.relative(root, filename)
    const key = `/${pathLabel(relative)}`
    entries.push([key, componentNameFromSource(source, relative)])
  }
  entries.sort(([left], [right]) => left.localeCompare(right, 'en'))
  return Object.fromEntries(entries)
}

export function serializePathMap(pathMap) {
  const sorted = Object.fromEntries(
    Object.entries(pathMap).sort(([left], [right]) => left.localeCompare(right, 'en'))
  )
  return `${JSON.stringify(sorted, null, 2)}\n`
}

export async function writePathMap({ rootDir, outputFile }) {
  const pathMap = await buildPathMap({ rootDir })
  const content = serializePathMap(pathMap)
  let current = null
  try {
    current = await readFile(outputFile, 'utf8')
  } catch (error) {
    if (error?.code !== 'ENOENT') {
      throw pathMapError(outputFile, 'failed to read existing pathInfo.json', error)
    }
  }
  if (current === content) return { changed: false, count: Object.keys(pathMap).length }

  const temporary = `${outputFile}.${process.pid}.${randomUUID()}.tmp`
  try {
    await writeFile(temporary, content, { encoding: 'utf8', flag: 'wx' })
    await rename(temporary, outputFile)
  } catch (error) {
    await unlink(temporary).catch(() => undefined)
    throw pathMapError(outputFile, 'failed to write pathInfo.json', error)
  }
  return { changed: true, count: Object.keys(pathMap).length }
}
