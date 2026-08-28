import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listPaymentChannels = (includeArchived = false) => service({ url: '/reader/payment/channels', method: 'get', params: { includeArchived } })
export const getPaymentChannel = (id) => service({ url: appendLongId('/reader/payment/channels', id), method: 'get' })
export const createPaymentChannel = (data) => service({ url: '/reader/payment/channels', method: 'post', data })
export const updatePaymentChannel = (id, data) => service({ url: appendLongId('/reader/payment/channels', id), method: 'put', data })
export const archivePaymentChannel = (id) => service({ url: appendLongId('/reader/payment/channels', id), method: 'delete' })
export const checkPaymentChannel = (id) => service({ url: `${appendLongId('/reader/payment/channels', id)}/check`, method: 'post' })
