import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listCrawlBoards = (params) => service({ url: '/novel/crawl/boards', method: 'get', params })
export const createCrawlBoard = (data) => service({ url: '/novel/crawl/boards', method: 'post', data })
export const updateCrawlBoard = (id, data) => service({ url: appendLongId('/novel/crawl/boards', id), method: 'put', data })
export const deleteCrawlBoard = (id) => service({ url: appendLongId('/novel/crawl/boards', id), method: 'delete' })
