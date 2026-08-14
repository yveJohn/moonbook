<template>
  <div class="gva-form-box profile-page">
    <header class="page-head">
      <div><h2>作品资料补全</h2><el-tag :type="config.autoScanEnabled ? 'success' : 'info'">{{ config.autoScanEnabled ? '自动扫描' : '手动生成' }}</el-tag></div>
      <el-button type="primary" :icon="MagicStick" @click="openGenerate">生成建议</el-button>
    </header>

    <el-tabs v-model="activeTab">
      <el-tab-pane label="建议审核" name="suggestions">
        <div class="toolbar">
          <el-input v-model="query.bookId" inputmode="numeric" clearable placeholder="作品 ID" @input="query.bookId=digits(query.bookId)" @keyup.enter="resetAndLoad" />
          <el-input v-model="query.bookName" clearable placeholder="作品名称" @keyup.enter="resetAndLoad" />
          <el-select v-model="query.status" clearable placeholder="状态" @change="resetAndLoad"><el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select>
          <el-select v-model="query.triggerType" clearable placeholder="触发方式" @change="resetAndLoad"><el-option label="手动生成" value="manual" /><el-option label="自动扫描" value="auto_scan" /><el-option label="重新生成" value="regenerate" /></el-select>
          <el-select v-model="query.suggestedCategoryCode" clearable filterable placeholder="建议分类" @change="resetAndLoad"><el-option v-for="item in primaryCategories" :key="item.code" :label="item.name" :value="item.code" /></el-select>
          <el-button :icon="Search" @click="resetAndLoad">查询</el-button>
          <el-button :icon="Refresh" aria-label="刷新" @click="loadSuggestions" />
        </div>
        <el-table v-loading="loading" :data="rows" border row-key="id" @selection-change="selection=$event">
          <el-table-column type="selection" width="44" :selectable="row => row.status === 'failed'" />
          <el-table-column label="建议 ID" min-width="165"><template #default="{row}"><code>{{ row.id }}</code></template></el-table-column>
          <el-table-column prop="bookName" label="作品" min-width="150" show-overflow-tooltip />
          <el-table-column label="状态" width="92"><template #default="{row}"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
          <el-table-column label="触发" width="90"><template #default="{row}">{{ triggerLabel(row.triggerType) }}</template></el-table-column>
          <el-table-column label="建议分类" min-width="125"><template #default="{row}">{{ row.suggested?.categoryName || row.suggested?.categoryCode || '-' }}</template></el-table-column>
          <el-table-column label="建议书名" min-width="160" show-overflow-tooltip><template #default="{row}">{{ row.suggested?.bookName || '-' }}</template></el-table-column>
          <el-table-column prop="errorMessage" label="错误" min-width="180" show-overflow-tooltip />
          <el-table-column label="操作" width="124" fixed="right"><template #default="{row}">
            <el-tooltip content="查看与审核"><el-button link type="primary" :icon="View" aria-label="查看与审核" @click="openDetail(row.id)" /></el-tooltip>
            <el-tooltip v-if="row.status==='failed'" content="重新生成"><el-button link type="primary" :icon="RefreshRight" aria-label="重新生成" @click="regenerate(row)" /></el-tooltip>
          </template></el-table-column>
        </el-table>
        <div class="table-foot"><el-button :icon="RefreshRight" :disabled="!selection.length" @click="batchRegenerate">批量重试</el-button><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="loadSuggestions" @current-change="loadSuggestions" /></div>
      </el-tab-pane>

      <el-tab-pane label="运行配置" name="config">
        <el-form ref="configRef" :model="config" :rules="configRules" label-position="top" class="config-form">
          <div class="switch-row"><el-form-item label="自动扫描"><el-switch v-model="config.autoScanEnabled" /></el-form-item><el-form-item label="自动应用"><el-switch v-model="config.autoApplyEnabled" /></el-form-item><el-form-item label="拒答备用模型"><el-switch v-model="config.processAiRefusalEnabled" /></el-form-item></div>
          <div class="form-grid"><el-form-item label="主 AI 配置" prop="aiConfigId"><el-select v-model="config.aiConfigId" class="w-full"><el-option v-for="item in aiOptions" :key="item.id" :label="`${item.configName} / ${item.currentModelName}`" :value="String(item.id)" /></el-select></el-form-item><el-form-item label="拒答备用 AI"><el-select v-model="config.refusalFallbackAiConfigId" clearable class="w-full"><el-option v-for="item in aiOptions" :key="item.id" :label="`${item.configName} / ${item.currentModelName}`" :value="String(item.id)" /></el-select></el-form-item></div>
          <el-form-item label="系统提示词" prop="systemPrompt"><el-input v-model="config.systemPrompt" type="textarea" :rows="7" /></el-form-item>
          <div class="number-grid"><el-form-item label="温度"><el-input-number v-model="config.temperature" :min="0" :max="2" :step="0.1" /></el-form-item><el-form-item label="最大 Token"><el-input-number v-model="config.maxTokens" :min="1" :max="1000000" /></el-form-item><el-form-item label="超时（秒）"><el-input-number v-model="config.timeoutSeconds" :min="1" :max="1800" /></el-form-item><el-form-item label="请求间隔（毫秒）"><el-input-number v-model="config.requestIntervalMs" :min="0" :max="600000" /></el-form-item><el-form-item label="重试次数"><el-input-number v-model="config.retryCount" :min="0" :max="20" /></el-form-item><el-form-item label="扫描批量"><el-input-number v-model="config.scanBatchSize" :min="1" :max="1000" /></el-form-item><el-form-item label="输入字符预算"><el-input-number v-model="config.maxInputChars" :min="1000" :max="1000000" /></el-form-item></div>
          <el-button type="primary" :icon="Check" :loading="saving" @click="saveConfig">保存配置</el-button>
        </el-form>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="generateVisible" title="生成作品资料建议" width="min(480px,94vw)">
      <el-form ref="generateRef" :model="generateForm" :rules="generateRules" label-position="top"><el-form-item label="作品 ID" prop="bookId"><el-input v-model="generateForm.bookId" inputmode="numeric" maxlength="20" @input="generateForm.bookId=digits(generateForm.bookId)" /></el-form-item></el-form>
      <template #footer><el-button @click="generateVisible=false">取消</el-button><el-button type="primary" :loading="generating" @click="submitGenerate">提交</el-button></template>
    </el-dialog>

    <el-drawer v-model="detailVisible" title="建议审核" size="min(920px,96vw)" destroy-on-close>
      <div v-if="detail" class="detail-body">
        <div class="compare-grid"><section><h3>原始资料</h3><el-descriptions :column="1" border><el-descriptions-item label="书名">{{ detail.original.bookName }}</el-descriptions-item><el-descriptions-item label="主分类">{{ detail.original.categoryName }}</el-descriptions-item><el-descriptions-item label="副分类">{{ categoryNames(detail.original.subCategories) }}</el-descriptions-item><el-descriptions-item label="简介"><pre>{{ detail.original.bookDesc || '-' }}</pre></el-descriptions-item></el-descriptions></section><section><h3>AI 建议</h3><el-descriptions :column="1" border><el-descriptions-item label="书名">{{ detail.suggested?.bookName || '-' }}</el-descriptions-item><el-descriptions-item label="主分类">{{ detail.suggested?.categoryName || detail.suggested?.unmatchedCategoryName || '-' }}</el-descriptions-item><el-descriptions-item label="副分类">{{ categoryNames(detail.suggested?.subCategories) }}</el-descriptions-item><el-descriptions-item label="简介"><pre>{{ detail.suggested?.bookDesc || '-' }}</pre></el-descriptions-item></el-descriptions></section></div>
        <el-form v-if="reviewable" ref="reviewRef" :model="review" :rules="reviewRules" label-position="top" class="review-form"><div class="form-grid"><el-form-item label="最终书名" prop="bookName"><el-input v-model="review.bookName" maxlength="100" show-word-limit /></el-form-item><el-form-item label="最终主分类" prop="categoryCode"><el-select v-model="review.categoryCode" filterable class="w-full"><el-option v-for="item in primaryCategories" :key="item.code" :label="item.name" :value="item.code" /></el-select></el-form-item></div><el-form-item label="最终副分类"><el-select v-model="review.subCategoryCodes" multiple filterable clearable class="w-full"><el-option v-for="item in subCategories" :key="item.code" :label="item.name" :value="item.code" /></el-select></el-form-item><el-form-item label="最终简介" prop="bookDesc"><el-input v-model="review.bookDesc" type="textarea" :rows="6" maxlength="2000" show-word-limit /></el-form-item><el-form-item label="拒绝原因"><el-input v-model="review.rejectReason" maxlength="1000" /></el-form-item></el-form>
        <el-tabs><el-tab-pane label="生成输入"><pre class="raw-block">{{ pretty(detail.inputSnapshot) }}</pre></el-tab-pane><el-tab-pane label="原始响应"><pre class="raw-block">{{ detail.rawResponse || '-' }}</pre></el-tab-pane></el-tabs>
      </div>
      <template #footer><el-button v-if="reviewable" type="danger" :icon="Close" :loading="reviewing" @click="rejectSuggestion">拒绝</el-button><el-button v-if="reviewable" type="primary" :icon="Check" :loading="reviewing" @click="applySuggestion">应用建议</el-button></template>
    </el-drawer>
  </div>
