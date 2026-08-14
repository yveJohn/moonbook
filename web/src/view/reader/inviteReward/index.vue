<template>
  <div class="gva-form-box invite-reward-page">
    <div class="page-head"><div><h2>邀请奖励</h2><p>配置邀请人和新读者注册后的奖励，奖励写入不可变钱包流水。</p></div><el-button :icon="Refresh" @click="load">刷新</el-button></div>
    <el-form v-loading="loading" ref="formRef" :model="form" :rules="rules" label-position="top" class="form">
      <el-form-item label="启用奖励" prop="enabled"><el-switch v-model="form.enabled" /></el-form-item>
      <el-form-item label="邀请人奖励（奖励币）" prop="inviterRewardCoin"><el-input v-model="form.inviterRewardCoin" inputmode="numeric" /></el-form-item>
      <el-form-item label="新读者奖励（奖励币）" prop="inviteeRewardCoin"><el-input v-model="form.inviteeRewardCoin" inputmode="numeric" /></el-form-item>
      <el-form-item label="备注" prop="remark"><el-input v-model="form.remark" type="textarea" maxlength="255" show-word-limit /></el-form-item>
      <el-button type="primary" :loading="saving" @click="save">保存配置</el-button>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getInviteReward, updateInviteReward } from '@/api/reader/inviteReward'
defineOptions({ name: 'ReaderInviteReward' })
const formRef = ref(); const loading = ref(false); const saving = ref(false)
const form = reactive({ enabled: false, inviterRewardCoin: '0', inviteeRewardCoin: '0', remark: '' })
const amountRule = { validator: (_, value, callback) => /^(0|[1-9]\d*)$/.test(value) ? callback() : callback(new Error('请输入非负整数')), trigger: 'blur' }
const rules = { inviterRewardCoin: [amountRule], inviteeRewardCoin: [amountRule], remark: [{ required: true, max: 255, message: '请输入备注', trigger: 'blur' }] }
const load = async () => { loading.value = true; try { const res = await getInviteReward(); Object.assign(form, res.data || {}) } finally { loading.value = false } }
const save = async () => { if (!(await formRef.value?.validate().catch(() => false))) return; saving.value = true; try { await updateInviteReward({ ...form }); ElMessage.success('保存成功') } finally { saving.value = false } }
load()
</script>
<style scoped>.invite-reward-page{padding:20px}.page-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.page-head h2{margin:0}.page-head p{margin:6px 0 0;color:var(--el-text-color-secondary)}.form{max-width:560px}</style>
