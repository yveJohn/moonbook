<template>
  <div class="gva-form-box payment-channels-page">
    <div class="page-head">
      <h2>支付渠道</h2>
      <div class="head-actions">
        <span class="archive-filter">显示已归档</span>
        <el-switch v-model="includeArchived" @change="load" />
        <el-tooltip content="刷新" placement="top"><el-button :icon="Refresh" circle @click="load" /></el-tooltip>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建渠道</el-button>
      </div>
    </div>

    <el-table v-table-display v-loading="loading" :data="rows" border>
      <el-table-column label="渠道" min-width="180">
        <template #default="{ row }"><strong>{{ row.displayName }}</strong><code>{{ row.id }}</code></template>
      </el-table-column>
      <el-table-column prop="provider" label="提供商" width="120" />
      <el-table-column label="资产" width="160"><template #default="{ row }">{{ row.token.toUpperCase() }} / {{ row.network.toUpperCase() }}</template></el-table-column>
      <el-table-column label="配置" width="110"><template #default="{ row }"><el-tag :type="row.configured ? 'success' : 'warning'">{{ row.configured ? '完整' : '未完成' }}</el-tag></template></el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag v-if="row.archivedAt" type="info">已归档</el-tag>
          <el-switch v-else v-model="row.enabled" :disabled="!row.configured" @change="toggle(row)" />
        </template>
      </el-table-column>
      <el-table-column prop="updatedAt" label="更新时间" min-width="185"  :formatter="adminTimeColumn" />
      <el-table-column label="连通性" min-width="170">
        <template #default="{ row }"><el-tag v-if="row.checkStatus" :type="checkType(row.checkStatus)">{{ row.checkMessage }}</el-tag><span v-else>-</span></template>
      </el-table-column>
      <el-table-column label="操作" width="152" fixed="right">
        <template #default="{ row }">
          <el-tooltip content="编辑" placement="top"><el-button :icon="Edit" circle :disabled="Boolean(row.archivedAt)" @click="openEdit(row)" /></el-tooltip>
          <el-tooltip content="检查连通性" placement="top"><el-button :icon="Connection" circle :loading="row.checking" :disabled="Boolean(row.archivedAt)" @click="check(row)" /></el-tooltip>
          <el-tooltip content="归档" placement="top"><el-button :icon="Delete" circle type="danger" plain :disabled="Boolean(row.archivedAt)" @click="archive(row)" /></el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑支付渠道' : '新建支付渠道'" width="720px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <div class="form-grid">
          <el-form-item label="渠道名称" prop="displayName"><el-input v-model="form.displayName" maxlength="100" /></el-form-item>
          <el-form-item label="提供商"><el-select v-model="form.provider" disabled><el-option label="EPUSDT" value="epusdt" /></el-select></el-form-item>
          <el-form-item label="币种"><el-input v-model="form.currency" disabled /></el-form-item>
          <el-form-item label="资产 / 网络"><el-input :model-value="'USDT / TRON'" disabled /></el-form-item>
        </div>

        <div class="section-title">商户凭据</div>
        <div class="form-grid">
          <el-form-item label="商户 PID" prop="merchantPid"><el-input v-model="form.merchantPid" type="password" show-password autocomplete="new-password" :placeholder="editingId && currentPIDConfigured ? '留空保持不变' : ''" /></el-form-item>
          <el-form-item label="Secret" prop="secret"><el-input v-model="form.secret" type="password" show-password autocomplete="new-password" :placeholder="editingId && currentSecretConfigured ? '留空保持不变' : ''" /></el-form-item>
        </div>

        <div class="section-title">接口地址</div>
        <div class="form-grid">
          <el-form-item label="EPUSDT 基础地址" prop="epusdtBaseUrl"><el-input v-model="form.epusdtBaseUrl" placeholder="https://pay.example.com" /></el-form-item>
          <el-form-item label="读者端基础地址" prop="readerBaseUrl"><el-input v-model="form.readerBaseUrl" placeholder="https://reader.example.com" /></el-form-item>
        </div>
        <div class="endpoint-list">
          <el-form-item v-for="endpoint in endpointPreview" :key="endpoint.key" :label="endpoint.label">
            <el-input :model-value="endpoint.value" readonly>
              <template #append>
                <el-tooltip content="复制" placement="top"><el-button :icon="CopyDocument" :disabled="!endpoint.value" @click="copyEndpoint(endpoint.value)" /></el-tooltip>
              </template>
            </el-input>
          </el-form-item>
        </div>

        <div class="section-title">超时设置</div>
        <div class="form-grid three-columns">
          <el-form-item label="连接超时（毫秒）" prop="connectTimeoutMs"><el-input v-model="form.connectTimeoutMs" inputmode="numeric" /></el-form-item>
          <el-form-item label="请求超时（毫秒）" prop="requestTimeoutMs"><el-input v-model="form.requestTimeoutMs" inputmode="numeric" /></el-form-item>
          <el-form-item label="未知订单释放（分钟）" prop="unknownReleaseMinutes"><el-input v-model="form.unknownReleaseMinutes" inputmode="numeric" /></el-form-item>
        </div>
        <el-form-item label="启用渠道"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { adminTimeColumn } from '@/utils/adminDisplay'
