import service from '@/utils/request'

export const getDashboardOverview = () => service({
  url: '/dashboard/overview',
  method: 'get',
  donNotShowLoading: true
})
