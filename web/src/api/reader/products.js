import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listProducts = (params) => service({ url: '/reader/products', method: 'get', params })
export const createProduct = (data) => service({ url: '/reader/products', method: 'post', data })
export const updateProduct = (id, data) => service({ url: appendLongId('/reader/products', id), method: 'put', data })
export const deleteProduct = (id) => service({ url: appendLongId('/reader/products', id), method: 'delete' })
