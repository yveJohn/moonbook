import service from '@/utils/request'

export const getRechargeSettings = () => service({ url: '/reader/payment/rechargeSettings', method: 'get' })
export const updateRechargeSettings = (data) => service({ url: '/reader/payment/rechargeSettings', method: 'put', data })
