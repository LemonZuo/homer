import type { SVGProps } from 'react'

export function Logo(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.7}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <path d="M4.5 11.4 12 4.9l7.5 6.5" />
      <path d="M6.3 10.3v8.45h11.4V10.3" />
      <path d="m9.3 14.5 1.9 1.9 3.5-3.9" />
    </svg>
  )
}
