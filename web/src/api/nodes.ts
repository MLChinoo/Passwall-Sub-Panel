import { client } from './client'
import type { ListResponse, Node, UnmanagedInbound } from './types'

export interface ImportNodeRequest {
  panel_id: number
  inbound_id: number
  display_name: string
  server_address: string
  region: string
  tags?: string[]
  sort_order?: number
}

export interface UpdateNodeMetadataRequest {
  display_name?: string
  server_address?: string
  region?: string
  tags?: string[]
  sort_order?: number
}

export async function listNodes() {
  const { data } = await client.get<{ items: Node[] }>('/admin/nodes')
  return data.items
}

export async function getNode(id: number) {
  const { data } = await client.get<{ node: Node; clients: unknown[] }>(`/admin/nodes/${id}`)
  return data
}

export async function importNode(req: ImportNodeRequest) {
  const { data } = await client.post<Node>('/admin/nodes/import', req)
  return data
}

export interface InboundSpec {
  remark: string
  enable: boolean
  listen: string
  port: number
  protocol: string
  settings: string
  stream_settings: string
  sniffing: string
  allocate: string
}

export interface CreateInboundRequest {
  panel_id: number
  display_name: string
  server_address: string
  region: string
  tags?: string[]
  sort_order?: number
  inbound: InboundSpec
}

export async function createInbound(req: CreateInboundRequest) {
  const { data } = await client.post<Node | { queued: true }>('/admin/nodes', req)
  return data
}

export interface RealityKeypair {
  private_key: string
  public_key: string
  short_id: string
}

export async function generateRealityKeypair() {
  const { data } = await client.post<RealityKeypair>('/admin/nodes/generate-reality-keypair')
  return data
}

export async function updateNodeMetadata(id: number, req: UpdateNodeMetadataRequest) {
  const { data } = await client.put<Node>(`/admin/nodes/${id}/metadata`, req)
  return data
}

export async function setNodeEnabled(id: number, enabled: boolean) {
  await client.post(`/admin/nodes/${id}/set-enabled`, { enabled })
}

export async function deleteNode(id: number) {
  await client.delete(`/admin/nodes/${id}`)
}

export async function listUnmanagedInbounds() {
  const { data } = await client.get<ListResponse<UnmanagedInbound>>('/admin/nodes/unmanaged')
  return data
}

export async function claimClient(req: {
  user_id: number
  panel_id: number
  inbound_id: number
  client_email: string
  client_uuid: string
}) {
  await client.post('/admin/nodes/-/claim', req)
}
