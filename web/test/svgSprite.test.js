import assert from 'node:assert/strict'
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { buildSvgSprite, svgToSymbol } from '../vitePlugin/svgSprite/generator.js'
import { createSvgSpritePlugin } from '../vitePlugin/svgSprite/index.js'

async function fixture() {
  const rootDir = await mkdtemp(path.join(tmpdir(), 'moonbook-svg-sprite-'))
  await mkdir(path.join(rootDir, 'src/assets/icons'), { recursive: true })
  await mkdir(path.join(rootDir, 'src/plugin/mail/assets/icons'), { recursive: true })
  return rootDir
}

test('builds stable system and plugin symbol identifiers', async () => {
  const rootDir = await fixture()
  await writeFile(path.join(rootDir, 'src/assets/icons/close.svg'), '<svg width="16" height="12"><path d="M0 0"/></svg>')
  await writeFile(path.join(rootDir, 'src/plugin/mail/assets/icons/send.svg'), '<svg viewBox="0 0 24 24"><path d="M1 1"/></svg>')

  assert.deepEqual(await buildSvgSprite({ rootDir }), {
    count: 2,
    content: '<symbol id="close" viewBox="0 0 16 12"><path d="M0 0"/></symbol>\n<symbol id="mail-send" viewBox="0 0 24 24"><path d="M1 1"/></symbol>'
  })
})

test('rejects malformed SVG and missing dimensions', () => {
  assert.throws(() => svgToSymbol('<path/>', 'bad', 'bad.svg'), /invalid svg root/)
  assert.throws(() => svgToSymbol('<svg><path/></svg>', 'bad', 'bad.svg'), /missing viewBox or dimensions/)
})

test('injects the generated sprite without bundle mutation hooks', async () => {
  const plugin = createSvgSpritePlugin({
    generate: async () => ({ count: 1, content: '<symbol id="close"></symbol>' })
  })
  plugin.configResolved({ root: '/workspace/web', logger: { error() {} } })
  await plugin.buildStart.call({ error: (error) => { throw error } })
  const html = await plugin.transformIndexHtml('<html><body><main></main></body></html>')

  assert.match(html, /<body><svg[^>]+><symbol id="close">/)
  assert.equal('renderChunk' in plugin, false)
  assert.equal('generateBundle' in plugin, false)
})
