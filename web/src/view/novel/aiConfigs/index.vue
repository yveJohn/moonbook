<template>
  <div class="gva-form-box ai-config-page">
    <header class="page-head">
      <h2>AI 配置</h2>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增配置</el-button>
    </header>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="名称、地址或模型" :prefix-icon="Search" @keyup.enter="resetAndLoad" />
      <el-select v-model="query.enabled" clearable placeholder="状态" @change="resetAndLoad">
        <el-option label="启用" value="true" />
        <el-option label="停用" value="false" />
      </el-select>
      <el-button :icon="Search" @click="resetAndLoad">查询</el-button>
      <el-button :icon="Refresh" aria-label="刷新" @click="load" />
    </div>

    <el-table v-loading="loading" :data="rows" border row-key="id">
      <el-table-column label="配置 ID" min-width="170"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
      <el-table-column prop="configName" label="名称" min-width="150" show-overflow-tooltip />
      <el-table-column prop="baseUrl" label="基础地址" min-width="250" show-overflow-tooltip />
      <el-table-column prop="currentModelName" label="当前模型" min-width="160" show-overflow-tooltip />
      <el-table-column label="模型" width="80" align="right"><template #default="{ row }">{{ row.modelCount }}</template></el-table-column>
      <el-table-column label="调用模式" width="105"><template #default="{ row }"><el-tag :type="streamType(row.streamMode)">{{ streamLabel(row.streamMode) }}</el-tag></template></el-table-column>
      <el-table-column label="失败进度" width="105" align="right"><template #default="{ row }">{{ row.consecutiveFailures }} / {{ row.failureThreshold }}</template></el-table-column>
      <el-table-column label="Secret" width="105"><template #default="{ row }"><el-tag :type="row.secretConfigured ? 'success' : 'warning'">{{ row.secretConfigured ? '已注入' : '未注入' }}</el-tag></template></el-table-column>
      <el-table-column label="状态" width="85"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="132" fixed="right">
        <template #default="{ row }">
          <el-tooltip content="编辑"><el-button link type="primary" :icon="Edit" aria-label="编辑" @click="openEdit(row.id)" /></el-tooltip>
          <el-tooltip content="重置模型状态"><el-button link type="primary" :icon="RefreshRight" aria-label="重置模型状态" @click="resetState(row)" /></el-tooltip>
          <el-tooltip content="删除"><el-button link type="danger" :icon="Delete" aria-label="删除" @click="remove(row)" /></el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :total="total" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" />
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑 AI 配置' : '新增 AI 配置'" width="min(760px, 96vw)" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <div class="form-grid">
          <el-form-item label="配置名称" prop="configName"><el-input v-model="form.configName" maxlength="100" show-word-limit /></el-form-item>
          <el-form-item label="调用模式" prop="streamMode">
            <el-segmented v-model="form.streamMode" :options="streamOptions" block />
          </el-form-item>
        </div>
        <el-form-item label="OpenAI 兼容基础地址" prop="baseUrl"><el-input v-model="form.baseUrl" maxlength="2048" placeholder="https://api.example.com/v1" /></el-form-item>
        <div class="form-grid">
          <el-form-item label="连续失败阈值" prop="failureThreshold"><el-input v-model="form.failureThreshold" inputmode="numeric" maxlength="4" @input="cleanThreshold" /></el-form-item>
          <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
        </div>
        <el-form-item label="Secret 环境变量" prop="secretEnvName">
          <el-input v-model="form.secretEnvName" maxlength="128" placeholder="MOONBOOK_AI_PRIMARY_API_KEY" />
        </el-form-item>

        <section class="model-editor">
          <div class="section-head"><h3>模型顺序</h3><el-button :icon="Plus" @click="addModel">添加模型</el-button></div>
          <el-form-item prop="models">
            <div class="model-list">
              <div v-for="(model, index) in form.models" :key="model.key" class="model-row">
                <span>{{ index + 1 }}</span>
                <el-input v-model="model.modelName" maxlength="100" placeholder="模型名称" />
                <el-tooltip content="上移"><el-button :icon="ArrowUp" :disabled="index === 0" aria-label="上移" @click="moveModel(index, -1)" /></el-tooltip>
                <el-tooltip content="下移"><el-button :icon="ArrowDown" :disabled="index === form.models.length - 1" aria-label="下移" @click="moveModel(index, 1)" /></el-tooltip>
                <el-tooltip content="删除"><el-button :icon="Delete" :disabled="form.models.length === 1" aria-label="删除模型" @click="removeModel(index)" /></el-tooltip>
              </div>
            </div>
          </el-form-item>
        </section>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="3" maxlength="500" show-word-limit /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
  import { reactive, ref } from 'vue'
  import { ArrowDown, ArrowUp, Delete, Edit, Plus, Refresh, RefreshRight, Search } from '@element-plus/icons-vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { createAIConfig, deleteAIConfig, getAIConfig, listAIConfigs, resetAIConfigModelState, updateAIConfig } from '@/api/novel/aiConfigs'
  import { assertLongId } from '@/utils/longId'

  defineOptions({ name: 'NovelAiConfigs' })

  let modelKey = 0
  const newModel = (modelName = '') => ({ key: `model-${modelKey++}`, modelName })
  const defaults = () => ({ configName: '', baseUrl: '', streamMode: 'AUTO', models: [newModel()], failureThreshold: '5', secretEnvName: '', enabled: true, remark: '' })
  const rows = ref([])
  const total = ref(0)
  const loading = ref(false)
  const saving = ref(false)
  const dialogVisible = ref(false)
  const editingId = ref('')
  const formRef = ref()
  const query = reactive({ keyword: '', enabled: '', page: 1, pageSize: 20 })
  const form = reactive(defaults())
  const streamOptions = [{ label: '自动', value: 'AUTO' }, { label: '流式', value: 'STREAM' }, { label: '非流式', value: 'NON_STREAM' }]
  const rules = {
    configName: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
    baseUrl: [{ required: true, type: 'url', message: '请输入合法 URL', trigger: 'blur' }],
    streamMode: [{ required: true, message: '请选择调用模式', trigger: 'change' }],
    failureThreshold: [{ required: true, pattern: /^(?:[1-9]|[1-9]\d{1,2}|1000)$/, message: '请输入 1 到 1000', trigger: 'blur' }],
    secretEnvName: [{ pattern: /^(?:|MOONBOOK_AI_[A-Z0-9_]+_API_KEY)$/, message: '格式应为 MOONBOOK_AI_*_API_KEY', trigger: 'blur' }],
    models: [{ validator: (_rule, value, callback) => { const names=(value||[]).map(item=>item.modelName.trim()); callback(names.length && names.every(Boolean) && new Set(names).size===names.length ? undefined : new Error('模型名称不能为空或重复')) }, trigger: 'change' }]
  }

  const load = async () => { loading.value=true; try { const response=await listAIConfigs(query); rows.value=response.data?.list||[]; total.value=response.data?.total||0 } finally { loading.value=false } }
  const resetAndLoad = () => { query.page=1; load() }
  const openCreate = () => { editingId.value=''; Object.assign(form, defaults()); dialogVisible.value=true }
  const openEdit = async (id) => { const safeId=assertLongId(id); const response=await getAIConfig(safeId); const data=response.data; editingId.value=safeId; Object.assign(form, { configName:data.configName, baseUrl:data.baseUrl, streamMode:data.streamMode, models:data.models.map(item=>newModel(item.modelName)), failureThreshold:String(data.failureThreshold), secretEnvName:data.secretEnvName, enabled:data.enabled, remark:data.remark }); dialogVisible.value=true }
  const cleanThreshold = (value) => { form.failureThreshold=String(value||'').replace(/\D/g,'').replace(/^0+/,'').slice(0,4) }
  const addModel = () => form.models.push(newModel())
  const removeModel = (index) => { if(form.models.length>1)form.models.splice(index,1) }
  const moveModel = (index, offset) => { const target=index+offset; if(target<0||target>=form.models.length)return; const [item]=form.models.splice(index,1); form.models.splice(target,0,item) }
  const payload = () => ({ configName:form.configName, baseUrl:form.baseUrl, streamMode:form.streamMode, models:form.models.map(item=>({modelName:item.modelName.trim()})), failureThreshold:form.failureThreshold, secretEnvName:form.secretEnvName, enabled:form.enabled, remark:form.remark })
  const save = async () => { if(!(await formRef.value?.validate().catch(()=>false)))return; saving.value=true; try { if(editingId.value)await updateAIConfig(assertLongId(editingId.value),payload());else await createAIConfig(payload()); ElMessage.success('AI 配置已保存'); dialogVisible.value=false; load() } finally { saving.value=false } }
  const resetState = async (row) => { await ElMessageBox.confirm(`将“${row.configName}”切回首个模型并清零失败计数，确定继续吗？`,'重置模型状态',{type:'warning'}); await resetAIConfigModelState(assertLongId(row.id)); ElMessage.success('模型状态已重置'); load() }
  const remove = async (row) => { await ElMessageBox.confirm(`确定删除“${row.configName}”吗？`,'删除确认',{type:'warning'}); await deleteAIConfig(assertLongId(row.id)); ElMessage.success('删除成功'); load() }
  const streamLabel = (value) => ({AUTO:'自动',STREAM:'流式',NON_STREAM:'非流式'})[value]||value
  const streamType = (value) => ({AUTO:'primary',STREAM:'success',NON_STREAM:'info'})[value]||'info'
  load()
</script>

<style scoped>
  .ai-config-page{padding:20px}.page-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:18px}.page-head h2,.section-head h3{margin:0;letter-spacing:0}.page-head h2{font-size:20px}.toolbar{display:grid;grid-template-columns:minmax(220px,360px) 140px auto auto;gap:10px;margin-bottom:14px}.pager{display:flex;justify-content:flex-end;margin-top:16px}.form-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:0 18px}.model-editor{margin:6px 0 18px;padding-top:16px;border-top:1px solid var(--el-border-color-lighter)}.section-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:12px}.section-head h3{font-size:15px}.model-list{display:grid;gap:8px;width:100%}.model-row{display:grid;grid-template-columns:28px minmax(0,1fr) 40px 40px 40px;gap:8px;align-items:center}.model-row>span{text-align:center;color:var(--el-text-color-secondary);font-variant-numeric:tabular-nums}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}@media(max-width:720px){.ai-config-page{padding:12px}.toolbar,.form-grid{grid-template-columns:minmax(0,1fr)}.model-row{grid-template-columns:24px minmax(0,1fr) 36px 36px 36px}}
</style>
