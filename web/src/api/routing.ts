import { request } from './client'

export interface RoutingRule {
  id: number
  position: number
  rule_tag: string
  domains_json: string
  ips_json: string
  ports_json: string
  source_ips_json: string
  network: string
  inbound_tags_json: string
  protocol_json: string
  outbound_tag: string
  enabled: boolean
  created_at: number
}

export interface RoutingRuleInput {
  rule_tag: string
  domains_json: string
  ips_json: string
  ports_json: string
  source_ips_json: string
  network: string
  inbound_tags_json: string
  protocol_json: string
  outbound_tag: string
  enabled: boolean
}

export interface Preset {
  name: string
  description: string
  rule_count: number
}

export async function listRoutingRules(): Promise<RoutingRule[]> {
  const res = await request<{ rules: RoutingRule[] }>('/routing/rules')
  return res.rules ?? []
}

export async function createRoutingRule(input: RoutingRuleInput): Promise<number> {
  const res = await request<{ id: number }>('/routing/rules', {
    method: 'POST',
    body: input,
    withCSRF: true,
  })
  return res.id
}

export async function updateRoutingRule(id: number, input: RoutingRuleInput): Promise<void> {
  await request(`/routing/rules/${id}`, {
    method: 'PUT',
    body: input,
    withCSRF: true,
  })
}

export async function deleteRoutingRule(id: number): Promise<void> {
  await request(`/routing/rules/${id}`, {
    method: 'DELETE',
    withCSRF: true,
  })
}

export async function moveRoutingRule(id: number, direction: 'up' | 'down'): Promise<void> {
  await request(`/routing/rules/${id}/move?direction=${direction}`, {
    method: 'POST',
    withCSRF: true,
  })
}

export async function listPresets(): Promise<Preset[]> {
  const res = await request<{ presets: Preset[] }>('/routing/presets')
  return res.presets ?? []
}

export async function applyPreset(name: string): Promise<number> {
  const res = await request<{ ok: boolean; created: number }>(`/routing/presets/${name}`, {
    method: 'POST',
    withCSRF: true,
  })
  return res.created
}
