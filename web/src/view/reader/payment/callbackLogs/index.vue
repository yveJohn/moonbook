<template>
  <div class="gva-form-box callback-logs-page">
    <div class="page-head"><div><h2>支付回调日志</h2><p>审计签名校验、处理结果和网关响应，不显示原始敏感报文。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
    <div class="toolbar"><el-input v-model="keyword" clearable placeholder="商户订单号或网关交易号" @keyup.enter="load" /><el-select v-model="result" clearable placeholder="处理结果" @change="load"><el-option label="成功" value="success" /><el-option label="失败" value="failure" /><el-option label="拒绝" value="rejected" /></el-select><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-loading="loading" :data="rows" border><el-table-column label="日志 ID" min-width="180"><template #default="{row}"><code>{{ row.id }}</code></template></el-table-column><el-table-column prop="merchantOrderNo" label="商户订单号" min-width="170" /><el-table-column prop="gatewayTradeId" label="网关交易号" min-width="160" /><el-table-column prop="processingResult" label="处理结果" width="120" /><el-table-column label="签名" width="100"><template #default="{row}"><el-tag :type="row.signatureValid ? 'success' : 'danger'">{{ row.signatureValid ? '有效' : '无效' }}</el-tag></template></el-table-column><el-table-column prop="responseStatus" label="响应状态" width="110" /><el-table-column prop="createdAt" label="时间" min-width="180" /></el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
  </div>
</template>
<script setup>
import { ref } from 'vue'
import { Refresh, Search } from '@element-plus/icons-vue'
import { listPaymentCallbackLogs } from '@/api/reader/paymentCallbackLogs'
defineOptions({ name: 'ReaderPaymentCallbackLogs' })
const rows=ref([]);const total=ref(0);const page=ref(1);const pageSize=ref(20);const keyword=ref('');const result=ref('');const loading=ref(false)
const load=async()=>{loading.value=true;try{const res=await listPaymentCallbackLogs({keyword:keyword.value,processingResult:result.value,page:page.value,pageSize:pageSize.value});rows.value=res.data?.list||[];total.value=res.data?.total||0}finally{loading.value=false}}
load()
</script>
<style scoped>.callback-logs-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.toolbar .el-input{max-width:340px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
