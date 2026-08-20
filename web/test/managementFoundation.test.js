import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const files = {
  page: new URL('../src/view/systemTools/system/system.vue', import.meta.url),
  api: new URL('../src/api/system.js', import.meta.url),
  serverRoutes: new URL('../../server/router/system/sys_system.go', import.meta.url),
  plugins: new URL('../../server/initialize/plugin_biz_v1.go', import.meta.url)
}

test('management foundation exposes only read-only secret status', async () => {
  const [page, api, serverRoutes, plugins] = await Promise.all(Object.values(files).map((file) => readFile(file, 'utf8')))

  for (const contract of ['环境变量托管', 'signing-key-configured', 'password-configured', 'token-configured', 'access-key-configured', 'secret-configured']) {
    assert.ok(page.includes(contract), `missing safe configuration contract: ${contract}`)
  }
  assert.doesNotMatch(page, /v-model[^\n]*(?:password|secret|signing-key|access-key)/i)
  assert.doesNotMatch(page, /emailTest|setSystemConfig|保存配置|提交配置/)
  assert.doesNotMatch(serverRoutes, /POST\("setSystemConfig"/)
  assert.doesNotMatch(plugins, /CreateEmailPlug|PluginInit/)
  assert.match(api, /url: '\/system\/getSystemConfig'/)
})
