import { useMemo, useState } from 'react'
import { Box, ChevronDown, ChevronUp, Network, Server, Waypoints } from 'lucide-react'
import { Card } from '../ui/card'
import portConnected from '../../assets/esxi/portConnected16.png'
import portDisconnected from '../../assets/esxi/portDisconnected16.png'

interface NIC {
  name: string
  driver?: string
  mac: string
  link_status: string
  speed_mbps: number
  duplex?: string
}
interface VSwitchInfo {
  name: string
  uplinks: string[]
  portgroups: string[]
}
interface VMNICLink {
  vm_name: string
  vswitch: string
  portgroup: string
  mac: string
  team_uplink: string
}
export interface NetTopology {
  vswitches: VSwitchInfo[]
  vm_nics: VMNICLink[]
}
type VMInfo = { vmName: string; teamUplink: string; mac: string }

// 列宽/行高:整套像素布局都依赖这套常量。
// 关键尺寸严格对齐 ESXi vswitch-diagram CSS:
//   STRIP_W = 22 ( .esx-networking-viz-center-portgroup-container width )
//   LINE_ZONE_W = 21 ( .esx-networking-viz-center-port-container background-size 21px )
//   PORT_INSET_IN_STRIP = 2 ( .esx-networking-viz-center-portgroup-container padding-left:2 )
const COL_W_PG = 268
const COL_W_CH = 200 // ESXi 的 chassis 宽得多,内部塞两根端口条 + 中央 BUS 线
const COL_W_UP = 248
const LINE_ZONE_W = 21 // ESXi 的端口横线就是 21px,刚好走过 chassis 与 PG/UP 卡之间的间隙
const STRIP_W = 22 // 端口条
const PORT_INSET_IN_STRIP = 2 // 端口图标距端口条外边沿的内边距
const ICON_SIZE = 12
const ICON_HALF = ICON_SIZE / 2
const PG_HEADER = 30
const PG_VM_ROW = 36
const PG_FOOTER = 6
const PG_EMPTY = 50
const UP_CARD_H = 62
const COL_GAP = 10
const SW_GAP = 28
const VM_ROW_CENTER = 12

// ESXi UI 实际配色
const CHASSIS_BG = '#ccccd0'
const CHASSIS_BORDER = '#9a9a9c'
const STRIP_BG = '#bababa'
const STRIP_BORDER = '#8d8d8f'
const LINE_COLOR = '#000'

function pgHeight(vms: number) {
  if (vms === 0) return PG_EMPTY
  return PG_HEADER + PG_VM_ROW * vms + PG_FOOTER
}

function fmtBitrate(mbps?: number): string {
  if (mbps == null || mbps < 0) return '—'
  if (mbps >= 1000) return `${(mbps / 1000).toFixed(mbps % 1000 === 0 ? 0 : 1)} Gbps`
  return `${mbps} Mbps`
}

