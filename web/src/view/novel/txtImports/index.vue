<template>
  <div class="gva-form-box txt-import-page">
    <div class="page-head"><div><h2>TXT 导入</h2><p>上传 UTF-8 TXT 文件，解析后异步写入目标书籍章节。</p></div><el-button type="primary" :icon="Upload" @click="dialog=true">上传 TXT</el-button></div>
    <div class="toolbar"><el-input v-model="keyword" clearable placeholder="文件名" @keyup.enter="load" /><el-select v-model="status" clearable placeholder="状态" @change="load"><el-option label="待处理" value="pending" /><el-option label="运行中" value="running" /><el-option label="成功" value="succeeded" /><el-option label="失败" value="failed" /><el-option label="已取消" value="cancelled" /></el-select><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-loading="loading" :data="rows" border><el-table-column label="任务 ID" width="170"><template #default="{row}"><code>{{ row.id }}</code></template></el-table-column><el-table-column label="目标书籍 ID" width="180"><template #default="{row}"><code>{{ row.targetBookId }}</code></template></el-table-column><el-table-column prop="originalFilename" label="文件名" min-width="220" show-overflow-tooltip /><el-table-column prop="status" label="状态" width="100" /><el-table-column label="进度" width="120"><template #default="{row}">{{ row.importedChapterCount }} / {{ row.totalChapterCount }}</template></el-table-column><el-table-column prop="qualityStatus" label="质量" width="100" /><el-table-column prop="createdAt" label="创建时间" width="180" /><el-table-column label="操作" width="160"><template #default="{row}"><el-button link type="warning" :disabled="!['failed','cancelled'].includes(row.status)" @click="retry(row)">重试</el-button><el-button link type="danger" :disabled="!['pending','running'].includes(row.status)" @click="cancel(row)">取消</el-button></template></el-table-column></el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
    <el-dialog v-model="dialog" title="上传 TXT" width="520px"><el-form ref="formRef" :model="form" :rules="rules" label-position="top"><el-form-item label="目标书籍 ID" prop="targetBookId"><el-input v-model="form.targetBookId" inputmode="numeric" /></el-form-item><el-form-item label="TXT 文件" prop="file"><el-upload drag :auto-upload="false" :limit="1" accept=".txt,text/plain" :on-change="pick"><el-icon><UploadFilled /></el-icon><div>选择 TXT 文件</div></el-upload></el-form-item></el-form><template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">上传并排队</el-button></template></el-dialog>
  </div>
</template>
<script setup>
import { reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Upload, UploadFilled } from '@element-plus/icons-vue'
import { listTxtImports, uploadTxtImport, retryTxtImport, cancelTxtImport } from '@/api/novel/txtImports'
import { assertLongId } from '@/utils/longId'
defineOptions({ name: 'NovelTxtImports' })
const rows=ref([]), total=ref(0), page=ref(1), pageSize=ref(20), keyword=ref(''), status=ref(''), loading=ref(false), dialog=ref(false), saving=ref(false), formRef=ref()
const form=reactive({targetBookId:'',file:null}); const rules={targetBookId:[{required:true,validator:(_,v,done)=>{try{assertLongId(v);done()}catch{done(new Error('请输入正整数书籍 ID'))}},trigger:'blur'}],file:[{required:true,message:'请选择 TXT 文件',trigger:'change'}]}
const load=async()=>{loading.value=true;try{const r=await listTxtImports({keyword:keyword.value,status:status.value,page:page.value,pageSize:pageSize.value});rows.value=r.data?.list||[];total.value=r.data?.total||0}finally{loading.value=false}}
const pick=(file)=>{form.file=file.raw}
const submit=async()=>{if(!(await formRef.value?.validate().catch(()=>false))||!form.file)return;saving.value=true;try{const data=new FormData();data.append('targetBookId',form.targetBookId);data.append('file',form.file,form.file.name);await uploadTxtImport(data);ElMessage.success('上传成功，已排队');dialog.value=false;form.targetBookId='';form.file=null;load()}finally{saving.value=false}}
const retry=async(row)=>{await retryTxtImport(row.id);ElMessage.success('已重新排队');load()}
const cancel=async(row)=>{try{await ElMessageBox.confirm('确定取消该 TXT 导入任务吗？','取消确认',{type:'warning'});await cancelTxtImport(row.id);ElMessage.success('已取消');load()}catch{ElMessage.info('已取消操作')}}
load()
</script>
<style scoped>.txt-import-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.toolbar .el-input{max-width:280px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
