import { useEffect, useState } from 'react'
import { Alert, Snackbar } from '@mui/material'

// Module-level pub-sub so non-React code (axios interceptors) can push
// snackbars without prop drilling or context.
type Severity = 'info' | 'success' | 'warning' | 'error'
interface SnackEvent {
  id: number
  message: string
  severity: Severity
}

let nextId = 1
let listener: ((evt: SnackEvent) => void) | null = null

export function pushSnack(message: string, severity: Severity = 'info') {
  if (!message) return
  listener?.({ id: nextId++, message, severity })
}

export default function SnackbarHost() {
  const [current, setCurrent] = useState<SnackEvent | null>(null)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    listener = (evt) => {
      setCurrent(evt)
      setOpen(true)
    }
    return () => { listener = null }
  }, [])

  return (
    <Snackbar
      key={current?.id}
      open={open}
      autoHideDuration={4000}
      onClose={(_, reason) => { if (reason !== 'clickaway') setOpen(false) }}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
    >
      {current ? (
        <Alert onClose={() => setOpen(false)} severity={current.severity} variant="filled" sx={{ minWidth: 280 }}>
          {current.message}
        </Alert>
      ) : undefined}
    </Snackbar>
  )
}
