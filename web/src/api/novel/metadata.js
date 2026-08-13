import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listNovelCategories = (params) =>
  service({ url: '/novel/categories', method: 'get', params })

export const createNovelCategory = (data) =>
  service({ url: '/novel/categories', method: 'post', data })

export const updateNovelCategory = (id, data) =>
  service({ url: appendLongId('/novel/categories', id), method: 'put', data })

export const deleteNovelCategory = (id) =>
  service({ url: appendLongId('/novel/categories', id), method: 'delete' })

export const listNovelAuthors = (params) =>
  service({ url: '/novel/authors', method: 'get', params })

export const createNovelAuthor = (data) =>
  service({ url: '/novel/authors', method: 'post', data })

export const updateNovelAuthor = (id, data) =>
  service({ url: appendLongId('/novel/authors', id), method: 'put', data })

export const deleteNovelAuthor = (id) =>
  service({ url: appendLongId('/novel/authors', id), method: 'delete' })
