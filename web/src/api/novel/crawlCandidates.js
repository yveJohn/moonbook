import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listCrawlCandidates = (params) => service({ url: '/novel/crawl/candidates', method: 'get', params })
export const getCrawlCandidate = (id) => service({ url: appendLongId('/novel/crawl/candidates', id), method: 'get' })
export const skipCrawlCandidates = (ids) => service({ url: '/novel/crawl/candidates/skip', method: 'put', data: { ids } })
export const restoreCrawlCandidates = (ids) => service({ url: '/novel/crawl/candidates/restore', method: 'put', data: { ids } })
export const deleteCrawlCandidates = (ids) => service({ url: '/novel/crawl/candidates', method: 'delete', data: { ids } })
export const discoverCrawlCandidates = (boardId) => service({ url: '/novel/crawl/candidates/discover', method: 'post', data: { boardId: String(boardId).trim() } })
