/** Isotipo RDL del prototipo (design/referencias/RDL Icono). Decorativo: el nombre va en texto al lado. */
export function BrandMark({ size = 30 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden focusable="false">
      <rect width="64" height="64" rx="13" fill="var(--color-bg-nav-active)" />
      <text
        x="32"
        y="39.5"
        textAnchor="middle"
        fontFamily="IBM Plex Sans"
        fontWeight="700"
        fontSize="20"
        letterSpacing="1.2"
        fill="#FFFFFF"
      >
        RDL
      </text>
      <rect x="14" y="45" width="36" height="3" rx="1.5" fill="var(--color-brand-bar)" />
    </svg>
  )
}
