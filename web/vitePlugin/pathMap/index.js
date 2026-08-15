import path from 'node:path'

import { writePathMap } from './generator.js'

const watchedRoots = ['src/view', 'src/plugin']
const watchedEvents = ['add', 'change', 'unlink']

function isWatchedVueFile(rootDir, filename) {
  if (typeof filename !== 'string' || !filename.endsWith('.vue')) return false

  const absolute = path.resolve(filename)
  return watchedRoots.some((sourceRoot) => {
    const directory = path.resolve(rootDir, sourceRoot)
    const relative = path.relative(directory, absolute)
    return relative !== '' && !relative.startsWith('..') && !path.isAbsolute(relative)
  })
}

export function createPathMapPlugin({ generate = writePathMap, debounceMs = 50 } = {}) {
  let config
  let timer
  let closed = false

  const generateMap = () => generate({
    rootDir: path.resolve(config.root),
    outputFile: path.resolve(config.root, 'src/pathInfo.json')
  })

  return {
    name: 'moonbook-path-map',

    configResolved(resolvedConfig) {
      config = resolvedConfig
    },

    async buildStart() {
      try {
        await generateMap()
      } catch (error) {
        this.error(error)
      }
    },

    configureServer(server) {
      const schedule = (filename) => {
        if (closed || !isWatchedVueFile(config.root, filename)) return
        clearTimeout(timer)
        timer = setTimeout(async () => {
          timer = undefined
          try {
            await generateMap()
          } catch (error) {
            const message = error instanceof Error ? error.message : String(error)
            config.logger.error(`[moonbook-path-map] ${message}`)
            server.ws.send({
              type: 'error',
              err: { message: `[moonbook-path-map] ${message}`, stack: error?.stack }
            })
          }
        }, debounceMs)
      }

      for (const event of watchedEvents) server.watcher.on(event, schedule)

      server.httpServer?.once('close', () => {
        closed = true
        clearTimeout(timer)
        timer = undefined
        for (const event of watchedEvents) server.watcher.off(event, schedule)
      })
    }
  }
}

export default createPathMapPlugin()
