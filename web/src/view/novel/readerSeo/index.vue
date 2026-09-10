<template>
  <div class="reader-seo-page">
    <div class="gva-form-box reader-seo-shell">
      <header class="reader-seo-header">
        <div>
          <h2>SEO 设置</h2>
          <span v-if="form.id" class="reader-seo-meta">配置 ID {{ form.id }} · 更新于 {{ formatTime(form.updatedAt) }}</span>
        </div>
        <el-button :icon="Refresh" :loading="loading" aria-label="刷新" @click="loadConfig" />
      </header>

      <el-form ref="formRef" v-loading="loading" :model="form" :rules="rules" label-position="top">
        <section class="reader-seo-section">
          <div class="reader-seo-section__heading">
            <h3>基础设置</h3>
            <el-switch v-model="form.seoEnabled" inline-prompt active-text="启" inactive-text="停" />
          </div>
          <div class="reader-seo-grid">
            <el-form-item label="站点名称" prop="siteName">
              <el-input v-model="form.siteName" maxlength="100" show-word-limit />
            </el-form-item>
            <el-form-item label="正式站点域名" prop="siteUrl">
              <el-input v-model="form.siteUrl" maxlength="2048" placeholder="https://ybsc.me" />
            </el-form-item>
          </div>
          <el-form-item label="全站默认描述" prop="defaultDescription">
            <el-input v-model="form.defaultDescription" type="textarea" :rows="3" maxlength="2000" show-word-limit resize="vertical" />
          </el-form-item>
        </section>

        <section class="reader-seo-section">
          <div class="reader-seo-section__heading"><h3>页面文案</h3></div>
          <div class="reader-seo-grid">
            <el-form-item label="首页标题" prop="homeTitle">
              <el-input v-model="form.homeTitle" maxlength="500" show-word-limit />
            </el-form-item>
            <el-form-item label="首页描述" prop="homeDescription">
              <el-input v-model="form.homeDescription" type="textarea" :rows="3" maxlength="2000" show-word-limit resize="vertical" />
            </el-form-item>
          </div>

          <div class="reader-seo-template-block">
            <div class="reader-seo-template-block__title">
              <strong>书库模板</strong>
              <span>{siteName} · {keyword} · {categoryName} · {subCategoryName}</span>
            </div>
            <div class="reader-seo-grid">
              <el-form-item label="标题模板" prop="booksTitleTemplate">
                <el-input v-model="form.booksTitleTemplate" maxlength="500" show-word-limit />
              </el-form-item>
              <el-form-item label="描述模板" prop="booksDescriptionTemplate">
                <el-input v-model="form.booksDescriptionTemplate" type="textarea" :rows="3" maxlength="2000" show-word-limit resize="vertical" />
              </el-form-item>
            </div>
            <div class="reader-seo-preview">
              <span>{{ renderPreview(form.booksTitleTemplate) || '—' }}</span>
              <small>{{ renderPreview(form.booksDescriptionTemplate) || '—' }}</small>
            </div>
          </div>

          <div class="reader-seo-template-block">
            <div class="reader-seo-template-block__title">
              <strong>书籍详情模板</strong>
              <span>{siteName} · {bookName} · {authorName} · {categoryName} · {bookDesc}</span>
            </div>
            <div class="reader-seo-grid">
              <el-form-item label="标题模板" prop="bookTitleTemplate">
                <el-input v-model="form.bookTitleTemplate" maxlength="500" show-word-limit />
              </el-form-item>
              <el-form-item label="描述模板" prop="bookDescriptionTemplate">
                <el-input v-model="form.bookDescriptionTemplate" type="textarea" :rows="3" maxlength="2000" show-word-limit resize="vertical" />
              </el-form-item>
            </div>
            <div class="reader-seo-preview">
              <span>{{ renderPreview(form.bookTitleTemplate) || '—' }}</span>
              <small>{{ renderPreview(form.bookDescriptionTemplate) || '—' }}</small>
            </div>
          </div>
        </section>

        <section class="reader-seo-section reader-seo-controls">
          <div class="reader-seo-section__heading"><h3>收录控制</h3></div>
          <label>
            <span><strong>允许搜索引擎收录</strong></span>
            <el-switch v-model="form.indexingEnabled" />
          </label>
          <label>
            <span><strong>生成站点地图</strong></span>
            <el-switch v-model="form.sitemapEnabled" />
          </label>
        </section>

        <footer class="reader-seo-actions">
          <el-button :icon="Refresh" @click="loadConfig">撤销更改</el-button>
          <el-button type="primary" :icon="CircleCheck" :loading="saving" @click="saveConfig">保存设置</el-button>
        </footer>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { adminDateTime } from '@/utils/adminDisplay'
  import { reactive, ref } from 'vue'
  import { CircleCheck, Refresh } from '@element-plus/icons-vue'
  import { ElMessage } from 'element-plus'
  import { getReaderSEOConfig, updateReaderSEOConfig } from '@/api/novel/readerSeo'
  import {
    BOOK_PLACEHOLDERS,
    BOOKS_PLACEHOLDERS,
    invalidPlaceholders,
    isSiteRootURL,
    renderSEOTemplate
  } from './seoTemplate'

  defineOptions({ name: 'NovelReaderSeo' })

  const defaults = () => ({
    id: '', seoEnabled: true, indexingEnabled: true, sitemapEnabled: true,
    siteName: '', siteUrl: '', defaultDescription: '', homeTitle: '', homeDescription: '',
    booksTitleTemplate: '', booksDescriptionTemplate: '', bookTitleTemplate: '', bookDescriptionTemplate: '',
    createdAt: '', updatedAt: ''
  })
  const form = reactive(defaults())
  const formRef = ref()
  const loading = ref(false)
  const saving = ref(false)
  const required = (message) => ({ required: true, message, trigger: 'blur' })
  const templateRule = (allowed) => (_rule, value, callback) => {
    const invalid = invalidPlaceholders(value, allowed)
    callback(invalid.length ? new Error(`不支持的占位符：${invalid.join('、')}`) : undefined)
  }
  const rules = {
    siteName: [required('请输入站点名称')],
    siteUrl: [required('请输入站点域名'), { validator: (_rule, value, callback) => callback(isSiteRootURL(value) ? undefined : new Error('请输入不含路径的 HTTP 或 HTTPS 站点根地址')), trigger: 'blur' }],
    defaultDescription: [required('请输入默认描述')],
    homeTitle: [required('请输入首页标题')],
    homeDescription: [required('请输入首页描述')],
    booksTitleTemplate: [required('请输入书库标题模板'), { validator: templateRule(BOOKS_PLACEHOLDERS), trigger: 'blur' }],
    booksDescriptionTemplate: [required('请输入书库描述模板'), { validator: templateRule(BOOKS_PLACEHOLDERS), trigger: 'blur' }],
    bookTitleTemplate: [required('请输入书籍标题模板'), { validator: templateRule(BOOK_PLACEHOLDERS), trigger: 'blur' }],
    bookDescriptionTemplate: [required('请输入书籍描述模板'), { validator: templateRule(BOOK_PLACEHOLDERS), trigger: 'blur' }]
  }

  const loadConfig = async () => {
    loading.value = true
    try {
      const response = await getReaderSEOConfig()
      if (response.code === 0) {
        Object.assign(form, defaults(), response.data)
        formRef.value?.clearValidate()
      }
    } finally {
      loading.value = false
    }
  }

  const saveConfig = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    saving.value = true
    try {
      const payload = {
        seoEnabled: form.seoEnabled,
        indexingEnabled: form.indexingEnabled,
        sitemapEnabled: form.sitemapEnabled,
        siteName: form.siteName,
        siteUrl: form.siteUrl,
        defaultDescription: form.defaultDescription,
        homeTitle: form.homeTitle,
        homeDescription: form.homeDescription,
        booksTitleTemplate: form.booksTitleTemplate,
        booksDescriptionTemplate: form.booksDescriptionTemplate,
        bookTitleTemplate: form.bookTitleTemplate,
        bookDescriptionTemplate: form.bookDescriptionTemplate
      }
      const response = await updateReaderSEOConfig(payload)
      if (response.code === 0) {
        Object.assign(form, response.data)
        ElMessage.success('SEO 设置已保存')
      }
    } finally {
      saving.value = false
    }
  }

  const formatTime = (value) => value ? adminDateTime(value) : '-'
  const renderPreview = (template) => renderSEOTemplate(template, { siteName: form.siteName })
  loadConfig()
