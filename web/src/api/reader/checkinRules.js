import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listCheckinRules = (params) => service({ url: '/reader/checkinRules', method: 'get', params })
export const createCheckinRule = (data) => service({ url: '/reader/checkinRules', method: 'post', data })
export const updateCheckinRule = (id, data) => service({ url: appendLongId('/reader/checkinRules', id), method: 'put', data })
export const deleteCheckinRule = (id) => service({ url: appendLongId('/reader/checkinRules', id), method: 'delete' })
