<template>
  <div class="gva-form-box recharge-products-page">
    <div class="page-head">
      <div><h2>充值档位</h2><p>维护读者充值钻石和 USDT 定价。</p></div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增档位</el-button>
    </div>
    <div class="toolbar"><el-input v-model="keyword" clearable placeholder="搜索档位名称" @keyup.enter="load" /><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-table-display v-loading="loading" :data="rows" border>
      <el-table-column label="档位 ID" min-width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
      <el-table-column prop="productName" label="名称" min-width="160" />
      <el-table-column label="钻石数量" min-width="130"><template #default="{ row }"><code>{{ row.diamondAmount }}</code></template></el-table-column>
      <el-table-column label="价格（USDT）" min-width="140"><template #default="{ row }"><code>{{ row.priceUsdt }}</code></template></el-table-column>
      <el-table-column label="状态" width="110"><template #default="{ row }"><el-tag :type="row.saleStatus === 'on_sale' ? 'success' : 'info'">{{ row.saleStatus === 'on_sale' ? '在售' : '停售' }}</el-tag></template></el-table-column>
      <el-table-column prop="sortOrder" label="排序" width="90" />
      <el-table-column label="操作" width="170" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-popconfirm title="确认删除此充值档位？" @confirm="remove(row)"><el-button link type="danger">删除</el-button></el-popconfirm></template></el-table-column>
    </el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑充值档位' : '新增充值档位'" width="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="档位名称" prop="productName"><el-input v-model="form.productName" maxlength="100" /></el-form-item>
        <el-form-item label="钻石数量" prop="diamondAmount"><el-input v-model="form.diamondAmount" inputmode="numeric" /></el-form-item>
        <el-form-item label="USDT 价格" prop="priceUsdt"><el-input v-model="form.priceUsdt" inputmode="decimal" /></el-form-item>
        <el-form-item label="销售状态" prop="saleStatus"><el-select v-model="form.saleStatus"><el-option label="在售" value="on_sale" /><el-option label="停售" value="off_sale" /></el-select></el-form-item>
        <el-form-item label="排序" prop="sortOrder"><el-input-number v-model="form.sortOrder" :min="0" :max="99999" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
  import { reactive, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { Plus, Search } from '@element-plus/icons-vue'
  import { createRechargeProduct, deleteRechargeProduct, listRechargeProducts, updateRechargeProduct } from '@/api/reader/rechargeProducts'

  defineOptions({ name: 'ReaderRechargeProducts' })
  const rows = ref([]); const total = ref(0); const page = ref(1); const pageSize = ref(20); const keyword = ref(''); const loading = ref(false); const saving = ref(false); const dialogVisible = ref(false); const editing = ref(false); const formRef = ref()
  const form = reactive({ id: '', productName: '', diamondAmount: '', priceUsdt: '', saleStatus: 'off_sale', sortOrder: 0 })
  const rules = { productName: [{ required: true, message: '请输入档位名称', trigger: 'blur' }], diamondAmount: [{ required: true, pattern: /^[1-9]\d*$/, message: '请输入正整数', trigger: 'blur' }], priceUsdt: [{ required: true, pattern: /^(?:\d+\.\d{1,8}|\d+)$/, message: '请输入有效价格', trigger: 'blur' }] }
  const load = async () => { loading.value = true; try { const res = await listRechargeProducts({ keyword: keyword.value, page: page.value, pageSize: pageSize.value }); rows.value = res.data?.list || []; total.value = res.data?.total || 0 } finally { loading.value = false } }
  const resetForm = () => Object.assign(form, { id: '', productName: '', diamondAmount: '', priceUsdt: '', saleStatus: 'off_sale', sortOrder: 0 })
  const openCreate = () => { editing.value = false; resetForm(); dialogVisible.value = true }
  const openEdit = (row) => { editing.value = true; Object.assign(form, { id: row.id, productName: row.productName, diamondAmount: row.diamondAmount, priceUsdt: row.priceUsdt, saleStatus: row.saleStatus, sortOrder: row.sortOrder }); dialogVisible.value = true }
  const save = async () => { if (!(await formRef.value?.validate().catch(() => false))) return; saving.value = true; try { const payload = { productName: form.productName, diamondAmount: form.diamondAmount, priceUsdt: form.priceUsdt, saleStatus: form.saleStatus, sortOrder: form.sortOrder }; if (editing.value) await updateRechargeProduct(form.id, payload); else await createRechargeProduct(payload); ElMessage.success('保存成功'); dialogVisible.value = false; await load() } finally { saving.value = false } }
  const remove = async (row) => { await deleteRechargeProduct(row.id); ElMessage.success('删除成功'); await load() }
  load()
</script>

<style scoped>
.recharge-products-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.toolbar .el-input{max-width:300px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
</style>
