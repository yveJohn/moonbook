<template>
  <el-select :model-value="modelValue" filterable remote clearable :remote-method="search" :loading="loading" placeholder="搜索作品名称或完整 ID" class="admin-book-select" @update:model-value="update" @visible-change="visible => visible && search('')">
    <template #label="{ value }">{{ bookLabel(value, options.find(book => book.id === value)?.bookName) }}</template>
    <el-option v-for="book in options" :key="book.id" :value="book.id" :label="bookLabel(book.id, book.bookName)" />
  </el-select>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import { listNovelBooks } from '@/api/novel/books'
import { bookLabel } from '@/utils/adminDisplay'
import { resolveBook } from './books'
const props = defineProps({ modelValue: { type: String, default: '' } })
const emit = defineEmits(['update:modelValue', 'change'])
const rows = ref([]), selected = ref(null), loading = ref(false)
let request = 0
const options = computed(() => selected.value && !rows.value.some(book => book.id === selected.value.id) ? [selected.value, ...rows.value] : rows.value)
const update = value => { emit('update:modelValue', value || ''); emit('change', value || '') }
watch(() => props.modelValue, async (id, _old, onCleanup) => {
  let stale = false
  onCleanup(() => { stale = true })
  selected.value = id ? { id, bookName: '加载中' } : null
  if (!id) return
  const book = rows.value.find(row => row.id === id) || await resolveBook(id)
  if (!stale) selected.value = book || { id, bookName: '作品不可用' }
}, { immediate: true })
async function search(keyword) {
  const version = ++request
  loading.value = true
  try {
    const query = keyword.trim()
    const [response, exact] = await Promise.all([
      listNovelBooks({ keyword: query, page: 1, pageSize: 30 }),
      /^[1-9]\d*$/.test(query) ? resolveBook(query) : null
    ])
    if (version !== request) return
    const list = response.data?.list || []
    rows.value = exact && !list.some(book => book.id === exact.id) ? [exact, ...list] : list
  } catch {
    if (version === request) rows.value = []
  } finally {
    if (version === request) loading.value = false
  }
}
</script>

<style scoped>
.admin-book-select { width: 100%; min-width: min(240px, 75vw); }
</style>
