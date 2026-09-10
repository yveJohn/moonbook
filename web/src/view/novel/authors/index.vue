<template>
  <div class="p-4">
    <div class="gva-search-box">
      <el-form :inline="true" @submit.prevent="resetAndLoad">
        <el-form-item label="作者">
          <el-input v-model="query.keyword" clearable placeholder="笔名" @keyup.enter="resetAndLoad" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" clearable placeholder="全部" style="width: 130px">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
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
        <el-button type="primary" :icon="Plus" @click="openCreate">新增作者</el-button>
      </div>
      <el-table v-table-display v-loading="loading" :data="rows" row-key="id">
        <el-table-column label="笔名" prop="penName" min-width="180" />
        <el-table-column label="状态" width="110">
          <template #default="scope">
            <el-tag :type="statusType(scope.row.status)">{{ statusLabel(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="作品方向" width="110">
          <template #default="scope">{{ directionLabel(scope.row.workDirection) }}</template>
        </el-table-column>
        <el-table-column label="来源" prop="source" width="150" />
        <el-table-column label="历史作者 ID" min-width="170">
          <template #default="scope">{{ scope.row.legacyAuthorId || scope.row.legacyBookAuthorId || '-' }}</template>
        </el-table-column>
        <el-table-column label="更新时间" min-width="180">
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
          <span class="text-base">{{ editingId ? '编辑作者' : '新增作者' }}</span>
          <div>
            <el-button @click="drawerVisible = false">取消</el-button>
            <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
          </div>
        </div>
      </template>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="96px">
        <el-form-item label="作者笔名" prop="penName">
          <el-input v-model="form.penName" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="form.status" class="w-full">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="作品方向">
          <el-select v-model="form.workDirection" clearable placeholder="不限" class="w-full">
            <el-option label="男频" value="0" />
            <el-option label="女频" value="1" />
          </el-select>
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
    createNovelAuthor,
    deleteNovelAuthor,
    listNovelAuthors,
    updateNovelAuthor
  } from '@/api/novel/metadata'

  defineOptions({ name: 'NovelAuthors' })

  const statusOptions = [
    { label: '待审核', value: 'pending' },
    { label: '正常', value: 'active' },
    { label: '封禁', value: 'blocked' }
  ]
  const query = reactive({ page: 1, pageSize: 10, keyword: '', status: '' })
  const rows = ref([])
  const total = ref(0)
  const loading = ref(false)
  const drawerVisible = ref(false)
  const submitting = ref(false)
  const editingId = ref('')
  const formRef = ref()
  const defaultForm = () => ({ penName: '', status: 'active', workDirection: '' })
  const form = reactive(defaultForm())
  const rules = { penName: [{ required: true, message: '请输入作者笔名', trigger: 'blur' }] }

  const loadData = async () => {
    loading.value = true
    try {
      const response = await listNovelAuthors({ ...query })
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
    Object.assign(query, { page: 1, pageSize: 10, keyword: '', status: '' })
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
    Object.assign(form, { penName: row.penName, status: row.status, workDirection: row.workDirection || '' })
    drawerVisible.value = true
  }
  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      const payload = { ...form, workDirection: form.workDirection || null }
      const response = editingId.value
        ? await updateNovelAuthor(editingId.value, payload)
        : await createNovelAuthor(payload)
      if (response.code === 0) {
        ElMessage.success(editingId.value ? '作者已更新' : '作者已创建')
        drawerVisible.value = false
        loadData()
      }
    } finally {
      submitting.value = false
    }
  }
  const removeRow = async (row) => {
    await ElMessageBox.confirm(`确定删除作者“${row.penName}”吗？`, '删除作者', { type: 'warning' })
    const response = await deleteNovelAuthor(row.id)
    if (response.code === 0) {
      ElMessage.success('作者已删除')
      if (rows.value.length === 1 && query.page > 1) query.page -= 1
      loadData()
    }
  }
  const statusLabel = (value) => ({ pending: '待审核', active: '正常', blocked: '封禁' })[value] || value
  const statusType = (value) => ({ pending: 'warning', active: 'success', blocked: 'danger' })[value] || 'info'
  const directionLabel = (value) => ({ 0: '男频', 1: '女频' })[value] || '不限'
  const formatTime = (value) => (value ? adminDateTime(value) : '-')

  loadData()
</script>
