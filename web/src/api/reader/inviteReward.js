import service from '@/utils/request'

export const getInviteReward = () => service({ url: '/reader/inviteReward', method: 'get' })
export const updateInviteReward = (data) => service({ url: '/reader/inviteReward', method: 'put', data })
