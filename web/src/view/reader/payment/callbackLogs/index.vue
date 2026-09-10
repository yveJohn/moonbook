<template>
  <div class="gva-form-box callback-logs-page">
    <header class="page-head">
      <h2>支付回调日志</h2>
      <el-tooltip content="刷新" placement="bottom">
        <el-button :icon="Refresh" circle aria-label="刷新" @click="load" />
      </el-tooltip>
    </header>

    <div class="toolbar">
      <el-input v-model="filters.keyword" clearable placeholder="商户订单号或网关交易号" @keyup.enter="search" />
      <el-select v-model="filters.processingResult" clearable placeholder="处理结果" @change="search">
        <el-option v-for="option in resultOptions" :key="option.value" :label="option.label" :value="option.value" />
      </el-select>
      <el-select v-model="filters.signatureStatus" clearable placeholder="签名状态" @change="search">
        <el-option label="有效" value="valid" />
        <el-option label="无效" value="invalid" />
        <el-option label="未校验" value="not_checked" />
      </el-select>
      <el-select v-model="filters.failureCode" clearable filterable placeholder="失败码" @change="search">
        <el-option v-for="code in failureCodes" :key="code" :label="code" :value="code" />
      </el-select>
      <el-select v-model="filters.responseStatus" clearable placeholder="HTTP 状态" @change="search">
        <el-option label="处理中（0）" value="0" />
        <el-option label="成功（200）" value="200" />
        <el-option label="请求错误（400）" value="400" />
        <el-option label="鉴权失败（401）" value="401" />
        <el-option label="服务不可用（503）" value="503" />
      </el-select>
      <el-date-picker
        v-model="filters.timeRange"
        type="datetimerange"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        range-separator="至"
        :clearable="true"
        @change="search"
      />
      <el-button type="primary" :icon="Search" @click="search">查询</el-button>
      <el-tooltip content="重置筛选" placement="bottom">
        <el-button :icon="RefreshLeft" aria-label="重置筛选" @click="reset" />
      </el-tooltip>
    </div>

    <el-table v-table-display v-loading="loading" :data="rows" border row-key="id">
      <el-table-column label="日志 ID" min-width="178">
        <template #default="{ row }"><code>{{ row.id }}</code></template>
      </el-table-column>
      <el-table-column prop="merchantOrderNo" label="商户订单号" min-width="168" show-overflow-tooltip />
      <el-table-column prop="gatewayTradeId" label="网关交易号" min-width="154" show-overflow-tooltip />
      <el-table-column label="处理结果" width="126">
        <template #default="{ row }">
          <el-tag :type="resultTagType(row.processingResult)" effect="plain">{{ resultLabel(row.processingResult) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="签名" width="96">
        <template #default="{ row }">
          <el-tag :type="signatureTag(row.signatureStatus).type" effect="plain">{{ signatureTag(row.signatureStatus).label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="failureCode" label="失败码" min-width="168" show-overflow-tooltip>
        <template #default="{ row }">{{ row.failureCode || '-' }}</template>
      </el-table-column>
      <el-table-column label="HTTP" width="84">
        <template #default="{ row }"><code>{{ row.responseStatus }}</code></template>
      </el-table-column>
      <el-table-column label="状态" width="104">
        <template #default="{ row }">
          <el-tag v-if="row.interrupted" type="warning" effect="dark">处理中断</el-tag>
          <span v-else>{{ row.completedAt ? '已完成' : '处理中' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="requestId" label="Request ID" min-width="166" show-overflow-tooltip />
      <el-table-column prop="requestTime" label="回调时间" min-width="185"  :formatter="adminTimeColumn" />
      <el-table-column fixed="right" label="详情" width="70" align="center">
        <template #default="{ row }">
          <el-tooltip content="查看详情" placement="left">
            <el-button link :icon="View" aria-label="查看详情" @click="openDetail(row.id)" />
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @size-change="load"
        @current-change="load"
      />
    </div>

    <el-drawer v-model="drawerVisible" title="回调审计详情" size="min(680px, 94vw)">
      <el-skeleton v-if="detailLoading" :rows="10" animated />
      <el-descriptions v-else-if="detail" :column="1" border>
        <el-descriptions-item label="日志 ID"><code>{{ detail.id }}</code></el-descriptions-item>
        <el-descriptions-item label="订单 ID"><code>{{ detail.orderId || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="处理结果">{{ resultLabel(detail.processingResult) }}</el-descriptions-item>
        <el-descriptions-item label="签名状态">{{ signatureTag(detail.signatureStatus).label }}</el-descriptions-item>
        <el-descriptions-item label="失败码"><code>{{ detail.failureCode || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="失败原因">{{ detail.failureReason || '-' }}</el-descriptions-item>
        <el-descriptions-item label="HTTP 响应"><code>{{ detail.responseStatus }} {{ detail.responseBody }}</code></el-descriptions-item>
        <el-descriptions-item label="Request ID"><code>{{ detail.requestId || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="Trace ID"><code>{{ detail.traceId || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="Payload SHA-256"><code class="hash">{{ detail.payloadHash }}</code></el-descriptions-item>
        <el-descriptions-item label="Payload 字节">{{ detail.payloadBytes }}{{ detail.payloadTruncated ? '（已截断）' : '' }}</el-descriptions-item>
        <el-descriptions-item label="请求时间">{{ adminDateTime(detail.requestTime) }}</el-descriptions-item>
        <el-descriptions-item label="完成时间">{{ adminDateTime(detail.completedAt) }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ adminDateTime(detail.createdAt) }}</el-descriptions-item>
        <el-descriptions-item label="业务快照">
          <dl class="snapshot">
            <template v-for="item in snapshotItems" :key="item.label">
              <dt>{{ item.label }}</dt><dd><code>{{ item.value || '-' }}</code></dd>
            </template>
          </dl>
        </el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<script setup>
import { adminDateTime, adminTimeColumn } from '@/utils/adminDisplay'
import { computed, reactive, ref } from 'vue'
import { Refresh, RefreshLeft, Search, View } from '@element-plus/icons-vue'
import { getPaymentCallbackLog, listPaymentCallbackLogs } from '@/api/reader/paymentCallbackLogs'

defineOptions({ name: 'ReaderPaymentCallbackLogs' })

const resultOptions = [
  { label: '已接收', value: 'received' }, { label: '成功', value: 'success' },
  { label: '幂等', value: 'idempotent' }, { label: '拒绝', value: 'rejected' },
  { label: '失败', value: 'failed' }, { label: '人工成功（历史）', value: 'manual_success' },
  { label: '同步待支付', value: 'sync_pending' }, { label: '同步已过期', value: 'sync_expired' },
  { label: '等待选择支付方式', value: 'sync_select' }, { label: '已支付但缺少回调', value: 'paid_no_callback' },
  { label: '同步失败', value: 'sync_rejected' }
]
const failureCodes = [
  'PAYLOAD_TOO_LARGE', 'REQUEST_READ_FAILED', 'INVALID_PAYLOAD', 'UNKNOWN_ORDER', 'UNKNOWN_CREDENTIAL',
  'PID_MISMATCH', 'SIGNATURE_INVALID', 'SNAPSHOT_MISMATCH', 'REPLAY_DETECTED', 'DEPENDENCY_FAILED',
  'TRANSACTION_FAILED', 'PANIC'
]
const filters = reactive({ keyword: '', processingResult: '', signatureStatus: '', failureCode: '', responseStatus: '', timeRange: [] })
const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const drawerVisible = ref(false)
const detailLoading = ref(false)
const detail = ref(null)

const resultLabel = (value) => resultOptions.find((option) => option.value === value)?.label || value || '-'
const resultTagType = (value) => ({ success: 'success', idempotent: 'success', rejected: 'warning', failed: 'danger', received: 'info' }[value] || 'info')
const signatureTag = (value) => ({
  valid: { label: '有效', type: 'success' }, invalid: { label: '无效', type: 'danger' }, not_checked: { label: '未校验', type: 'info' }
}[value] || { label: '未校验', type: 'info' })
const snapshotItems = computed(() => {
  const snapshot = detail.value?.snapshot || {}
  return [
    ['订单号', snapshot.order_id], ['网关交易号', snapshot.trade_id], ['订单金额', snapshot.amount],
    ['实际金额', snapshot.actual_amount], ['收款地址', snapshot.receive_address], ['Token', snapshot.token],
    ['链上交易号', snapshot.block_transaction_id], ['网关状态', snapshot.status]
  ].map(([label, value]) => ({ label, value }))
})

const queryParams = () => {
  const params = {
    keyword: filters.keyword.trim(), processingResult: filters.processingResult, signatureStatus: filters.signatureStatus,
    failureCode: filters.failureCode, responseStatus: filters.responseStatus, page: page.value, pageSize: pageSize.value
  }
  if (filters.timeRange?.length === 2) {
    params.startTime = filters.timeRange[0].toISOString()
    params.endTime = filters.timeRange[1].toISOString()
  }
  return params
}
const load = async () => {
  loading.value = true
  try {
    const response = await listPaymentCallbackLogs(queryParams())
    rows.value = response.data?.list || []
    total.value = response.data?.total || 0
  } finally {
    loading.value = false
  }
}
const search = () => { page.value = 1; load() }
const reset = () => {
  Object.assign(filters, { keyword: '', processingResult: '', signatureStatus: '', failureCode: '', responseStatus: '', timeRange: [] })
  search()
}
const openDetail = async (id) => {
  drawerVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    const response = await getPaymentCallbackLog(id)
    detail.value = response.data || null
  } finally {
    detailLoading.value = false
  }
}

load()
</script>

<style scoped>
.callback-logs-page { padding: 20px; }
.page-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.page-head h2 { margin: 0; font-size: 20px; letter-spacing: 0; }
.toolbar { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 14px; }
.toolbar > .el-input { width: min(320px, 100%); }
.toolbar > .el-select { width: 168px; }
.toolbar :deep(.el-date-editor) { width: 360px; max-width: 100%; }
.pager { display: flex; justify-content: flex-end; margin-top: 16px; overflow-x: auto; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; overflow-wrap: anywhere; }
.hash { font-size: 12px; }
.snapshot { display: grid; grid-template-columns: 112px minmax(0, 1fr); gap: 8px 12px; margin: 0; }
.snapshot dt { color: var(--el-text-color-secondary); }
.snapshot dd { min-width: 0; margin: 0; }
@media (max-width: 720px) {
  .callback-logs-page { padding: 12px; }
  .toolbar > .el-input, .toolbar > .el-select, .toolbar :deep(.el-date-editor) { width: 100%; }
  .snapshot { grid-template-columns: 1fr; gap: 3px; }
  .snapshot dd { margin-bottom: 7px; }
}
</style>
