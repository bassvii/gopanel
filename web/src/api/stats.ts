import { request } from './client'
import type { StatsSummary } from './types'

export async function getStatsSummary(): Promise<StatsSummary> {
  return request<StatsSummary>('/stats/summary')
}
