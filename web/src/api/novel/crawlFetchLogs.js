import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listCrawlFetchLogs = (params) => service({ url: '/novel/crawl/fetchLogs', method: 'get', params })
export const getCrawlFetchLog = (id) => service({ url: appendLongId('/novel/crawl/fetchLogs', id), method: 'get' })
