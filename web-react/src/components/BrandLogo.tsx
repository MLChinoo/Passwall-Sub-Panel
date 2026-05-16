import { Box } from '@mui/material'
import { useSiteStore, selectLogoLight, selectLogoDark } from '@/stores/site'
import { useAppearanceStore } from '@/stores/appearance'

interface Props {
  // Logo height in px. The image keeps its aspect ratio; width grows naturally.
  height?: number
}

// Switches between light/dark logo automatically based on the active theme mode.
// Use this anywhere you want the brand mark; it's NOT the favicon (that's handled
// by site store's applyDocumentBranding() using site.iconUrl).
export default function BrandLogo({ height = 32 }: Props) {
  const mode = useAppearanceStore(s => s.mode)
  const logoLight = useSiteStore(selectLogoLight)
  const logoDark = useSiteStore(selectLogoDark)
  const src = mode === 'dark' ? logoDark : logoLight
  return (
    <Box
      component="img"
      src={src}
      alt="Logo"
      sx={{ height, width: 'auto', display: 'block', objectFit: 'contain' }}
    />
  )
}
