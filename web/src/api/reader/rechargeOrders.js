import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listRechargeOrders = (params) => service({ url: '/reader/payment/orders', method: 'get', params })
export const getRechargeOrder = (id) => service({ url: appendLongId('/reader/payment/orders', id), method: 'get' })
export const manualPayRechargeOrder = (id, data) => service({ url: `${appendLongId('/reader/payment/orders', id)}/manualPay`, method: 'post', data })
export const syncRechargeOrder = (id) => service({ url: `${appendLongId('/reader/payment/orders', id)}/sync`, method: 'post' })