</template>

<script setup>
  import { computed, reactive, ref } from 'vue'
  import { Check, Close, MagicStick, Refresh, RefreshRight, Search, View } from '@element-plus/icons-vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { applyBookProfileSuggestion, batchRegenerateBookProfileSuggestions, generateBookProfileSuggestion, getBookProfileConfig, getBookProfileSuggestion, listBookProfileSuggestions, rejectBookProfileSuggestion, saveBookProfileConfig } from '@/api/novel/bookProfile'
  import { listEnabledAIConfigs } from '@/api/novel/aiConfigs'
  import { listNovelCategories } from '@/api/novel/metadata'
  import { assertLongId } from '@/utils/longId'

  defineOptions({ name: 'NovelBookProfile' })
  const defaultPrompt='你是小说编辑。根据作品元数据、分类候选和章节内容生成可审核的资料建议。只输出 JSON，必须包含 book_name、book_desc、category_name、category_reason、sub_category_names、sub_category_codes、sub_category_reason、confidence、reason。'
  const defaults=()=>({autoScanEnabled:false,autoApplyEnabled:false,processAiRefusalEnabled:false,aiConfigId:'',refusalFallbackAiConfigId:'',systemPrompt:defaultPrompt,temperature:0.1,maxTokens:2000,timeoutSeconds:120,requestIntervalMs:0,retryCount:1,scanBatchSize:5,maxInputChars:60000})
  const activeTab=ref('suggestions'), rows=ref([]), total=ref(0), loading=ref(false), saving=ref(false), generating=ref(false), reviewing=ref(false), selection=ref([]), aiOptions=ref([]), primaryCategories=ref([]), subCategories=ref([])
  const query=reactive({bookId:'',bookName:'',status:'',triggerType:'',suggestedCategoryCode:'',page:1,pageSize:20}), config=reactive(defaults()), generateForm=reactive({bookId:''}), review=reactive({bookName:'',categoryCode:'',bookDesc:'',subCategoryCodes:[],rejectReason:''})
  const configRef=ref(),generateRef=ref(),reviewRef=ref(),generateVisible=ref(false),detailVisible=ref(false),detail=ref()
  const statusOptions=[['running','生成中'],['pending','待审核'],['approved','已审核'],['applied','已应用'],['rejected','已拒绝'],['failed','失败'],['retried','已重试'],['recovered','已恢复'],['expired','已过期']].map(([value,label])=>({value,label}))
  const configRules={aiConfigId:[{required:true,message:'请选择主 AI 配置',trigger:'change'}],systemPrompt:[{required:true,message:'请输入系统提示词',trigger:'blur'}]};const generateRules={bookId:[{required:true,pattern:/^[1-9]\d*$/,message:'请输入有效作品 ID',trigger:'blur'}]};const reviewRules={bookName:[{required:true,message:'请输入最终书名',trigger:'blur'}],categoryCode:[{required:true,message:'请选择最终主分类',trigger:'change'}],bookDesc:[{required:true,message:'请输入最终简介',trigger:'blur'}]}
  const digits=value=>String(value||'').replace(/\D/g,'').replace(/^0+/,'').slice(0,20)
  const statusLabel=value=>statusOptions.find(item=>item.value===value)?.label||value
  const statusType=value=>({running:'warning',pending:'primary',approved:'success',applied:'success',rejected:'info',failed:'danger',retried:'warning'})[value]||'info'
  const triggerLabel=value=>({manual:'手动',auto_scan:'自动',regenerate:'重试'})[value]||value
  const categoryNames=value=>(value||[]).map(item=>item.name||item.categoryName||item.code||item.categoryCode).join('、')||'-'
  const pretty=value=>typeof value==='string'?value:JSON.stringify(value,null,2)
  const reviewable=computed(()=>['pending','failed'].includes(detail.value?.status))
  const loadSuggestions=async()=>{loading.value=true;try{const response=await listBookProfileSuggestions(query);rows.value=response.data?.list||[];total.value=response.data?.total||0}finally{loading.value=false}}
  const resetAndLoad=()=>{query.page=1;loadSuggestions()}
  const loadOptions=async()=>{const [ai,primary,sub]=await Promise.all([listEnabledAIConfigs(),listNovelCategories({kind:'primary',enabled:'true',page:1,pageSize:100}),listNovelCategories({kind:'sub',enabled:'true',page:1,pageSize:100})]);aiOptions.value=ai.data||[];primaryCategories.value=primary.data?.list||[];subCategories.value=sub.data?.list||[]}
  const loadConfig=async()=>{try{const response=await getBookProfileConfig();Object.assign(config,response.data,{aiConfigId:String(response.data.aiConfigId||''),refusalFallbackAiConfigId:String(response.data.refusalFallbackAiConfigId||'')})}catch{Object.assign(config,defaults())}}
  const saveConfig=async()=>{if(!(await configRef.value?.validate().catch(()=>false)))return;if(config.refusalFallbackAiConfigId&&config.refusalFallbackAiConfigId===config.aiConfigId){ElMessage.warning('备用 AI 不能与主 AI 相同');return}saving.value=true;try{await saveBookProfileConfig({...config,aiConfigId:assertLongId(config.aiConfigId),refusalFallbackAiConfigId:config.refusalFallbackAiConfigId?assertLongId(config.refusalFallbackAiConfigId):''});ElMessage.success('配置已保存')}finally{saving.value=false}}
  const openGenerate=()=>{generateForm.bookId='';generateVisible.value=true}
  const submitGenerate=async()=>{if(!(await generateRef.value?.validate().catch(()=>false)))return;generating.value=true;try{await generateBookProfileSuggestion({bookId:assertLongId(generateForm.bookId)});ElMessage.success('生成任务已提交');generateVisible.value=false;loadSuggestions()}finally{generating.value=false}}
  const openDetail=async id=>{const response=await getBookProfileSuggestion(assertLongId(id));detail.value=response.data;const source=response.data.suggested||response.data.original;Object.assign(review,{bookName:source?.bookName||'',categoryCode:source?.categoryCode||'',bookDesc:source?.bookDesc||'',subCategoryCodes:(source?.subCategories||[]).map(item=>item.code||item.categoryCode),rejectReason:''});detailVisible.value=true}
  const regenerate=async row=>{await generateBookProfileSuggestion({bookId:assertLongId(row.bookId),regenerate:true,sourceSuggestionId:assertLongId(row.id)});ElMessage.success('重新生成任务已提交');loadSuggestions()}
  const batchRegenerate=async()=>{const ids=selection.value.map(item=>assertLongId(item.id));const response=await batchRegenerateBookProfileSuggestions({suggestionIds:ids});ElMessage.success(`已提交 ${response.data?.successCount||0} 条任务`);loadSuggestions()}
  const applySuggestion=async()=>{if(!(await reviewRef.value?.validate().catch(()=>false)))return;await ElMessageBox.confirm('应用后将事务性更新作品资料，确定继续吗？','应用建议',{type:'warning'});reviewing.value=true;try{await applyBookProfileSuggestion(assertLongId(detail.value.id),review);ElMessage.success('建议已应用');detailVisible.value=false;loadSuggestions()}finally{reviewing.value=false}}
  const rejectSuggestion=async()=>{await ElMessageBox.confirm('确定拒绝这条建议吗？','拒绝建议',{type:'warning'});reviewing.value=true;try{await rejectBookProfileSuggestion(assertLongId(detail.value.id),{rejectReason:review.rejectReason});ElMessage.success('建议已拒绝');detailVisible.value=false;loadSuggestions()}finally{reviewing.value=false}}
  Promise.all([loadSuggestions(),loadConfig(),loadOptions()])
