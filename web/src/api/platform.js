import service from '@/utils/request'
import { appendLongId } from '@/utils/longId'

export const getPlatformJobs = (params) => service({
  url: '/platform/jobs',
  method: 'get',
  params
})

export const getPlatformJob = (id) => service({
  url: appendLongId('/platform/jobs', id),
  method: 'get'
})
