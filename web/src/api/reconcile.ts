import { client } from './client'

export interface ReconcileReport {
  scanned: number
  fixed: number
  issues: string[]
}

export async function runReconcile(): Promise<ReconcileReport> {
  const res = await client.post('/admin/reconcile/run')
  return res.data
}
