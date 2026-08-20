import { lstat, readFile, readdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const forbiddenPackages = [
  'vite-auto-import-svg',
  'vite-check-multiple-dom',
  'vite-vue-path-map',
  'vue3-sfc-loader',
  '@form-create/designer',
  '@form-create/element-ui'
]
const forbiddenLockEntries = [
  { fingerprint: 'forbidden-postcss-7-lock-entry', pattern: /(^|[\s/'"]+)postcss@7(?:\.|:|[\s'"]|$)/m },
  { fingerprint: 'forbidden-wangeditor-4-lock-entry', pattern: /(^|[\s/'"]+)wangeditor@4(?:\.|:|[\s'"]|$)/m }
]
const aiPreviewTarget = 'src/plugin/ai/view/picture/picture.vue'
const aiPreviewRequirements = [
  { fingerprint: 'missing-empty-iframe-sandbox', pattern: /sandbox\s*=\s*["']["']/ },
  { fingerprint: 'missing-preview-csp', pattern: /Content-Security-Policy/ },
  { fingerprint: 'missing-preview-sanitizer', pattern: /DOMPurify\.sanitize\s*\(/ }
]
const aiPreviewForbidden = [
  { fingerprint: 'forbidden-dynamic-sfc-loader', pattern: /\bloadModule\s*\(|vue3-sfc-loader/ },
  { fingerprint: 'forbidden-dynamic-vue-mount', pattern: /\bVue\.(?:createApp|h)\s*\(/ }
]
const sourceTargets = ['vite.config.js', 'vitePlugin', 'src']
const sourceExtensions = new Set(['.js', '.mjs', '.cjs', '.ts', '.tsx', '.vue', '.json'])
const base64Candidate = /[A-Za-z0-9+/]{32,}={0,2}/g

function relativeLabel(rootDir, filename) {
  return path.relative(rootDir, filename).replaceAll(path.sep, '/') || '.'
}

function decodedBase64(value) {
  if (value.length % 4 === 1) return null
  try {
    const decoded = Buffer.from(value, 'base64').toString('utf8')
    return decoded.includes('\uFFFD') ? null : decoded
  } catch {
    return null
  }
}

function contentFingerprints(content) {
  const findings = new Set()
  if (/gva-project-name|gva-secret/.test(content)) findings.add('hidden-global')
  if (/plugin\.gin-vue-admin\.com\/api\/shopImage\/view\?name=logo\.svg/i.test(content)) {
    findings.add('remote-control-url')
  }
  if (/项目未授权/.test(content)) {
    findings.add('unauthorized-page')
  }
  const hasDocumentOpen = /document\s*\.\s*open\s*\(/.test(content)
  const hasDocumentWrite = /document\s*\.\s*write\s*\(/.test(content)
  if (hasDocumentOpen && hasDocumentWrite) {
    findings.add('document-open')
    findings.add('document-write')
  }

  for (const candidate of content.match(base64Candidate) ?? []) {
    const decoded = decodedBase64(candidate)
    if (!decoded) continue
    if (/plugin\.gin-vue-admin\.com\/api\/shopImage\/view\?name=logo\.svg/i.test(decoded)) {
      findings.add('base64-remote-control-url')
    }
    if (/项目未授权/.test(decoded)) {
      findings.add('base64-unauthorized-page')
    }
  }
  return [...findings]
}

async function pathExists(filename) {
  try {
    return await lstat(filename)
  } catch (error) {
    if (error?.code === 'ENOENT') return null
    throw error
  }
}

async function collectFiles(target, extensions, files) {
  const metadata = await pathExists(target)
  if (!metadata || metadata.isSymbolicLink()) return
  if (metadata.isFile()) {
    if (!extensions || extensions.has(path.extname(target))) files.push(target)
    return
  }
  if (!metadata.isDirectory()) return

  const entries = await readdir(target, { withFileTypes: true })
  entries.sort((left, right) => left.name.localeCompare(right.name, 'en'))
  for (const entry of entries) {
    await collectFiles(path.join(target, entry.name), extensions, files)
  }
}

function addFinding(findings, rootDir, filename, fingerprint) {
  findings.push(`${relativeLabel(rootDir, filename)}: ${fingerprint}`)
}

export async function checkSupplyChain({ rootDir }) {
  const root = path.resolve(rootDir)
  const findings = []
  const packageFile = path.join(root, 'package.json')
  const lockFile = path.join(root, 'pnpm-lock.yaml')

  const packageJson = JSON.parse(await readFile(packageFile, 'utf8'))
  const lockContent = await readFile(lockFile, 'utf8')
  for (const packageName of forbiddenPackages) {
    for (const section of ['dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies']) {
      if (Object.hasOwn(packageJson[section] ?? {}, packageName)) {
        addFinding(findings, root, packageFile, `forbidden-package-${section}`)
      }
    }

    if (new RegExp(`(^|[\\s/'"]+)${packageName}(?:@|:|[\\s'"]|$)`, 'm').test(lockContent)) {
      addFinding(findings, root, lockFile, 'forbidden-package-lock-entry')
    }

    const installedPackage = path.join(root, 'node_modules', packageName)
    if (await pathExists(installedPackage)) {
      addFinding(findings, root, installedPackage, 'forbidden-installed-package')
    }
  }
  for (const { fingerprint, pattern } of forbiddenLockEntries) {
    if (pattern.test(lockContent)) addFinding(findings, root, lockFile, fingerprint)
  }

  const aiPreviewFile = path.join(root, aiPreviewTarget)
  const aiPreviewMetadata = await pathExists(aiPreviewFile)
  if (!aiPreviewMetadata?.isFile()) {
    addFinding(findings, root, aiPreviewFile, 'missing-secure-ai-preview')
  } else {
    const content = await readFile(aiPreviewFile, 'utf8')
    for (const { fingerprint, pattern } of aiPreviewRequirements) {
      if (!pattern.test(content)) addFinding(findings, root, aiPreviewFile, fingerprint)
    }
    for (const { fingerprint, pattern } of aiPreviewForbidden) {
      if (pattern.test(content)) addFinding(findings, root, aiPreviewFile, fingerprint)
    }
  }

  const sourceFiles = []
  for (const target of sourceTargets) {
    await collectFiles(path.join(root, target), sourceExtensions, sourceFiles)
  }
  for (const filename of sourceFiles) {
    const content = await readFile(filename, 'utf8')
    for (const fingerprint of contentFingerprints(content)) {
      addFinding(findings, root, filename, fingerprint)
    }
  }

  const distDir = path.join(root, 'dist')
  const distFiles = []
  await collectFiles(distDir, new Set(['.js', '.mjs', '.cjs']), distFiles)
  const nonEmptyDistFiles = []
  for (const filename of distFiles) {
    const content = await readFile(filename, 'utf8')
    if (content.trim()) nonEmptyDistFiles.push(filename)
    for (const fingerprint of contentFingerprints(content)) {
      addFinding(findings, root, filename, fingerprint)
    }
  }
  if (nonEmptyDistFiles.length === 0) {
    addFinding(findings, root, distDir, 'missing-nonempty-javascript')
  }

  if (findings.length) {
    throw new Error(`supply-chain check failed:\n${findings.sort().join('\n')}`)
  }
  return { sourceFiles: sourceFiles.length, distJavaScriptFiles: nonEmptyDistFiles.length }
}

const invokedFile = process.argv[1] ? path.resolve(process.argv[1]) : ''
if (invokedFile === fileURLToPath(import.meta.url)) {
  const rootDir = process.argv[2] ? path.resolve(process.argv[2]) : path.resolve(import.meta.dirname, '..')
  try {
    const result = await checkSupplyChain({ rootDir })
    console.log(`supply-chain check passed: source=${result.sourceFiles}, dist-js=${result.distJavaScriptFiles}`)
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error))
    process.exitCode = 1
  }
}
