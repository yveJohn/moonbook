import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'
export const listInviteCodes = (params) => service({ url: '/reader/inviteCodes', method: 'get', params })
export const createInviteCode = (data) => service({ url: '/reader/inviteCodes', method: 'post', data })
export const setInviteCodeStatus = (id, status) => service({ url: `${appendLongId('/reader/inviteCodes', id)}/status`, method: 'put', data: { status } })
export const deleteInviteCode = (id) => service({ url: appendLongId('/reader/inviteCodes', id), method: 'delete' })
