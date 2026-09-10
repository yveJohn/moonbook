<template>
  <div class="p-4">
    <div class="gva-search-box">
      <el-form :inline="true" @submit.prevent="resetAndLoad">
        <el-form-item label="关键词"><el-input v-model="query.keyword" clearable placeholder="新书名或目标书 ID" @keyup.enter="resetAndLoad" /></el-form-item>
        <el-form-item label="状态"><el-select v-model="query.status" clearable placeholder="全部" style="width: 130px"><el-option label="执行中" value="running" /><el-option label="成功" value="succeeded" /><el-option label="失败" value="failed" /></el-select></el-form-item>
        <el-form-item><el-button type="primary" :icon="Search" @click="resetAndLoad">查询</el-button><el-button :icon="Refresh" @click="resetQuery">重置</el-button></el-form-item>
      </el-form>
    </div>

    <div class="gva-table-box">
      <div class="gva-btn-list"><el-button type="primary" :icon="Plus" @click="openCreate">创建合并</el-button></div>
      <el-table v-table-display v-loading="loading" :data="rows" row-key="id">
        <el-table-column label="任务 ID" width="170"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
        <el-table-column prop="targetBookName" label="目标书" min-width="180" show-overflow-tooltip ><template #default="{ row }"><BookReference :id="row.targetBookId || ''" :name="row.targetBookName || ''" /></template></el-table-column>

        <el-table-column prop="status" label="状态" width="100"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column prop="sourceCount" label="源书" width="80" align="right" />
        <el-table-column label="章节" width="140"><template #default="{ row }">{{ row.includedChapterCount }} / {{ row.chapterCount }}</template></el-table-column>
        <el-table-column prop="duplicateChapterCount" label="疑似重复" width="100" align="right" />
        <el-table-column prop="operatorName" label="操作人" width="120" show-overflow-tooltip />
        <el-table-column prop="createdAt" label="创建时间" width="180"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
        <el-table-column label="操作" width="70" fixed="right"><template #default="{ row }"><el-tooltip content="详情"><el-button link type="primary" :icon="View" aria-label="详情" @click="openDetail(row.id)" /></el-tooltip></template></el-table-column>
      </el-table>
      <div class="gva-pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :page-sizes="[10,20,50]" :total="total" layout="total, sizes, prev, pager, next" @current-change="load" @size-change="resetAndLoad" /></div>
    </div>

    <el-dialog v-model="createVisible" title="创建书籍合并" width="min(1180px, 96vw)" top="4vh" destroy-on-close>
      <el-tabs v-model="createTab">
        <el-tab-pane label="来源与新书" name="setup">
          <el-form :inline="true" @submit.prevent="loadEligible">
            <el-form-item label="来源检索"><el-input v-model="eligibleKeyword" clearable placeholder="书名、作者或帖子标题" @keyup.enter="loadEligible" /></el-form-item>
            <el-form-item><el-button :icon="Search" @click="loadEligible">查询</el-button><el-tag type="info">已选 {{ selectedBooks.length }} 本</el-tag></el-form-item>
          </el-form>
          <el-table v-table-display ref="eligibleTable" v-loading="eligibleLoading" :data="eligibleBooks" row-key="bookId" max-height="260" @selection-change="changeSelection">
            <el-table-column type="selection" width="48" :reserve-selection="true" />
            <el-table-column prop="bookName" label="源书" min-width="180" show-overflow-tooltip ><template #default="{ row }"><BookReference :id="row.bookId || ''" :name="row.bookName || ''" /></template></el-table-column>
            <el-table-column prop="authorName" label="作者" min-width="120" />
            <el-table-column prop="sourceName" label="来源" min-width="120" show-overflow-tooltip />
            <el-table-column prop="threadTitle" label="帖子" min-width="220" show-overflow-tooltip />
            <el-table-column prop="chapterCount" label="章节" width="80" align="right" />
            <el-table-column label="导入任务 ID" width="180"><template #default="{ row }"><code>{{ row.importTaskId }}</code></template></el-table-column>
          </el-table>
          <el-divider />
          <el-form ref="targetFormRef" :model="target" :rules="targetRules" label-width="90px">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-x-4">
              <el-form-item label="新书名称" prop="bookName"><el-input v-model="target.bookName" maxlength="100" /></el-form-item>
              <el-form-item label="作者" prop="authorId"><el-select v-model="target.authorId" filterable remote reserve-keyword :remote-method="searchAuthors" :loading="authorLoading" class="w-full"><el-option v-for="item in authors" :key="item.id" :label="item.penName" :value="item.id" /></el-select></el-form-item>
              <el-form-item label="主分类" prop="categoryCode"><el-select v-model="target.categoryCode" class="w-full"><el-option v-for="item in categories" :key="item.id" :label="item.name" :value="item.code" /></el-select></el-form-item>
              <el-form-item label="作品方向"><el-select v-model="target.workDirection" clearable class="w-full"><el-option label="男频" value="0" /><el-option label="女频" value="1" /></el-select></el-form-item>
              <el-form-item label="作品状态" prop="bookStatus"><el-segmented v-model="target.bookStatus" :options="bookStatusOptions" /></el-form-item>
              <el-form-item label="发布状态" prop="publishStatus"><el-segmented v-model="target.publishStatus" :options="publishOptions" /></el-form-item>
              <el-form-item label="操作人"><el-input v-model="target.operatorName" maxlength="64" /></el-form-item>
            </div>
            <el-form-item label="简介"><el-input v-model="target.description" type="textarea" :rows="3" maxlength="2000" show-word-limit /></el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="章节预览" name="preview" :disabled="!preview">
          <div v-if="preview" class="merge-summary">
            <el-statistic title="章节总数" :value="preview.chapterCount" /><el-statistic title="疑似重复" :value="preview.duplicateChapterCount" /><el-statistic title="已排除" :value="excludedChapterIds.length" /><el-statistic title="来源书籍" :value="preview.sources.length" />
          </div>
          <el-table v-table-display v-if="preview" :data="preview.chapters" row-key="sourceChapterId" max-height="520">
            <el-table-column label="排除" width="70"><template #default="{ row }"><el-checkbox :model-value="isExcluded(row.sourceChapterId)" @change="toggleExcluded(row.sourceChapterId, $event)" /></template></el-table-column>
            <el-table-column prop="targetChapterNo" label="目标序号" width="90" align="right" />
            <el-table-column prop="targetChapterName" label="目标章节" min-width="180" show-overflow-tooltip />
            <el-table-column label="来源书" width="180"><template #default="{ row }"><code><BookReference :id="row.sourceBookId || ''" :name="row.sourceBookName || ''" /></code></template></el-table-column>
            <el-table-column prop="sourceChapterNo" label="源序号" width="80" align="right" />
            <el-table-column prop="contentSource" label="正文来源" width="100" :formatter="adminEnumColumn" />
            <el-table-column prop="wordCount" label="字数" width="90" align="right" />
            <el-table-column label="疑似重复" width="130"><template #default="{ row }"><el-tag v-if="row.duplicateFlag" type="warning">{{ row.duplicateReason }}</el-tag><span v-else>-</span></template></el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="createVisible=false">取消</el-button>
        <el-button :loading="previewLoading" :disabled="selectedBooks.length < 2" :icon="View" @click="runPreview">生成预览</el-button>
        <el-button type="primary" :loading="executing" :disabled="!canExecute" @click="runExecute">执行合并</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="detailVisible" title="合并任务详情" width="min(1080px, 96vw)">
      <el-descriptions v-if="detail" :column="2" border>
        <el-descriptions-item label="任务 ID"><code>{{ detail.id }}</code></el-descriptions-item><el-descriptions-item label="状态">{{ statusLabel(detail.status) }}</el-descriptions-item>
        <el-descriptions-item label="目标书"><code><BookReference :id="detail.targetBookId || ''" :name="detail.targetBookName || ''" /></code></el-descriptions-item>
        <el-descriptions-item label="纳入 / 总数">{{ detail.includedChapterCount }} / {{ detail.chapterCount }}</el-descriptions-item><el-descriptions-item label="疑似重复">{{ detail.duplicateChapterCount }}</el-descriptions-item>
        <el-descriptions-item label="错误摘要" :span="2">{{ detail.errorSummary || '-' }}</el-descriptions-item>
      </el-descriptions>
      <el-tabs v-if="detail" class="mt-4">
        <el-tab-pane label="来源血缘"><el-table v-table-display :data="detail.sources" max-height="360"><el-table-column prop="sourceOrder" label="顺序" width="70" /><el-table-column prop="sourceBookName" label="源书" min-width="170" ><template #default="{ row }"><BookReference :id="row.sourceBookId || ''" :name="row.sourceBookName || ''" /></template></el-table-column><el-table-column prop="sourceThreadTitle" label="帖子" min-width="200" show-overflow-tooltip /><el-table-column prop="oldPublishStatus" label="原状态" width="100"  :formatter="adminStatusColumn" /><el-table-column prop="archivedPublishStatus" label="归档状态" width="100"  :formatter="adminStatusColumn" /></el-table></el-tab-pane>
        <el-tab-pane label="章节血缘"><el-table v-table-display :data="detail.chapters" max-height="420"><el-table-column prop="sourceChapterName" label="源章节" min-width="180" /><el-table-column label="源章节 ID" width="180"><template #default="{ row }"><code>{{ row.sourceChapterId }}</code></template></el-table-column><el-table-column prop="targetChapterNo" label="目标序号" width="90" /><el-table-column label="目标章节 ID" width="180"><template #default="{ row }"><code>{{ row.targetChapterId || '-' }}</code></template></el-table-column><el-table-column prop="excluded" label="排除" width="80"><template #default="{ row }">{{ row.excluded ? '是' : '否' }}</template></el-table-column></el-table></el-tab-pane>
      </el-tabs>
    </el-dialog>
  </div>
