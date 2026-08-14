import service from '@/utils/request'

export const getReaderSEOConfig = () => service({
  url: '/novel/readerSeo/config',
  method: 'get'
})

export const updateReaderSEOConfig = (data) => service({
  url: '/novel/readerSeo/config',
  method: 'put',
  data
})
