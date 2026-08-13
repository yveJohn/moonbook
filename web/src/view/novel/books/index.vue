<template>
  <div class="p-4">
    <div class="gva-search-box">
      <el-form :inline="true" @submit.prevent="resetAndLoad">
        <el-form-item label="关键词"><el-input v-model="query.keyword" clearable placeholder="书名或作者" @keyup.enter="resetAndLoad" /></el-form-item>
        <el-form-item label="发布状态"><el-select v-model="query.publishStatus" clearable placeholder="全部" style="width: 120px"><el-option v-for="item in publishOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
        <el-form-item label="作品状态"><el-select v-model="query.bookStatus" clearable placeholder="全部" style="width: 120px"><el-option v-for="item in bookStatusOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
        <el-form-item><el-button type="primary" :icon="Search" @click="resetAndLoad">查询</el-button><el-button :icon="Refresh" @click="resetQuery">重置</el-button></el-form-item>
      </el-form>
    </div>
    <div class="gva-table-box">
      <div class="gva-btn-list"><el-button type="primary" :icon="Plus" @click="openCreate">新增书籍</el-button></div>
      <el-table v-loading="loading" :data="rows" row-key="id">
        <el-table-column label="书名" prop="bookName" min-width="190" show-overflow-tooltip />
        <el-table-column label="作者" prop="authorName" min-width="130" />
        <el-table-column label="主分类" min-width="120"><template #default="scope">{{ scope.row.primaryCategory?.name || '-' }}</template></el-table-column>
        <el-table-column label="副分类" min-width="180"><template #default="scope"><el-tag v-for="item in scope.row.subCategories" :key="item.id" class="mr-1" type="info">{{ item.name }}</el-tag><span v-if="!scope.row.subCategories?.length">-</span></template></el-table-column>
        <el-table-column label="作品状态" width="100"><template #default="scope">{{ labelOf(bookStatusOptions, scope.row.bookStatus) }}</template></el-table-column>
        <el-table-column label="发布状态" width="100"><template #default="scope"><el-tag :type="publishType(scope.row.publishStatus)">{{ labelOf(publishOptions, scope.row.publishStatus) }}</el-tag></template></el-table-column>
        <el-table-column label="收费模式" width="130"><template #default="scope">{{ labelOf(chargeOptions, scope.row.chargeMode) }}</template></el-table-column>
        <el-table-column label="字数" prop="wordCount" width="100" align="right" />
        <el-table-column label="更新时间" min-width="180"><template #default="scope">{{ formatTime(scope.row.updatedAt) }}</template></el-table-column>
        <el-table-column label="操作" width="150" fixed="right"><template #default="scope"><el-tooltip content="编辑"><el-button link type="primary" :icon="Edit" aria-label="编辑" @click="openEdit(scope.row.id)" /></el-tooltip><el-tooltip content="删除"><el-button link type="danger" :icon="Delete" aria-label="删除" @click="removeRow(scope.row)" /></el-tooltip></template></el-table-column>
      </el-table>
      <div class="gva-pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :page-sizes="[10,30,50,100]" :total="total" layout="total, sizes, prev, pager, next, jumper" @current-change="loadData" @size-change="resetAndLoad" /></div>
    </div>
    <el-drawer v-model="drawerVisible" :show-close="false" size="min(680px, 100%)" @closed="resetForm">
      <template #header><div class="flex justify-between items-center w-full"><span class="text-base">{{ editingId ? '编辑书籍' : '新增书籍' }}</span><div><el-button @click="drawerVisible=false">取消</el-button><el-button type="primary" :loading="submitting" @click="submit">保存</el-button></div></div></template>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-x-4">
          <el-form-item label="书名" prop="bookName"><el-input v-model="form.bookName" maxlength="100" /></el-form-item>
          <el-form-item label="作者" prop="authorId"><el-select v-model="form.authorId" filterable remote reserve-keyword :remote-method="searchAuthors" :loading="authorLoading" placeholder="输入笔名检索" class="w-full"><el-option v-for="item in authors" :key="item.id" :label="item.penName" :value="item.id" /></el-select></el-form-item>
          <el-form-item label="主分类" prop="categoryCode"><el-select v-model="form.categoryCode" class="w-full"><el-option v-for="item in primaryCategories" :key="item.id" :label="item.name" :value="item.code" /></el-select></el-form-item>
          <el-form-item label="副分类"><el-select v-model="form.subCategoryCodes" multiple filterable class="w-full"><el-option v-for="item in subCategories" :key="item.id" :label="item.name" :value="item.code" /></el-select></el-form-item>
          <el-form-item label="作品状态" prop="bookStatus"><el-select v-model="form.bookStatus" class="w-full"><el-option v-for="item in bookStatusOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
          <el-form-item label="发布状态" prop="publishStatus"><el-select v-model="form.publishStatus" class="w-full"><el-option v-for="item in publishOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
          <el-form-item label="作品方向"><el-select v-model="form.workDirection" clearable class="w-full"><el-option label="男频" value="0" /><el-option label="女频" value="1" /></el-select></el-form-item>
          <el-form-item label="来源" prop="sourceType"><el-select v-model="form.sourceType" class="w-full"><el-option v-for="item in sourceOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
          <el-form-item label="评分" prop="score"><el-input v-model="form.score" inputmode="decimal" placeholder="0-10，最多两位小数" /></el-form-item>
          <el-form-item label="收费模式" prop="chargeMode"><el-select v-model="form.chargeMode" class="w-full"><el-option v-for="item in chargeOptions" :key="item.value" v-bind="item" /></el-select></el-form-item>
          <el-form-item v-if="form.chargeMode==='fixed_price'" label="整书售价" prop="fixedPriceCoin"><el-input v-model="form.fixedPriceCoin" inputmode="numeric" @input="cleanPositiveLong('fixedPriceCoin')" /></el-form-item>
          <el-form-item label="标签"><el-select v-model="form.tags" multiple filterable allow-create default-first-option class="w-full" /></el-form-item>
          <el-form-item label="精选"><el-switch v-model="form.featured" /></el-form-item>
          <el-form-item label="精选排序"><el-input-number v-model="form.featuredSort" :min="0" :max="999999" class="w-full" /></el-form-item>
        </div>
        <el-form-item label="精选文案"><el-input v-model="form.featuredNote" maxlength="255" /></el-form-item>
        <el-form-item label="封面文件">
          <div class="w-full">
            <el-image v-if="coverPreviewUrl" :src="coverPreviewUrl" fit="cover" class="mb-3 h-40 w-28 border border-gray-200" />
            <el-upload :auto-upload="false" :limit="1" :on-change="selectCover" :on-remove="clearCover" accept="image/jpeg,image/png,image/webp,image/gif">
              <el-button :icon="Upload">选择封面</el-button>
              <template #tip><span class="ml-3 text-gray-500">JPEG、PNG、WebP 或 GIF，最大 10 MiB</span></template>
            </el-upload>
          </div>
        </el-form-item>
        <el-form-item label="历史封面"><el-input v-model="form.legacyCoverUrl" maxlength="500" placeholder="迁移前的封面来源 URL" /></el-form-item>
        <el-form-item label="简介"><el-input v-model="form.description" type="textarea" :rows="6" maxlength="2000" show-word-limit /></el-form-item>
      </el-form>
    </el-drawer>
  </div>
