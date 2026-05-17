import { cn } from '../../lib/utils'

export function AliyunIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      role="img"
      aria-label="阿里云"
      fill="currentColor"
      className={cn('text-[#FF6A00]', className)}
    >
      <path d="M5.483 7.18C2.46 7.18 0 9.64 0 12.665v.001c0 3.022 2.46 5.482 5.483 5.482h2.05l-.736-2.527H6.18a2.96 2.96 0 0 1-2.954-2.955v-.001A2.96 2.96 0 0 1 6.181 9.71h1.32l.74-2.531H5.484v.001zM18.517 7.179h-2.05l.737 2.531h.616a2.96 2.96 0 0 1 2.955 2.955v.001a2.96 2.96 0 0 1-2.955 2.955h-1.318l-.741 2.527h2.756c3.022 0 5.483-2.46 5.483-5.482v-.001c0-3.024-2.46-5.485-5.483-5.485v-.001zM8.834 14.069h6.354l-.747-2.115H9.55l-.716 2.115z" />
    </svg>
  )
}
