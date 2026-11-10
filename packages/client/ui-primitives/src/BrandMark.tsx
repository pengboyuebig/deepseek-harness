// Placeholder GLKJ brand mark: a 2x2 tile grid (three solid tiles, one hollow).
// Replace with the official logo art when company assets arrive — see BRANDING.md.

import type { IconProps } from './icons/props.ts'

/** Native viewBox of {@link FISH_LOGO_PATH} (square, 24x24 user units). */
export const FISH_LOGO_VIEWBOX = { width: 24, height: 24 }

/**
 * Tile geometry of the GLKJ mark, exported for consumers that compose their own
 * svg (entrance effects, masks) around the same shape.
 */
export const FISH_LOGO_PATH =
  'M5 3h4a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Z' +
  'M15 3h4a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Z' +
  'M5 13h4a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2Z'

/**
 * Render the GLKJ brand mark.
 * @param props.size - width/height in px (default 24; the mark is square).
 * @param props.className - extra class for layout placement.
 * @returns the logo svg (aria-hidden; pair with the wordmark for accessibility).
 */
export function BrandMark({ size = 24, className }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      className={className}
      viewBox={`0 0 ${FISH_LOGO_VIEWBOX.width} ${FISH_LOGO_VIEWBOX.height}`}
      fill="none"
      aria-hidden="true"
    >
      <path d={FISH_LOGO_PATH} fill="currentColor" />
      <rect x="14.5" y="14.5" width="5" height="5" rx="1.5" stroke="currentColor" strokeWidth="2" />
    </svg>
  )
}
