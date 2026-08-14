import service from '@/utils/request'

const base = '/novel/chapterSummary'

export const getChapterSummaryConfig = () => service({ url: `${base}/config`, method: 'get' })
export const saveChapterSummaryConfig = (data) => service({ url: `${base}/config`, method: 'put', data })
export const getChapterSummaryStatus = () => service({ url: `${base}/status`, method: 'get' })
export const listChapterSummaryTasks = (params) => service({ url: `${base}/tasks`, method: 'get', params })
export const getChapterSummaryTask = (id) => service({ url: `${base}/tasks/${id}`, method: 'get' })
export const startChapterSummaryTask = (data = {}) => service({ url: `${base}/tasks`, method: 'post', data })
export const stopChapterSummaryTask = (id) => service({ url: `${base}/tasks/${id}/stop`, method: 'post' })
export const resumeChapterSummaryTask = (id) => service({ url: `${base}/tasks/${id}/resume`, method: 'post' })
