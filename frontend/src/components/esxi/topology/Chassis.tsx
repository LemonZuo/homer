import {
  BUS_W,
  CHASSIS_BG,
  CHASSIS_BORDER,
  COL_W_CH,
  LINE_COLOR,
  PG_LINE_W,
  STRIP_BG,
  STRIP_BORDER,
  STRIP_W,
} from './constants'
import { PortIcon } from './PortIcon'
import type { StripBlock } from './types'

// Chassis:对齐 ESXi 的 .esx-networking-viz-center —— 浅灰底盒(#ccccd0) + 1px 边框 + 3px 圆角。
// 内部左/右贴若干 #bababa 端口条 (对齐 .esx-networking-viz-center-portgroup-container / -vmnic-box),
// 每个 strip block 三面 border + 圆角朝向 chassis 内部,左 strip 无左 border,右 strip 无右 border。
// 端口图标贴在 strip 内偏外侧 (padding-left:2 / padding-right:2)。
// 中央贯穿一根 1px 黑色 BUS 线 (对齐 ESXi blackPixelVertical.png 在 portgroup-block 右沿画的竖线),
// BUS 线 y 范围 = 全部 anchor 的 min y → max y。
// 名字徽章悬在顶部正中。
export function Chassis({
  name,
  height,
  leftBlocks,
  rightBlocks,
}: {
  name: string
  height: number
  leftBlocks: StripBlock[]
  rightBlocks: StripBlock[]
}) {
  const busX = COL_W_CH / 2

  return (
    <div
      className="relative rounded-[3px]"
      style={{
        width: COL_W_CH,
        height,
        backgroundColor: CHASSIS_BG,
        border: `1px solid ${CHASSIS_BORDER}`,
        boxShadow: '0 3px 5px rgba(0,0,0,0.25)',
        backgroundImage:
          'linear-gradient(to bottom, rgba(255,255,255,0.18), transparent 60%)',
      }}
    >
      {/* 每个 strip block 只有一根从 strip 内沿到 BUS 的横线 (对齐 ESXi portgroup-block-border-horizontal,
          按 PG/uplink 分组渲染,不按虚拟机数量渲染),y 取该 block 所有 anchor 的几何中心 */}
      <svg
        width={COL_W_CH}
        height={height}
        className="pointer-events-none absolute inset-0"
        aria-hidden
      >
        {leftBlocks.map((b, bi) => {
          const y = (b.anchors[0].y + b.anchors[b.anchors.length - 1].y) / 2
          return (
            <line
              key={`lh-${bi}`}
              x1={STRIP_W}
              y1={y}
              x2={busX}
              y2={y}
              stroke={LINE_COLOR}
              strokeWidth={PG_LINE_W}
              shapeRendering="crispEdges"
            />
          )
        })}
        {rightBlocks.map((b, bi) => {
          const y = (b.anchors[0].y + b.anchors[b.anchors.length - 1].y) / 2
          return (
            <line
              key={`rh-${bi}`}
              x1={busX}
              y1={y}
              x2={COL_W_CH - STRIP_W}
              y2={y}
              stroke={LINE_COLOR}
              strokeWidth={PG_LINE_W}
              shapeRendering="crispEdges"
            />
          )
        })}
      </svg>

      {/* 中央 BUS 垂直线:贯穿整个机箱,2px 黑,比横线粗 */}
      <div
        className="absolute"
        style={{
          left: busX - BUS_W / 2,
          top: 0,
          width: BUS_W,
          height: '100%',
          backgroundColor: LINE_COLOR,
        }}
      />

      {/* 左侧 strip blocks */}
      {leftBlocks.map((b, i) => (
        <div
          key={`lb-${i}`}
          className="absolute"
          style={{
            left: 0,
            top: b.topY,
            width: STRIP_W,
            height: b.bottomY - b.topY,
            backgroundColor: STRIP_BG,
            borderTop: `1px solid ${STRIP_BORDER}`,
            borderBottom: `1px solid ${STRIP_BORDER}`,
            borderRight: `1px solid ${STRIP_BORDER}`,
            borderRadius: '0 5px 5px 0',
            backgroundImage:
              'linear-gradient(45deg, rgba(255,255,255,0.25), transparent)',
          }}
        >
          {b.anchors.map((a, ai) => (
            <PortIcon
              key={`la-${ai}`}
              active={a.active}
              side="left"
              y={a.y - b.topY}
            />
          ))}
        </div>
      ))}

      {/* 右侧 strip blocks */}
      {rightBlocks.map((b, i) => (
        <div
          key={`rb-${i}`}
          className="absolute"
          style={{
            right: 0,
            top: b.topY,
            width: STRIP_W,
            height: b.bottomY - b.topY,
            backgroundColor: STRIP_BG,
            borderTop: `1px solid ${STRIP_BORDER}`,
            borderBottom: `1px solid ${STRIP_BORDER}`,
            borderLeft: `1px solid ${STRIP_BORDER}`,
            borderRadius: '5px 0 0 5px',
            backgroundImage:
              'linear-gradient(225deg, rgba(255,255,255,0.25), transparent)',
          }}
        >
          {b.anchors.map((a, ai) => (
            <PortIcon
              key={`ra-${ai}`}
              active={a.active}
              side="right"
              y={a.y - b.topY}
            />
          ))}
        </div>
      ))}

      {/* vSwitch 名嵌入到机箱右上角 (z-10 浮于 strip 之上) */}
      <div
        className="pointer-events-auto absolute z-10 whitespace-nowrap font-mono text-[10px] font-medium leading-none text-foreground"
        style={{ top: 4, right: 4 }}
        title={name}
      >
        {name}
      </div>
    </div>
  )
}
