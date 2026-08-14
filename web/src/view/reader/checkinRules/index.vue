<template>
  <div class="gva-form-box checkin-rules-page">
    <div class="page-head"><div><h2>签到奖励规则</h2><p>维护每日签到和连续签到的奖励规则，同一类型只能启用一条对应规则。</p></div><el-button type="primary" :icon="Plus" @click="openCreate">新增规则</el-button></div>
    <div class="toolbar"><el-select v-model="ruleType" clearable placeholder="规则类型" @change="load"><el-option label="每日签到" value="daily" /><el-option label="连续签到" value="continuous" /></el-select><el-button :icon="Search" @click="load">查询</el-button></div>
    <el-table v-loading="loading" :data="rows" border>
      <el-table-column label="规则 ID" min-width="180"><template #default="{ row }"><code>{{ row.id }}</code></template></el-table-column>
      <el-table-column label="类型" width="130"><template #default="{ row }">{{ row.ruleType === 'daily' ? '每日签到' : `连续 ${row.continuousDays} 天` }}</template></el-table-column>
      <el-table-column label="奖励" min-width="160"><template #default="{ row }">{{ row.rewardMode === 'fixed' ? `${row.fixedCoin} 金币` : `${row.minCoin} - ${row.maxCoin} 金币` }}</template></el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'">{{ row.status === 'enabled' ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column prop="sortOrder" label="排序" width="80" />
      <el-table-column prop="remark" label="备注" min-width="180" show-overflow-tooltip />
      <el-table-column label="操作" width="150" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-popconfirm title="确认删除此规则？" @confirm="remove(row)"><el-button link type="danger">删除</el-button></el-popconfirm></template></el-table-column>
    </el-table>
    <div class="pager"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" /></div>
    <el-dialog v-model="dialog" :title="editing ? '编辑签到规则' : '新增签到规则'" width="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="规则类型" prop="ruleType"><el-select v-model="form.ruleType"><el-option label="每日签到" value="daily" /><el-option label="连续签到" value="continuous" /></el-select></el-form-item>
        <el-form-item v-if="form.ruleType === 'continuous'" label="连续天数" prop="continuousDays"><el-input v-model="form.continuousDays" inputmode="numeric" /></el-form-item>
        <el-form-item label="奖励模式" prop="rewardMode"><el-select v-model="form.rewardMode"><el-option label="固定奖励" value="fixed" /><el-option label="随机奖励" value="random" /></el-select></el-form-item>
        <el-form-item v-if="form.rewardMode === 'fixed'" label="奖励金币" prop="fixedCoin"><el-input v-model="form.fixedCoin" inputmode="numeric" /></el-form-item>
        <template v-else><el-form-item label="最小金币" prop="minCoin"><el-input v-model="form.minCoin" inputmode="numeric" /></el-form-item><el-form-item label="最大金币" prop="maxCoin"><el-input v-model="form.maxCoin" inputmode="numeric" /></el-form-item></template>
        <el-form-item label="状态" prop="status"><el-select v-model="form.status"><el-option label="启用" value="enabled" /><el-option label="停用" value="disabled" /></el-select></el-form-item>
        <el-form-item label="排序"><el-input v-model="form.sortOrder" inputmode="numeric" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" maxlength="255" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import { createCheckinRule, deleteCheckinRule, listCheckinRules, updateCheckinRule } from '@/api/reader/checkinRules'

defineOptions({ name: 'ReaderCheckinRules' })
const rows = ref([]); const total = ref(0); const page = ref(1); const pageSize = ref(20); const ruleType = ref(''); const loading = ref(false); const saving = ref(false); const dialog = ref(false); const editing = ref(false); const formRef = ref()
const blank = () => ({ id: '', ruleType: 'daily', continuousDays: '', rewardMode: 'fixed', fixedCoin: '10', minCoin: '', maxCoin: '', status: 'disabled', sortOrder: '0', remark: '' })
const form = reactive(blank())
const positive = { pattern: /^[1-9]\d*$/, message: '请输入正整数', trigger: 'blur' }
const rules = { ruleType: [{ required: true, message: '请选择规则类型', trigger: 'change' }], continuousDays: [positive], rewardMode: [{ required: true, message: '请选择奖励模式', trigger: 'change' }], fixedCoin: [positive], minCoin: [positive], maxCoin: [positive], status: [{ required: true, message: '请选择状态', trigger: 'change' }] }
const load = async () => { loading.value = true; try { const res = await listCheckinRules({ ruleType: ruleType.value, page: page.value, pageSize: pageSize.value }); rows.value = res.data?.list || []; total.value = res.data?.total || 0 } finally { loading.value = false } }
const openCreate = () => { editing.value = false; Object.assign(form, blank()); dialog.value = true }
const openEdit = (row) => { editing.value = true; Object.assign(form, blank(), row); dialog.value = true }
const save = async () => { if (!(await formRef.value?.validate().catch(() => false))) return; saving.value = true; try { const data = { ruleType: form.ruleType, continuousDays: form.ruleType === 'continuous' ? form.continuousDays : '', rewardMode: form.rewardMode, fixedCoin: form.rewardMode === 'fixed' ? form.fixedCoin : '', minCoin: form.rewardMode === 'random' ? form.minCoin : '', maxCoin: form.rewardMode === 'random' ? form.maxCoin : '', status: form.status, sortOrder: form.sortOrder, remark: form.remark }; if (editing.value) await updateCheckinRule(form.id, data); else await createCheckinRule(data); ElMessage.success('保存成功'); dialog.value = false; await load() } finally { saving.value = false } }
const remove = async (row) => { await deleteCheckinRule(row.id); ElMessage.success('删除成功'); await load() }
load()
</script>
<style scoped>.checkin-rules-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.toolbar{display:flex;gap:10px;margin-bottom:14px}.pager{display:flex;justify-content:flex-end;margin-top:16px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
