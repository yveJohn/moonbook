import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listNovelChapters = (params) => service({ url: '/novel/chapters', method: 'get', params })
export const getNovelChapter = (id) => service({ url: appendLongId('/novel/chapters', id), method: 'get' })
export const getNovelChapterContent = (id) => service({ url: `${appendLongId('/novel/chapters', id)}/content`, method: 'get' })
export const createNovelChapter = (data) => service({ url: '/novel/chapters', method: 'post', data })
export const updateNovelChapter = (id, data) => service({ url: appendLongId('/novel/chapters', id), method: 'put', data })
export const deleteNovelChapter = (id) => service({ url: appendLongId('/novel/chapters', id), method: 'delete' })
