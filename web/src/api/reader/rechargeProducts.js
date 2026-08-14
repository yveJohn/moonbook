import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listRechargeProducts = (params) => service({ url: '/reader/payment/rechargeProducts', method: 'get', params })
export const createRechargeProduct = (data) => service({ url: '/reader/payment/rechargeProducts', method: 'post', data })
export const updateRechargeProduct = (id, data) => service({ url: appendLongId('/reader/payment/rechargeProducts', id), method: 'put', data })
export const deleteRechargeProduct = (id) => service({ url: appendLongId('/reader/payment/rechargeProducts', id), method: 'delete' })
