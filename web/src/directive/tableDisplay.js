import { ElMessage } from 'element-plus'

const interactive = 'a, button, input, textarea, select, [role="button"], [role="switch"], [role="checkbox"], .el-table__expand-icon, .el-image, .el-select, .el-switch'
export async function copyCellText(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }
  const previous = document.activeElement
  const input = document.createElement('textarea')
  input.value = text
  input.style.cssText = 'position:fixed;left:-9999px;top:0'
  document.body.append(input)
  try {
    input.select()
    if (!document.execCommand('copy')) throw new Error('copy failed')
  } finally {
    input.remove()
    previous?.focus?.()
  }
}
export default {
  mounted(el) {
    const refresh = () => {
      el.querySelectorAll('td .cell').forEach(cell => {
        const eligible = !cell.querySelector(interactive) && !!cell.textContent.trim()
        cell.classList.toggle('admin-copy-cell', eligible)
        if (eligible) {
          cell.title = cell.textContent.trim()
          cell.tabIndex = 0
          cell.setAttribute('aria-label', `点击复制：${cell.textContent.trim()}`)
        } else if (cell.hasAttribute('data-admin-copy')) {
          cell.removeAttribute('title')
          cell.removeAttribute('tabindex')
          cell.removeAttribute('aria-label')
        }
        cell.toggleAttribute('data-admin-copy', eligible)
      })
    }
    const copy = async event => {
      if (event.type === 'keydown' && !['Enter', ' '].includes(event.key)) return
      const cell = event.target.closest('.admin-copy-cell')
      if (!cell || !el.contains(cell) || event.target.closest(interactive)) return
      if (event.type === 'keydown') event.preventDefault()
      try { await copyCellText(cell.textContent.trim()); ElMessage.success('已复制') }
      catch { ElMessage.error('复制失败，请手动选择文本复制') }
    }
    const observer = new MutationObserver(refresh)
    observer.observe(el, { childList: true, subtree: true, characterData: true })
    el.addEventListener('click', copy)
    el.addEventListener('keydown', copy)
    refresh()
    el.__adminTableCleanup = () => {
      observer.disconnect()
      el.removeEventListener('click', copy)
      el.removeEventListener('keydown', copy)
    }
  },
  beforeUnmount(el) { el.__adminTableCleanup?.(); delete el.__adminTableCleanup }
}