</script>

<style scoped>
  .profile-page{padding:20px}.page-head,.page-head>div,.table-foot{display:flex;align-items:center;justify-content:space-between;gap:12px}.page-head{margin-bottom:12px}.page-head h2{margin:0;font-size:20px;letter-spacing:0}.toolbar{display:grid;grid-template-columns:160px minmax(180px,280px) 120px 120px 150px auto auto;gap:10px;margin-bottom:14px}.table-foot{margin-top:16px}.config-form{max-width:980px;padding-top:8px}.form-grid,.number-grid,.switch-row,.compare-grid{display:grid;gap:0 18px}.form-grid,.compare-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.number-grid{grid-template-columns:repeat(3,minmax(0,1fr))}.switch-row{grid-template-columns:repeat(3,160px)}.compare-grid h3{font-size:15px;letter-spacing:0}.compare-grid pre,.raw-block{margin:0;white-space:pre-wrap;word-break:break-word;font-family:inherit}.review-form{margin-top:20px;padding-top:18px;border-top:1px solid var(--el-border-color-lighter)}.raw-block{max-height:420px;overflow:auto;padding:12px;background:var(--el-fill-color-light);font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}@media(max-width:1180px){.toolbar{grid-template-columns:repeat(3,minmax(0,1fr))}}@media(max-width:760px){.profile-page{padding:12px}.toolbar,.form-grid,.number-grid,.switch-row,.compare-grid{grid-template-columns:minmax(0,1fr)}.table-foot{align-items:flex-start;flex-direction:column}.page-head{align-items:flex-start}}
</style>