// 直接用 ESXi Host Client 的 portConnected16.png / portDisconnected16.png。
// 端口图标贴在 strip 内偏外侧的位置 (ESXi padding-left:2 → 端口距 strip 外边沿 2px 内)。
function PortIcon({
  active,
  side,
  y,
}: {
  active: boolean
  side: 'left' | 'right'
  y: number
}) {
  const style: React.CSSProperties =
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

function PortgroupCard({ pg, vms }: { pg: string; vms: VMInfo[] }) {
  return (
    <div
      className="rounded-md border border-border bg-background shadow-sm"
      style={{ width: COL_W_PG }}
    >
      <div className="flex items-center gap-1.5 border-b border-border bg-amber-500/10 px-2.5 py-1">
        <Network className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
        <span
          className="truncate text-[12px] font-semibold text-foreground"
          title={pg}
        >
          {pg}
        </span>
        <span className="ml-auto text-[10px] text-muted-foreground">
          {vms.length} 虚机
        </span>
      </div>
      {vms.length === 0 ? (
        <div className="px-2.5 py-1.5 text-[10.5px] text-muted-foreground">
          无虚机
        </div>
      ) : (
        <div className="pb-1.5 pt-0">
          {vms.map((v) => (
            <div
              key={v.vmName + v.mac}
              className="flex flex-col justify-center px-2.5"
              style={{ height: PG_VM_ROW }}
              title={`MAC ${v.mac || '—'}${v.teamUplink ? ` · ${v.teamUplink}` : ''}`}
            >
              <div className="flex items-center gap-1 leading-tight">
                <Box className="h-3 w-3 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate text-[11.5px] text-foreground">
                  {v.vmName}
                </span>
                {v.teamUplink ? (
                  <span className="shrink-0 font-mono text-[9.5px] text-muted-foreground">
                    {v.teamUplink}
                  </span>
                ) : null}
              </div>
              <div className="truncate pl-4 font-mono text-[9.5px] leading-tight text-muted-foreground">
                {v.mac || '—'}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function UplinkCard({ up, nic }: { up: string; nic?: NIC }) {
  const linkUp = nic ? nic.link_status === 'Up' : false
  return (
    <div
      className="rounded-md border border-border bg-background shadow-sm"
      style={{ width: COL_W_UP, height: UP_CARD_H }}
    >
      <div className="flex items-center gap-1.5 border-b border-border bg-sky-500/10 px-2.5 py-1">
        <Network className="h-3.5 w-3.5 text-sky-600 dark:text-sky-400" />
        <span className="text-[12px] font-semibold text-foreground">
          物理适配器
        </span>
        <span className="ml-auto text-[10px] text-muted-foreground">
          {linkUp ? fmtBitrate(nic?.speed_mbps) : '未上线'}
        </span>
      </div>
      <div className="px-2.5 py-1">
        <div className="flex items-center gap-1 leading-tight">
          <Server className="h-3 w-3 shrink-0 text-muted-foreground" />
          <span
            className="truncate font-mono text-[11.5px] font-medium text-foreground"
            title={up}
          >
            {up}
          </span>
        </div>
        {nic?.mac ? (
          <div className="truncate pl-4 font-mono text-[9.5px] leading-tight text-muted-foreground">
            {nic.mac}
          </div>
        ) : null}
      </div>
    </div>
  )
}

interface SideAnchor {
  y: number
  active: boolean
}

// StripBlock:对齐 ESXi 的 portgroup-block —— 每个分组 (一个 PG 或一个 uplink) 一根独立的 strip 小条,
// 而不是整根贯穿。同一个 PG 内的多 anchor 会拼成连续条,单 anchor 会变成 ESXi 那种"突出小框"。
interface StripBlock {
  topY: number
  bottomY: number
  anchors: SideAnchor[]
}

const STRIP_PAD = 10 // 每个 block 在首/末 anchor 之外多留多少 px

// Chassis:对齐 ESXi 的 .esx-networking-viz-center —— 浅灰底盒(#ccccd0) + 1px 边框 + 3px 圆角。
// 内部左/右贴若干 #bababa 端口条 (对齐 .esx-networking-viz-center-portgroup-container / -vmnic-box),
// 每个 strip block 三面 border + 圆角朝向 chassis 内部,左 strip 无左 border,右 strip 无右 border。
// 端口图标贴在 strip 内偏外侧 (padding-left:2 / padding-right:2)。
// 中央贯穿一根 1px 黑色 BUS 线 (对齐 ESXi blackPixelVertical.png 在 portgroup-block 右沿画的竖线),
// BUS 线 y 范围 = 全部 anchor 的 min y → max y。
// 名字徽章悬在顶部正中。
function Chassis({
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
  const allY = [...leftBlocks, ...rightBlocks].flatMap((b) =>
    b.anchors.map((a) => a.y),
  )
  const busYStart = allY.length > 0 ? Math.min(...allY) : 0
  const busYEnd = allY.length > 0 ? Math.max(...allY) : 0
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
      {/* 中央 BUS 垂直线:1px 黑,只在第一个 anchor 到最后一个 anchor 之间 */}
      {allY.length > 1 ? (
        <div
          className="absolute"
          style={{
            left: busX,
            top: busYStart,
            width: 1,
            height: busYEnd - busYStart,
            backgroundColor: LINE_COLOR,
          }}
        />
      ) : null}

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

      {/* 名字徽章 */}
      <div
        className="absolute left-1/2 z-10 -translate-x-1/2 whitespace-nowrap rounded border bg-background px-1.5 py-[1px] font-mono text-[10px] font-medium text-foreground shadow-sm"
        style={{
          top: 6,
          borderColor: CHASSIS_BORDER,
        }}
        title={name}
      >
        {name}
      </div>
    </div>
  )
}

// VSwitchBlock:每个 vSwitch 一行 [ PG列 | 左连线槽 | chassis | 右连线槽 | UP列 ]。
// 端口锚点 (左 VM 行中心 / 右 uplink 卡中心) 决定连线 y 坐标,SVG 画水平线收到 chassis 边缘。
function VSwitchBlock({
  sw,
  vmsByPG,
  nicByName,
}: {
  sw: VSwitchInfo
  vmsByPG: Map<string, VMInfo[]>
  nicByName: Map<string, NIC>
}) {
  const pgs = sw.portgroups ?? []
  const ups = sw.uplinks ?? []

  const pgInfos = pgs.map((pg) => {
    const vms = vmsByPG.get(sw.name + '||' + pg) ?? []
    return { pg, vms, height: pgHeight(vms.length) }
  })

  const pgColH =
    pgInfos.reduce((s, p) => s + p.height, 0) +
    Math.max(0, pgInfos.length - 1) * COL_GAP
  const upColH = ups.length * UP_CARD_H + Math.max(0, ups.length - 1) * COL_GAP
  const innerH = Math.max(pgColH, upColH, 80)

  // 左侧:每个 PG 一个 strip block,block 高度覆盖该 PG 所有 anchor 范围 + STRIP_PAD 缓冲。
  // 同 PG 多 VM 视觉连续条;不同 PG 之间分离,BUS 竖线在分离处穿过空白。
  const leftBlocks: StripBlock[] = []
  const leftAnchors: SideAnchor[] = []
  {
    let pgY = (innerH - pgColH) / 2
    for (const info of pgInfos) {
      if (info.vms.length === 0) {
        pgY += info.height + COL_GAP
        continue
      }
      const blockAnchors: SideAnchor[] = info.vms.map((v, vi) => ({
        y: pgY + PG_HEADER + PG_VM_ROW * vi + VM_ROW_CENTER,
        active: !!v.teamUplink,
      }))
      const firstY = blockAnchors[0].y
      const lastY = blockAnchors[blockAnchors.length - 1].y
      leftBlocks.push({
        topY: firstY - STRIP_PAD,
        bottomY: lastY + STRIP_PAD,
        anchors: blockAnchors,
      })
      leftAnchors.push(...blockAnchors)
      pgY += info.height + COL_GAP
    }
  }

  // 右侧:每个 uplink 单独一个 strip block (ESXi 那种"突出小框")。
  const rightBlocks: StripBlock[] = []
  const rightAnchors: SideAnchor[] = []
  {
    let upY = (innerH - upColH) / 2
    for (const up of ups) {
      const nic = nicByName.get(up)
      const anchor: SideAnchor = {
        y: upY + UP_CARD_H / 2,
        active: nic?.link_status === 'Up',
      }
      rightBlocks.push({
        topY: anchor.y - STRIP_PAD,
        bottomY: anchor.y + STRIP_PAD,
        anchors: [anchor],
      })
      rightAnchors.push(anchor)
      upY += UP_CARD_H + COL_GAP
    }
  }

  const pgTopOffset = (innerH - pgColH) / 2
  const upTopOffset = (innerH - upColH) / 2

  return (
    <div className="flex items-start" style={{ height: innerH }}>
      <div
        className="flex flex-col"
        style={{
          width: COL_W_PG,
          marginTop: pgTopOffset,
          rowGap: COL_GAP,
        }}
      >
        {pgInfos.map((info) => (
          <PortgroupCard key={info.pg} pg={info.pg} vms={info.vms} />
        ))}
      </div>

      {/* 左连线槽:从 PG 卡右边沿 (x=0) 一直连到 chassis 左边沿 (x=LINE_ZONE_W) */}
      <div
        className="relative"
        style={{ width: LINE_ZONE_W, height: innerH }}
      >
        <svg
          width={LINE_ZONE_W}
          height={innerH}
          className="absolute inset-0"
          aria-hidden
        >
          {leftAnchors.map((a, i) => (
            <line
              key={i}
              x1={0}
              y1={a.y}
              x2={LINE_ZONE_W}
              y2={a.y}
              stroke={LINE_COLOR}
              strokeWidth={1}
              shapeRendering="crispEdges"
            />
          ))}
        </svg>
      </div>

      <Chassis
        name={sw.name}
        height={innerH}
        leftBlocks={leftBlocks}
        rightBlocks={rightBlocks}
      />

      {/* 右连线槽:从 chassis 右边沿 (x=0) 一直连到 UP 卡左边沿 (x=LINE_ZONE_W) */}
      <div
        className="relative"
        style={{ width: LINE_ZONE_W, height: innerH }}
      >
        <svg
          width={LINE_ZONE_W}
          height={innerH}
          className="absolute inset-0"
          aria-hidden
        >
          {rightAnchors.map((a, i) => (
            <line
              key={i}
              x1={0}
              y1={a.y}
              x2={LINE_ZONE_W}
              y2={a.y}
              stroke={LINE_COLOR}
              strokeWidth={1}
              shapeRendering="crispEdges"
            />
          ))}
        </svg>
      </div>

      <div
        className="flex flex-col"
        style={{
          width: COL_W_UP,
          marginTop: upTopOffset,
          rowGap: COL_GAP,
        }}
      >
        {ups.map((up) => (
          <UplinkCard key={up} up={up} nic={nicByName.get(up)} />
        ))}
      </div>
    </div>
  )
}

export function NetTopologyFlow({
  topo,
  nics,
}: {
  topo: NetTopology
  nics?: NIC[]
}) {
  const [expanded, setExpanded] = useState(false)

  const vmsByPG = useMemo(() => {
    const m = new Map<string, VMInfo[]>()
    for (const link of topo.vm_nics ?? []) {
      const key = link.vswitch + '||' + link.portgroup
      const arr = m.get(key) ?? []
      if (!arr.find((v) => v.vmName === link.vm_name)) {
        arr.push({
          vmName: link.vm_name,
          teamUplink: link.team_uplink,
          mac: link.mac,
        })
      }
      m.set(key, arr)
    }
    return m
  }, [topo])

  const nicByName = useMemo(
    () => new Map<string, NIC>((nics ?? []).map((n) => [n.name, n])),
    [nics],
  )

  const vswitches = topo.vswitches ?? []
  const pNICCount = nics?.length ?? new Set(vswitches.flatMap((s) => s.uplinks ?? [])).size

  return (
    <Card className="px-3 py-3">
      <button
        type="button"
        className="flex w-full items-center gap-1.5 text-left"
        onClick={() => setExpanded((v) => !v)}
      >
        <Waypoints className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-[12.5px] font-medium text-foreground">网络拓扑</span>
        <span className="ml-auto rounded-md border border-border bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
          {pNICCount} 物理 · {vswitches.length} vSwitch · {topo.vm_nics?.length ?? 0} vNIC
        </span>
        {expanded ? (
          <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        )}
      </button>
      {expanded ? (
        <div className="mt-2 w-full overflow-x-auto rounded-md border border-border bg-muted/10 p-3">
          <div
            className="flex flex-col"
            style={{ rowGap: SW_GAP, minWidth: COL_W_PG + LINE_ZONE_W * 2 + COL_W_CH + COL_W_UP }}
          >
            {vswitches.map((sw) => (
              <VSwitchBlock
                key={sw.name}
                sw={sw}
                vmsByPG={vmsByPG}
                nicByName={nicByName}
              />
            ))}
          </div>
        </div>
      ) : null}
    </Card>
  )
}
