<template>
  <div class="gva-form-box users-page">
    <div class="page-head">
      <div>
        <h2>读者用户</h2>
        <p>管理读者账号状态、会员发放和货币发放。</p>
      </div>
      <el-button :icon="Refresh" @click="load">刷新</el-button>
    </div>
    <div class="toolbar">
      <el-input v-model="keyword" clearable placeholder="账号或昵称" @keyup.enter="load" />
      <el-select v-model="status" clearable placeholder="状态" @change="load">
        <el-option label="启用" value="enabled" />
        <el-option label="停用" value="disabled" />
        <el-option label="已删除" value="deleted" />
      </el-select>
      <el-button :icon="Search" @click="load">查询</el-button>
    </div>
    <el-table v-table-display v-loading="loading" :data="rows" border>
      <el-table-column label="用户 ID" min-width="180">
        <template #default="{ row }"><code>{{ row.id }}</code></template>
      </el-table-column>
      <el-table-column prop="username" label="账号" min-width="160" />
      <el-table-column prop="nickname" label="昵称" min-width="140" />
      <el-table-column label="钻石" min-width="110">
        <template #default="{ row }">{{ row.rechargeBalance || '0' }}</template>
      </el-table-column>
      <el-table-column label="金币" min-width="110">
        <template #default="{ row }">{{ row.bonusBalance || '0' }}</template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="110" :formatter="adminStatusColumn" />
      <el-table-column prop="lastLoginAt" label="最近登录" min-width="185" :formatter="adminTimeColumn" />
      <el-table-column label="操作" width="430">
        <template #default="{ row }">
          <el-switch :model-value="row.status === 'enabled'" :disabled="row.status === 'deleted'" @change="toggle(row)" />
          <el-button link type="primary" :disabled="row.status === 'deleted'" @click="openPassword(row)">重置密码</el-button>
          <el-button link type="success" :disabled="row.status === 'deleted'" @click="openMembership(row)">发放会员</el-button>
          <el-button link type="warning" :disabled="row.status === 'deleted'" @click="openCoin(row)">发放钻石/金币</el-button>
          <el-button link type="primary" @click="openLogs(row)">日志</el-button>
        </template>
      </el-table-column>
    </el-table>
    <div class="pager">
      <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="load" @current-change="load" />
    </div>
    <el-dialog v-model="passwordDialog" title="重置读者密码" width="440px">
      <el-form ref="passwordRef" :model="passwordForm" :rules="passwordRules" label-position="top">
        <el-form-item label="新密码" prop="password"><el-input v-model="passwordForm.password" type="password" show-password autocomplete="new-password" /></el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword"><el-input v-model="passwordForm.confirmPassword" type="password" show-password autocomplete="new-password" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialog=false">取消</el-button>
        <el-button type="primary" :loading="passwordSaving" @click="savePassword">保存</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="membershipDialog" title="发放会员" width="480px">
      <el-form ref="membershipRef" :model="membershipForm" :rules="membershipRules" label-position="top">
        <el-form-item label="读者"><el-input :model-value="membershipUserLabel" disabled /></el-form-item>
        <el-form-item label="会员商品" prop="productId">
          <el-select v-model="membershipForm.productId" filterable :loading="membershipProductsLoading" placeholder="请选择消费商品中的会员">
            <el-option v-for="item in membershipProducts" :key="item.id" :label="membershipProductLabel(item)" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="selectedMembershipProduct" label="有效期">
          <el-input :model-value="membershipDurationLabel(selectedMembershipProduct)" disabled />
        </el-form-item>
        <el-form-item label="备注" prop="remark"><el-input v-model="membershipForm.remark" maxlength="255" type="textarea" show-word-limit /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="membershipDialog=false">取消</el-button>
        <el-button type="primary" :loading="membershipSaving" @click="saveMembership">发放</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="coinDialog" title="发放钻石/金币" width="480px">
      <el-form ref="coinRef" :model="coinForm" :rules="coinRules" label-position="top">
        <el-form-item label="读者"><el-input :model-value="coinUserLabel" disabled /></el-form-item>
        <el-form-item label="币种" prop="coinType">
          <el-select v-model="coinForm.coinType">
            <el-option label="钻石" value="recharge" />
            <el-option label="金币" value="bonus" />
          </el-select>
        </el-form-item>
        <el-form-item label="数量" prop="amount"><el-input v-model="coinForm.amount" inputmode="numeric" /></el-form-item>
        <el-form-item label="备注" prop="reason"><el-input v-model="coinForm.reason" maxlength="255" type="textarea" show-word-limit /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="coinDialog=false">取消</el-button>
        <el-button type="primary" :loading="coinSaving" @click="saveCoin">发放</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="logDialog" :title="logTitle" width="760px">
      <el-table v-loading="logLoading" :data="logRows" border>
        <el-table-column label="时间" min-width="180" :formatter="adminTimeColumn" prop="createdAt" />
        <el-table-column label="操作人" min-width="140">
          <template #default="{ row }">{{ row.operatorName || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="120">
          <template #default="{ row }">{{ row.action || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态码" width="90">
          <template #default="{ row }">{{ row.status || '-' }}</template>
        </el-table-column>
        <el-table-column label="摘要" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.summary || row.errorMessage || '-' }}</template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="logPage" v-model:page-size="logPageSize" :total="logTotal" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @size-change="loadLogs" @current-change="loadLogs" />
      </div>
    </el-dialog>
  </div>
