import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listOrders = (params) => service({ url: '/reader/orders', method: 'get', params })
export const getOrder = (id) => service({ url: appendLongId('/reader/orders', id), method: 'get' })
export const createMockRecharge = (data) => service({ url: '/reader/orders/mockRecharge', method: 'post', data })
export const confirmMockRecharge = (id) => service({ url: `${appendLongId('/reader/orders', id)}/confirmMockRecharge`, method: 'post' })
