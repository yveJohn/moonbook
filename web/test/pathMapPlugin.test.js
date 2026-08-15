import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import path from 'node:path'
import test from 'node:test'

import { createPathMapPlugin } from '../vitePlugin/pathMap/index.js'

function wait(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds))
}

test('generates the map before Vite loads application modules', async () => {
  const calls = []
  const plugin = createPathMapPlugin({
    generate: async (options) => {
      calls.push(options)
      return { changed: true, count: 3 }
    }
  })
  plugin.configResolved({ root: '/workspace/web', logger: { info() {}, error() {} } })
  await plugin.buildStart.call({ error: (error) => { throw error } })

  assert.deepEqual(calls, [{
    rootDir: path.resolve('/workspace/web'),
    outputFile: path.resolve('/workspace/web/src/pathInfo.json')
  }])
  assert.equal('generateBundle' in plugin, false)
})

test('debounces approved Vue file events and ignores unrelated files', async () => {
  const watcher = new EventEmitter()
  const httpServer = new EventEmitter()
  const calls = []
  const plugin = createPathMapPlugin({
    debounceMs: 5,
    generate: async () => {
      calls.push('generated')
      return { changed: true, count: 1 }
    }
  })
  plugin.configResolved({ root: '/workspace/web', logger: { info() {}, error() {} } })
  plugin.configureServer({ watcher, httpServer, ws: { send() {} } })

  watcher.emit('change', '/workspace/web/src/api/menu.js')
  watcher.emit('add', '/workspace/web/src/view/books/index.vue')
  watcher.emit('change', '/workspace/web/src/view/books/index.vue')
  watcher.emit('unlink', '/workspace/web/src/plugin/mail/view/index.vue')
  await wait(20)

  assert.deepEqual(calls, ['generated'])
  httpServer.emit('close')
  watcher.emit('add', '/workspace/web/src/view/after-close.vue')
  await wait(20)
  assert.deepEqual(calls, ['generated'])
})

test('reports development generation failures through logger and error overlay', async () => {
  const watcher = new EventEmitter()
  const httpServer = new EventEmitter()
  const errors = []
  const overlays = []
  const plugin = createPathMapPlugin({
    debounceMs: 1,
    generate: async () => {
      throw new Error('path map failed')
    }
  })
  plugin.configResolved({
    root: '/workspace/web',
    logger: { info() {}, error: (message) => errors.push(message) }
  })
  plugin.configureServer({ watcher, httpServer, ws: { send: (message) => overlays.push(message) } })

  watcher.emit('change', '/workspace/web/src/view/books/index.vue')
  await wait(20)

  assert.equal(errors.length, 1)
  assert.match(errors[0], /path map failed/)
  assert.equal(overlays.length, 1)
  assert.equal(overlays[0].type, 'error')
  assert.match(overlays[0].err.message, /path map failed/)
})

test('fails a production build when generation fails', async () => {
  const plugin = createPathMapPlugin({
    generate: async () => {
      throw new Error('cannot write path map')
    }
  })
  plugin.configResolved({ root: '/workspace/web', logger: { info() {}, error() {} } })

  await assert.rejects(
    plugin.buildStart.call({ error: (error) => { throw error } }),
    /cannot write path map/
  )
})
