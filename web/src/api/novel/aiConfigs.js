import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listAIConfigs = (params) => service({ url: '/novel/aiConfigs', method: 'get', params })
export const listEnabledAIConfigs = () => service({ url: '/novel/aiConfigs/enabledOptions', method: 'get' })
export const getAIConfig = (id) => service({ url: appendLongId('/novel/aiConfigs', id), method: 'get' })
export const createAIConfig = (data) => service({ url: '/novel/aiConfigs', method: 'post', data })
export const updateAIConfig = (id, data) => service({ url: appendLongId('/novel/aiConfigs', id), method: 'put', data })
export const resetAIConfigModelState = (id) => service({ url: `${appendLongId('/novel/aiConfigs', id)}/resetModelState`, method: 'post' })
export const deleteAIConfig = (id) => service({ url: appendLongId('/novel/aiConfigs', id), method: 'delete' })
