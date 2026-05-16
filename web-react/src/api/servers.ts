import { client } from './client'

export interface Server {
  id: number
  name: string
  url: string
  username?: string
  remark?: string
  has_api_token: boolean
  has_password: boolean
}

export interface CreateServerRequest {
  name: string
  url: string
  api_token?: string
  username?: string
  password?: string
  remark?: string
}

export interface UpdateServerRequest {
  name?: string
  url?: string
  api_token?: string
  username?: string
  password?: string
  remark?: string
}

export interface TestResult {
  ok: boolean
  error?: string
  inbound_count?: number
}

export async function listServers() {
  const { data } = await client.get<{ items: Server[] }>('/admin/servers')
  return data.items
}

export async function createServer(req: CreateServerRequest) {
  const { data } = await client.post<Server>('/admin/servers', req)
  return data
}

export async function updateServer(id: number, req: UpdateServerRequest) {
  const { data } = await client.put<Server>(`/admin/servers/${id}`, req)
  return data
}

export async function deleteServer(id: number) {
  await client.delete(`/admin/servers/${id}`)
}

export async function testServer(id: number) {
  const { data } = await client.post<TestResult>('/admin/servers/probe', { id })
  return data
}
