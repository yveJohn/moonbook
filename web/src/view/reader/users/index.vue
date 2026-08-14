<template>
  <div class="gva-form-box users-page">
    <div class="page-head"><div><h2>读者用户</h2><p>管理读者账号状态和基础资料。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
    <div class="toolbar"><el-input v-model="keyword" clearable placeholder="账号或昵称" @keyup.enter="load" /><el-select v-model="status" clearable placeholder="状态" @change="load"><el-option label="启用" value="enabled" /><el-option label="停用" value="disabled" /><el-option label="已删除" value="deleted" /></el-select><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-loading="loading" :data="rows" border><el-table-column label="用户 ID" min-width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column><el-table-column prop="username" label="账号" min-width="160" /><el-table-column prop="nickname" label="昵称" min-width="140" /><el-table-column prop="status" label="状态" width="110" /><el-table-column prop="lastLoginAt" label="最近登录" min-width="180" /><el-table-column label="操作" width="190"><template #default="{ row }"><el-switch :model-value="row.status === 'enabled'" :disabled="row.status === 'deleted'" @change="toggle(row)" /><el-button link type="primary" :disabled="row.status === 'deleted'" @click="openPassword(row)">重置密码</el-button></template></el-table-column></el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
    <el-dialog v-model="passwordDialog" title="重置读者密码" width="440px"><el-form ref="passwordRef" :model="passwordForm" :rules="passwordRules" label-position="top"><el-form-item label="新密码" prop="password"><el-input v-model="passwordForm.password" type="password" show-password autocomplete="new-password" /></el-form-item><el-form-item label="确认密码" prop="confirmPassword"><el-input v-model="passwordForm.confirmPassword" type="password" show-password autocomplete="new-password" /></el-form-item></el-form><template #footer><el-button @click="passwordDialog=false">取消</el-button><el-button type="primary" :loading="passwordSaving" @click="savePassword">保存</el-button></template></el-dialog>
  </div>
</template>
<script setup>
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { listReaderUsers, resetReaderUserPassword, setReaderUserStatus } from '@/api/reader/users'
defineOptions({ name: 'ReaderUsers' })
const rows=ref([]);const total=ref(0);const page=ref(1);const pageSize=ref(20);const keyword=ref('');const status=ref('');const loading=ref(false);const passwordDialog=ref(false);const passwordSaving=ref(false);const passwordRef=ref();const passwordUserId=ref('');const passwordForm=reactive({password:'',confirmPassword:''})
const passwordRules={password:[{required:true,min:6,max:64,message:'密码长度需为 6-64 个字符',trigger:'blur'}],confirmPassword:[{required:true,message:'请再次输入密码',trigger:'blur'}]}
const load=async()=>{loading.value=true;try{const res=await listReaderUsers({keyword:keyword.value,status:status.value,page:page.value,pageSize:pageSize.value});rows.value=res.data?.list||[];total.value=res.data?.total||0}finally{loading.value=false}}
const toggle=async(row)=>{const next=row.status==='enabled'?'disabled':'enabled';try{await setReaderUserStatus(row.id,next);row.status=next;ElMessage.success('更新成功')}catch{}}
const openPassword=(row)=>{passwordUserId.value=row.id;Object.assign(passwordForm,{password:'',confirmPassword:''});passwordDialog.value=true}
const savePassword=async()=>{if(!(await passwordRef.value?.validate().catch(()=>false)))return;if(passwordForm.password!==passwordForm.confirmPassword){ElMessage.error('两次输入的密码不一致');return};passwordSaving.value=true;try{await resetReaderUserPassword(passwordUserId.value,{...passwordForm});ElMessage.success('密码已重置');passwordDialog.value=false}finally{passwordSaving.value=false}}
load()
</script>
<style scoped>.users-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.toolbar .el-input{max-width:300px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
