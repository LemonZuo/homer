# `upsc 1050201` 字段含义

> 这里记录 NUT `upsc` 命令查 CyberPower UT1050EGC 的输出字段含义，方便排查问题时对照。
> 设备 NUT 名: `1050201`，驱动: `usbhid-ups` (USB HID PDC 标准)。

## 示例输出 (满电时)

```
battery.charge: 100
battery.charge.low: 20
battery.charge.warning: 35
battery.mfr.date: CPS
battery.runtime: 2744
battery.runtime.low: 300
battery.type: PbAc
battery.voltage: 28.2
battery.voltage.nominal: 24
device.mfr: CPS
device.model: CP1500 AVR LCD
device.serial: 000000000000
device.type: ups
driver.name: usbhid-ups
driver.parameter.pollfreq: 30
driver.parameter.pollinterval: 2
driver.parameter.port: auto
driver.parameter.synchronous: auto
driver.version: 2.7.4
driver.version.data: CyberPower HID 0.4
driver.version.internal: 0.41
input.transfer.high: 295
input.transfer.low: 145
input.voltage: 224.0
input.voltage.nominal: 230
output.voltage: 260.0
ups.beeper.status: enabled
ups.delay.shutdown: 20
ups.delay.start: 30
ups.load: 18
ups.mfr: CPS
ups.model: CP1500 AVR LCD
ups.productid: 0501
ups.realpower.nominal: 900
ups.status: OL
ups.test.result: No test initiated
ups.timer.shutdown: -60
ups.timer.start: -60
ups.vendorid: 0764
```

## 字段分类详解

### battery.\* — 电池

| 字段 | 含义 | 备注 |
|---|---|---|
| `battery.charge` | 当前电量百分比 (0-100) | UPS 电量主指标 |
| `battery.charge.low` | 低电量告警阈值 (%) | UPS 固件内部用 |
| `battery.charge.warning` | 警告阈值 (%) | 比 low 早一点提醒 |
| `battery.mfr.date` | 电池生产日期 | UT1050EGC 不报，显示为 `CPS` |
| `battery.runtime` | 剩余续航**秒数** | 满电 18% 负载下 2744s ≈ 45.7 分钟 |
| `battery.runtime.low` | 低续航阈值 (s) | 剩余 < 300s (5 min) 触发 LB 状态 |
| `battery.type` | 电池类型 | `PbAc` = 铅酸 (Pb=铅, Ac=酸) |
| `battery.voltage` | 当前电池电压 (V) | 满电浮充 28.2V ≈ 单块 14.1V |
| `battery.voltage.nominal` | 标称电压 (V) | **24V** → 2 块 12V 串联的关键证据 |

### device.\* / ups.\* — 设备身份和状态

| 字段 | 含义 | 备注 |
|---|---|---|
| `device.mfr` / `ups.mfr` | 厂家 | `CPS` = CyberPower Systems |
| `device.model` / `ups.model` | 设备型号字符串 | 固件里写死的 `CP1500 AVR LCD`，**不等同实际型号**（实际是 UT1050EGC） |
| `device.serial` | 序列号 | 这型号未烧录，全 0 正常 |
| `device.type` | 设备类型 | `ups` |
| `ups.productid` | USB Product ID (hex) | `0501` |
| `ups.vendorid` | USB Vendor ID (hex) | `0764` = CyberPower |
| `ups.beeper.status` | 蜂鸣器状态 | `enabled` = 停电时会叫 |
| `ups.delay.shutdown` | 收到关机命令后延迟切断输出 (s) | 默认 20 |
| `ups.delay.start` | 市电恢复后延迟重新输出 (s) | 默认 30 |
| `ups.load` | 当前负载百分比 (%) | 18% × 630W ≈ 113W (实测) |
| `ups.realpower.nominal` | 标称有功功率 (W) | 显示 `900`，但 UT1050EGC 实际是 **630W**，固件报的是 CP1500 通用值 |
| `ups.status` | **状态码** | 见下表 |
| `ups.timer.shutdown` | 关机倒计时 (s) | `-60` = 未在倒计时 |
| `ups.timer.start` | 启动倒计时 (s) | `-60` = 未在倒计时 |
| `ups.test.result` | 上次电池自检结果 | `No test initiated` = 从未做过 |

#### `ups.status` 状态码

| 码 | 含义 |
|---|---|
| `OL` | OnLine，市电正常供电 |
| `OB` | On Battery，电池供电中 |
| `LB` | Low Battery，电量低 |
| `FSD` | Forced Shutdown，强制关机倒计时 |
| `CHRG` | Charging，充电中 |
| `DISCHRG` | Discharging，放电中（通常和 OB 同时出现） |

多个状态可同时存在，空格分隔，例如 `OB DISCHRG`。

### input.\* / output.\* — 电压输入输出

| 字段 | 含义 | 备注 |
|---|---|---|
| `input.voltage` | 当前市电电压 (V) | 国标 220V±10%，224V 正常 |
| `input.voltage.nominal` | 标称输入电压 (V) | 固件默认 230 |
| `input.transfer.high` | 高于此电压切换到电池 (V) | 295 |
| `input.transfer.low` | 低于此电压切换到电池 (V) | 145 |
| `output.voltage` | 输出电压 (V) | **市电模式下不准**。UT1050EGC 是 line-interactive，市电直通，固件这里报的是内部推算值。OB 模式下才是真实逆变输出 |

### driver.\* — NUT 驱动信息

| 字段 | 含义 | 备注 |
|---|---|---|
| `driver.name` | NUT 驱动名 | `usbhid-ups` |
| `driver.version` | NUT 主版本 | |
| `driver.version.data` | 数据文件版本 | `CyberPower HID 0.4` |
| `driver.version.internal` | 内部协议版本 | |
| `driver.parameter.pollfreq` | 完整查询周期 (s) | 默认 30 |
| `driver.parameter.pollinterval` | 状态轮询周期 (s) | 驱动内部默认 2 |
| `driver.parameter.port` | USB 端口 | `auto` = 自动识别 |
| `driver.parameter.synchronous` | 同步模式 | `auto` |

## 满电 vs 充电中的差异

| 字段 | 满电 (100%) | 充电中 (85-91%) |
|---|---|---|
| `ups.status` | `OL` | `OL CHRG` |
| `battery.voltage` | 28.2V (浮充) | 27.6-28.0V (吸收充电) |
| `battery.charge` | 100 | 85-91 |
| `battery.runtime` | 满负载推算值 | 满负载推算值（按当前电量比例缩放） |

## 排查命令速查

```bash
# 全量字段
upsc 1050201

# 单个字段（脚本里用）
upsc 1050201 battery.charge       # 电量
upsc 1050201 ups.status           # 状态
upsc 1050201 battery.runtime      # 剩余秒数
upsc 1050201 ups.load             # 负载 %

# 静默错误（脚本里推荐）
timeout 2 upsc 1050201 ups.status 2>/dev/null
```