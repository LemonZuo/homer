import { useMemo, useState } from 'react'
import {
  ReactFlow,
  Handle,
  Position,
  type Edge,
  type Node,
  type NodeTypes,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Box, ChevronDown, ChevronUp, Network, Server, Waypoints } from 'lucide-react'
import { Card } from '../ui/card'

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

// 像素几何常量,所有 handle / 节点高度都按这套尺寸算
const COL_X_PG = 0
const COL_W_PG = 268
const COL_X_CH = 320
const COL_W_CH = 52
const COL_X_UP = 420
const COL_W_UP = 248
const PG_HEADER = 30
const PG_VM_ROW = 36 // 名字行 + MAC 行
const PG_FOOTER = 6
const PG_EMPTY = 50
const UP_CARD_H = 62
const COL_GAP = 10
const SW_GAP = 32
const SW_HEAD = 32
const VM_ROW_CENTER = 12 // handle 落在名字行中央

function pgHeight(vms: number) {
  if (vms === 0) return PG_EMPTY
  return PG_HEADER + PG_VM_ROW * vms + PG_FOOTER
}

function fmtBitrate(mbps?: number): string {
  if (mbps == null || mbps < 0) return '—'
  if (mbps >= 1000) return `${(mbps / 1000).toFixed(mbps % 1000 === 0 ? 0 : 1)} Gbps`
  return `${mbps} Mbps`
}

// handle 只做连线锚点,视觉交给叠在上面的 RJ45 图标
function handleStyle(y: number): React.CSSProperties {
  return {
    top: y,
    width: 2,
    height: 2,
    minWidth: 2,
    minHeight: 2,
    background: 'transparent',
    border: 'none',
    opacity: 0,
  }
}

// RJ45 风格网口:顶部卡扣 + 矩形主体 + 排针槽。active=绿色填实,否则空心。
function PortIcon({ active }: { active: boolean }) {
  const fill = active ? '#22c55e' : '#ffffff'
  const stroke = active ? '#166534' : '#94a3b8'
  const slot = active ? '#14532d' : '#cbd5e1'
  return (
    <svg width="14" height="16" viewBox="0 0 14 16" className="drop-shadow-sm" aria-hidden>
      <rect x="4" y="0.5" width="6" height="3" rx="0.6" fill={fill} stroke={stroke} strokeWidth="0.7" />
      <rect x="0.6" y="3" width="12.8" height="12.5" rx="1.2" fill={fill} stroke={stroke} strokeWidth="0.7" />
      <rect x="2.3" y="4.8" width="9.4" height="1.8" rx="0.4" fill={slot} opacity={active ? 0.55 : 1} />
    </svg>
  )
}

// 把 RJ45 钉在节点左/右边缘的 (edge, y) 处,中心正好压住 handle 锚点
function PortJack({ active, y, side }: { active: boolean; y: number; side: 'left' | 'right' }) {
  return (
    <div
      className="pointer-events-none absolute z-10"
      style={{
        top: y,
        ...(side === 'left' ? { left: 0 } : { right: 0 }),
        transform: `translate(${side === 'left' ? '-50%' : '50%'}, -50%)`,
      }}
    >
      <PortIcon active={active} />
    </div>
  )
}

