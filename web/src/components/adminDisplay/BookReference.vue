<template><span>{{ bookLabel(id, name || resolvedName) }}</span></template>
<script setup>
import { ref, watch } from 'vue'
import { bookLabel } from '@/utils/adminDisplay'
import { resolveBook } from './books'
const props = defineProps({ id: { type: String, default: '' }, name: { type: String, default: '' } })
const resolvedName = ref('')
watch(() => [props.id, props.name], async ([id, name], _old, onCleanup) => {
  let stale = false
  onCleanup(() => { stale = true })
  resolvedName.value = ''
  if (!id || name) return
  const book = await resolveBook(id)
  if (!stale) resolvedName.value = book?.bookName || ''
}, { immediate: true })
</script>