</template>

<script setup>
  import { onBeforeUnmount, reactive, ref } from 'vue'
  import { Delete, Edit, Plus, Refresh, Search, Upload } from '@element-plus/icons-vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { createNovelBook, deleteNovelBook, getNovelBook, getNovelBookCover, listNovelBooks, updateNovelBook, uploadNovelBookCover } from '@/api/novel/books'
  import { listNovelAuthors, listNovelCategories } from '@/api/novel/metadata'
  defineOptions({ name: 'NovelBooks' })
  const bookStatusOptions=[{label:'连载中',value:'serializing'},{label:'已完结',value:'completed'}]
  const publishOptions=[{label:'入库',value:'draft'},{label:'上架',value:'published'},{label:'弃用',value:'deprecated'}]
  const sourceOptions=[{label:'手工创建',value:'manual'},{label:'历史迁移',value:'legacy'},{label:'TXT 导入',value:'txt_import'},{label:'论坛采集',value:'forum_crawl'}]
  const chargeOptions=[{label:'按字收费',value:'word_charge'},{label:'会员专享',value:'membership_only'},{label:'登录免费',value:'login_free'},{label:'整书定价',value:'fixed_price'}]
  const query=reactive({page:1,pageSize:10,keyword:'',publishStatus:'',bookStatus:''});const rows=ref([]);const total=ref(0);const loading=ref(false)
  const authors=ref([]);const authorLoading=ref(false);const primaryCategories=ref([]);const subCategories=ref([]);const drawerVisible=ref(false);const submitting=ref(false);const editingId=ref('');const formRef=ref();const coverFile=ref(null);const coverPreviewUrl=ref('')
  const defaults=()=>({workDirection:'',categoryCode:'',legacyCoverUrl:'',bookName:'',authorId:'',description:'',score:'0',bookStatus:'serializing',publishStatus:'draft',sourceType:'manual',featured:false,featuredSort:0,featuredNote:'',chargeMode:'word_charge',fixedPriceCoin:null,subCategoryCodes:[],tags:[]})
  const form=reactive(defaults());const rules={bookName:[{required:true,message:'请输入书名'}],authorId:[{required:true,message:'请选择作者'}],categoryCode:[{required:true,message:'请选择主分类'}],score:[{pattern:/^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/,message:'请输入最多两位小数的评分'}],fixedPriceCoin:[{validator:(_,value,done)=>form.chargeMode!=='fixed_price'||/^[1-9][0-9]*$/.test(value||'')?done():done(new Error('请输入正整数售价'))}]}
  const loadData=async()=>{loading.value=true;try{const res=await listNovelBooks({...query});if(res.code===0){rows.value=res.data.list||[];total.value=res.data.total||0}}finally{loading.value=false}}
  const loadAllCategories=async(kind)=>{const items=[];let page=1;let total=0;do{const res=await listNovelCategories({page,pageSize:100,kind,enabled:true});const batch=res.data?.list||[];items.push(...batch);total=res.data?.total||0;page+=1}while(items.length<total);return items}
  const loadCategories=async()=>{[primaryCategories.value,subCategories.value]=await Promise.all([loadAllCategories('primary'),loadAllCategories('sub')])}
  const searchAuthors=async(keyword='')=>{authorLoading.value=true;try{const res=await listNovelAuthors({page:1,pageSize:30,status:'active',keyword:String(keyword).trim()});authors.value=res.data?.list||[]}finally{authorLoading.value=false}}
  const resetAndLoad=()=>{query.page=1;loadData()};const resetQuery=()=>{Object.assign(query,{page:1,pageSize:10,keyword:'',publishStatus:'',bookStatus:''});loadData()}
  const releaseCoverPreview=()=>{if(coverPreviewUrl.value){URL.revokeObjectURL(coverPreviewUrl.value);coverPreviewUrl.value=''}}
  const resetForm=()=>{editingId.value='';coverFile.value=null;releaseCoverPreview();Object.assign(form,defaults());formRef.value?.clearValidate()}
  const openCreate=async()=>{resetForm();await Promise.all([loadCategories(),searchAuthors()]);drawerVisible.value=true}
  const openEdit=async(id)=>{resetForm();const [res,coverRes]=await Promise.all([getNovelBook(id),getNovelBookCover(id),loadCategories()]);if(res.code!==0)return;const b=res.data;const coverType=String(coverRes.headers?.['content-type']||'');if(coverRes.status===200&&coverType.startsWith('image/')&&coverRes.data?.size)coverPreviewUrl.value=URL.createObjectURL(coverRes.data);await searchAuthors(b.authorName);if(!authors.value.some(item=>item.id===b.authorId))authors.value.unshift({id:b.authorId,penName:b.authorName});editingId.value=b.id;Object.assign(form,{workDirection:b.workDirection||'',categoryCode:b.primaryCategory.code,legacyCoverUrl:b.legacyCoverUrl||'',bookName:b.bookName,authorId:b.authorId,description:b.description,score:b.score,bookStatus:b.bookStatus,publishStatus:b.publishStatus,sourceType:b.sourceType,featured:b.featured,featuredSort:b.featuredSort,featuredNote:b.featuredNote,chargeMode:b.chargeMode,fixedPriceCoin:b.fixedPriceCoin,subCategoryCodes:(b.subCategories||[]).map(i=>i.code),tags:b.tags||[]});drawerVisible.value=true}
  const cleanPositiveLong=(key)=>{form[key]=String(form[key]||'').replace(/[^0-9]/g,'').replace(/^0+/,'')||null}
  const selectCover=(uploadFile)=>{const file=uploadFile.raw;if(!file)return;const allowed=['image/jpeg','image/png','image/webp','image/gif'];if(!allowed.includes(file.type)||file.size<1||file.size>10*1024*1024){ElMessage.error('请选择不超过 10 MiB 的 JPEG、PNG、WebP 或 GIF');coverFile.value=null;return}coverFile.value=file;releaseCoverPreview();coverPreviewUrl.value=URL.createObjectURL(file)}
  const clearCover=()=>{coverFile.value=null;releaseCoverPreview()}
  const submit=async()=>{const valid=await formRef.value?.validate().catch(()=>false);if(!valid)return;submitting.value=true;try{const payload={...form,workDirection:form.workDirection||null,fixedPriceCoin:form.chargeMode==='fixed_price'?form.fixedPriceCoin:null};const res=editingId.value?await updateNovelBook(editingId.value,payload):await createNovelBook(payload);if(res.code!==0)return;const bookId=res.data.id;if(coverFile.value){const coverRes=await uploadNovelBookCover(bookId,coverFile.value);if(coverRes.code!==0){ElMessage.warning('书籍已保存，封面上传失败，可重新编辑后重试');return}}ElMessage.success(editingId.value?'书籍已更新':'书籍已创建');drawerVisible.value=false;loadData()}finally{submitting.value=false}}
  const removeRow=async(row)=>{await ElMessageBox.confirm(`确定删除书籍“${row.bookName}”吗？`,'删除书籍',{type:'warning'});const res=await deleteNovelBook(row.id);if(res.code===0){ElMessage.success('书籍已删除');loadData()}}
  const labelOf=(options,value)=>options.find(i=>i.value===value)?.label||value;const publishType=(value)=>({draft:'info',published:'success',deprecated:'warning'})[value]||'info';const formatTime=(value)=>value?new Date(value).toLocaleString():'-'
  loadData()
  onBeforeUnmount(releaseCoverPreview)
</script>