</template>
<script setup>
import { adminStatusColumn, adminTimeColumn } from '@/utils/adminDisplay'
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import { listReaderUsers, resetReaderUserPassword, setReaderUserStatus, grantReaderMembership, listReaderUserOperations } from '@/api/reader/users'
import { listProducts } from '@/api/reader/products'
import { adjustReaderWallet } from '@/api/reader/walletAdmin'
defineOptions({ name: 'ReaderUsers' })
const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const status = ref('')
const loading = ref(false)
const passwordDialog = ref(false)
const passwordSaving = ref(false)
const passwordRef = ref()
const passwordUserId = ref('')
const passwordForm = reactive({ password: '', confirmPassword: '' })
const passwordRules = {
  password: [{ required: true, min: 6, max: 64, message: '密码长度需为 6-64 个字符', trigger: 'blur' }],
  confirmPassword: [{ required: true, message: '请再次输入密码', trigger: 'blur' }]
}
const membershipDialog = ref(false)
const membershipSaving = ref(false)
const membershipRef = ref()
const membershipUserId = ref('')
const membershipUserLabel = ref('')
const membershipProducts = ref([])
const membershipProductsLoading = ref(false)
const membershipForm = reactive({ productId: '', remark: '' })
const membershipRules = {
  productId: [{ required: true, message: '请选择会员商品', trigger: 'change' }],
  remark: [{ required: true, max: 255, message: '请输入备注', trigger: 'blur' }]
}
const selectedMembershipProduct = computed(() => membershipProducts.value.find((item) => item.id === membershipForm.productId) || null)
const coinDialog = ref(false)
const coinSaving = ref(false)
const coinRef = ref()
const coinUserId = ref('')
const coinUserLabel = ref('')
const coinForm = reactive({ coinType: 'recharge', amount: '', reason: '' })
const coinRules = {
  coinType: [{ required: true, message: '请选择币种', trigger: 'change' }],
  amount: [{ required: true, pattern: /^[1-9]\d*$/, message: '请输入正整数', trigger: 'blur' }],
  reason: [{ required: true, max: 255, message: '请输入备注', trigger: 'blur' }]
}
const logDialog = ref(false)
const logLoading = ref(false)
const logTitle = ref('操作日志')
const logUserId = ref('')
const logRows = ref([])
const logTotal = ref(0)
const logPage = ref(1)
const logPageSize = ref(20)
const membershipDurationLabel = (item) => {
  if (!item) return ''
  return item.durationDays ? `${item.durationDays} 天` : '永久'
}
const membershipProductLabel = (item) => {
  const sale = item.saleStatus === 'on_sale' ? '在售' : '停售'
  return `${item.productName}（${membershipDurationLabel(item)} / ${sale}）`
}
const loadMembershipProducts = async () => {
  membershipProductsLoading.value = true
  try {
    const res = await listProducts({ productType: 'membership', page: 1, pageSize: 100 })
    membershipProducts.value = res.data?.list || []
  } finally {
    membershipProductsLoading.value = false
  }
}
const load = async () => {
  loading.value = true
  try {
    const res = await listReaderUsers({ keyword: keyword.value, status: status.value, page: page.value, pageSize: pageSize.value })
    rows.value = res.data?.list || []
    total.value = res.data?.total || 0
  } finally {
    loading.value = false
  }
}
const toggle = async (row) => {
  const next = row.status === 'enabled' ? 'disabled' : 'enabled'
  try {
    await setReaderUserStatus(row.id, next)
    row.status = next
    ElMessage.success('更新成功')
  } catch {
    ElMessage.error('更新失败')
  }
}
const openPassword = (row) => {
  passwordUserId.value = row.id
  Object.assign(passwordForm, { password: '', confirmPassword: '' })
  passwordDialog.value = true
}
const savePassword = async () => {
  if (!(await passwordRef.value?.validate().catch(() => false))) return
  if (passwordForm.password !== passwordForm.confirmPassword) {
    ElMessage.error('两次输入的密码不一致')
    return
  }
  passwordSaving.value = true
  try {
    await resetReaderUserPassword(passwordUserId.value, { ...passwordForm })
    ElMessage.success('密码已重置')
    passwordDialog.value = false
  } finally {
    passwordSaving.value = false
  }
}
const openMembership = async (row) => {
  membershipUserId.value = row.id
  membershipUserLabel.value = `${row.username}（${row.id}）`
  Object.assign(membershipForm, { productId: '', remark: '' })
  membershipDialog.value = true
  await loadMembershipProducts()
  if (!membershipProducts.value.length) {
    ElMessage.warning('暂无会员商品，请先在消费商品中创建')
  }
}
const saveMembership = async () => {
  if (!(await membershipRef.value?.validate().catch(() => false))) return
  membershipSaving.value = true
  try {
    await grantReaderMembership(membershipUserId.value, { productId: membershipForm.productId, remark: membershipForm.remark.trim() })
    ElMessage.success('会员已发放')
    membershipDialog.value = false
  } finally {
    membershipSaving.value = false
  }
}
const openCoin = (row) => {
  coinUserId.value = row.id
  coinUserLabel.value = `${row.username}（${row.id}）`
  Object.assign(coinForm, { coinType: 'recharge', amount: '', reason: '' })
  coinDialog.value = true
}
const saveCoin = async () => {
  if (!(await coinRef.value?.validate().catch(() => false))) return
  coinSaving.value = true
  try {
    await adjustReaderWallet(coinUserId.value, {
      coinType: coinForm.coinType,
      direction: 'income',
      amount: coinForm.amount,
      reason: coinForm.reason.trim()
    })
    ElMessage.success('发放成功')
    coinDialog.value = false
    await load()
  } finally {
    coinSaving.value = false
  }
}
const openLogs = (row) => {
  logUserId.value = row.id
  logTitle.value = `操作日志（${row.username}）`
  logPage.value = 1
  logDialog.value = true
  loadLogs()
}
const loadLogs = async () => {
  if (!logUserId.value) return
  logLoading.value = true
  try {
    const res = await listReaderUserOperations(logUserId.value, { page: logPage.value, pageSize: logPageSize.value })
    logRows.value = res.data?.list || []
    logTotal.value = res.data?.total || 0
  } finally {
    logLoading.value = false
  }
}
load()
</script>
<style scoped>
.users-page{padding:20px}
.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}
.page-head h2{margin:0}
.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}
.toolbar{display:flex;gap:10px;margin-bottom:14px}
.toolbar .el-input{max-width:300px}
.pager{display:flex;justify-content:flex-end;margin-top:16px}
code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
</style>
