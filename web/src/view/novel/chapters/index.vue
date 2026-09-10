<template>
  <div class="p-4">
    <div class="gva-search-box">
      <el-form :inline="true" @submit.prevent="resetAndLoad">
        <el-form-item label="书籍"><BookSelect v-model="query.bookId" @change="resetAndLoad" /></el-form-item>
        <el-form-item label="章节名称"><el-input v-model="query.keyword" clearable placeholder="关键词" @keyup.enter="resetAndLoad" /></el-form-item>
        <el-form-item label="章节状态"><el-select v-model="query.chapterStatus" clearable placeholder="全部" style="width: 120px"><el-option v-for="item in statusOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
        <el-form-item label="清洗状态"><el-select v-model="query.aiCleanStatus" clearable placeholder="全部" style="width: 120px"><el-option v-for="item in cleanOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
        <el-form-item><el-button type="primary" :icon="Search" @click="resetAndLoad">查询</el-button><el-button :icon="Refresh" @click="resetQuery">重置</el-button></el-form-item>
      </el-form>
    </div>
    <div class="gva-table-box">
      <div class="gva-btn-list"><el-button type="primary" :icon="Plus" @click="openCreate">新增章节</el-button></div>
      <el-table v-table-display v-loading="loading" :data="rows" row-key="id">
        <el-table-column label="书籍" prop="bookId" min-width="165" ><template #default="{ row }"><BookReference :id="row.bookId || ''" :name="row.bookName || ''" /></template></el-table-column>
        <el-table-column label="序号" prop="chapterNo" width="90" align="right" />
        <el-table-column label="章节名称" prop="chapterName" min-width="220" show-overflow-tooltip />
        <el-table-column label="字数" prop="wordCount" width="100" align="right" />
        <el-table-column label="收费" width="90"><template #default="scope"><el-tag :type="scope.row.isVip?'warning':'success'">{{ scope.row.isVip?'收费':'免费' }}</el-tag></template></el-table-column>
        <el-table-column label="价格" prop="bookPriceCoin" min-width="130" align="right" />
        <el-table-column label="章节状态" width="100"><template #default="scope">{{ labelOf(statusOptions, scope.row.chapterStatus) }}</template></el-table-column>
        <el-table-column label="清洗状态" width="110"><template #default="scope">{{ labelOf(cleanOptions, scope.row.aiCleanStatus) }}</template></el-table-column>
        <el-table-column label="更新时间" min-width="180"><template #default="scope">{{ formatTime(scope.row.updatedAt) }}</template></el-table-column>
        <el-table-column label="操作" width="150" fixed="right"><template #default="scope"><el-tooltip content="正文"><el-button link type="primary" :icon="View" aria-label="正文" @click="openContent(scope.row)" /></el-tooltip><el-tooltip content="编辑"><el-button link type="primary" :icon="Edit" aria-label="编辑" @click="openEdit(scope.row.id)" /></el-tooltip><el-tooltip content="删除"><el-button link type="danger" :icon="Delete" aria-label="删除" @click="removeRow(scope.row)" /></el-tooltip></template></el-table-column>
      </el-table>
      <div class="gva-pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :page-sizes="[10,30,50,100]" :total="total" layout="total, sizes, prev, pager, next, jumper" @current-change="loadData" @size-change="resetAndLoad" /></div>
    </div>
    <el-drawer v-model="drawerVisible" :show-close="false" size="min(760px, 100%)" @closed="resetForm">
      <template #header><div class="flex justify-between items-center w-full"><span class="text-base">{{ editingId?'编辑章节':'新增章节' }}</span><div><el-button @click="drawerVisible=false">取消</el-button><el-button type="primary" :loading="submitting" @click="submit">保存</el-button></div></div></template>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-x-4">
          <el-form-item label="书籍" prop="bookId"><BookSelect v-model="form.bookId" /></el-form-item>
          <el-form-item label="章节序号"><el-input-number v-model="form.chapterNo" :min="0" :max="2147483647" class="w-full" /></el-form-item>
          <el-form-item label="章节名称" prop="chapterName"><el-input v-model="form.chapterName" maxlength="255" /></el-form-item>
          <el-form-item label="章节价格" prop="bookPriceCoin"><el-input v-model="form.bookPriceCoin" inputmode="numeric" @input="cleanLong(form, 'bookPriceCoin', true)" /></el-form-item>
          <el-form-item label="是否收费"><el-switch v-model="form.isVip" /></el-form-item>
          <el-form-item label="章节状态"><el-select v-model="form.chapterStatus" class="w-full"><el-option v-for="item in statusOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
          <el-form-item label="清洗状态"><el-select v-model="form.aiCleanStatus" class="w-full"><el-option v-for="item in cleanOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
        </div>
        <el-form-item label="章节正文"><el-input v-model="form.content" type="textarea" :rows="22" resize="vertical" /></el-form-item>
      </el-form>
    </el-drawer>
    <el-dialog v-model="contentVisible" :title="contentTitle" width="min(860px, 92vw)" destroy-on-close><el-input v-model="contentText" type="textarea" :rows="24" readonly resize="vertical" /></el-dialog>
  </div>
