import type { SVGProps } from 'react'

// 屋檐下的管家服务铃：屋顶=home/私人，铃铛=主动提醒（生日/事项/告警）+ 西式管家 service bell 双关。
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
      <path d="M2.5 8.8 12 2.5l9.5 6.3" />
      <path d="M12 6.1v2.7" />
      <path d="M7.5 17.5c0-5 1.9-8.8 4.5-8.8s4.5 3.8 4.5 8.8" />
      <path d="M6.4 17.5h11.2" />
      <circle cx="12" cy="20.1" r="1.2" fill="currentColor" stroke="none" />
    </svg>
  )
}