</script>

<style scoped>
  .reader-seo-page { padding: 16px; }
  .reader-seo-shell { max-width: 1080px; margin: 0 auto; padding: 0; overflow: hidden; }
  .reader-seo-header { display: flex; align-items: center; justify-content: space-between; padding: 20px 24px; border-bottom: 1px solid var(--el-border-color-lighter); }
  .reader-seo-header h2, .reader-seo-section h3 { margin: 0; letter-spacing: 0; }
  .reader-seo-header h2 { font-size: 20px; font-weight: 600; }
  .reader-seo-meta { display: block; margin-top: 5px; color: var(--el-text-color-secondary); font-size: 12px; }
  .reader-seo-section { padding: 24px; border-bottom: 1px solid var(--el-border-color-lighter); }
  .reader-seo-section__heading { display: flex; align-items: center; justify-content: space-between; min-height: 32px; margin-bottom: 18px; }
  .reader-seo-section h3 { font-size: 15px; font-weight: 600; }
  .reader-seo-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 0 18px; }
  .reader-seo-template-block + .reader-seo-template-block { margin-top: 24px; padding-top: 24px; border-top: 1px dashed var(--el-border-color); }
  .reader-seo-template-block__title { display: flex; align-items: baseline; justify-content: space-between; gap: 16px; margin-bottom: 12px; }
  .reader-seo-template-block__title span { color: var(--el-text-color-secondary); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; overflow-wrap: anywhere; }
  .reader-seo-preview { display: grid; gap: 5px; padding: 12px 14px; border-left: 3px solid var(--el-color-success); background: var(--el-fill-color-light); }
  .reader-seo-preview span { color: var(--el-text-color-primary); font-size: 14px; font-weight: 600; overflow-wrap: anywhere; }
  .reader-seo-preview small { color: var(--el-text-color-secondary); line-height: 1.6; overflow-wrap: anywhere; }
  .reader-seo-controls { display: grid; gap: 0; }
  .reader-seo-controls label { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 48px; border-top: 1px solid var(--el-border-color-extra-light); }
  .reader-seo-controls label:first-of-type { border-top: 0; }
  .reader-seo-actions { position: sticky; bottom: 0; z-index: 2; display: flex; justify-content: flex-end; gap: 10px; padding: 14px 24px; background: color-mix(in srgb, var(--el-bg-color) 94%, transparent); border-top: 1px solid var(--el-border-color-lighter); backdrop-filter: blur(8px); }
  @media (max-width: 720px) {
    .reader-seo-page { padding: 10px; }
    .reader-seo-header, .reader-seo-section { padding: 18px 16px; }
    .reader-seo-grid { grid-template-columns: minmax(0, 1fr); }
    .reader-seo-template-block__title { align-items: flex-start; flex-direction: column; gap: 6px; }
    .reader-seo-actions { padding: 12px 16px; }
  }
</style>