</template>

<script setup>
import BookReference from '@/components/adminDisplay/BookReference.vue'
import BookSelect from '@/components/adminDisplay/BookSelect.vue'
import { adminDateTime } from '@/utils/adminDisplay'
  import { reactive, ref } from 'vue'
  import { useRoute } from 'vue-router'
  import { Delete, Edit, Plus, Refresh, Search, View } from '@element-plus/icons-vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { createNovelChapter, deleteNovelChapter, getNovelChapter, getNovelChapterContent, listNovelChapters, updateNovelChapter } from '@/api/novel/chapters'
  defineOptions({ name: 'NovelChapters' })
  const statusOptions=[{label:'启用',value:'enabled'},{label:'禁用',value:'disabled'}]
  const cleanOptions=[{label:'未清洗',value:'pending'},{label:'清洗中',value:'cleaning'},{label:'已清洗',value:'cleaned'},{label:'已丢弃',value:'discarded'},{label:'失败',value:'failed'},{label:'已过期',value:'expired'},{label:'已跳过',value:'skipped'}]
  const query=reactive({page:1,pageSize:10,bookId:'',keyword:'',chapterStatus:'',aiCleanStatus:''});const rows=ref([]);const total=ref(0);const loading=ref(false)
  const drawerVisible=ref(false);const submitting=ref(false);const editingId=ref('');const formRef=ref();const contentVisible=ref(false);const contentTitle=ref('');const contentText=ref('')
  const defaults=()=>({bookId:'',chapterNo:null,chapterName:'',isVip:false,bookPriceCoin:'0',chapterStatus:'enabled',aiCleanStatus:'pending',content:''})
  const form=reactive(defaults());const rules={bookId:[{required:true,message:'请输入书籍 ID'},{pattern:/^[1-9][0-9]*$/,message:'请输入正整数书籍 ID'}],chapterName:[{required:true,message:'请输入章节名称'}],bookPriceCoin:[{required:true,message:'请输入章节价格'},{pattern:/^[0-9]+$/,message:'请输入非负整数价格'}]}
  const cleanLong=(target,key,allowZero=false)=>{const digits=String(target[key]??'').replace(/[^0-9]/g,'');target[key]=allowZero?(digits.replace(/^0+(?=\d)/,'')||''):(digits.replace(/^0+/,'')||'')}
  const loadData=async()=>{loading.value=true;try{const params={...query,bookId:query.bookId||undefined};const res=await listNovelChapters(params);if(res.code===0){rows.value=res.data.list||[];total.value=res.data.total||0}}finally{loading.value=false}}
  const resetAndLoad=()=>{query.page=1;loadData()};const resetQuery=()=>{Object.assign(query,{page:1,pageSize:10,bookId:'',keyword:'',chapterStatus:'',aiCleanStatus:''});loadData()}
  const resetForm=()=>{editingId.value='';Object.assign(form,defaults());formRef.value?.clearValidate()}
  const openCreate=()=>{resetForm();if(query.bookId)form.bookId=query.bookId;drawerVisible.value=true}
  const openEdit=async(id)=>{resetForm();const [chapterRes,contentRes]=await Promise.all([getNovelChapter(id),getNovelChapterContent(id)]);if(chapterRes.code!==0||contentRes.code!==0)return;editingId.value=id;Object.assign(form,chapterRes.data,{content:contentRes.data.content});drawerVisible.value=true}
  const submit=async()=>{const valid=await formRef.value?.validate().catch(()=>false);if(!valid)return;submitting.value=true;try{const payload={...form,chapterNo:form.chapterNo===null?null:form.chapterNo};const res=editingId.value?await updateNovelChapter(editingId.value,payload):await createNovelChapter(payload);if(res.code===0){ElMessage.success(editingId.value?'章节已更新':'章节已创建');drawerVisible.value=false;loadData()}}finally{submitting.value=false}}
  const openContent=async(row)=>{const res=await getNovelChapterContent(row.id);if(res.code!==0)return;contentText.value=res.data.content;contentTitle.value=`${row.chapterName} · v${res.data.version}`;contentVisible.value=true}
  const removeRow=async(row)=>{await ElMessageBox.confirm(`确定删除章节“${row.chapterName}”吗？`,'删除章节',{type:'warning'});const res=await deleteNovelChapter(row.id);if(res.code===0){ElMessage.success('章节已删除');loadData()}}
  const labelOf=(options,value)=>options.find(item=>item.value===value)?.label||value;const formatTime=(value)=>value?adminDateTime(value):'-'
  const routeBookId=String(useRoute().query.bookId||'');if(/^[1-9][0-9]*$/.test(routeBookId))query.bookId=routeBookId
  loadData()
</script>
