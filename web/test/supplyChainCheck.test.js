import assert from 'node:assert/strict'
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { checkSupplyChain } from '../scripts/check-supply-chain.mjs'

async function fixture() {
  const rootDir = await mkdtemp(path.join(tmpdir(), 'moonbook-supply-chain-'))
  await mkdir(path.join(rootDir, 'src'), { recursive: true })
  await mkdir(path.join(rootDir, 'vitePlugin'), { recursive: true })
  await mkdir(path.join(rootDir, 'dist/assets'), { recursive: true })
  await writeFile(path.join(rootDir, 'package.json'), JSON.stringify({ dependencies: { vue: '3.5.0' } }))
  await writeFile(path.join(rootDir, 'pnpm-lock.yaml'), "lockfileVersion: '9.0'\n")
  await writeFile(path.join(rootDir, 'vite.config.js'), 'export default {}\n')
  await writeFile(path.join(rootDir, 'dist/assets/index.js'), 'console.log("trusted")\n')
  return rootDir
}

test('accepts trusted declarations, sources and a nonempty production bundle', async () => {
  const rootDir = await fixture()
  assert.deepEqual(await checkSupplyChain({ rootDir }), {
    sourceFiles: 1,
    distJavaScriptFiles: 1
  })
})

test('rejects the malicious dependency in package declarations and the lockfile', async () => {
  const rootDir = await fixture()
  const packageName = ['vite', 'vue', 'path', 'map'].join('-')
  await writeFile(path.join(rootDir, 'package.json'), JSON.stringify({ dependencies: { [packageName]: '1.0.2' } }))
  await writeFile(path.join(rootDir, 'pnpm-lock.yaml'), `packages:\n  ${packageName}@1.0.2: {}\n`)

  await assert.rejects(checkSupplyChain({ rootDir }), (error) => {
    assert.match(error.message, /forbidden-package-dependencies/)
    assert.match(error.message, /forbidden-package-lock-entry/)
    return true
  })
})

test('rejects hidden globals and plain or Base64 remote control fingerprints', async () => {
  const rootDir = await fixture()
  const remote = 'https://plugin.gin-vue-admin.com/api/shopImage/view?name=logo.svg'
  await writeFile(path.join(rootDir, 'vitePlugin/injected.js'), [
    `global[${JSON.stringify('gva-secret')}] = ''`,
    JSON.stringify(remote),
    JSON.stringify(Buffer.from(remote).toString('base64'))
  ].join('\n'))

  await assert.rejects(checkSupplyChain({ rootDir }), (error) => {
    assert.match(error.message, /hidden-global/)
    assert.match(error.message, /remote-control-url/)
    assert.match(error.message, /base64-remote-control-url/)
    return true
  })
})

test('rejects plain and Base64 unauthorized replacement pages', async () => {
  const rootDir = await fixture()
  const page = '<title>项目未授权</title><a href="https://plugin.gin-vue-admin.com/license">license</a>'
  await writeFile(path.join(rootDir, 'dist/assets/index.js'), [
    JSON.stringify(page),
    JSON.stringify(Buffer.from(page).toString('base64'))
  ].join('\n'))

  await assert.rejects(checkSupplyChain({ rootDir }), (error) => {
    assert.match(error.message, /unauthorized-page/)
    assert.match(error.message, /base64-unauthorized-page/)
    return true
  })
})

test('rejects document replacement primitives in production chunks', async () => {
  const rootDir = await fixture()
  await writeFile(path.join(rootDir, 'dist/assets/index.js'), 'document.open(); document.write("blocked");\n')

  await assert.rejects(checkSupplyChain({ rootDir }), (error) => {
    assert.match(error.message, /document-open/)
    assert.match(error.message, /document-write/)
    return true
  })
})

test('rejects missing, empty or non-JavaScript production output', async () => {
  for (const setup of [
    async () => {},
    async (rootDir) => writeFile(path.join(rootDir, 'dist/assets/index.js'), ''),
    async (rootDir) => writeFile(path.join(rootDir, 'dist/assets/index.js'), '   \n')
  ]) {
    const rootDir = await fixture()
    await writeFile(path.join(rootDir, 'dist/assets/index.js'), '')
    await setup(rootDir)
    await assert.rejects(checkSupplyChain({ rootDir }), /missing-nonempty-javascript/)
  }
})
