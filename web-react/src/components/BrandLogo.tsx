import { Box, useTheme } from '@mui/material'
import { useSiteStore, selectLogoLight, selectLogoDark } from '@/stores/site'

interface Props {
  // Logo height in px. The image keeps its aspect ratio; width grows naturally.
  height?: number
}

// Switches between light/dark logo based on the EFFECTIVE theme mode.
// We read theme.palette.mode (not the appearance store's raw mode) because
// the App layer already resolved 'auto' against prefers-color-scheme — so
// when the user picks Auto and the OS is dark, the dark logo wins. Use
// this anywhere you want the brand mark; it's NOT the favicon (handled by
// site store's applyDocumentBranding() using site.iconUrl).
export default function BrandLogo({ height = 32 }: Props) {
  const theme = useTheme()
  const logoLight = useSiteStore(selectLogoLight)
  const logoDark = useSiteStore(selectLogoDark)
  const src = theme.palette.mode === 'dark' ? logoDark : logoLight
  return (
    <Box
      component="img"
      src={src}
      alt="Logo"
      sx={{ height, width: 'auto', display: 'block', objectFit: 'contain' }}
    />
  )
}
