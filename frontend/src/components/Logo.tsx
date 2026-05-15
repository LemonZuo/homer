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
      <path d="M12 1.5 20 4v7c0 5-3.6 9.4-8 11-4.4-1.6-8-6-8-11V4z" />
      <path d="m7.05 8.65 4.95 7.7 4.95-7.7" />
    </svg>
  )
}
