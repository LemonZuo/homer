import { useId } from 'react'

import type { PowerSource } from './types'

export function BatteryCell({
  percent,
  powerSource = 'unknown',
  charging = false,
}: {
  percent: number
  powerSource?: PowerSource
  charging?: boolean
}) {
  const hasData = percent >= 0
  const safe = hasData ? Math.max(0, Math.min(100, percent)) : 0
  const tone = !hasData
    ? { top: '#a1a1aa', bot: '#71717a' }
    : safe <= 20
      ? { top: '#f43f5e', bot: '#be123c' }
      : safe <= 50
        ? { top: '#f59e0b', bot: '#b45309' }
        : { top: '#34d399', bot: '#10b981' }

  const W = 50
  const H = 60
  const r = 10
  const fillH = hasData ? (safe / 100) * H : 0
  const fillTop = H - fillH

  const uid = useId().replace(/:/g, '')
  const gradId = `bat-${uid}-g`
  const clipId = `bat-${uid}-c`

  // mains 走极轻液面呼吸,battery / low_battery 走更明显的脉动,lb 节奏更急。
  const showMainsBreath = hasData && powerSource === 'mains'
  const showBlink = hasData && (powerSource === 'battery' || powerSource === 'low_battery')
  const blinkDur = powerSource === 'low_battery' ? '1.1s' : '2s'

  return (
    <svg
      width={W}
      height={H}
      viewBox={`0 0 ${W} ${H}`}
      className="shrink-0 text-muted-foreground"
      aria-label={hasData ? `电量 ${safe}%` : '电量未知'}
    >
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={tone.top} />
          <stop offset="100%" stopColor={tone.bot} />
        </linearGradient>
        <clipPath id={clipId}>
          <rect x={0} y={0} width={W} height={H} rx={r} ry={r} />
        </clipPath>
      </defs>

      <rect width={W} height={H} rx={r} ry={r} fill="currentColor" opacity={0.1} />

      <g clipPath={`url(#${clipId})`}>
        {hasData && (
          <rect x={0} y={fillTop} width={W} height={fillH} fill={`url(#${gradId})`}>
            {showBlink && (
              <animate
                attributeName="opacity"
                values="1;0.55;1"
                dur={blinkDur}
                repeatCount="indefinite"
              />
            )}
            {showMainsBreath && (
              <animate
                attributeName="opacity"
                values="1;0.72;1"
                dur="5s"
                calcMode="spline"
                keySplines="0.4 0 0.6 1;0.4 0 0.6 1"
                repeatCount="indefinite"
              />
            )}
          </rect>
        )}
      </g>

      {/* 充电闪电:浮在液面之上,淡黄填 + 深色描边保证任意背景对比;opacity 缓脉动。*/}
      {hasData && charging && (
        <g transform="translate(8.2 13.2) scale(1.4)" opacity={0.9}>
          <path
            d="M13 2 L3 14 L12 14 L11 22 L21 10 L12 10 L13 2 Z"
            fill="#fef08a"
            stroke="#ca8a04"
            strokeWidth={0.6}
            strokeLinejoin="round"
            vectorEffect="non-scaling-stroke"
          />
          <animate
            attributeName="opacity"
            values="0.5;1;0.5"
            dur="2.4s"
            repeatCount="indefinite"
          />
        </g>
      )}

      {!hasData && (
        <text
          x={W / 2}
          y={H / 2 + 4}
          textAnchor="middle"
          fontSize={12}
          fill="currentColor"
          opacity={0.5}
        >
          —
        </text>
      )}
    </svg>
  )
}
