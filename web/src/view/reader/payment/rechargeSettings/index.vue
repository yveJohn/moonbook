<template>
  <div class="gva-form-box recharge-settings-page">
    <div class="page-head"><div><h2>自定义充值规则</h2><p>控制任意钻石数量充值的开关、兑换比例和金额范围。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
    <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="settings-form">
      <el-form-item label="允许自定义充值"><el-switch v-model="form.customEnabled" /></el-form-item>
      <el-form-item label="钻石 / USDT" prop="diamondsPerUsdt"><el-input v-model="form.diamondsPerUsdt" inputmode="decimal" /></el-form-item>
      <el-form-item label="最低钻石数" prop="minDiamondAmount"><el-input v-model="form.minDiamondAmount" inputmode="numeric" /></el-form-item>
      <el-form-item label="最高钻石数" prop="maxDiamondAmount"><el-input v-model="form.maxDiamondAmount" inputmode="numeric" /></el-form-item>
      <el-form-item><el-button type="primary" :loading="saving" @click="save">保存规则</el-button></el-form-item>
    </el-form>
    <el-descriptions v-if="setting" :column="2" border class="meta">
      <el-descriptions-item label="金额精度">{{ setting.amountScale }} 位小数</el-descriptions-item>
      <el-descriptions-item label="舍入方式">{{ setting.roundingMode }}</el-descriptions-item>
      <el-descriptions-item label="配置 ID"><code>{{ setting.id }}</code></el-descriptions-item>
    </el-descriptions>
  </div>
</template>
<script setup>
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getRechargeSettings, updateRechargeSettings } from '@/api/reader/rechargeSettings'

defineOptions({ name: 'ReaderRechargeSettings' })
const formRef = ref(); const setting = ref(null); const saving = ref(false)
const form = reactive({ customEnabled: true, diamondsPerUsdt: '', minDiamondAmount: '', maxDiamondAmount: '' })
const positiveInteger = { pattern: /^[1-9]\d*$/, message: '请输入正整数', trigger: 'blur' }
const rules = { diamondsPerUsdt: [{ required: true, pattern: /^(?:[1-9]\d*(?:\.\d{1,8})?|0\.\d*[1-9]\d*)$/, message: '请输入最多 8 位小数的正数', trigger: 'blur' }], minDiamondAmount: [positiveInteger], maxDiamondAmount: [positiveInteger] }
const load = async () => { const res = await getRechargeSettings(); const v = res.data || {}; setting.value = v; Object.assign(form, { customEnabled: v.customEnabled, diamondsPerUsdt: v.diamondsPerUsdt, minDiamondAmount: v.minDiamondAmount, maxDiamondAmount: v.maxDiamondAmount }) }
const save = async () => { if (!(await formRef.value?.validate().catch(() => false))) return; const min = form.minDiamondAmount.replace(/^0+/, '') || '0'; const max = form.maxDiamondAmount.replace(/^0+/, '') || '0'; if (min.length > max.length || (min.length === max.length && min > max)) { ElMessage.error('最低钻石数不能大于最高钻石数'); return }; saving.value = true; try { const res = await updateRechargeSettings({ ...form }); setting.value = res.data; ElMessage.success('保存成功') } finally { saving.value = false } }
load()
</script>
<style scoped>.recharge-settings-page{padding:20px;max-width:760px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:22px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.settings-form{max-width:420px}.settings-form :deep(.el-input){max-width:260px}.meta{margin-top:22px}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>