import { computed, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, CopyDocument, Delete, Edit, Plus, Refresh } from '@element-plus/icons-vue'
import { archivePaymentChannel, checkPaymentChannel, createPaymentChannel, listPaymentChannels, updatePaymentChannel } from '@/api/reader/paymentChannels'

defineOptions({ name: 'ReaderPaymentChannels' })

const rows = ref([])
const loading = ref(false)
const includeArchived = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref('')
const currentPIDConfigured = ref(false)
const currentSecretConfigured = ref(false)
const formRef = ref()

const defaults = () => ({
  displayName: 'EPUSDT', provider: 'epusdt', enabled: false, currency: 'usd', token: 'usdt', network: 'tron',
  merchantPid: '', secret: '', epusdtBaseUrl: '', readerBaseUrl: '',
  connectTimeoutMs: '3000', requestTimeoutMs: '10000', unknownReleaseMinutes: '15'
})
const form = reactive(defaults())
const normalizeBaseUrl = (value) => {
  const parsed = new URL(value.trim())
  if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) throw new Error()
  return parsed.origin
}
const baseUrlRule = (_, value, callback) => {
  try { normalizeBaseUrl(value); callback() } catch { callback(new Error('请输入仅包含协议、域名或 IP、端口的 HTTP/HTTPS 地址')) }
}
const integerRule = (minimum, maximum) => (_, value, callback) => {
  if (!/^\d+$/.test(value) || Number(value) < minimum || Number(value) > maximum) callback(new Error(`请输入 ${minimum} 至 ${maximum} 的整数`))
  else callback()
}
const credentialRule = (field) => (_, value, callback) => {
  if (!editingId.value && !value.trim()) callback(new Error(`${field}不能为空`))
  else callback()
}
const rules = {
  displayName: [{ required: true, message: '渠道名称不能为空', trigger: 'blur' }],
  merchantPid: [{ validator: credentialRule('商户 PID'), trigger: 'blur' }],
  secret: [{ validator: credentialRule('Secret'), trigger: 'blur' }],
  epusdtBaseUrl: [{ validator: baseUrlRule, trigger: 'blur' }], readerBaseUrl: [{ validator: baseUrlRule, trigger: 'blur' }],
  connectTimeoutMs: [{ validator: integerRule(100, 30000), trigger: 'blur' }],
  requestTimeoutMs: [{ validator: integerRule(100, 120000), trigger: 'blur' }],
  unknownReleaseMinutes: [{ validator: integerRule(1, 1440), trigger: 'blur' }]
}
const endpointPreview = computed(() => {
  let epusdt = ''
  let reader = ''
  try { epusdt = normalizeBaseUrl(form.epusdtBaseUrl) } catch { /* validation displays the error */ }
  try { reader = normalizeBaseUrl(form.readerBaseUrl) } catch { /* validation displays the error */ }
  return [
    { key: 'create', label: '创建订单地址', value: epusdt ? `${epusdt}/payments/gmpay/v1/order/create-transaction` : '' },
    { key: 'notify', label: '回调地址', value: reader ? `${reader}/prod-api/reader/payment/epusdt/notify` : '' },
    { key: 'redirect', label: '跳转地址', value: reader ? `${reader}/me/recharge` : '' },
    { key: 'health', label: '健康检查地址', value: epusdt ? `${epusdt}/` : '' },
    { key: 'sync', label: '订单同步地址', value: epusdt ? `${epusdt}/pay/check-status/{trade_id}` : '' }
  ]
})

