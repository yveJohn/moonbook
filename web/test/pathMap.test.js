import assert from 'node:assert/strict'
import { mkdtemp, mkdir, readFile, stat, symlink, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

import {
  buildPathMap,
  componentNameFromSource,
  serializePathMap,
  writePathMap
} from '../vitePlugin/pathMap/generator.js'

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

async function fixture() {
  const rootDir = await mkdtemp(path.join(tmpdir(), 'moonbook-path-map-'))
  await mkdir(path.join(rootDir, 'src/view/books'), { recursive: true })
  await mkdir(path.join(rootDir, 'src/plugin/mail/view'), { recursive: true })
  return rootDir
}

async function vue(rootDir, relativePath, source = '<template><div /></template>') {
  const file = path.join(rootDir, relativePath)
  await mkdir(path.dirname(file), { recursive: true })
  await writeFile(file, source, 'utf8')
  return file
}

test('builds a sorted map from view and plugin Vue files', async () => {
  const rootDir = await fixture()
  await vue(rootDir, 'src/view/books/chapter-clean.vue')
  await vue(rootDir, 'src/view/books/scanUpload.vue')
  await vue(rootDir, 'src/plugin/mail/view/index.vue', `
    <script setup lang="ts">
    defineOptions({
      name: 'MailSettings'
    })
    </script>
    <template><div /></template>
  `)

  const result = await buildPathMap({ rootDir })

  assert.deepEqual(result, {
    '/src/plugin/mail/view/index.vue': 'MailSettings',
    '/src/view/books/chapter-clean.vue': 'ChapterClean',
    '/src/view/books/scanUpload.vue': 'ScanUpload'
  })
  assert.equal(serializePathMap(result), `${JSON.stringify(result, null, 2)}\n`)
})

test('extracts only a single static defineOptions name', () => {
  assert.equal(componentNameFromSource(`
    <script setup>
    defineOptions({ name: 'ReaderOrders' })
    </script>
  `, 'src/view/orders.vue'), 'ReaderOrders')

  for (const [source, message] of [
    [`<script setup>defineOptions({ name: dynamicName })</script>`, 'static string'],
    [`<script setup>defineOptions({ name: '' })</script>`, 'must not be empty'],
    [`<script setup>defineOptions({ name: 'One' }); defineOptions({ name: 'Two' })</script>`, 'multiple'],
    [`<script setup>const broken = </script>`, 'parse']
  ]) {
    assert.throws(
      () => componentNameFromSource(source, 'src/view/orders.vue'),
      (error) => error instanceof Error && error.message.includes('src/view/orders.vue') && error.message.toLowerCase().includes(message)
    )
  }
})

test('does not follow directory symlinks', async (t) => {
  const rootDir = await fixture()
  const outside = await mkdtemp(path.join(tmpdir(), 'moonbook-path-map-outside-'))
  await vue(outside, 'hidden.vue', `<script setup>defineOptions({ name: 'Hidden' })</script>`)
  try {
    await symlink(outside, path.join(rootDir, 'src/view/linked'), 'dir')
  } catch (error) {
    if (error?.code === 'EPERM') {
      t.skip('directory symlinks are not permitted in this environment')
      return
    }
    throw error
  }

  assert.deepEqual(await buildPathMap({ rootDir }), {})
})

test('writes atomically and leaves unchanged content untouched', async () => {
  const rootDir = await fixture()
  await vue(rootDir, 'src/view/index.vue', `<script setup>defineOptions({ name: 'Dashboard' })</script>`)
  const outputFile = path.join(rootDir, 'src/pathInfo.json')

  const first = await writePathMap({ rootDir, outputFile })
  const firstStat = await stat(outputFile)
  const second = await writePathMap({ rootDir, outputFile })
  const secondStat = await stat(outputFile)

  assert.deepEqual(first, { changed: true, count: 1 })
  assert.deepEqual(second, { changed: false, count: 1 })
  assert.equal(secondStat.mtimeMs, firstStat.mtimeMs)
  assert.equal(await readFile(outputFile, 'utf8'), '{\n  "/src/view/index.vue": "Dashboard"\n}\n')
})

test('removes deleted files on the next complete generation', async () => {
  const rootDir = await fixture()
  const outputFile = path.join(rootDir, 'src/pathInfo.json')
  const target = await vue(rootDir, 'src/view/temporary.vue')
  await writePathMap({ rootDir, outputFile })
  await import('node:fs/promises').then(({ unlink }) => unlink(target))
  await writePathMap({ rootDir, outputFile })

  assert.equal(await readFile(outputFile, 'utf8'), '{}\n')
})

test('reports an output failure without replacing an existing map', async () => {
  const rootDir = await fixture()
  await vue(rootDir, 'src/view/index.vue')
  const outputFile = path.join(rootDir, 'src/pathInfo.json')
  await writeFile(outputFile, '{"stable":true}\n', 'utf8')

  await assert.rejects(
    writePathMap({ rootDir, outputFile: path.join(outputFile, 'nested.json') }),
    /pathInfo\.json/
  )
  assert.equal(await readFile(outputFile, 'utf8'), '{"stable":true}\n')
})

test('matches every tracked management Vue component', async () => {
  const generated = await buildPathMap({ rootDir: webRoot })
  const committedContent = await readFile(path.join(webRoot, 'src/pathInfo.json'), 'utf8')
  const committed = JSON.parse(committedContent)

  assert.equal(Object.keys(generated).length, 133)
  assert.deepEqual(generated, committed)
  assert.equal(serializePathMap(generated), committedContent)
  assert.equal(generated['/src/view/novel/books/index.vue'], 'NovelBooks')
  assert.equal(generated['/src/view/reader/orders/index.vue'], 'ReaderOrders')
  assert.equal(generated['/src/view/dashboard/index.vue'], 'Dashboard')
})
