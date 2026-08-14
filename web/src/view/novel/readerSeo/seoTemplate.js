export const BOOKS_PLACEHOLDERS = ['siteName', 'keyword', 'categoryName', 'subCategoryName']
export const BOOK_PLACEHOLDERS = ['siteName', 'bookName', 'authorName', 'categoryName', 'bookDesc']

const placeholderPattern = /\{([A-Za-z][A-Za-z0-9]*)}/g

const exampleValues = {
  siteName: '月白书城',
  keyword: '仙侠',
  categoryName: '玄幻',
  subCategoryName: '东方玄幻',
  bookName: '月下长歌',
  authorName: '示例作者',
  bookDesc: '少年踏上修行之路，在乱世中寻找自己的答案。'
}

export const invalidPlaceholders = (template, allowed) => {
  const invalid = new Set()
  for (const match of String(template || '').matchAll(placeholderPattern)) {
    if (!allowed.includes(match[1])) invalid.add(match[1])
  }
  const remaining = String(template || '').replace(placeholderPattern, '')
  if (remaining.includes('{') || remaining.includes('}')) invalid.add('格式错误')
  return [...invalid]
}

export const renderSEOTemplate = (template, values = {}) => {
  const resolvedValues = { ...exampleValues, ...values }
  return String(template || '')
    .replace(placeholderPattern, (_, name) => String(resolvedValues[name] || '').trim())
    .trim()
    .replace(/\s+/g, ' ')
    .replace(/(?:\s*[-|·]\s*){2,}/g, ' - ')
    .replace(/^[\s\-|·]+|[\s\-|·]+$/g, '')
    .trim()
}

export const isSiteRootURL = (value) => {
  try {
    const parsed = new URL(String(value || '').trim())
    return ['http:', 'https:'].includes(parsed.protocol) &&
      !parsed.username && !parsed.password && !parsed.search && !parsed.hash &&
      (parsed.pathname === '/' || parsed.pathname === '')
  } catch {
    return false
  }
}
