import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listReaderUsers = (params) => service({ url: '/reader/users', method: 'get', params })
export const getReaderUser = (id) => service({ url: appendLongId('/reader/users', id), method: 'get' })
export const setReaderUserStatus = (id, status) => service({ url: `${appendLongId('/reader/users', id)}/status`, method: 'put', data: { status } })
export const resetReaderUserPassword = (id, data) => service({ url: `${appendLongId('/reader/users', id)}/password`, method: 'put', data })
export const grantReaderMembership = (id, data) => service({ url: `${appendLongId('/reader/users', id)}/membership`, method: 'post', data })
export const listReaderUserOperations = (id, params) => service({ url: `${appendLongId('/reader/users', id)}/operations`, method: 'get', params })