</template>

<script setup>
import { adminEnumColumn } from '@/utils/adminEnums'
import BookReference from '@/components/adminDisplay/BookReference.vue'
import { adminDateTime, adminStatusColumn } from '@/utils/adminDisplay'
  import { computed, nextTick, reactive, ref } from 'vue'
  import { Plus, Refresh, Search, View } from '@element-plus/icons-vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { executeBookMerge, getBookMerge, listBookMerges, listEligibleMergeBooks, previewBookMerge } from '@/api/novel/bookMerges'
  import { listNovelAuthors, listNovelCategories } from '@/api/novel/metadata'
  import { assertLongId } from '@/utils/longId'

  defineOptions({ name: 'NovelBookMerges' })
  const query=reactive({page:1,pageSize:20,keyword:'',status:''});const rows=ref([]);const total=ref(0);const loading=ref(false)
  const createVisible=ref(false);const createTab=ref('setup');const eligibleKeyword=ref('');const eligibleBooks=ref([]);const eligibleLoading=ref(false);const eligibleTable=ref();const selectedBooks=ref([])
  const targetFormRef=ref();const authors=ref([]);const authorLoading=ref(false);const categories=ref([]);const preview=ref(null);const previewLoading=ref(false);const executing=ref(false);const excludedChapterIds=ref([])
  const detailVisible=ref(false);const detail=ref(null)
  const defaults=()=>({bookName:'',authorId:'',categoryCode:'',workDirection:'',description:'',bookStatus:'completed',publishStatus:'draft',operatorName:''})
  const target=reactive(defaults());const targetRules={bookName:[{required:true,message:'请输入新书名称'}],authorId:[{required:true,message:'请选择作者'}],categoryCode:[{required:true,message:'请选择主分类'}],bookStatus:[{required:true,message:'请选择作品状态'}],publishStatus:[{required:true,message:'请选择发布状态'}]}
  const bookStatusOptions=[{label:'连载中',value:'serializing'},{label:'已完结',value:'completed'}];const publishOptions=[{label:'草稿',value:'draft'},{label:'发布',value:'published'}]
  const load=async()=>{loading.value=true;try{const res=await listBookMerges({...query});rows.value=res.data?.list||[];total.value=res.data?.total||0}finally{loading.value=false}}
  const resetAndLoad=()=>{query.page=1;load()};const resetQuery=()=>{Object.assign(query,{page:1,pageSize:20,keyword:'',status:''});load()}
  const loadEligible=async()=>{eligibleLoading.value=true;try{const res=await listEligibleMergeBooks({keyword:eligibleKeyword.value});eligibleBooks.value=res.data||[];await nextTick();for(const item of selectedBooks.value)eligibleTable.value?.toggleRowSelection(item,true)}finally{eligibleLoading.value=false}}
  const changeSelection=(items)=>{selectedBooks.value=items.map(item=>({...item,bookId:assertLongId(item.bookId),importTaskId:assertLongId(item.importTaskId) }))}
  const searchAuthors=async(keyword='')=>{authorLoading.value=true;try{const res=await listNovelAuthors({page:1,pageSize:30,status:'active',keyword:String(keyword).trim()});authors.value=res.data?.list||[]}finally{authorLoading.value=false}}
  const loadCategories=async()=>{const res=await listNovelCategories({page:1,pageSize:100,kind:'primary',enabled:true});categories.value=res.data?.list||[]}
  const openCreate=async()=>{Object.assign(target,defaults());selectedBooks.value=[];preview.value=null;excludedChapterIds.value=[];eligibleKeyword.value='';createTab.value='setup';createVisible.value=true;await Promise.all([loadEligible(),loadCategories(),searchAuthors()])}
  const sourceBookIds=()=>selectedBooks.value.map(item=>assertLongId(item.bookId))
  const runPreview=async()=>{if(selectedBooks.value.length<2)return;previewLoading.value=true;try{const res=await previewBookMerge({sourceBookIds:sourceBookIds()});preview.value=res.data;excludedChapterIds.value=[];createTab.value='preview'}finally{previewLoading.value=false}}
  const isExcluded=(id)=>excludedChapterIds.value.includes(String(id))
  const toggleExcluded=(id,checked)=>{const value=assertLongId(id);excludedChapterIds.value=checked?[...new Set([...excludedChapterIds.value,value])]:excludedChapterIds.value.filter(item=>item!==value)}
  const canExecute=computed(()=>preview.value&&preview.value.chapters.length>excludedChapterIds.value.length)
  const runExecute=async()=>{const valid=await targetFormRef.value?.validate().catch(()=>false);if(!valid||!canExecute.value)return;await ElMessageBox.confirm(`将创建“${target.bookName}”并下架 ${selectedBooks.value.length} 本源书，确定执行吗？`,'执行书籍合并',{type:'warning'});executing.value=true;try{const payload={sourceBookIds:sourceBookIds(),excludedChapterIds:excludedChapterIds.value,targetBook:{...target,authorId:assertLongId(target.authorId),workDirection:target.workDirection||null}};await executeBookMerge(payload);ElMessage.success('书籍合并成功');createVisible.value=false;load()}finally{executing.value=false}}
  const openDetail=async(id)=>{const res=await getBookMerge(assertLongId(id));detail.value=res.data;detailVisible.value=true}
  const statusLabel=(value)=>({running:'执行中',succeeded:'成功',failed:'失败'})[value]||value;const statusType=(value)=>({running:'warning',succeeded:'success',failed:'danger'})[value]||'info';const formatTime=(value)=>value?adminDateTime(value):'-'
  load()
</script>

<style scoped>
  .merge-summary{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px;padding:12px 0 20px;border-bottom:1px solid var(--el-border-color-lighter);margin-bottom:12px}
  @media (max-width: 720px){.merge-summary{grid-template-columns:repeat(2,minmax(0,1fr))}}
</style>
