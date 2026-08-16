// Placeholder GLKJ brand mark: a 2x2 tile grid (three solid tiles, one hollow).
// Replace with the official logo art when company assets arrive — see BRANDING.md.

import type { IconProps } from './icons/props.ts'

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
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <rect x="3" y="3" width="8" height="8" rx="2" fill="currentColor" />
      <rect x="13" y="3" width="8" height="8" rx="2" fill="currentColor" />
      <rect x="3" y="13" width="8" height="8" rx="2" fill="currentColor" />
      <rect x="14.5" y="14.5" width="5" height="5" rx="1.5" stroke="currentColor" strokeWidth="2" />
    </svg>
  )
}
