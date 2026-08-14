import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listCrawlSources = (params) => service({ url: '/novel/crawl/sources', method: 'get', params })
export const createCrawlSource = (data) => service({ url: '/novel/crawl/sources', method: 'post', data })
export const updateCrawlSource = (id, data) => service({ url: appendLongId('/novel/crawl/sources', id), method: 'put', data })
export const deleteCrawlSource = (id) => service({ url: appendLongId('/novel/crawl/sources', id), method: 'delete' })
