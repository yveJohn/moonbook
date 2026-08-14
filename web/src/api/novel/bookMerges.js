import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listBookMerges = (params) => service({ url: '/novel/bookMerges', method: 'get', params })
export const getBookMerge = (id) => service({ url: appendLongId('/novel/bookMerges', id), method: 'get' })
export const listEligibleMergeBooks = (params) => service({ url: '/novel/bookMerges/eligibleBooks', method: 'get', params })
export const previewBookMerge = (data) => service({ url: '/novel/bookMerges/preview', method: 'post', data })
export const executeBookMerge = (data) => service({ url: '/novel/bookMerges/execute', method: 'post', data })
