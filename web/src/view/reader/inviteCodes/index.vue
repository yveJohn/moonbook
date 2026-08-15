<template>
  <div class="gva-form-box invite-page">
    <div class="page-head">
      <div>
        <h2>邀请码</h2>
        <p>生成和管理读者注册邀请码，读者自动邀请码不可删除。</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">生成邀请码</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="keyword" clearable placeholder="搜索邀请码" @keyup.enter="load" />
      <el-button :icon="Search" @click="load">查询</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column label="邀请码 ID" min-width="180">
        <template #default="{ row }"><code>{{ row.id }}</code></template>
      </el-table-column>
      <el-table-column prop="code" label="邀请码" min-width="180" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }">{{ row.inviterReaderId ? '读者自动' : '人工' }}</template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column label="使用次数" width="130">
        <template #default="{ row }">{{ row.usedCount }} / {{ row.maxUseCount || '不限' }}</template>
      </el-table-column>
      <el-table-column prop="expiresAt" label="过期时间" min-width="180" />
      <el-table-column prop="remark" label="备注" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <el-switch
            :model-value="row.status === 'enabled'"
            :disabled="row.status === 'expired'"
            @change="toggle(row)"
          />
          <el-tooltip content="编辑" placement="top">
            <el-button link type="primary" :icon="Edit" aria-label="编辑" @click="openEdit(row)" />
          </el-tooltip>
          <el-tooltip content="删除" placement="top">
            <el-button
              link
              type="danger"
              :icon="Delete"
              :disabled="Boolean(row.inviterReaderId) || row.usedCount !== '0'"
              aria-label="删除"
              @click="remove(row)"
            />
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @size-change="load"
        @current-change="load"
      />
    </div>

    <el-dialog
      v-model="dialog"
      :title="editingId ? '编辑邀请码' : '生成邀请码'"
      width="min(460px, calc(100vw - 24px))"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="邀请码" prop="code">
          <el-input
            v-model="form.code"
            :disabled="editingAutomatic"
            maxlength="64"
            :placeholder="editingId ? '请输入邀请码' : '留空则自动生成'"
          />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="form.status">
            <el-radio value="enabled">启用</el-radio>
            <el-radio value="disabled">停用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="最大使用次数" prop="maxUseCount">
          <el-input
            v-model="form.maxUseCount"
            :disabled="editingAutomatic"
            inputmode="numeric"
            placeholder="留空表示不限"
          />
        </el-form-item>
        <el-form-item label="过期时间" prop="expiresAt">
          <el-date-picker
            v-model="form.expiresAt"
            :disabled="editingAutomatic"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            clearable
          />
        </el-form-item>
        <el-form-item label="备注" prop="remark">
          <el-input v-model="form.remark" type="textarea" maxlength="255" :rows="3" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus, Search } from '@element-plus/icons-vue'
import {
  createInviteCode,
  deleteInviteCode,
  listInviteCodes,
  setInviteCodeStatus,
  updateInviteCode
} from '@/api/reader/inviteCodes'

defineOptions({ name: 'ReaderInviteCodes' })

const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const loading = ref(false)
const dialog = ref(false)
const saving = ref(false)
const formRef = ref()
const editingId = ref('')
const editingAutomatic = ref(false)
const form = reactive({ code: '', status: 'enabled', maxUseCount: '', expiresAt: '', remark: '' })
const rules = {
  code: [
    {
      validator: (_rule, value, callback) => {
        if (editingId.value && !String(value || '').trim()) callback(new Error('请输入邀请码'))
        else callback()
      },
      trigger: 'blur'
    }
  ],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }],
  maxUseCount: [
    {
      validator: (_rule, value, callback) => {
        if (!value || /^[1-9]\d*$/.test(value)) callback()
        else callback(new Error('请输入正整数字符串'))
      },
      trigger: 'blur'
    }
  ]
}

const load = async () => {
  loading.value = true
  try {
    const res = await listInviteCodes({ keyword: keyword.value, page: page.value, pageSize: pageSize.value })
    rows.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}

const resetForm = () => {
  editingId.value = ''
  editingAutomatic.value = false
  Object.assign(form, { code: '', status: 'enabled', maxUseCount: '', expiresAt: '', remark: '' })
  formRef.value?.clearValidate()
}

const openCreate = () => {
  resetForm()
  dialog.value = true
}

const normalizeExpiresAt = (value) => {
  if (!value) return ''
  const date = new Date(value)
  return date.toString() === 'Invalid Date' ? '' : date.toISOString().replace(/\.\d{3}Z$/, 'Z')
}

const openEdit = (row) => {
  editingId.value = row.id
  editingAutomatic.value = Boolean(row.inviterReaderId)
  Object.assign(form, {
    code: row.code || '',
    status: row.status === 'expired' ? 'disabled' : row.status || 'enabled',
    maxUseCount: row.maxUseCount || '',
    expiresAt: normalizeExpiresAt(row.expiresAt),
    remark: row.remark || ''
  })
  formRef.value?.clearValidate()
  dialog.value = true
}

const save = async () => {
  if (!(await formRef.value?.validate().catch(() => false))) return
  saving.value = true
  try {
    const payload = {
      code: form.code.trim(),
      status: form.status,
      maxUseCount: form.maxUseCount.trim(),
      expiresAt: form.expiresAt || '',
      remark: form.remark.trim()
    }
    const res = editingId.value
      ? await updateInviteCode(editingId.value, payload)
      : await createInviteCode(payload)
    ElMessage.success(`邀请码 ${res.data?.code || ''} 已保存`)
    dialog.value = false
    await load()
  } finally {
    saving.value = false
  }
}

const toggle = async (row) => {
  const next = row.status === 'enabled' ? 'disabled' : 'enabled'
  try {
    await setInviteCodeStatus(row.id, next)
    row.status = next
    ElMessage.success('更新成功')
  } catch {
    // 全局请求拦截器负责展示错误，保留原状态。
  }
}

const remove = async (row) => {
  await ElMessageBox.confirm('确认删除该邀请码？', '删除确认')
  await deleteInviteCode(row.id)
  ElMessage.success('删除成功')
  await load()
}

load()
</script>

<style scoped>
.invite-page {
  padding: 20px;
}
.page-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
}
.page-head h2 {
  margin: 0;
}
.page-head p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
}
.toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
}
.toolbar .el-input {
  max-width: 300px;
}
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
@media (max-width: 640px) {
  .page-head {
    align-items: flex-start;
    flex-direction: column;
  }
  .toolbar .el-input {
    max-width: none;
  }
}
</style>
