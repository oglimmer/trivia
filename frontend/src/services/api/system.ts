import { request } from './http'
import type { BackendBuildInfo } from '@/types'

export const systemApi = {
  version: () => request<BackendBuildInfo>('GET', '/version'),
}
