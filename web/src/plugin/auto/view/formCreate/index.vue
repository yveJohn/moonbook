<template>
  <div class="form-designer-container">
    <div class="designer-toolbar">
      <el-form :inline="true" :model="formOptions">
        <el-form-item label="标签宽度">
          <el-input v-model="formOptions.labelWidth" inputmode="numeric" class="option-input" />
        </el-form-item>
        <el-form-item label="标签位置">
          <el-select v-model="formOptions.labelPosition" class="option-select">
            <el-option label="右侧" value="right" />
            <el-option label="左侧" value="left" />
            <el-option label="顶部" value="top" />
          </el-select>
        </el-form-item>
        <el-form-item label="控件尺寸">
          <el-select v-model="formOptions.size" class="option-select">
            <el-option label="默认" value="default" />
            <el-option label="大" value="large" />
            <el-option label="小" value="small" />
          </el-select>
        </el-form-item>
      </el-form>
      <div class="toolbar-actions">
        <el-button :icon="Plus" @click="addField">添加字段</el-button>
        <el-button type="primary" :icon="DocumentCopy" @click="exportVueTemplate">生成代码</el-button>
      </div>
    </div>

    <el-table :data="fields" row-key="id" border class="field-table">
      <el-table-column label="标签" min-width="150">
        <template #default="{ row }"><el-input v-model="row.title" /></template>
      </el-table-column>
      <el-table-column label="字段名" min-width="150">
        <template #default="{ row }"><el-input v-model="row.field" /></template>
      </el-table-column>
      <el-table-column label="类型" width="160">
        <template #default="{ row }">
          <el-select v-model="row.type">
            <el-option v-for="option in fieldTypes" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="占位提示" min-width="180">
        <template #default="{ row }"><el-input v-model="row.placeholder" /></template>
      </el-table-column>
      <el-table-column label="必填" width="80" align="center">
        <template #default="{ row }"><el-switch v-model="row.required" /></template>
      </el-table-column>
      <el-table-column label="操作" width="70" align="center">
        <template #default="{ $index }">
          <el-button :icon="Delete" circle text type="danger" title="删除字段" @click="removeField($index)" />
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="生成的 Vue 模板代码" width="70%" top="5vh">
      <el-input 
        type="textarea" 
        :rows="25" 
        v-model="vueCode" 
        readonly 
        class="code-input"
        resize="none"
      />
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">关闭</el-button>
          <el-button type="primary" @click="copyCode">一键复制</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
  import { ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { Delete, DocumentCopy, Plus } from '@element-plus/icons-vue'

  defineOptions({
    name: 'FormGenerator'
  })

  const dialogVisible = ref(false)
  const vueCode = ref('')
  let fieldSequence = 1
  const fieldTypes = [
    { label: '文本', value: 'input' },
    { label: '数字', value: 'inputNumber' },
    { label: '选择', value: 'select' },
    { label: '开关', value: 'switch' },
    { label: '日期', value: 'datePicker' },
    { label: '多选', value: 'checkbox' }
  ]
  const formOptions = ref({ labelWidth: '100px', labelPosition: 'right', size: 'default' })
  const createField = () => {
    const sequence = fieldSequence++
    return { id: sequence, title: `字段 ${sequence}`, field: `field${sequence}`, type: 'input', placeholder: '', required: false }
  }
  const fields = ref([createField()])
  const addField = () => fields.value.push(createField())
  const removeField = (index) => fields.value.splice(index, 1)

  const kebabCase = (str) => {
    return str.replace(/([A-Z])/g, '-$1').toLowerCase()
  }

  const escapeAttribute = (value) => String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')

  const generateVueCode = (rules, options) => {
    let formDataInit = []
    let formRules = []

    const parseRule = (rule) => {
      if (rule.type === 'row') {
        const propsStr = rule.props ? Object.entries(rule.props).map(([k, v]) => `:${k}="${v}"`).join(' ') : ''
        let childrenStr = rule.children ? rule.children.map(c => parseRule(c)).join('\n') : ''
        return `\n    <el-row ${propsStr}>${childrenStr}\n    </el-row>`
      }
      if (rule.type === 'col') {
        const propsStr = rule.props ? Object.entries(rule.props).map(([k, v]) => `:${k}="${v}"`).join(' ') : ''
        let childrenStr = rule.children ? rule.children.map(c => parseRule(c)).join('\n') : ''
        return `\n      <el-col ${propsStr}>${childrenStr}\n      </el-col>`
      }

      if (!rule.field) return ''

      let tag = rule.type
      
      const typeMap = {
        input: 'el-input',
        inputNumber: 'el-input-number',
        select: 'el-select',
        radio: 'el-radio-group',
        checkbox: 'el-checkbox-group',
        switch: 'el-switch',
        timePicker: 'el-time-picker',
        datePicker: 'el-date-picker',
        slider: 'el-slider',
        rate: 'el-rate',
        colorPicker: 'el-color-picker',
        cascader: 'el-cascader',
        upload: 'el-upload'
      }

      const elTag = typeMap[tag] || (tag.startsWith('el-') ? tag : `el-${tag}`)

      let propsStr = ''
      if (rule.props) {
        for (const [key, value] of Object.entries(rule.props)) {
          if (value === null || value === undefined) continue
          if (typeof value === 'boolean') {
            propsStr += value ? ` ${kebabCase(key)}` : ` :${kebabCase(key)}="false"`
          } else if (typeof value === 'string') {
            propsStr += ` ${kebabCase(key)}="${escapeAttribute(value)}"`
          } else {
            propsStr += ` :${kebabCase(key)}='${JSON.stringify(value)}'`
          }
        }
      }

      let innerContent = ''
      if (rule.options && Array.isArray(rule.options)) {
        if (tag === 'select') {
          innerContent = rule.options.map(opt => `\n        <el-option label="${escapeAttribute(opt.label)}" value="${escapeAttribute(opt.value)}" />`).join('') + '\n      '
        } else if (tag === 'radio') {
          innerContent = rule.options.map(opt => `\n        <el-radio label="${escapeAttribute(opt.value)}">${escapeAttribute(opt.label)}</el-radio>`).join('') + '\n      '
        } else if (tag === 'checkbox') {
          innerContent = rule.options.map(opt => `\n        <el-checkbox label="${escapeAttribute(opt.value)}">${escapeAttribute(opt.label)}</el-checkbox>`).join('') + '\n      '
        }
      }

      let initVal = rule.value !== undefined ? rule.value : (tag === 'checkbox' ? [] : null)
      formDataInit.push(`  ${rule.field}: ${JSON.stringify(initVal)}`)

      if (rule.$required || (rule.effect && rule.effect.required)) {
        formRules.push(`  ${rule.field}: [{ required: true, message: ${JSON.stringify(`${rule.title}不能为空`)}, trigger: 'blur' }]`)
      } else if (rule.validate) {
        formRules.push(`  ${rule.field}: ${JSON.stringify(rule.validate)}`)
      }

      return `
    <el-form-item label="${escapeAttribute(rule.title)}" prop="${rule.field}">
      <${elTag} v-model="formData.${rule.field}"${propsStr}>${innerContent}</${elTag}>
    </el-form-item>`
    }

    const formItems = rules.map(parseRule).join('')

    const formConfig = options.form || {}
    let formPropsStr = []
    if (formConfig.labelWidth) formPropsStr.push(`label-width="${formConfig.labelWidth}"`)
    if (formConfig.size) formPropsStr.push(`size="${formConfig.size}"`)
    if (formConfig.labelPosition) formPropsStr.push(`label-position="${formConfig.labelPosition}"`)
    if (formConfig.hideRequiredAsterisk) formPropsStr.push(`hide-required-asterisk`)

    // 8. 拼装成标准的 <template> 和 <script setup> 闭环代码
    return `<template>
  <div>
    <el-form ref="formRef" :model="formData" :rules="rules" ${formPropsStr.join(' ')}>
${formItems}
      <el-form-item>
        <el-button type="primary" @click="submitForm">提交</el-button>
        <el-button @click="resetForm">重置</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'

const formRef = ref(null)

const formData = reactive({
${formDataInit.join(',\n')}
})

const rules = reactive({
${formRules.join(',\n')}
})

const submitForm = async () => {
  if (!formRef.value) return
  await formRef.value.validate((valid) => {
    if (valid) {
      ElMessage.success('表单校验通过，准备提交')
      console.log('提交的数据: ', formData)
    } else {
      ElMessage.error('表单校验失败')
    }
  })
}

const resetForm = () => {
  if (!formRef.value) return
  formRef.value.resetFields()
}
<${'/'}script>
`
  }

  const exportVueTemplate = () => {
    if (!fields.value.length) {
      ElMessage.warning('请先添加字段')
      return
    }
    const invalid = fields.value.find((field) => !/^[A-Za-z_$][\w$]*$/.test(field.field) || !field.title.trim())
    if (invalid) {
      ElMessage.error('字段名必须是有效的 JavaScript 标识符，且标签不能为空')
      return
    }
    if (new Set(fields.value.map((field) => field.field)).size !== fields.value.length) {
      ElMessage.error('字段名不能重复')
      return
    }
    const rules = fields.value.map((field) => ({
      type: field.type,
      field: field.field,
      title: field.title.trim(),
      value: field.type === 'checkbox' ? [] : null,
      props: field.placeholder ? { placeholder: field.placeholder } : {},
      $required: field.required
    }))
    const options = { form: formOptions.value }
    vueCode.value = generateVueCode(rules, options)
    dialogVisible.value = true
  }

  const copyCode = async () => {
    try {
      await navigator.clipboard.writeText(vueCode.value)
      ElMessage.success('代码已成功复制到剪贴板！')
      dialogVisible.value = false
    } catch (err) {
      ElMessage.error('复制失败，请手动选择复制')
    }
  }
</script>

<style scoped>
  .form-designer-container {
    padding: 16px;
  }

  .designer-toolbar {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 12px;
  }

  .designer-toolbar :deep(.el-form-item) {
    margin-bottom: 8px;
  }

  .option-input,
  .option-select {
    width: 120px;
  }

  .toolbar-actions {
    display: flex;
    flex: 0 0 auto;
    gap: 8px;
  }

  .field-table {
    width: 100%;
  }

  @media (max-width: 900px) {
    .designer-toolbar {
      align-items: stretch;
      flex-direction: column;
    }
  }
</style>
