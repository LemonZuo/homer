import {
  COL_GAP,
  COL_W_PG,
  COL_W_UP,
  LINE_COLOR,
  LINE_ZONE_W,
  PG_HEADER,
  PG_VM_ROW,
  PG_VMK_ROW,
  PG_VMK_SECTION,
  PORT_LINE_W,
  STRIP_PAD,
  UP_CARD_H,
  VMK_ROW_CENTER,
  VM_ROW_CENTER,
} from './constants'
import { Chassis } from './Chassis'
import { PortgroupCard } from './PortgroupCard'
import type { NIC, SideAnchor, StripBlock, VMInfo, VMKInfo, VSwitchInfo } from './types'
import { UplinkCard } from './UplinkCard'
import { pgHeight } from './utils'

// VSwitchBlock:每个 vSwitch 一行 [ PG列 | 左连线槽 | chassis | 右连线槽 | UP列 ]。
// 端口锚点 (左 VM 行中心 / 右 uplink 卡中心) 决定连线 y 坐标,SVG 画水平线收到 chassis 边缘。
export function VSwitchBlock({
  sw,
  vmsByPG,
  vmksByPG,
  nicByName,
}: {
  sw: VSwitchInfo
  vmsByPG: Map<string, VMInfo[]>
  vmksByPG: Map<string, VMKInfo[]>
  nicByName: Map<string, NIC>
}) {
  const pgs = sw.portgroups ?? []
  const ups = sw.uplinks ?? []

  const pgInfos = pgs.map((pg) => {
    const vms = vmsByPG.get(sw.name + '||' + pg) ?? []
    const vmks = vmksByPG.get(sw.name + '||' + pg) ?? []
    return { pg, vms, vmks, height: pgHeight(vms.length, vmks.length) }
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
      if (info.vms.length === 0 && info.vmks.length === 0) {
        pgY += info.height + COL_GAP
        continue
      }
      const blockAnchors: SideAnchor[] = []
      if (info.vmks.length > 0) {
        const vmkStartY = pgY + PG_HEADER + PG_VMK_SECTION
        for (let i = 0; i < info.vmks.length; i += 1) {
          const vmk = info.vmks[i]
          blockAnchors.push({
            y: vmkStartY + PG_VMK_ROW * i + VMK_ROW_CENTER,
            active: vmk.enabled === true,
          })
        }
      }
      const vmStartY =
        pgY +
        PG_HEADER +
        (info.vmks.length > 0 ? PG_VMK_SECTION + PG_VMK_ROW * info.vmks.length : 0)
      for (let i = 0; i < info.vms.length; i += 1) {
        const v = info.vms[i]
        blockAnchors.push({
          y: vmStartY + PG_VM_ROW * i + VM_ROW_CENTER,
          active: !!v.teamUplink,
        })
      }
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
    <div className="flex items-start">
      <div
        className="flex flex-col"
        style={{
          width: COL_W_PG,
          marginTop: pgTopOffset,
          rowGap: COL_GAP,
        }}
      >
        {pgInfos.map((info) => (
          <PortgroupCard key={info.pg} pg={info.pg} vms={info.vms} vmks={info.vmks} />
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
              strokeWidth={PORT_LINE_W}
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
              strokeWidth={PORT_LINE_W}
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
