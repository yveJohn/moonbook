<template>
  <div class="gva-form-box payment-channels-page">
    <div class="page-head"><div><h2>支付渠道</h2><p>管理充值渠道启停并检查凭据配置状态。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
    <el-table v-loading="loading" :data="rows" border>
      <el-table-column label="渠道 ID" min-width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
      <el-table-column prop="provider" label="提供商" width="130" />
      <el-table-column label="资产" width="160"><template #default="{ row }">{{ row.token.toUpperCase() }} / {{ row.network.toUpperCase() }}</template></el-table-column>
      <el-table-column label="凭据" width="120"><template #default="{ row }"><el-tag :type="row.configured ? 'success' : 'danger'">{{ row.configured ? '已配置' : '未配置' }}</el-tag></template></el-table-column>
      <el-table-column label="启用" width="120"><template #default="{ row }"><el-switch v-model="row.enabled" :disabled="!row.configured" @change="toggle(row)" /></template></el-table-column>
      <el-table-column label="连通性" width="170"><template #default="{ row }"><el-button link type="primary" :loading="row.checking" @click="check(row)">检查</el-button><el-tag v-if="row.checkStatus" :type="checkType(row.checkStatus)">{{ row.checkMessage }}</el-tag></template></el-table-column>
    </el-table>
  </div>
</template>
<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { checkPaymentChannel, listPaymentChannels, setPaymentChannelEnabled } from '@/api/reader/paymentChannels'
defineOptions({ name: 'ReaderPaymentChannels' })
const rows = ref([]); const loading = ref(false)
const load = async () => { loading.value = true; try { const res = await listPaymentChannels(); rows.value = res.data || [] } finally { loading.value = false } }
const toggle = async (row) => { try { await setPaymentChannelEnabled(row.id, row.enabled); ElMessage.success('更新成功') } catch { row.enabled = !row.enabled } }
const checkType = (status) => ({ reachable: 'success', disabled: 'info', not_configured: 'warning', unreachable: 'danger', unsupported: 'info' }[status] || 'info')
const check = async (row) => { row.checking = true; try { const res = await checkPaymentChannel(row.id); const value = res.data || {}; row.checkStatus = value.status; row.checkMessage = value.message; ElMessage.info(value.message || '检查完成') } finally { row.checking = false } }
load()
</script>
<style scoped>.payment-channels-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
