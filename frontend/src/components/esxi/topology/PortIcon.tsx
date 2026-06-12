import type { CSSProperties } from 'react'

import portConnected from '../../../assets/esxi/portConnected16.png'
import portDisconnected from '../../../assets/esxi/portDisconnected16.png'
import { ICON_HALF, ICON_SIZE, PORT_INSET_IN_STRIP } from './constants'

// 直接用 ESXi Host Client 的 portConnected16.png / portDisconnected16.png。
// 端口图标贴在 strip 内偏外侧的位置 (ESXi padding-left:2 → 端口距 strip 外边沿 2px 内)。
export function PortIcon({
  active,
  side,
  y,
}: {
  active: boolean
  side: 'left' | 'right'
  y: number
}) {
  const style: CSSProperties =
    side === 'left'
      ? { left: PORT_INSET_IN_STRIP, top: y - ICON_HALF }
      : { right: PORT_INSET_IN_STRIP, top: y - ICON_HALF }
  return (
    <img
      src={active ? portConnected : portDisconnected}
      width={ICON_SIZE}
      height={ICON_SIZE}
      alt=""
      draggable={false}
      className="pointer-events-none absolute block"
      style={style}
    />
  )
}
