import service from '@/utils/request'
import { assertLongId } from '@/utils/longId'
export const listReaderWallets = (params) => service({ url: '/reader/wallets', method: 'get', params })
export const listReaderWalletLedgers = (id, params) => service({ url: `/reader/wallets/${encodeURIComponent(assertLongId(id))}/ledgers`, method: 'get', params })
export const adjustReaderWallet = (id, data) => service({ url: `/reader/wallets/${encodeURIComponent(assertLongId(id))}/adjust`, method: 'post', data })
export const listReaderCheckins = (params) => service({ url: '/reader/checkins', method: 'get', params })
export const listReaderInviteRewards = (params) => service({ url: '/reader/inviteRewards', method: 'get', params })
