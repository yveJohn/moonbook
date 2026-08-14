<template>
  <div class="gva-form-box recharge-orders-page">
    <div class="page-head"><div><h2>充值订单</h2><p>检索充值订单、支付状态和网关回执。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
    <div class="toolbar"><el-input v-model="keyword" clearable placeholder="订单号或读者账号" @keyup.enter="load" /><el-select v-model="status" clearable placeholder="订单状态" @change="load"><el-option v-for="item in statuses" :key="item" :label="item" :value="item" /></el-select><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-loading="loading" :data="rows" border><el-table-column label="订单 ID" min-width="180"><template #default="{row}"><code>{{ row.id }}</code></template></el-table-column><el-table-column prop="orderNo" label="订单号" min-width="170" /><el-table-column prop="readerUsername" label="读者" min-width="140" /><el-table-column label="钻石/USDT" min-width="150"><template #default="{row}"><code>{{ row.diamondAmount }}</code> / <code>{{ row.priceUsdt }}</code></template></el-table-column><el-table-column prop="status" label="状态" width="140" /><el-table-column prop="createdAt" label="创建时间" min-width="180" /><el-table-column label="详情" width="90"><template #default="{row}"><el-button link type="primary" @click="show(row)">查看</el-button></template></el-table-column></el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
    <el-drawer v-model="drawer" title="充值订单详情" size="520px"><el-descriptions v-if="detail" :column="1" border><el-descriptions-item label="订单 ID"><code>{{ detail.id }}</code></el-descriptions-item><el-descriptions-item label="订单号">{{ detail.orderNo }}</el-descriptions-item><el-descriptions-item label="读者">{{ detail.readerUsername }}（{{ detail.readerId }}）</el-descriptions-item><el-descriptions-item label="状态">{{ detail.status }}</el-descriptions-item><el-descriptions-item label="网关交易号">{{ detail.gatewayTradeId || '-' }}</el-descriptions-item><el-descriptions-item label="链上交易号">{{ detail.blockTransactionId || '-' }}</el-descriptions-item><el-descriptions-item label="支付地址">{{ detail.receiveAddress || '-' }}</el-descriptions-item></el-descriptions></el-drawer>
  </div>
</template>
<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { getRechargeOrder, listRechargeOrders } from '@/api/reader/rechargeOrders'
defineOptions({ name: 'ReaderRechargeOrders' })
const rows=ref([]);const total=ref(0);const page=ref(1);const pageSize=ref(20);const keyword=ref('');const status=ref('');const loading=ref(false);const drawer=ref(false);const detail=ref(null);const statuses=['creating','pending','gateway_unknown','create_failed','superseded','expired','callback_exception','paid']
const load=async()=>{loading.value=true;try{const res=await listRechargeOrders({keyword:keyword.value,status:status.value,page:page.value,pageSize:pageSize.value});rows.value=res.data?.list||[];total.value=res.data?.total||0}finally{loading.value=false}}
const show=async(row)=>{try{const res=await getRechargeOrder(row.id);detail.value=res.data;drawer.value=true}catch{ElMessage.error('加载订单失败')}}
load()
</script>
<style scoped>.recharge-orders-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.toolbar .el-input{max-width:300px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
