import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listPaymentChannels = () => service({ url: '/reader/payment/channels', method: 'get' })
export const setPaymentChannelEnabled = (id, enabled) => service({ url: appendLongId('/reader/payment/channels', id), method: 'put', data: { enabled } })
export const checkPaymentChannel = (id) => service({ url: `${appendLongId('/reader/payment/channels', id)}/check`, method: 'post' })
