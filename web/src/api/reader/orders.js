import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listOrders = (params) => service({ url: '/reader/orders', method: 'get', params })
export const getOrder = (id) => service({ url: appendLongId('/reader/orders', id), method: 'get' })
