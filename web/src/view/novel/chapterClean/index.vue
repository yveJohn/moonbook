<template>
  <div class="p-4">
    <div class="gva-search-box clean-toolbar">
      <el-tabs v-model="tab">
        <el-tab-pane label="清洗任务" name="tasks" />
        <el-tab-pane label="结果审核" name="results" />
        <el-tab-pane label="简介补全" name="summary" />
      </el-tabs>
      <div class="toolbar-actions">
        <el-button :icon="Setting" @click="openConfig">配置</el-button>
        <el-button type="primary" :icon="Plus" @click="openStart">
          {{ tab === 'summary' ? '启动补全' : '启动清洗' }}
        </el-button>
      </div>
    </div>

    <div v-if="tab === 'summary'" class="summary-strip">
      <span>待补全 <strong>{{ summaryStatus.pendingCount || 0 }}</strong></span>
      <span>自动补全 <el-tag size="small" :type="summaryStatus.enabled ? 'success' : 'info'">{{ summaryStatus.enabled ? '已启用' : '未启用' }}</el-tag></span>
      <span>运行状态 <el-tag size="small" :type="summaryStatus.running ? 'warning' : 'info'">{{ summaryStatus.running ? '执行中' : '空闲' }}</el-tag></span>
    </div>

    <div class="gva-table-box">
      <el-table v-if="tab === 'tasks'" v-loading="loading" :data="tasks" row-key="id">
        <el-table-column label="任务 ID" width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
        <el-table-column prop="bookName" label="书籍" min-width="180" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100"><template #default="{ row }"><el-tag :type="taskType(row.status)">{{ taskLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column label="进度" width="150"><template #default="{ row }">{{ row.processedCount }} / {{ row.totalCount }}</template></el-table-column>
        <el-table-column prop="successCount" label="成功" width="75" />
        <el-table-column prop="discardCount" label="丢弃" width="75" />
        <el-table-column prop="failCount" label="失败" width="75" />
        <el-table-column prop="errorSummary" label="错误摘要" min-width="180" show-overflow-tooltip />
        <el-table-column label="操作" width="100" fixed="right"><template #default="{ row }"><el-tooltip v-if="row.status === 'running'" content="停止"><el-button link type="danger" :icon="VideoPause" @click="stopClean(row.id)" /></el-tooltip><el-tooltip v-else content="续跑"><el-button link type="primary" :icon="RefreshRight" @click="resumeClean(row.id)" /></el-tooltip></template></el-table-column>
      </el-table>

      <el-table v-else-if="tab === 'results'" v-loading="loading" :data="results" row-key="id">
        <el-table-column label="结果 ID" width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
        <el-table-column prop="bookName" label="书籍" min-width="150" />
        <el-table-column prop="chapterName" label="章节" min-width="190" show-overflow-tooltip />
        <el-table-column prop="contentType" label="类型" width="110" />
        <el-table-column prop="cleanedWordCount" label="清洗字数" width="90" />
        <el-table-column prop="status" label="状态" width="130" />
        <el-table-column label="操作" width="120" fixed="right"><template #default="{ row }"><el-tooltip content="审核"><el-button link type="primary" :icon="View" @click="openReview(row.id)" /></el-tooltip><el-tooltip v-if="['failed','discarded','manual_discarded'].includes(row.status)" content="重洗"><el-button link type="warning" :icon="RefreshRight" @click="reclean(row.id)" /></el-tooltip></template></el-table-column>
      </el-table>

      <el-table v-else v-loading="loading" :data="summaryTasks" row-key="id">
        <el-table-column label="任务 ID" width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
        <el-table-column prop="status" label="状态" width="100"><template #default="{ row }"><el-tag :type="taskType(row.status)">{{ taskLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column label="触发方式" width="100"><template #default="{ row }">{{ row.automatic ? '自动' : '人工' }}</template></el-table-column>
        <el-table-column label="进度" width="150"><template #default="{ row }">{{ row.processedCount }} / {{ row.totalCount }}</template></el-table-column>
        <el-table-column prop="successCount" label="成功" width="75" />
        <el-table-column prop="failCount" label="失败" width="75" />
        <el-table-column label="当前位置" min-width="210"><template #default="{ row }"><code v-if="row.currentChapterId">章节 {{ row.currentChapterId }}</code><span v-else>-</span></template></el-table-column>
        <el-table-column prop="errorSummary" label="最近错误" min-width="200" show-overflow-tooltip />
        <el-table-column label="操作" width="110" fixed="right"><template #default="{ row }"><el-tooltip content="详情"><el-button link type="primary" :icon="View" @click="openSummaryTask(row.id)" /></el-tooltip><el-tooltip v-if="row.status === 'running'" content="停止"><el-button link type="danger" :icon="VideoPause" @click="stopSummary(row.id)" /></el-tooltip><el-tooltip v-else-if="['stopped','failed'].includes(row.status)" content="续跑"><el-button link type="primary" :icon="RefreshRight" @click="resumeSummary(row.id)" /></el-tooltip></template></el-table-column>
      </el-table>

      <div class="gva-pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @change="load" /></div>
    </div>

    <el-dialog v-model="startVisible" :title="tab === 'summary' ? '启动章节简介补全' : '启动章节清洗'" width="480px">
      <el-form label-width="90px">
        <template v-if="tab !== 'summary'">
          <el-form-item label="书籍 ID"><el-input v-model="startForm.bookId" inputmode="numeric" placeholder="请输入完整书籍 ID" /></el-form-item>
          <el-form-item label="强制重洗"><el-switch v-model="startForm.forceReclean" /></el-form-item>
        </template>
        <el-form-item label="操作人"><el-input v-model="startForm.operatorName" maxlength="64" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="startVisible = false">取消</el-button><el-button type="primary" @click="start">启动</el-button></template>
    </el-dialog>

    <el-dialog v-model="configVisible" title="章节清洗配置" width="min(760px, 94vw)">
      <el-form label-width="150px">
        <el-form-item label="启用"><el-switch v-model="config.enabled" /></el-form-item>
        <el-form-item label="AI 配置"><el-select v-model="config.aiConfigId" class="w-full"><el-option v-for="item in aiOptions" :key="item.id" :label="`${item.configName} / ${item.currentModelName}`" :value="item.id" /></el-select></el-form-item>
        <el-form-item label="拒答备用配置"><el-select v-model="config.refusalFallbackAiConfigId" clearable class="w-full"><el-option v-for="item in aiOptions" :key="item.id" :label="`${item.configName} / ${item.currentModelName}`" :value="item.id" /></el-select></el-form-item>
        <el-form-item label="系统提示词"><el-input v-model="config.systemPrompt" type="textarea" :rows="5" /></el-form-item>
        <div class="config-grid">
          <el-form-item label="温度"><el-input-number v-model="config.temperature" :min="0" :max="2" :step="0.1" /></el-form-item>
          <el-form-item label="最大输出 Token"><el-input-number v-model="config.maxTokens" :min="0" /></el-form-item>
          <el-form-item label="超时（秒）"><el-input-number v-model="config.timeoutSeconds" :min="1" :max="1800" /></el-form-item>
          <el-form-item label="最低章节字数"><el-input-number v-model="config.minChapterWordCount" :min="0" /></el-form-item>
          <el-form-item label="最低保留比例"><el-input-number v-model="config.minCleanedTextPercent" :min="0" :max="100" /></el-form-item>
          <el-form-item label="原文成功阈值"><el-input-number v-model="config.autoSuccessWordCount" :min="0" /></el-form-item>
          <el-form-item label="失败重试"><el-input-number v-model="config.retryCount" :min="0" :max="20" /></el-form-item>
          <el-form-item label="失败后继续"><el-switch v-model="config.continueOnFailure" /></el-form-item>
        </div>
        <el-form-item label="请求间隔（毫秒）"><el-input-number v-model="config.requestIntervalMs" :min="0" :max="600000" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="configVisible = false">取消</el-button><el-button type="primary" @click="saveCleanConfig">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="summaryConfigVisible" title="章节简介补全配置" width="min(760px, 94vw)">
      <el-form label-width="150px">
        <el-form-item label="自动补全"><el-switch v-model="summaryConfig.enabled" /></el-form-item>
        <el-form-item label="AI 配置"><el-select v-model="summaryConfig.aiConfigId" class="w-full"><el-option v-for="item in aiOptions" :key="item.id" :label="`${item.configName} / ${item.currentModelName}`" :value="item.id" /></el-select></el-form-item>
        <el-form-item label="拒答备用配置"><el-select v-model="summaryConfig.refusalFallbackAiConfigId" clearable class="w-full"><el-option v-for="item in aiOptions" :key="item.id" :label="`${item.configName} / ${item.currentModelName}`" :value="item.id" /></el-select></el-form-item>
        <el-form-item label="系统提示词"><el-input v-model="summaryConfig.systemPrompt" type="textarea" :rows="5" /></el-form-item>
        <div class="config-grid">
          <el-form-item label="温度"><el-input-number v-model="summaryConfig.temperature" :min="0" :max="2" :step="0.1" /></el-form-item>
          <el-form-item label="最大输出 Token"><el-input-number v-model="summaryConfig.maxTokens" :min="0" /></el-form-item>
          <el-form-item label="最大输入字符"><el-input-number v-model="summaryConfig.maxInputChars" :min="1" :max="1000000" /></el-form-item>
          <el-form-item label="超时（秒）"><el-input-number v-model="summaryConfig.timeoutSeconds" :min="1" :max="1800" /></el-form-item>
          <el-form-item label="请求间隔（毫秒）"><el-input-number v-model="summaryConfig.requestIntervalMs" :min="0" :max="600000" /></el-form-item>
          <el-form-item label="失败重试"><el-input-number v-model="summaryConfig.retryCount" :min="0" :max="20" /></el-form-item>
          <el-form-item label="每批数量"><el-input-number v-model="summaryConfig.batchSize" :min="1" :max="1000" /></el-form-item>
        </div>
      </el-form>
      <template #footer><el-button @click="summaryConfigVisible = false">取消</el-button><el-button type="primary" @click="saveSummaryConfig">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="reviewVisible" title="清洗结果审核" width="min(1000px, 96vw)" top="4vh">
      <el-descriptions v-if="review" :column="2" border><el-descriptions-item label="书籍">{{ review.bookName }}</el-descriptions-item><el-descriptions-item label="章节">{{ review.chapterName }}</el-descriptions-item><el-descriptions-item label="AI 类型">{{ review.contentType }}</el-descriptions-item><el-descriptions-item label="置信度">{{ review.confidence ?? '-' }}</el-descriptions-item></el-descriptions>
      <div v-if="review" class="review-grid"><el-form-item label="原文"><el-input :model-value="review.originalText" type="textarea" :rows="18" readonly /></el-form-item><el-form-item label="清洗稿"><el-input v-model="review.cleanedText" type="textarea" :rows="18" /></el-form-item></div>
      <template #footer><el-button type="danger" @click="submitReview('manual_discarded')">丢弃章节</el-button><el-button type="primary" @click="submitReview('success')">采用清洗稿</el-button></template>
    </el-dialog>

    <el-dialog v-model="summaryTaskVisible" title="简介补全任务详情" width="min(820px, 94vw)">
      <el-descriptions v-if="summaryTaskDetail" :column="2" border>
        <el-descriptions-item label="任务 ID"><code>{{ summaryTaskDetail.id }}</code></el-descriptions-item>
        <el-descriptions-item label="状态">{{ taskLabel(summaryTaskDetail.status) }}</el-descriptions-item>
        <el-descriptions-item label="当前结果 ID"><code>{{ summaryTaskDetail.currentResultId || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="当前章节 ID"><code>{{ summaryTaskDetail.currentChapterId || '-' }}</code></el-descriptions-item>
        <el-descriptions-item label="最近错误" :span="2">{{ summaryTaskDetail.latestErrorMessage || '-' }}</el-descriptions-item>
      </el-descriptions>
      <el-form-item v-if="summaryTaskDetail" class="raw-response" label="最近原始响应">
        <el-input :model-value="summaryTaskDetail.latestRawResponse || ''" type="textarea" :rows="12" readonly />
      </el-form-item>
    </el-dialog>
  </div>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import { Plus, RefreshRight, Setting, VideoPause, View } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listEnabledAIConfigs } from '@/api/novel/aiConfigs'
import { getChapterCleanConfig, getChapterCleanResult, listChapterCleanResults, listChapterCleanTasks, recleanChapterCleanResult, resumeChapterCleanTask, reviewChapterCleanResult, saveChapterCleanConfig, startChapterCleanTask, stopChapterCleanTask } from '@/api/novel/chapterClean'
import { getChapterSummaryConfig, getChapterSummaryStatus, getChapterSummaryTask, listChapterSummaryTasks, resumeChapterSummaryTask, saveChapterSummaryConfig, startChapterSummaryTask, stopChapterSummaryTask } from '@/api/novel/chapterSummary'
import { assertLongId } from '@/utils/longId'

defineOptions({ name: 'NovelChapterClean' })

const tab = ref('tasks')
const loading = ref(false)
const tasks = ref([])
const results = ref([])
const summaryTasks = ref([])
const summaryStatus = reactive({})
const total = ref(0)
const query = reactive({ page: 1, pageSize: 20 })
const startVisible = ref(false)
const startForm = reactive({ bookId: '', forceReclean: false, operatorName: '' })
const configVisible = ref(false)
const config = reactive({})
const summaryConfigVisible = ref(false)
const summaryConfig = reactive({})
const aiOptions = ref([])
const reviewVisible = ref(false)
const review = ref(null)
const summaryTaskVisible = ref(false)
const summaryTaskDetail = ref(null)

const load = async () => {
  loading.value = true
  try {
    if (tab.value === 'tasks') {
      const response = await listChapterCleanTasks(query)
      tasks.value = response.data?.list || []
      total.value = response.data?.total || 0
    } else if (tab.value === 'results') {
      const response = await listChapterCleanResults(query)
      results.value = response.data?.list || []
      total.value = response.data?.total || 0
    } else {
      const [taskResponse, statusResponse] = await Promise.all([listChapterSummaryTasks(query), getChapterSummaryStatus()])
      summaryTasks.value = taskResponse.data?.list || []
      total.value = taskResponse.data?.total || 0
      Object.assign(summaryStatus, statusResponse.data || {})
    }
  } finally {
    loading.value = false
  }
}

watch(tab, () => { query.page = 1; load() })

const openStart = () => { startVisible.value = true }
const start = async () => {
  if (tab.value === 'summary') await startChapterSummaryTask({ operatorName: startForm.operatorName })
  else await startChapterCleanTask({ ...startForm, bookId: assertLongId(startForm.bookId) })
  startVisible.value = false
  ElMessage.success('任务已启动')
  load()
}

const openConfig = async () => {
  const options = await listEnabledAIConfigs()
  aiOptions.value = options.data || []
  if (tab.value === 'summary') {
    const response = await getChapterSummaryConfig()
    Object.assign(summaryConfig, response.data || {})
    summaryConfigVisible.value = true
  } else {
    const response = await getChapterCleanConfig()
    Object.assign(config, response.data || {})
    configVisible.value = true
  }
}

const saveCleanConfig = async () => {
  const payload = { ...config, aiConfigId: assertLongId(config.aiConfigId) }
  if (payload.refusalFallbackAiConfigId) payload.refusalFallbackAiConfigId = assertLongId(payload.refusalFallbackAiConfigId)
  await saveChapterCleanConfig(payload)
  configVisible.value = false
  ElMessage.success('配置已保存')
}

const saveSummaryConfig = async () => {
  const payload = { ...summaryConfig, aiConfigId: assertLongId(summaryConfig.aiConfigId) }
  if (payload.refusalFallbackAiConfigId) payload.refusalFallbackAiConfigId = assertLongId(payload.refusalFallbackAiConfigId)
  await saveChapterSummaryConfig(payload)
  summaryConfigVisible.value = false
  ElMessage.success('配置已保存')
  load()
}

const stopClean = async (id) => { await ElMessageBox.confirm('确定停止该任务吗？', '停止任务', { type: 'warning' }); await stopChapterCleanTask(assertLongId(id)); load() }
const resumeClean = async (id) => { await resumeChapterCleanTask(assertLongId(id)); load() }
const stopSummary = async (id) => { await ElMessageBox.confirm('确定停止简介补全任务吗？', '停止任务', { type: 'warning' }); await stopChapterSummaryTask(assertLongId(id)); load() }
const resumeSummary = async (id) => { await resumeChapterSummaryTask(assertLongId(id)); load() }
const openSummaryTask = async (id) => { const response = await getChapterSummaryTask(assertLongId(id)); summaryTaskDetail.value = response.data; summaryTaskVisible.value = true }
const openReview = async (id) => { const response = await getChapterCleanResult(assertLongId(id)); review.value = response.data; reviewVisible.value = true }
const submitReview = async (status) => { await reviewChapterCleanResult(assertLongId(review.value.id), { status, cleanedChapterName: review.value.cleanedChapterName, cleanedText: review.value.cleanedText }); reviewVisible.value = false; ElMessage.success('审核完成'); load() }
const reclean = async (id) => { await recleanChapterCleanResult(assertLongId(id)); ElMessage.success('已重置'); load() }
const taskLabel = (value) => ({ running: '执行中', completed: '已完成', stopped: '已停止', failed: '失败' })[value] || value
const taskType = (value) => ({ running: 'warning', completed: 'success', stopped: 'info', failed: 'danger' })[value] || 'info'

load()
</script>

<style scoped>
.clean-toolbar,.toolbar-actions,.summary-strip{display:flex;align-items:center}.clean-toolbar{justify-content:space-between}.toolbar-actions{gap:8px}.clean-toolbar :deep(.el-tabs__header){margin:0}.summary-strip{gap:28px;padding:0 4px 14px;color:var(--el-text-color-regular)}.summary-strip span{display:flex;align-items:center;gap:8px}.config-grid,.review-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 16px}.review-grid,.raw-response{margin-top:16px}@media(max-width:720px){.clean-toolbar{align-items:flex-start;gap:12px;flex-direction:column}.summary-strip{align-items:flex-start;gap:8px;flex-direction:column}.config-grid,.review-grid{grid-template-columns:1fr}}
</style>
