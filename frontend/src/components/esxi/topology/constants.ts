// 列宽/行高:整套像素布局都依赖这套常量。
// 关键尺寸严格对齐 ESXi vswitch-diagram CSS:
//   STRIP_W = 22 ( .esx-networking-viz-center-portgroup-container width )
//   LINE_ZONE_W = 21 ( .esx-networking-viz-center-port-container background-size 21px )
//   PORT_INSET_IN_STRIP = 2 ( .esx-networking-viz-center-portgroup-container padding-left:2 )
export const COL_W_PG = 268
export const COL_W_CH = 130 // 对齐 ESXi vSwitch 实物宽度,内部塞两根 strip + 中央 BUS
export const COL_W_UP = 248
export const LINE_ZONE_W = 21 // ESXi 的端口横线就是 21px,走在 PG/UP 卡和 chassis 之间
export const STRIP_W = 22 // 端口条
export const PORT_INSET_IN_STRIP = 2 // 端口图标距端口条外边沿的内边距
// 线宽层级:BUS 最粗,PortGroup→BUS 横线次之,RJ45 (端口↔卡片) 最细
export const BUS_W = 3
export const PG_LINE_W = 2
export const PORT_LINE_W = 1
export const ICON_SIZE = 12
export const ICON_HALF = ICON_SIZE / 2
export const PG_HEADER = 30
export const PG_VM_ROW = 36
export const PG_VMK_SECTION = 22
export const PG_VMK_ROW = 30
export const PG_FOOTER = 6
export const PG_EMPTY = 50
export const UP_CARD_H = 62
export const COL_GAP = 10
export const SW_GAP = 28
export const VM_ROW_CENTER = 12
export const VMK_ROW_CENTER = 15

// ESXi UI 实际配色
export const CHASSIS_BG = '#ccccd0'
export const CHASSIS_BORDER = '#9a9a9c'
export const STRIP_BG = '#bababa'
export const STRIP_BORDER = '#8d8d8f'
export const LINE_COLOR = '#000'

export const STRIP_PAD = 10 // 每个 block 在首/末 anchor 之外多留多少 px
