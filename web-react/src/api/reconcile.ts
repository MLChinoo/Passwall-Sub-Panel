import { client } from './client'

export interface ReconcileIssue {
  panel_name?: string
  client_email?: string
  detail?: string
}

export interface ReconcileReport {
  scanned: number
  fixed: number
  issues: ReconcileIssue[]
}

export async function runReconcile(): Promise<ReconcileReport> {
  const { data } = await client.post<ReconcileReport>('/admin/reconcile/run')
  return data
}