const load = async () => {
  loading.value = true
  try { const response = await listPaymentChannels(includeArchived.value); rows.value = response.data || [] } finally { loading.value = false }
}
const resetForm = () => Object.assign(form, defaults())
const openCreate = () => {
  editingId.value = ''; currentPIDConfigured.value = false; currentSecretConfigured.value = false; resetForm(); dialogVisible.value = true
}
const openEdit = (row) => {
  editingId.value = row.id; currentPIDConfigured.value = row.pidConfigured; currentSecretConfigured.value = row.secretConfigured
  Object.assign(form, defaults(), row, { merchantPid: '', secret: '', connectTimeoutMs: String(row.connectTimeoutMs), requestTimeoutMs: String(row.requestTimeoutMs), unknownReleaseMinutes: String(row.unknownReleaseMinutes) })
  dialogVisible.value = true
}
const payload = () => ({
  displayName: form.displayName.trim(), provider: form.provider, enabled: form.enabled, currency: form.currency, token: form.token, network: form.network,
  merchantPid: form.merchantPid.trim() || undefined, secret: form.secret || undefined,
  epusdtBaseUrl: form.epusdtBaseUrl.trim(), readerBaseUrl: form.readerBaseUrl.trim(),
  connectTimeoutMs: Number(form.connectTimeoutMs), requestTimeoutMs: Number(form.requestTimeoutMs), unknownReleaseMinutes: Number(form.unknownReleaseMinutes)
})
const copyEndpoint = async (value) => {
  try { await navigator.clipboard.writeText(value); ElMessage.success('地址已复制') }
  catch { ElMessage.error('复制失败') }
}
const save = async () => {
  await formRef.value.validate()
  if (Number(form.requestTimeoutMs) < Number(form.connectTimeoutMs)) return ElMessage.warning('请求超时不能小于连接超时')
  if (editingId.value && form.merchantPid.trim() && !form.secret) return ElMessage.warning('修改商户 PID 时必须同时填写 Secret')
  saving.value = true
  try {
    if (editingId.value) await updatePaymentChannel(editingId.value, payload())
    else await createPaymentChannel(payload())
    ElMessage.success(editingId.value ? '更新成功' : '新增成功'); dialogVisible.value = false; await load()
  } finally { saving.value = false }
}
const toggle = async (row) => {
  const data = {
    displayName: row.displayName, provider: row.provider, enabled: row.enabled, currency: row.currency, token: row.token, network: row.network,
    epusdtBaseUrl: row.epusdtBaseUrl, readerBaseUrl: row.readerBaseUrl,
    connectTimeoutMs: row.connectTimeoutMs, requestTimeoutMs: row.requestTimeoutMs, unknownReleaseMinutes: row.unknownReleaseMinutes
  }
  try { await updatePaymentChannel(row.id, data); ElMessage.success('状态已更新') }
  catch { row.enabled = !row.enabled }
}
const checkType = (status) => ({ reachable: 'success', disabled: 'info', archived: 'info', not_configured: 'warning', unreachable: 'danger' }[status] || 'info')
const check = async (row) => {
  row.checking = true
  try { const response = await checkPaymentChannel(row.id); const value = response.data || {}; row.checkStatus = value.status; row.checkMessage = value.message; ElMessage.info(value.message || '检查完成') }
  finally { row.checking = false }
}
const archive = async (row) => {
  await ElMessageBox.confirm(`确认归档支付渠道“${row.displayName}”？`, '归档确认', { type: 'warning', confirmButtonText: '归档' })
  await archivePaymentChannel(row.id); ElMessage.success('归档成功'); await load()
}

load()
</script>

<style scoped>
.payment-channels-page { padding: 20px; }
.page-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.page-head h2 { margin: 0; font-size: 20px; letter-spacing: 0; }
.head-actions { display: flex; align-items: center; gap: 10px; }
.archive-filter { color: var(--el-text-color-secondary); font-size: 14px; }
code { display: block; margin-top: 4px; color: var(--el-text-color-secondary); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.section-title { margin: 6px 0 14px; padding-bottom: 8px; border-bottom: 1px solid var(--el-border-color-lighter); color: var(--el-text-color-primary); font-size: 14px; font-weight: 600; }
.endpoint-list { margin-bottom: 18px; }
.endpoint-list :deep(.el-input-group__append) { padding: 0; }
.endpoint-list :deep(.el-input-group__append .el-button) { height: 30px; margin: 0; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 18px; }
.three-columns { grid-template-columns: repeat(3, minmax(0, 1fr)); }
:deep(.el-select) { width: 100%; }
@media (max-width: 760px) {
  .page-head { align-items: flex-start; flex-direction: column; }
  .head-actions { flex-wrap: wrap; }
  .form-grid, .three-columns { grid-template-columns: 1fr; }
}
</style>