function PortgroupNode({ data }: { data: { pg: string; vms: VMInfo[] } }) {
  const vms = data.vms
  return (
    <div
      className="rounded-md border border-border bg-background shadow-sm"
      style={{ width: COL_W_PG }}
    >
      <div className="flex items-center gap-1.5 border-b border-border bg-amber-500/10 px-2.5 py-1">
        <Network className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
        <span className="truncate text-[12px] font-semibold text-foreground" title={data.pg}>
          {data.pg}
        </span>
        <span className="ml-auto text-[10px] text-muted-foreground">{vms.length} 虚机</span>
      </div>
      {vms.length === 0 ? (
        <div className="px-2.5 py-1.5 text-[10.5px] text-muted-foreground">无虚机</div>
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
      {vms.map((v, i) => {
        const y = PG_HEADER + PG_VM_ROW * i + VM_ROW_CENTER
        return (
          <span key={`vm-${i}`}>
            <Handle type="source" position={Position.Right} id={`vm-${i}`} style={handleStyle(y)} />
            <PortJack active={!!v.teamUplink} y={y} side="right" />
          </span>
        )
      })}
    </div>
  )
}

interface ChassisHandleSpec {
  id: string
  y: number
  side: 'left' | 'right'
  active: boolean
}
function ChassisNode({
  data,
}: {
  data: { name: string; height: number; handles: ChassisHandleSpec[] }
}) {
  return (
    <div className="relative" style={{ width: COL_W_CH, height: data.height }}>
      {/* 机箱本体:中灰金属面板,介于浅卡片和"设备"之间 */}
      <div
        className="absolute inset-x-0 rounded-md border border-zinc-400/70 overflow-hidden shadow-[0_2px_6px_rgba(0,0,0,0.14),inset_0_1px_0_rgba(255,255,255,0.65)]"
        style={{
          top: 22,
          bottom: 0,
          background:
            'linear-gradient(180deg,#e4e4e7 0%,#fafafa 12%,#c7c7cc 50%,#e4e4e7 88%,#a1a1aa 100%)',
        }}
      >
        {/* 金属拉丝纹理:深色细线落在浅面板上 */}
        <div
          className="absolute inset-0 pointer-events-none opacity-55"
          style={{
            backgroundImage:
              'repeating-linear-gradient(90deg, rgba(0,0,0,0.05) 0, rgba(0,0,0,0.05) 0.5px, transparent 0.5px, transparent 2px)',
          }}
        />
        {/* 四角螺丝:中灰金属点 */}
        <span className="absolute left-[3px] top-[3px] h-[3px] w-[3px] rounded-full bg-zinc-500/75 shadow-[inset_0_0_0_0.5px_rgba(0,0,0,0.4)]" />
        <span className="absolute right-[3px] top-[3px] h-[3px] w-[3px] rounded-full bg-zinc-500/75 shadow-[inset_0_0_0_0.5px_rgba(0,0,0,0.4)]" />
        <span className="absolute left-[3px] bottom-[3px] h-[3px] w-[3px] rounded-full bg-zinc-500/75 shadow-[inset_0_0_0_0.5px_rgba(0,0,0,0.4)]" />
        <span className="absolute right-[3px] bottom-[3px] h-[3px] w-[3px] rounded-full bg-zinc-500/75 shadow-[inset_0_0_0_0.5px_rgba(0,0,0,0.4)]" />
      </div>
      {/* 顶部铭牌:深色 chip 贴在浅金属上,对比清晰 */}
      <div
        className="absolute left-1/2 z-10 -translate-x-1/2 rounded-md border border-zinc-700/50 bg-zinc-800 px-2 py-[2px] font-mono text-[10px] font-medium tracking-wide text-zinc-50 shadow-[0_2px_4px_rgba(0,0,0,0.25)] whitespace-nowrap"
        style={{ top: 6 }}
      >
        {data.name}
      </div>
      {data.handles.map((h) => (
        <span key={h.id}>
          <Handle
            type={h.side === 'left' ? 'target' : 'source'}
            position={h.side === 'left' ? Position.Left : Position.Right}
            id={h.id}
            style={handleStyle(h.y)}
          />
          <PortJack active={h.active} y={h.y} side={h.side} />
        </span>
      ))}
    </div>
  )
}

function UplinkNode({ data }: { data: { up: string; nic?: NIC } }) {
  const linkUp = data.nic ? data.nic.link_status === 'Up' : false
  return (
    <div
      className="relative rounded-md border border-border bg-background shadow-sm"
      style={{ width: COL_W_UP }}
    >
      <div className="flex items-center gap-1.5 border-b border-border bg-sky-500/10 px-2.5 py-1">
        <Network className="h-3.5 w-3.5 text-sky-600 dark:text-sky-400" />
        <span className="text-[12px] font-semibold text-foreground">物理适配器</span>
        <span className="ml-auto text-[10px] text-muted-foreground">
          {linkUp ? fmtBitrate(data.nic?.speed_mbps) : '未上线'}
        </span>
      </div>
      <div className="px-2.5 py-1">
        <div className="flex items-center gap-1 leading-tight">
          <Server className="h-3 w-3 shrink-0 text-muted-foreground" />
          <span
            className="truncate font-mono text-[11.5px] font-medium text-foreground"
            title={data.up}
          >
            {data.up}
          </span>
        </div>
        {data.nic?.mac ? (
          <div className="truncate pl-4 font-mono text-[9.5px] leading-tight text-muted-foreground">
            {data.nic.mac}
          </div>
        ) : null}
      </div>
      <Handle type="target" position={Position.Left} id="in" style={handleStyle(30)} />
      <PortJack active={linkUp} y={30} side="left" />
    </div>
  )
}

const nodeTypes: NodeTypes = {
  portgroup: PortgroupNode,
  chassis: ChassisNode,
  uplink: UplinkNode,
}

export function NetTopologyFlow({ topo, nics }: { topo: NetTopology; nics?: NIC[] }) {
  const { nodes, edges, totalHeight } = useMemo(() => {
    const ns: Node[] = []
    const es: Edge[] = []
    const vswitches = topo.vswitches ?? []
    const vmNics = topo.vm_nics ?? []
    const nicByName = new Map<string, NIC>((nics ?? []).map((n) => [n.name, n]))
    const vmsByPG = new Map<string, VMInfo[]>()
    for (const link of vmNics) {
      const key = link.vswitch + '||' + link.portgroup
      const arr = vmsByPG.get(key) ?? []
      if (!arr.find((v) => v.vmName === link.vm_name)) {
        arr.push({ vmName: link.vm_name, teamUplink: link.team_uplink, mac: link.mac })
      }
      vmsByPG.set(key, arr)
    }

    let yCursor = 0
    for (const sw of vswitches) {
      const pgs = sw.portgroups ?? []
      const ups = sw.uplinks ?? []
      const pgInfos = pgs.map((pg) => {
        const vms = vmsByPG.get(sw.name + '||' + pg) ?? []
        return { pg, vms, height: pgHeight(vms.length) }
      })
      const pgColH =
        pgInfos.reduce((s, p) => s + p.height, 0) + Math.max(0, pgInfos.length - 1) * COL_GAP
      const upColH = ups.length * UP_CARD_H + Math.max(0, ups.length - 1) * COL_GAP
      const swInnerH = Math.max(pgColH, upColH, 80)
      const swH = SW_HEAD + swInnerH

      // PG col vertical centering inside inner area
      const pgStartY = yCursor + SW_HEAD + (swInnerH - pgColH) / 2
      const upStartY = yCursor + SW_HEAD + (swInnerH - upColH) / 2

      let pgY = pgStartY
      const chHandles: ChassisHandleSpec[] = []
      for (const info of pgInfos) {
        ns.push({
          id: `pg:${sw.name}:${info.pg}`,
          type: 'portgroup',
          position: { x: COL_X_PG, y: pgY },
          data: { pg: info.pg, vms: info.vms },
          draggable: false,
          selectable: false,
        })
        info.vms.forEach((v, i) => {
          const absY = pgY + PG_HEADER + PG_VM_ROW * i + VM_ROW_CENTER
          chHandles.push({
            id: `lpg-${info.pg}-vm-${i}`,
            y: absY - yCursor,
            side: 'left',
            active: !!v.teamUplink,
          })
          es.push({
            id: `e:${sw.name}:${info.pg}:vm-${i}`,
            source: `pg:${sw.name}:${info.pg}`,
            sourceHandle: `vm-${i}`,
            target: `ch:${sw.name}`,
            targetHandle: `lpg-${info.pg}-vm-${i}`,
            style: { stroke: v.teamUplink ? '#9ca3af' : '#d4d4d8', strokeWidth: 1.3 },
          })
        })
        pgY += info.height + COL_GAP
      }

      let upY = upStartY
      for (const up of ups) {
        const nic = nicByName.get(up)
        const isUp = nic?.link_status === 'Up'
        ns.push({
          id: `up:${sw.name}:${up}`,
          type: 'uplink',
          position: { x: COL_X_UP, y: upY },
          data: { up, nic },
          draggable: false,
          selectable: false,
        })
        chHandles.push({
          id: `rup-${up}`,
          y: upY - yCursor + 30,
          side: 'right',
          active: isUp,
        })
        es.push({
          id: `e:${sw.name}:up-${up}`,
          source: `ch:${sw.name}`,
          sourceHandle: `rup-${up}`,
          target: `up:${sw.name}:${up}`,
          targetHandle: 'in',
          style: { stroke: isUp ? '#9ca3af' : '#d4d4d8', strokeWidth: 1.3 },
        })
        upY += UP_CARD_H + COL_GAP
      }

      ns.push({
        id: `ch:${sw.name}`,
        type: 'chassis',
        position: { x: COL_X_CH, y: yCursor },
        data: { name: sw.name, height: swH, handles: chHandles },
        draggable: false,
        selectable: false,
      })

      yCursor += swH + SW_GAP
    }
    return { nodes: ns, edges: es, totalHeight: Math.max(yCursor, 200) }
  }, [topo, nics])

  const pNICCount = nics?.length ?? new Set(topo.vswitches?.flatMap((s) => s.uplinks ?? [])).size
  const [expanded, setExpanded] = useState(false)

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
          {pNICCount} 物理 · {topo.vswitches?.length ?? 0} vSwitch · {topo.vm_nics?.length ?? 0} vNIC
        </span>
        {expanded ? (
          <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        )}
      </button>
      {expanded ? (
        <div
          className="mt-2 w-full overflow-hidden rounded-md border border-border bg-muted/10"
          style={{ height: totalHeight }}
        >
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            fitView
            fitViewOptions={{ padding: 0.04 }}
            nodesDraggable={false}
            nodesConnectable={false}
            elementsSelectable={false}
            edgesFocusable={false}
            nodesFocusable={false}
            panOnDrag={false}
            panOnScroll={false}
            zoomOnScroll={false}
            zoomOnPinch={false}
            zoomOnDoubleClick={false}
            preventScrolling={false}
            proOptions={{ hideAttribution: true }}
          />
        </div>
      ) : null}
    </Card>
  )
}
