import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listReaderFeedback = (params) => service({ url: '/reader/feedback', method: 'get', params })
export const getReaderFeedback = (id) => service({ url: appendLongId('/reader/feedback', id), method: 'get' })
export const replyReaderFeedback = (id, reply) => service({ url: `${appendLongId('/reader/feedback', id)}/reply`, method: 'put', data: { reply } })
