import path from 'node:path'

import { buildSvgSprite } from './generator.js'

const iconDirectories = ['src/assets/icons', 'src/plugin']

function isIconFile(rootDir, filename) {
  if (typeof filename !== 'string' || !filename.endsWith('.svg')) return false
  return iconDirectories.some((directory) => {
    const relative = path.relative(path.resolve(rootDir, directory), path.resolve(filename))
    return relative !== '' && !relative.startsWith('..') && !path.isAbsolute(relative)
  })
}

export function createSvgSpritePlugin({ generate = buildSvgSprite } = {}) {
  let config
  let sprite = ''

  const refresh = async () => {
    const result = await generate({ rootDir: path.resolve(config.root) })
    sprite = result.content
    return result
  }

  return {
    name: 'moonbook-svg-sprite',

    configResolved(resolvedConfig) {
      config = resolvedConfig
    },

    async buildStart() {
      try {
        await refresh()
      } catch (error) {
        this.error(error)
      }
    },

    async transformIndexHtml(html) {
      if (!sprite) await refresh()
      const container = `<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" style="position:absolute;width:0;height:0;overflow:hidden">${sprite}</svg>`
      return html.replace(/<body([^>]*)>/i, `<body$1>${container}`)
    },

    async handleHotUpdate(context) {
      if (!isIconFile(config.root, context.file)) return
      try {
        await refresh()
        context.server.ws.send({ type: 'full-reload' })
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error)
        config.logger.error(`[moonbook-svg-sprite] ${message}`)
        context.server.ws.send({ type: 'error', err: { message, stack: error?.stack } })
      }
      return []
    }
  }
}

export default createSvgSpritePlugin()
