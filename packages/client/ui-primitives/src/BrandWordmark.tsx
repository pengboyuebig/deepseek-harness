// Placeholder GLKJ Harness wordmark rendered as system-font text ("GLKJ" bold +
// "Harness" regular), kept at the historical native 182x24 so layouts do not
// shift. Replace with the official wordmark art when company assets arrive —
// see BRANDING.md.

import type { IconProps } from './icons/props.ts'

/**
 * Render the full GLKJ brand wordmark.
 * @param props.size - height in px (default 24; width keeps the 182:24 ratio).
 * @param props.className - extra class for layout placement.
 * @returns the wordmark svg (aria-hidden decorative brand art).
 */
export function BrandWordmark({ size = 24, className }: IconProps) {
  return (
    <svg
      width={(size * 182) / 24}
      height={size}
      className={className}
      viewBox="0 0 182 24"
      fill="none"
      aria-hidden="true"
    >
      <text
        x="0"
        y="18.5"
        fontFamily="system-ui, -apple-system, 'Segoe UI', sans-serif"
        fontSize="17"
        fontWeight="700"
        letterSpacing="0.5"
        fill="currentColor"
      >
        GLKJ
      </text>
      <text
        x="48"
        y="18.5"
        fontFamily="system-ui, -apple-system, 'Segoe UI', sans-serif"
        fontSize="16"
        fill="currentColor"
      >
        Harness
      </text>
    </svg>
  )
}
