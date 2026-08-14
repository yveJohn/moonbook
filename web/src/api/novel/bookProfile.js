import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

const base = '/novel/bookProfile'
export const getBookProfileConfig = () => service({ url: `${base}/config`, method: 'get' })
export const saveBookProfileConfig = (data) => service({ url: `${base}/config`, method: 'put', data })
export const listBookProfileSuggestions = (params) => service({ url: `${base}/suggestions`, method: 'get', params })
export const getBookProfileSuggestion = (id) => service({ url: appendLongId(`${base}/suggestions`, id), method: 'get' })
export const generateBookProfileSuggestion = (data) => service({ url: `${base}/suggestions`, method: 'post', data })
export const batchRegenerateBookProfileSuggestions = (data) => service({ url: `${base}/suggestions/batch-regenerate`, method: 'post', data })
export const rejectBookProfileSuggestion = (id, data) => service({ url: `${appendLongId(`${base}/suggestions`, id)}/reject`, method: 'put', data })
export const applyBookProfileSuggestion = (id, data) => service({ url: `${appendLongId(`${base}/suggestions`, id)}/apply`, method: 'put', data })
