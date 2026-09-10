<template>
  <div class="p-4">
    <div class="gva-search-box">
      <el-form :inline="true" @submit.prevent="loadData">
        <el-form-item label="分类类型">
          <el-segmented v-model="query.kind" :options="kindOptions" @change="resetAndLoad" />
        </el-form-item>
        <el-form-item label="关键词">
          <el-input v-model="query.keyword" clearable placeholder="编码或名称" @keyup.enter="resetAndLoad" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.enabled" clearable placeholder="全部" style="width: 110px">
            <el-option label="启用" value="true" />
            <el-option label="停用" value="false" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="resetAndLoad">查询</el-button>
          <el-button :icon="Refresh" @click="resetQuery">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-button type="primary" :icon="Plus" @click="openCreate">新增分类</el-button>
      </div>
      <el-table v-table-display v-loading="loading" :data="rows" row-key="id">
        <el-table-column label="名称" prop="name" min-width="160" />
        <el-table-column label="编码" prop="code" min-width="180" show-overflow-tooltip />
        <el-table-column label="类型" width="110">
          <template #default="scope">
            <el-tag :type="scope.row.kind === 'primary' ? 'primary' : 'info'">
              {{ scope.row.kind === 'primary' ? '一级分类' : '二级分类' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="作品方向" prop="workDirection" width="120">
          <template #default="scope">{{ directionLabel(scope.row.workDirection) }}</template>
        </el-table-column>
        <el-table-column label="排序" prop="sort" width="90" />
        <el-table-column label="状态" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.enabled ? 'success' : 'info'">{{ scope.row.enabled ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="来源" prop="source" width="130" />
        <el-table-column label="更新时间" prop="updatedAt" min-width="180">
          <template #default="scope">{{ formatTime(scope.row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="scope">
            <el-tooltip content="编辑" placement="top">
              <el-button link type="primary" :icon="Edit" aria-label="编辑" @click="openEdit(scope.row)" />
            </el-tooltip>
            <el-tooltip content="删除" placement="top">
              <el-button link type="danger" :icon="Delete" aria-label="删除" @click="removeRow(scope.row)" />
            </el-tooltip>
          </template>
        </el-table-column>
      </el-table>
      <div class="gva-pagination">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.pageSize"
          :page-sizes="[10, 30, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="loadData"
          @size-change="resetAndLoad"
        />
      </div>
    </div>

    <el-drawer v-model="drawerVisible" :show-close="false" size="440px" @closed="resetForm">
      <template #header>
        <div class="flex justify-between items-center w-full">
          <span class="text-base">{{ editingId ? '编辑分类' : '新增分类' }}</span>
          <div>
            <el-button @click="drawerVisible = false">取消</el-button>
            <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
          </div>
        </div>
      </template>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="96px">
        <el-form-item label="分类类型" prop="kind">
          <el-segmented v-model="form.kind" :options="kindOptions" class="w-full" />
        </el-form-item>
        <el-form-item label="分类名称" prop="name">
          <el-input v-model="form.name" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="分类编码" prop="code">
          <el-input v-model="form.code" maxlength="64" placeholder="稳定业务键，例如 fantasy" />
        </el-form-item>
        <el-form-item label="作品方向">
          <el-select v-model="form.workDirection" clearable placeholder="不限" class="w-full">
            <el-option label="男频" value="0" />
            <el-option label="女频" value="1" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序" prop="sort">
          <el-input-number v-model="form.sort" :min="0" :max="999999" class="w-full" />
        </el-form-item>
        <el-form-item label="状态" prop="enabled">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>
      </el-form>
    </el-drawer>
  </div>
</template>

<script setup>
import { adminDateTime } from '@/utils/adminDisplay'
  import { reactive, ref } from 'vue'
  import { Delete, Edit, Plus, Refresh, Search } from '@element-plus/icons-vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import {
    createNovelCategory,
    deleteNovelCategory,
    listNovelCategories,
    updateNovelCategory
  } from '@/api/novel/metadata'

  defineOptions({ name: 'NovelMetadata' })

  const kindOptions = [
    { label: '一级分类', value: 'primary' },
    { label: '二级分类', value: 'sub' }
  ]
  const query = reactive({ page: 1, pageSize: 10, keyword: '', kind: 'primary', enabled: '' })
  const rows = ref([])
  const total = ref(0)
  const loading = ref(false)
  const drawerVisible = ref(false)
  const submitting = ref(false)
  const editingId = ref('')
  const formRef = ref()
  const defaultForm = () => ({ code: '', name: '', kind: query.kind, workDirection: '', sort: 0, enabled: true })
  const form = reactive(defaultForm())
  const rules = {
    name: [{ required: true, message: '请输入分类名称', trigger: 'blur' }],
    code: [
      { required: true, message: '请输入分类编码', trigger: 'blur' },
      { pattern: /^[A-Za-z0-9][A-Za-z0-9_-]*$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
    ],
    kind: [{ required: true, message: '请选择分类类型', trigger: 'change' }]
  }

  const loadData = async () => {
    loading.value = true
    try {
      const params = { ...query }
      if (params.enabled === '') delete params.enabled
      const response = await listNovelCategories(params)
      if (response.code === 0) {
        rows.value = response.data.list || []
        total.value = response.data.total || 0
      }
    } finally {
      loading.value = false
    }
  }
  const resetAndLoad = () => {
    query.page = 1
    loadData()
  }
  const resetQuery = () => {
    Object.assign(query, { page: 1, pageSize: 10, keyword: '', kind: 'primary', enabled: '' })
    loadData()
  }
  const resetForm = () => {
    editingId.value = ''
    Object.assign(form, defaultForm())
    formRef.value?.clearValidate()
  }
  const openCreate = () => {
    resetForm()
    drawerVisible.value = true
  }
  const openEdit = (row) => {
    editingId.value = row.id
    Object.assign(form, {
      code: row.code,
      name: row.name,
      kind: row.kind,
      workDirection: row.workDirection || '',
      sort: row.sort,
      enabled: row.enabled
    })
    drawerVisible.value = true
  }
  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      const payload = { ...form, workDirection: form.workDirection || null }
      const response = editingId.value
        ? await updateNovelCategory(editingId.value, payload)
        : await createNovelCategory(payload)
      if (response.code === 0) {
        ElMessage.success(editingId.value ? '分类已更新' : '分类已创建')
        drawerVisible.value = false
        loadData()
      }
    } finally {
      submitting.value = false
    }
  }
  const removeRow = async (row) => {
    await ElMessageBox.confirm(`确定删除分类“${row.name}”吗？`, '删除分类', { type: 'warning' })
    const response = await deleteNovelCategory(row.id)
    if (response.code === 0) {
      ElMessage.success('分类已删除')
      if (rows.value.length === 1 && query.page > 1) query.page -= 1
      loadData()
    }
  }
  const directionLabel = (value) => ({ 0: '男频', 1: '女频' })[value] || '不限'
  const formatTime = (value) => (value ? adminDateTime(value) : '-')

  loadData()
</script>
