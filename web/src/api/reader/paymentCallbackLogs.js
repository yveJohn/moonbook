import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listPaymentCallbackLogs = (params) => service({ url: '/reader/payment/callbackLogs', method: 'get', params })
export const getPaymentCallbackLog = (id) => service({ url: appendLongId('/reader/payment/callbackLogs', id), method: 'get' })
