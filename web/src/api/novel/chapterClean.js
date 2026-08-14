import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

const base = '/novel/chapterClean'
export const getChapterCleanConfig = () => service({ url: `${base}/config`, method: 'get' })
export const saveChapterCleanConfig = (data) => service({ url: `${base}/config`, method: 'put', data })
export const listChapterCleanTasks = (params) => service({ url: `${base}/tasks`, method: 'get', params })
export const startChapterCleanTask = (data) => service({ url: `${base}/tasks`, method: 'post', data })
export const stopChapterCleanTask = (id) => service({ url: `${appendLongId(`${base}/tasks`, id)}/stop`, method: 'post' })
export const resumeChapterCleanTask = (id) => service({ url: `${appendLongId(`${base}/tasks`, id)}/resume`, method: 'post' })
export const listChapterCleanResults = (params) => service({ url: `${base}/results`, method: 'get', params })
export const getChapterCleanResult = (id) => service({ url: appendLongId(`${base}/results`, id), method: 'get' })
export const reviewChapterCleanResult = (id, data) => service({ url: `${appendLongId(`${base}/results`, id)}/review`, method: 'put', data })
export const recleanChapterCleanResult = (id) => service({ url: `${appendLongId(`${base}/results`, id)}/reclean`, method: 'post' })
