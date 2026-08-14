<template>
  <div class="gva-form-box wallet-page"><div class="page-head"><div><h2>钱包与流水</h2><p>只读核对读者余额汇总与不可变流水。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div><div class="toolbar"><el-input v-model="keyword" clearable placeholder="读者账号或昵称" @keyup.enter="load" /><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-loading="loading" :data="rows" border><el-table-column label="读者 ID" min-width="180"><template #default="{row}"><code>{{row.readerId}}</code></template></el-table-column><el-table-column prop="readerUsername" label="读者" min-width="150" /><el-table-column label="充值余额" width="130"><template #default="{row}"><code>{{row.rechargeBalance}}</code></template></el-table-column><el-table-column label="奖励余额" width="130"><template #default="{row}"><code>{{row.bonusBalance}}</code></template></el-table-column><el-table-column label="核对" width="90"><template #default="{row}"><el-button link type="primary" @click="showLedgers(row)">流水</el-button></template></el-table-column></el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
    <el-drawer v-model="drawer" title="钱包流水" size="760px"><el-table v-loading="ledgerLoading" :data="ledgers" border><el-table-column label="流水 ID" min-width="170"><template #default="{ row }"><code>{{row.id}}</code></template></el-table-column><el-table-column prop="ledgerNo" label="流水号" min-width="160" /><el-table-column prop="direction" label="方向" width="90" /><el-table-column prop="coinType" label="币种" width="90" /><el-table-column label="金额" width="110"><template #default="{ row }"><code>{{row.amount}}</code></template></el-table-column><el-table-column label="变更后" width="110"><template #default="{ row }"><code>{{row.balanceAfter}}</code></template></el-table-column><el-table-column prop="bizType" label="业务" min-width="140" /><el-table-column prop="createdAt" label="时间" min-width="180" /></el-table></el-drawer>
  </div>
</template>
<script setup>
import { ref } from 'vue'
import { Refresh, Search } from '@element-plus/icons-vue'
import { listReaderWalletLedgers, listReaderWallets } from '@/api/reader/walletAdmin'
defineOptions({ name: 'ReaderWallet' })
const rows=ref([]);const total=ref(0);const page=ref(1);const pageSize=ref(20);const keyword=ref('');const loading=ref(false);const drawer=ref(false);const ledgerLoading=ref(false);const ledgers=ref([])
const load=async()=>{loading.value=true;try{const res=await listReaderWallets({keyword:keyword.value,page:page.value,pageSize:pageSize.value});rows.value=res.data?.list||[];total.value=res.data?.total||0}finally{loading.value=false}}
const showLedgers=async(row)=>{drawer.value=true;ledgerLoading.value=true;try{const res=await listReaderWalletLedgers(row.readerId,{page:1,pageSize:100});ledgers.value=res.data?.list||[]}finally{ledgerLoading.value=false}}
load()
</script>
<style scoped>.wallet-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.toolbar .el-input{max-width:300px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
