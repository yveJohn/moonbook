import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const listNovelBooks = (params) => service({ url: '/novel/books', method: 'get', params })
export const getNovelBook = (id) => service({ url: appendLongId('/novel/books', id), method: 'get' })
export const createNovelBook = (data) => service({ url: '/novel/books', method: 'post', data })
export const updateNovelBook = (id, data) => service({ url: appendLongId('/novel/books', id), method: 'put', data })
export const deleteNovelBook = (id) => service({ url: appendLongId('/novel/books', id), method: 'delete' })

export const uploadNovelBookCover = (id, file) => {
  const data = new FormData()
  data.append('file', file)
  return service({
    url: `${appendLongId('/novel/books', id)}/cover`,
    method: 'post',
    data,
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export const getNovelBookCover = (id) => service({
  url: `${appendLongId('/novel/books', id)}/cover`,
  method: 'get',
  responseType: 'blob',
  donNotShowLoading: true,
  validateStatus: (status) => status === 200 || status === 404
})
