<template>
  <div v-loading="loading" class="gva-form-box config-page">
    <header class="page-head">
      <div>
        <h2>运行配置</h2>
        <el-tag type="info" effect="plain">环境变量托管</el-tag>
      </div>
      <el-tooltip content="重载服务" placement="bottom">
        <el-button :icon="Refresh" circle aria-label="重载服务" @click="reload" />
      </el-tooltip>
    </header>

    <section class="config-section">
      <h3>应用</h3>
      <el-descriptions :column="columns" border>
        <el-descriptions-item label="应用 ID">{{ value(config.app?.['app-id']) }}</el-descriptions-item>
        <el-descriptions-item label="环境">{{ value(config.app?.env) }}</el-descriptions-item>
        <el-descriptions-item label="节点">{{ value(config.app?.node) }}</el-descriptions-item>
        <el-descriptions-item label="监听地址">{{ value(config.system?.addr) }}</el-descriptions-item>
        <el-descriptions-item label="路由前缀">{{ value(config.system?.['router-prefix']) }}</el-descriptions-item>
        <el-descriptions-item label="数据库类型">{{ value(config.system?.['db-type']) }}</el-descriptions-item>
      </el-descriptions>
    </section>

    <section class="config-section">
      <h3>认证与指标</h3>
      <el-descriptions :column="columns" border>
        <el-descriptions-item label="JWT 签名密钥">
          <config-state :configured="config.jwt?.['signing-key-configured']" />
        </el-descriptions-item>
        <el-descriptions-item label="JWT 有效期">{{ value(config.jwt?.['expires-time']) }}</el-descriptions-item>
        <el-descriptions-item label="JWT 签发者">{{ value(config.jwt?.issuer) }}</el-descriptions-item>
        <el-descriptions-item label="严格角色模式">{{ enabled(config.system?.['use-strict-auth']) }}</el-descriptions-item>
        <el-descriptions-item label="指标采集">{{ enabled(config.metrics?.enabled) }}</el-descriptions-item>
        <el-descriptions-item label="指标访问令牌">
          <config-state :configured="config.metrics?.['token-configured']" />
        </el-descriptions-item>
      </el-descriptions>
    </section>

    <section class="config-section">
      <h3>PostgreSQL</h3>
      <el-descriptions :column="columns" border>
        <el-descriptions-item label="主机">{{ value(config.pgsql?.path) }}</el-descriptions-item>
        <el-descriptions-item label="端口">{{ value(config.pgsql?.port) }}</el-descriptions-item>
        <el-descriptions-item label="数据库">{{ value(config.pgsql?.['db-name']) }}</el-descriptions-item>
        <el-descriptions-item label="用户">{{ value(config.pgsql?.username) }}</el-descriptions-item>
        <el-descriptions-item label="密码">
          <config-state :configured="config.pgsql?.['password-configured']" />
        </el-descriptions-item>
        <el-descriptions-item label="最大连接数">{{ value(config.pgsql?.['max-open-conns']) }}</el-descriptions-item>
      </el-descriptions>
    </section>

    <section class="config-section">
      <h3>Redis 与 MinIO</h3>
      <el-descriptions :column="columns" border>
        <el-descriptions-item label="Redis 地址">{{ value(config.redis?.addr) }}</el-descriptions-item>
        <el-descriptions-item label="Redis DB">{{ value(config.redis?.db) }}</el-descriptions-item>
        <el-descriptions-item label="Redis 密码">
          <config-state :configured="config.redis?.['password-configured']" />
        </el-descriptions-item>
        <el-descriptions-item label="MinIO 地址">{{ value(config.minio?.endpoint) }}</el-descriptions-item>
        <el-descriptions-item label="MinIO Bucket">{{ value(config.minio?.['bucket-name']) }}</el-descriptions-item>
        <el-descriptions-item label="MinIO Access Key">
          <config-state :configured="config.minio?.['access-key-configured']" />
        </el-descriptions-item>
        <el-descriptions-item label="MinIO Secret Key">
          <config-state :configured="config.minio?.['secret-configured']" />
        </el-descriptions-item>
      </el-descriptions>
    </section>

    <section class="config-section">
      <h3>网络与日志</h3>
      <el-descriptions :column="columns" border>
        <el-descriptions-item label="CORS 模式">{{ value(config.cors?.mode) }}</el-descriptions-item>
        <el-descriptions-item label="日志级别">{{ value(config.zap?.level) }}</el-descriptions-item>
        <el-descriptions-item label="日志格式">{{ value(config.zap?.format) }}</el-descriptions-item>
        <el-descriptions-item label="控制台日志">{{ enabled(config.zap?.['log-in-console']) }}</el-descriptions-item>
      </el-descriptions>
    </section>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { useWindowSize } from '@vueuse/core'
import { ElMessage, ElMessageBox, ElTag } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getSystemConfig, reloadSystem } from '@/api/system'

defineOptions({ name: 'Config' })

const ConfigState = defineComponent({
  props: { configured: Boolean },
  setup: (props) => () => h(ElTag, { type: props.configured ? 'success' : 'info', effect: 'plain' }, () => props.configured ? '已配置' : '未配置')
})

const loading = ref(false)
const config = ref({})
const { width } = useWindowSize()
const columns = computed(() => width.value <= 720 ? 1 : 2)

const value = (input) => input === undefined || input === null || input === '' ? '-' : String(input)
const enabled = (input) => input ? '已启用' : '未启用'

const load = async () => {
  loading.value = true
  try {
    const response = await getSystemConfig()
    if (response.code === 0) config.value = response.data?.config || {}
  } finally {
    loading.value = false
  }
}

const reload = async () => {
  try {
    await ElMessageBox.confirm('确认重载服务配置？', '重载服务', {
      confirmButtonText: '确认',
      cancelButtonText: '取消',
      type: 'warning'
    })
    const response = await reloadSystem()
    if (response.code === 0) {
      ElMessage.success('重载请求已提交')
      await load()
    }
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

onMounted(load)
</script>

<style scoped>
.config-page { padding: 20px; }
.page-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 22px; }
.page-head > div { display: flex; align-items: center; gap: 12px; min-width: 0; }
.page-head h2 { margin: 0; font-size: 20px; letter-spacing: 0; }
.config-section { padding: 18px 0 20px; border-top: 1px solid var(--el-border-color-lighter); }
.config-section:first-of-type { border-top: 0; }
.config-section h3 { margin: 0 0 12px; font-size: 15px; font-weight: 600; letter-spacing: 0; }
:deep(.el-descriptions__label) { width: 150px; }
:deep(.el-descriptions__content) { overflow-wrap: anywhere; }
@media (max-width: 720px) {
  .config-page { padding: 12px; }
  .page-head { margin-bottom: 14px; }
  .page-head > div { align-items: flex-start; flex-direction: column; gap: 8px; }
  :deep(.el-descriptions__label) { width: 118px; }
}
</style>
