<template>
  <div class="gva-form-box users-page">
    <div class="page-head"><div><h2>读者用户</h2><p>管理读者账号状态和基础资料。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
    <div class="toolbar"><el-input v-model="keyword" clearable placeholder="账号或昵称" @keyup.enter="load" /><el-select v-model="status" clearable placeholder="状态" @change="load"><el-option label="启用" value="enabled" /><el-option label="停用" value="disabled" /><el-option label="已删除" value="deleted" /></el-select><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-loading="loading" :data="rows" border><el-table-column label="用户 ID" min-width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column><el-table-column prop="username" label="账号" min-width="160" /><el-table-column prop="nickname" label="昵称" min-width="140" /><el-table-column prop="status" label="状态" width="110" /><el-table-column prop="lastLoginAt" label="最近登录" min-width="180" /><el-table-column label="操作" width="120"><template #default="{ row }"><el-switch :model-value="row.status === 'enabled'" :disabled="row.status === 'deleted'" @change="toggle(row)" /></template></el-table-column></el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
  </div>
</template>
<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { listReaderUsers, setReaderUserStatus } from '@/api/reader/users'
defineOptions({ name: 'ReaderUsers' })
const rows=ref([]);const total=ref(0);const page=ref(1);const pageSize=ref(20);const keyword=ref('');const status=ref('');const loading=ref(false)
const load=async()=>{loading.value=true;try{const res=await listReaderUsers({keyword:keyword.value,status:status.value,page:page.value,pageSize:pageSize.value});rows.value=res.data?.list||[];total.value=res.data?.total||0}finally{loading.value=false}}
const toggle=async(row)=>{const next=row.status==='enabled'?'disabled':'enabled';try{await setReaderUserStatus(row.id,next);row.status=next;ElMessage.success('更新成功')}catch{}}
load()
</script>
<style scoped>.users-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.toolbar .el-input{max-width:300px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
