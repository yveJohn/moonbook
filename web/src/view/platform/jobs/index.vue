<template>
  <div class="gva-form-box platform-jobs-page">
    <header class="page-head">
      <h2>平台任务</h2>
      <el-tooltip content="刷新" placement="bottom">
        <el-button :icon="Refresh" circle aria-label="刷新" @click="load" />
      </el-tooltip>
    </header>
    <div class="toolbar">
      <el-input v-model.trim="filters.module" clearable placeholder="模块" @keyup.enter="load" />
      <el-input v-model.trim="filters.jobType" clearable placeholder="任务类型" @keyup.enter="load" />
      <el-select v-model="filters.status" clearable placeholder="状态" @change="load">
        <el-option label="待处理" value="pending" />
        <el-option label="运行中" value="running" />
        <el-option label="成功" value="succeeded" />
        <el-option label="失败" value="failed" />
        <el-option label="已取消" value="cancelled" />
      </el-select>
      <el-input v-model.trim="filters.leaseOwner" clearable placeholder="租约 Worker" @keyup.enter="load" />
      <el-date-picker v-model="filters.timeRange" type="datetimerange" start-placeholder="开始时间" end-placeholder="结束时间" range-separator="至" clearable @change="search" />
      <el-button type="primary" :icon="Search" @click="search">查询</el-button>
      <el-tooltip content="重置筛选" placement="bottom">
        <el-button :icon="RefreshLeft" aria-label="重置筛选" @click="reset" />
      </el-tooltip>
    </div>
    <el-table v-table-display v-loading="loading" :data="rows" border row-key="id">
      <el-table-column label="任务 ID" width="170"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
      <el-table-column prop="module" label="模块" width="120" :formatter="adminEnumColumn" />
      <el-table-column prop="jobType" label="类型" width="170" :formatter="adminEnumColumn" />
      <el-table-column prop="status" label="状态" width="100"  :formatter="adminStatusColumn" />
      <el-table-column label="尝试" width="100"><template #default="{ row }">{{ row.attemptCount }} / {{ row.maxAttempts }}</template></el-table-column>
      <el-table-column prop="leaseOwner" label="租约 Worker" min-width="160" show-overflow-tooltip />
      <el-table-column prop="lastErrorCode" label="错误代码" width="150" />
      <el-table-column prop="updatedAt" label="更新时间" min-width="185"  :formatter="adminTimeColumn" />
      <el-table-column fixed="right" label="详情" width="70" align="center"><template #default="{ row }"><el-tooltip content="查看详情" placement="left"><el-button link :icon="View" aria-label="查看详情" @click="detail(row)" /></el-tooltip></template></el-table-column>
    </el-table>
    <div class="pager">
      <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" />
    </div>
    <el-drawer v-model="dialog" title="平台任务详情" size="min(900px, 94vw)">
      <el-descriptions v-if="current" :column="2" border>
        <el-descriptions-item label="任务 ID"><code>{{ current.id }}</code></el-descriptions-item>
        <el-descriptions-item label="模块 / 类型">{{ adminEnum('module', current.module) }} / {{ adminEnum('jobType', current.jobType) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ adminStatus(current.status) }}</el-descriptions-item>
        <el-descriptions-item label="尝试">{{ current.attemptCount }} / {{ current.maxAttempts }}</el-descriptions-item>
        <el-descriptions-item label="租约">{{ current.leaseOwner || '-' }} / {{ adminDateTime(current.leaseExpiresAt) }}</el-descriptions-item>
        <el-descriptions-item label="错误">{{ current.lastErrorCode || '-' }} {{ current.lastErrorMessage || '' }}</el-descriptions-item>
        <el-descriptions-item label="可用时间">{{ adminDateTime(current.availableAt) }}</el-descriptions-item>
        <el-descriptions-item label="完成时间">{{ adminDateTime(current.finishedAt) }}</el-descriptions-item>
      </el-descriptions>
      <el-divider content-position="left">执行尝试</el-divider>
      <el-table v-table-display v-if="current" :data="current.attempts" border>
        <el-table-column label="Attempt ID" width="170"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
        <el-table-column prop="attemptNumber" label="序号" width="70" />
        <el-table-column prop="workerId" label="Worker" min-width="150" />
        <el-table-column prop="outcome" :formatter="adminStatusColumn" label="结果" width="100" />
        <el-table-column prop="errorCode" label="错误代码" width="140" />
        <el-table-column prop="errorMessage" label="错误信息" min-width="220" show-overflow-tooltip />
        <el-table-column prop="startedAt" label="开始时间" min-width="185"  :formatter="adminTimeColumn" />
      </el-table>
    </el-drawer>
  </div>
</template>

<script setup>
import { adminEnumColumn, adminEnum } from '@/utils/adminEnums'
import { adminDateTime, adminStatus, adminStatusColumn, adminTimeColumn } from '@/utils/adminDisplay'
import { Refresh, RefreshLeft, Search, View } from '@element-plus/icons-vue'
import { reactive, ref } from 'vue'
import { getPlatformJob, getPlatformJobs } from '@/api/platform'

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const dialog = ref(false)
const current = ref(null)
const filters = reactive({ module: '', jobType: '', status: '', leaseOwner: '', timeRange: [] })

defineOptions({ name: 'PlatformJobs' })

const query = () => {
  const params = { page: page.value, pageSize: pageSize.value, module: filters.module, jobType: filters.jobType, status: filters.status, leaseOwner: filters.leaseOwner }
  if (filters.timeRange?.length === 2) {
    params.from = filters.timeRange[0].toISOString()
    params.to = filters.timeRange[1].toISOString()
  }
  return params
}
const load = async () => {
  loading.value = true
  try {
    const response = await getPlatformJobs(query())
    rows.value = response.data?.list || []
    total.value = response.data?.total || 0
  } finally {
    loading.value = false
  }
}
const search = () => { page.value = 1; load() }
const reset = () => {
  Object.assign(filters, { module: '', jobType: '', status: '', leaseOwner: '', timeRange: [] })
  search()
}
const detail = async (row) => {
  const response = await getPlatformJob(row.id)
  current.value = response.data
  dialog.value = true
}
load()
</script>

<style scoped>
.platform-jobs-page { padding: 20px; }
.page-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.page-head h2 { margin: 0; font-size: 20px; letter-spacing: 0; }
.platform-jobs-page .toolbar { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 14px; }
.platform-jobs-page .toolbar .el-input, .platform-jobs-page .toolbar .el-select { width: 180px; }
.platform-jobs-page .toolbar :deep(.el-date-editor) { width: 360px; max-width: 100%; }
.platform-jobs-page .pager { display: flex; justify-content: flex-end; margin-top: 14px; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; overflow-wrap: anywhere; }
@media (max-width: 720px) {
  .platform-jobs-page { padding: 12px; }
  .platform-jobs-page .toolbar .el-input, .platform-jobs-page .toolbar .el-select, .platform-jobs-page .toolbar :deep(.el-date-editor) { width: 100%; }
  .platform-jobs-page .pager { overflow-x: auto; }
}
</style>
