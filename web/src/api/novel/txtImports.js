import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listTxtImports = (params) => service({ url: '/novel/txtImports', method: 'get', params })
export const getTxtImport = (id) => service({ url: appendLongId('/novel/txtImports', id), method: 'get' })
export const previewTxtImport = (id) => service({ url: appendLongId('/novel/txtImports', id) + '/preview', method: 'get' })
export const uploadTxtImport = (data) => service({ url: '/novel/txtImports', method: 'post', data, headers: { 'Content-Type': 'multipart/form-data' } })
export const retryTxtImport = (id) => service({ url: appendLongId('/novel/txtImports', id) + '/retry', method: 'post' })
export const cancelTxtImport = (id) => service({ url: appendLongId('/novel/txtImports', id) + '/cancel', method: 'post' })
