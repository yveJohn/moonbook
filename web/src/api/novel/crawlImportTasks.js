import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listCrawlImportTasks = (params) => service({ url: '/novel/crawl/importTasks', method: 'get', params })
export const getCrawlImportTask = (id) => service({ url: appendLongId('/novel/crawl/importTasks', id), method: 'get' })
export const createCrawlImportTask = (data) => service({ url: '/novel/crawl/importTasks', method: 'post', data })
export const retryCrawlImportTask = (id, data = {}) => service({ url: appendLongId('/novel/crawl/importTasks', id) + '/retry', method: 'post', data })
export const cancelCrawlImportTask = (id) => service({ url: appendLongId('/novel/crawl/importTasks', id) + '/cancel', method: 'post' })
