# ESXi 数据采集详解

本文档逐项说明 homer 的 `internal/esximon` 模块如何通过 SSH 远程执行 ESXi 命令采集监控数据,包括命令、真实输出、解析方式、重试 / 超时策略,以及为什么这么设计。

实际机器:`ESXi 7.0.3 build-24784741`(Lenovo,Intel Xeon E-2224G,32 GiB)。文中所有输出都是从一台 ESXi 主机(`esxi_host.id=1`)抓的真值。

所有命令的远端入口是 `internal/esximon/client.go` 的 `CollectAll(client *ssh.Client) HostMetrics`;外层 `internal/esximon/sampler.go` 在关键指标缺失时复跑一次并合并。

> 公共约定:命令一律在远端用 `{ <cmd>; true; } 2>/dev/null` 包裹
>
> 1. 屏蔽 stderr,避免 VMware 工具偶发往 stderr 打 banner 干扰 stdout 解析;
> 2. `; true` 兜住合批里某条命令非零退出(典型如 NVMe `smart get`),防止整段 stdout 被 `sshx.Run` 当作失败丢弃。
>
> 前缀注入 `export PATH=/bin:/sbin:/usr/lib/vmware/bin:/usr/lib/vmware/vsan/bin:$PATH;`,扛 bastion + Linux jump host 时非交互 session 路径不全的情况。
>
> 多数命令通过 `runEsxiRetry(name, cmd, timeout, attempts, validator)` 执行:validator 返回 false 也触发重试(语义上空也算失败),重试间隔 = `attempt × 150ms`,默认 `attempts=2`,默认超时 8 秒。

---

## 1. PlatformInfo — 厂商 / 型号 / 序列号 / UUID / ESXi 版本

### 1.1 平台信息

```bash
esxcli hardware platform get
```

实测输出:

```
Platform Information
   UUID: 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX 0xXX
   Product Name: XXXXXXXXXX
   Vendor Name: LENOVO
   Serial Number: J30A****
   Enclosure Serial Number: J30A****
   BIOS Asset Tag:
   IPMI Supported: false
```

- 解析:`parseKV` 按 `Key: Value` 切,字段名小写并把空格替换为下划线(`product_name`/`vendor_name`/`serial_number`/`uuid`/`ipmi_supported`)。
- `ipmi_supported`:看 value 是否 `true`(`strings.EqualFold`)。
- Validator:`Vendor / Product / UUID` 任一非空。
- 超时:8s,2 次尝试。
- 失败兜底:对应字段保持空串。

### 1.2 ESXi 版本

```bash
vmware -v; esxcli system version get
```

实测输出:

```
VMware ESXi 7.0.3 build-24784741
   Product: VMware ESXi
   Version: 7.0.3
   Build: Releasebuild-24784741
   Update: 3
   Patch: 150
```

- 解析:正则 `ESXi\s+(\S+)\s+build-(\d+)`,提取 `ESXiVersion = "7.0.3"` 与 `ESXiBuild = 24784741`。
- Validator:正则能匹配。

---

## 2. CPUStatic — 静态 CPU 信息

CPU 信息分四步,后采的覆盖前采的。

### 2.1 `esxcli hardware cpu list`(基础静态)

```bash
esxcli hardware cpu list
```

实测输出(取首块):

```
CPU:0
   Id: 0
   Package Id: 0
   Family: 6
   Model: 158
   Type: 0
   Stepping: 10
   Brand: GenuineIntel
   Core Speed: 3503999654
   Bus Speed: 23999996
   L2 Cache Size: 262144
   L2 Cache Associativity: 4
   L2 Cache Line Size: 64
   L2 Cache CPU Count: 1
   L3 Cache Size: 8388608
   L3 Cache Associativity: 16
   L3 Cache Line Size: 64
   L3 Cache CPU Count: 4
```

- 提取:`brand`("GenuineIntel" 是 vendor,不是型号)、`family=6`、`model=158`、`stepping=10`、`core_speed=3503999654`(Hz,实测整数)、`l2_cache_size=262144`(byte)、`l3_cache_size=8388608`(byte)。
- 频率折算:`parseFreqMHz` 检测裸数字 > 100000 视为 Hz,`3503999654 / 1_000_000 = 3503` MHz。
- Cache 单位:实测**byte**;不能用 `parseSizeKB`(按 KB/MB/GB 文本判断),也不能 Contains 匹配 `L2 Cache` —— 否则 `parseKV` 会被 `L2 Cache Line Size: 64` / `L2 Cache CPU Count: 1` 等同前缀字段覆盖(map 遍历无序)。直接按整数 `n / 1024` 得 KB。
- Validator:`Brand / Family / ModelID` 任一非零。

### 2.2 `esxcli hardware cpu global get`(补核心数)

```bash
esxcli hardware cpu global get
```

实测输出:

```
   CPU Packages: 1
   CPU Cores: 4
   CPU Threads: 4
   Hyperthreading Active: false
   Hyperthreading Supported: false
   Hyperthreading Enabled: true
   HV Support: 3
```

- 提取:`cpu_cores=4`。`esxcli hardware cpu list` 没有 cores 字段。

### 2.3 `smbiosDump`(真实 CPU 型号 / 当前频率 / 核心数)

```bash
smbiosDump 2>/dev/null | awk '/Processor Info \(Type 4\)/{p=NR+30} NR<=p'
```

实测输出:

```
  Processor Info (Type 4): #74
    Payload length: 0x30
    Socket: "U3E1"
    Socket Type: 0x32 (Socket LGA1151)
    Socket Status: Populated
    Type: 0x03 (CPU)
    Family: 0xb3 (Xeon)
    Manufacturer: "Intel(R) Corporation"
    Version: "Intel(R) Xeon(R) E-2224G CPU @ 3.50GHz"
    Serial: "To Be Filled By O.E.M."
    Asset Tag: "To Be Filled By O.E.M."
    Part Number: "To Be Filled By O.E.M."
    Processor ID: 0xbfebfbff000906ea
    Status: 0x01 (Enabled)
    Voltage: 1.0 V
    External Clock: 100 MHz
    Max. Speed: 4700 MHz
    Current Speed: 3500 MHz
    L1 Cache: #71
    L2 Cache: #72
    L3 Cache: #73
    Core Count: 4
    Core Enabled Count: 4
    Thread Count: 4
```

- 远端用 `awk` 仅截 Type 4 块后 30 行:`smbiosDump` 全量约 700 行/35 KiB,SSH 窗口流控下偶发拖慢。
- 提取(`fillFromSmbios`):
  - `Version: "Intel(R) Xeon(R) E-2224G CPU @ 3.50GHz"` → 真实型号,覆盖 `esxcli cpu list` 的 `Brand`。
  - `Current Speed: 3500 MHz` → `FreqMHz = 3500`。
  - `Core Count: 4` → `Cores = 4`。
- 跳过逻辑:`Version` 是 `"To Be Filled By O.E.M."` 直接忽略;块结束以遇到顶格(无前导空格)行为标志。
- 超时:12s,2 次尝试。

### 2.4 `vsish` 读 MSR 0x1A2(TjMax)

```bash
vsish -e get /hardware/msr/pcpu/0/addr/0x1A2
```

实测输出:

```
0x641400
```

- 解码(`decodeTjMax`):`(0x641400 >> 16) & 0xFF = 0x64 = 100`,所以 `TjMax = 100°C`。
- 含义:MSR_TEMPERATURE_TARGET bits 23:16。
- 超时:8s,2 次尝试。
- Validator:`decodeTjMax > 0`。

---

## 3. MemoryInfo — 内存总量 / 可用

### 3.1 `esxcli hardware memory get`

```bash
esxcli hardware memory get
```

实测输出:

```
   Physical Memory: 137262243840 Bytes
   Reliable Memory: 0 Bytes
   NUMA Node Count: 1
```

- 提取:`parseMemory` 找 key 含 `physical_memory` 的行,经 `parseBytes` 得 `TotalBytes = 137262243840`。
- `parseBytes` 支持 `Bytes / KB / MB / GB / GiB` 单位自动识别。

### 3.2 `vsish -e cat /memory/comprehensive`(拿可用内存 + 兜底总量)

```bash
vsish -e cat /memory/comprehensive
```

实测输出:

```
Comprehensive {
   Physical memory estimate:134045160 KB
   Given to VMKernel:134045160 KB
   Reliable memory:0 KB
   Discarded by VMKernel:1596 KB
   Kernel code region:24576 KB
   Kernel data and heap:16384 KB
   Other kernel:740776 KB
   Non-kernel:63205300 KB
   Reserved memory at low addresses:393212 KB
   Free:70056528 KB
}
```

- 提取:
  - `Free: 70056528 KB` → `FreeBytes = 70056528 × 1024 ≈ 71.7 GB`。
  - `Physical memory estimate: 134045160 KB` → 仅当 `esxcli` 没拿到 `TotalBytes` 时兜底。
- Validator:`TotalBytes > 0 || FreeBytes > 0`。

---

## 4. RuntimeUsage — 主机 CPU / 内存使用率

### `vim-cmd hostsvc/hostsummary`

```bash
vim-cmd hostsvc/hostsummary | grep -E 'overallCpuUsage|overallMemoryUsage|cpuMhz|numCpuCores|memorySize'
```

实测输出(只取关键字段):

```
      memorySize = 137262243840,
      cpuMhz = 3504,
      numCpuCores = 4,
      overallCpuUsage = 3915,
      overallMemoryUsage = 62806,
```

- 解析(`parseRuntimeUsage`):按 `key = value,` 格式扫;`overallCpuUsage` 单位 MHz,`overallMemoryUsage` 单位 MiB,`memorySize` 单位 byte。
- 计算:
  - CPU 容量 = `cpuMhz × numCpuCores = 3504 × 4 = 14016 MHz`。
  - CPU 使用率 = `3915 / 14016 ≈ 28%`。
  - 内存使用率 = `62806 MiB / 137262243840 byte ≈ 48%`。
- 缺数据时用 `CPUStatic.FreqMHz / Cores` 和 `MemoryInfo.TotalBytes` 兜底。
- 百分比四舍五入:`percentInt(used, total) = (used×100 + total/2) / total`。
- 超时:12s,2 次尝试。

---

## 5. CPUTemperature — 每核温度 / 头空间

```bash
for i in 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  tj=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x1A2 2>/dev/null)
  dro=$(vsish -e get /hardware/msr/pcpu/$i/addr/0x19C 2>/dev/null)
  printf 'CORE=%s TJ=%s DRO=%s\n' "$i" "$tj" "$dro"
  if [ -z "$tj" ] || [ -z "$dro" ]; then break; fi
done
```

实测输出(E-2224G 4 核):

```
CORE=0 TJ=0x641400 DRO=0x88310800
CORE=1 TJ=0x641400 DRO=0x882f0800
CORE=2 TJ=0x641400 DRO=0x88300800
CORE=3 TJ=0x641400 DRO=0x882d0800
```

- 含义:
  - MSR `0x1A2`(MSR_TEMPERATURE_TARGET):TjMax = `bits 23:16`。
  - MSR `0x19C`(IA32_THERM_STATUS):Digital Readout(DRO,头空间)= `bits 22:16`(7 bit,所以 `& 0x7F`)。
- 核温公式:**核温 = TjMax − DRO**。
- 计算 Core 0:
  - `TjMax = (0x641400 >> 16) & 0xFF = 0x64 = 100°C`。
  - `DRO  = (0x88310800 >> 16) & 0x7F = 0x31 = 49°C`(0x88310800 → 高位 0x8831 → `& 0x7F` = 0x31)。
  - `Temp = 100 − 49 = 51°C`。
- 聚合:`MaxC`(最高)、`AvgC`(均值,本机约 50°C)。
- 远端单 session 循环,扛 N×2 次 `ssh.NewSession` 开销;`tj`/`dro` 任一为空即 break(失败核之后不再尝试)。
- 超时:15s,3 次尝试。
- Validator:`len(Cores) >= CPUStatic.Cores`。

---

## 6. MCEHealth — Machine Check Error 健康

```bash
vsish -e cat /hardware/health/mce
```

实测输出:

```
Machine check error stats {
   Total corrected errors since boot:0
   EWMA of corrected errors per period:0
   Period in seconds:120
   Total uncorrected errors since boot:0
   Health state: 0 -> Green
}
```

- 提取:
  - `State`(`Green/Yellow/Red`):正则 `Health state:\s*\d+\s*->\s*(\w+)`。
  - 数值字段(`CorrectedTotal / EWMA / PeriodSeconds / UncorrectedTotal`):`parseTrailingInt64` 从行尾抓首串数字。
- Validator:`State != ""`。
- 超时:8s,2 次尝试。

---

## 7. DiskHealth — 磁盘 SMART / 容量 / 用量

最复杂的一段,5 个子步骤。本机 4 块盘:1× Samsung SSD 870 EVO 1TB(SATA SSD,系统盘)、1× WD HC550 16TB(SATA HDD)、1× WD WD6003FFBX 6TB(SATA HDD)、1× Samsung 990 PRO 4TB(NVMe)。

### 7.1 设备清单 — `esxcli storage core device list`

```bash
esxcli storage core device list
```

实测输出(取首块):

```
t10.ATA_____Samsung_SSD_870_EVO_1TB_________________XXXXXXXXXXXXXXX_____
   Display Name: Local ATA Disk (t10.ATA_____Samsung_SSD_870_EVO_1TB_________________XXXXXXXXXXXXXXX_____)
   Has Settable Display Name: true
   Size: 953869
   Device Type: Direct-Access
   Multipath Plugin: HPP
   Devfs Path: /vmfs/devices/disks/t10.ATA_____...
   Vendor: ATA
   Model: Samsung SSD 870
   Revision: 2B6Q
   ...
```

- 提取(`parseDeviceInventory`):每个设备 id 起首,后续缩进 KV(`Model:` / `Device Type:` / `Size:`)。
- 设备 id 正则:`(t10\.\S+|naa\.\S+|mpx\.\S+|eui\.\S+)`。
- 容量换算(`parseESXiDeviceSize`):`Size: 953869` 单位是 **MiB** → `953869 × 1024 × 1024 ≈ 1 TB`。带 `B/Bytes/GB` 后缀时走 `parseBytes`。
- 过滤(`isDiskDevice`):去掉 model/type 含 `dvd / cd-rom / cdrom / optical` 的设备。本机一台 `mpx.vmhba0:C0:T1:L0`(CD-ROM)会被剔除。
- 去重保序:同一 id 多次出现保留第一次。
- 超时:15s(多盘 SCSI inquiry 偶发 5-10s),2 次尝试。

### 7.2 SMART 合批 — `===DEV===` 分段

合批策略(伪代码):

```bash
printf '===DEV===%s\n' 't10.ATA_____Samsung_SSD_870_EVO_1TB_____...'
esxcli storage core device smart get -d 't10.ATA_____Samsung_SSD_870_EVO_1TB_____...'
printf '===DEV===%s\n' 't10.NVMe____Samsung_SSD_990_PRO_with_Heatsink_4TB___XXXXXXXXXXXXXXXX'
esxcli storage core device smart get -d 't10.NVMe____Samsung_SSD_990_PRO_with_Heatsink_4TB___XXXXXXXXXXXXXXXX'
# ... 每块盘一段
```

ATA SSD(Samsung 870)输出:

```
Parameter                  Value  Threshold  Worst  Raw
-------------------------  -----  ---------  -----  ---
Health Status              OK     N/A        N/A    N/A
Media Wearout Indicator    99     0          99     24
Write Error Count          100    10         100    0
Power-on Hours             93     0          93     233
Power Cycle Count          99     0          99     161
Reallocated Sector Count   100    10         100    0
Drive Temperature          62     0          49     38
Write Sectors TOT Count    99     0          99     213
Initial Bad Block Count    100    10         100    0
Program Fail Count         100    10         100    0
Erase Fail Count           100    10         100    0
Uncorrectable Error Count  100    0          100    0
```

ATA HDD(WD HC550)输出:

```
Parameter                          Value  Threshold  Worst  Raw
---------------------------------  -----  ---------  -----  ---
Health Status                      OK     N/A        N/A    N/A
Read Error Count                   0      16         N/A    0
Power-on Hours                     96     0          96     58
Power Cycle Count                  193    0          N/A    193
Reallocated Sector Count           0      5          N/A    0
Drive Temperature                  49     0          N/A    49
Sector Reallocation Event Count    0      0          N/A    0
Pending Sector Reallocation Count  0      0          N/A    0
Uncorrectable Sector Count         0      0          N/A    0
```

NVMe(Samsung 990 PRO)输出:

```
Parameter                 Value  Threshold  Worst  Raw
------------------------  -----  ---------  -----  ---
Health Status             OK     N/A        N/A    N/A
Power-on Hours            15319  N/A        N/A    N/A
Power Cycle Count         126    N/A        N/A    N/A
Reallocated Sector Count  0      90         N/A    N/A
Drive Temperature         49     82         N/A    N/A
```

三家行为对比:

| 设备 | Drive Temperature 行 | Raw 列 | Value 列 |
|---|---|---|---|
| ATA SSD | `62 0 49 38` | 38 = 真实温度°C | 62 = normalized |
| ATA HDD | `49 0 N/A 49` | 49 = 真实温度°C | 49 = 也是真值 |
| NVMe | `49 82 N/A N/A` | **N/A** | **49 = 真值** |

#### 关键规则:Raw 优先,Raw=N/A 回退 Value(`smartIntPick` / `smartInt64Pick`)

ESXi 对 ATA 盘 Raw 列只显示低 1 字节(Power-on Hours 应为大数,这里只看到 233/58 —— 已经被截断)。覆盖逻辑见 7.4。

#### `parseSMARTAttrs` 取值对照

| DiskHealth 字段 | SMART 参数名 |
|---|---|
| `HealthStatus` | `Health Status`(取 Value 列字符串) |
| `PowerOnHours` | `Power-on Hours` |
| `PowerCycleCount` | `Power Cycle Count` |
| `ReallocatedSectors` | `Reallocated Sector Count` |
| `UncorrectableErrors` | `Uncorrectable Error Count` / `Uncorrectable Sector Count`(SSD/HDD 名不同) |
| `MediaWearoutValue` | `Media Wearout Indicator`(SSD 独有,只取 Value 归一化,100=新 0=耗尽) |
| `ReadErrorCount` | `Read Error Count` |
| `PendingSectorReallocation` | `Pending Sector Reallocation Count` |
| `TempC` | `Drive Temperature`(Raw/Value 二选一) |
| `ThresholdC` | `Drive Temperature` 的 Threshold 列 |

- 列布局:`<参数名 N tokens> <Value> <Threshold> <Worst> <Raw>`,末尾固定 4 列。
- 参数名可能 1~多 token(如 `Pending Sector Reallocation Count` 4 token),按 `len(fields)-4` 切。
- 超时:30s(8 盘合批可能 12-15s),2 次尝试。
- Validator:`===DEV===` 出现次数 ≥ 盘数。
- 失败兜底:即便 `ssh.Run` 返回非零,只要 stdout 含 `===DEV===` 就 partial 解析。

### 7.3 SMART 单盘重试

合批后某盘 `TempC < 0`,对该盘单独再跑:

```bash
esxcli storage core device smart get -d t10.ATA_____Samsung_SSD_870_EVO_1TB_____...
```

10s 超时,2 次尝试。

### 7.4 ATA Raw 6 字节修正 — vsish

#### 为什么需要

ATA SSD Power-on Hours 实测 Raw=233(截到低 1 字节),真实值应该是几万小时。`esxcli` 在 ATA 盘上只暴露低 1 字节,得绕过它直接读 vsish 原始 buffer。

#### 命令

```bash
vsish -e get /storage/scsifw/devices/t10.ATA_____Samsung_SSD_870_EVO_1TB_____.../smart/valuesBuffer
```

实测输出(前 30 字节):

```
[0]: 0x01
[1]: 0x00
[2]: 0x05    ← attribute id = 5 (Reallocated Sectors)
[3]: 0x33
[4]: 0x00
[5]: 0x64    ← attribute 5 的 6-byte raw[0]
[6]: 0x64
[7]: 0x00
[8]: 0x00
[9]: 0x00
[10]: 0x00
[11]: 0x00
[12]: 0x00
[13]: 0x00
[14]: 0x09   ← attribute id = 9 (Power-on Hours)
[15]: 0x32
[16]: 0x00
[17]: 0x5d
[18]: 0x5d
[19]: 0xe9   ← attribute 9 的 6-byte raw[0]
[20]: 0x76
[21]: 0x00
...
```

#### 结构与解析(`parseATASMARTBuffer`)

vsish 返回 512 字节 ATA SMART data:`bytes[0:2]=revision`,然后 30 个 attribute entry × 12 字节:

```
offset = 2 + i*12          // attribute 在 buffer 里的起始位置
aid    = buf[offset]       // attribute id
raw    = bytes(offset+5..offset+10) 6 字节,小端序
```

例:Power-on Hours(attribute id 9)
- entry 起点 `offset = 14`,`buf[14] = 0x09` ✓
- `raw[0..5] = 0xe9 0x76 0x00 0x00 0x00 0x00`
- 小端组合:`0x000000007ee9 = 30441 小时 ≈ 3.47 年` —— 比 esxcli Raw=233 准确得多。

#### attribute id 对应字段

| ATA ID | 字段 |
|---|---|
| 5 | `ReallocatedSectors` |
| 9 | `PowerOnHours` |
| 12 | `PowerCycleCount` |
| 197 | `PendingSectorReallocation` |
| 198 | `UncorrectableErrors`(Offline Uncorrectable) |

- vsish 非 -1 的字段覆盖 esxcli 解析结果。
- NVMe 跳过:`t10.NVMe...` 前缀不调用 vsish(该路径返回 `Not supported`)。
- 合批 + `===DEV===` 分段,20s 超时,2 次尝试。
- 解析少于 50 字节视为截断,放弃覆盖。

### 7.5 容量 / 用量映射

#### 7.5.1 `esxcli storage filesystem list`

```bash
esxcli storage filesystem list
```

实测输出:

```
Mount Point                                        Volume Name                                 UUID                                 Mounted  Type              Size            Free
-------------------------------------------------  ------------------------------------------  -----------------------------------  -------  ------  --------------  --------------
/vmfs/volumes/6314fb31-cbf77190-ea99-XXXXXXXXXXXX  wdc_6t                                      6314fb31-cbf77190-ea99-XXXXXXXXXXXX     true  VMFS-6   6001143054336   3758807318528
/vmfs/volumes/63972d60-cbc9a7ec-9a08-XXXXXXXXXXXX  samsung_1t                                  63972d60-cbc9a7ec-9a08-XXXXXXXXXXXX     true  VMFS-6    980594720768    979069042688
/vmfs/volumes/66d056dc-fbc56f2e-6914-XXXXXXXXXXXX  samsung_4t                                  66d056dc-fbc56f2e-6914-XXXXXXXXXXXX     true  VMFS-6   4000762036224   1541203296256
/vmfs/volumes/686e51e3-dabd1d5e-a799-XXXXXXXXXXXX  hc550                                       686e51e3-dabd1d5e-a799-XXXXXXXXXXXX     true  VMFS-6  16000632225792  11600314499072
/vmfs/volumes/63972d60-b15becb2-68fc-XXXXXXXXXXXX  OSDATA-63972d60-b15becb2-68fc-XXXXXXXXXXXX  63972d60-b15becb2-68fc-XXXXXXXXXXXX     true  VFFS       10468982784      5367660544
/vmfs/volumes/582ca1fa-f4400e5a-dba6-XXXXXXXXXXXX  BOOTBANK1                                   582ca1fa-f4400e5a-dba6-XXXXXXXXXXXX     true  vfat        4293591040      4073783296
/vmfs/volumes/b5736b60-d6415986-8198-XXXXXXXXXXXX  BOOTBANK2                                   b5736b60-d6415986-8198-XXXXXXXXXXXX     true  vfat        4293591040      4073848832
```

- `parseStorageFilesystems`:只保留 `Mounted=true` 且 `Type` 前缀 `vmfs` 的 datastore。`VFFS / vfat` 直接过滤。
- 列从尾部倒着切(末 5 列固定),`Volume Name` 可能含空格。
- `Size / Free` 单位 byte。

#### 7.5.2 `esxcli storage vmfs extent list`

```bash
esxcli storage vmfs extent list
```

实测输出:

```
Volume Name                                 VMFS UUID                            Extent Number  Device Name                                                                Partition
------------------------------------------  -----------------------------------  -------------  -------------------------------------------------------------------------  ---------
wdc_6t                                      6314fb31-cbf77190-ea99-XXXXXXXXXXXX              0  t10.ATA_____WDC_WD6003FFBX2D68MU3N0__________________XXXXXXXX____________          1
samsung_1t                                  63972d60-cbc9a7ec-9a08-XXXXXXXXXXXX              0  t10.ATA_____Samsung_SSD_870_EVO_1TB_________________XXXXXXXXXXXXXXX_____           8
samsung_4t                                  66d056dc-fbc56f2e-6914-XXXXXXXXXXXX              0  t10.NVMe____Samsung_SSD_990_PRO_with_Heatsink_4TB___XXXXXXXXXXXXXXXX               1
hc550                                       686e51e3-dabd1d5e-a799-XXXXXXXXXXXX              0  t10.ATA_____WDC__WUH721816ALE6L4____________________XXXXXXXX____________           3
OSDATA-63972d60-b15becb2-68fc-XXXXXXXXXXXX  63972d60-b15becb2-68fc-XXXXXXXXXXXX              0  t10.ATA_____Samsung_SSD_870_EVO_1TB_________________XXXXXXXXXXXXXXX_____           7
```

#### 7.5.3 关联逻辑(`mapDiskUsage`)

实测拓扑:每块盘只承载 1 个 VMFS extent(单 extent datastore),映射结果:

| 设备 | datastore | UsedBytes | FreeBytes |
|---|---|---|---|
| Samsung 870 1TB | `samsung_1t` | `980594720768 - 979069042688 = 1525678080`(~1.4 GiB) | `979069042688` |
| WD 6T | `wdc_6t` | `6001143054336 - 3758807318528 ≈ 2.04 TiB` | `3758807318528` |
| Samsung 990 PRO 4TB | `samsung_4t` | `4000762036224 - 1541203296256 ≈ 2.24 TiB` | `1541203296256` |
| WD HC550 16T | `hc550` | `16000632225792 - 11600314499072 ≈ 4.0 TiB` | `11600314499072` |

注意 Samsung 870 实际承载了 `samsung_1t` 和 `OSDATA-...` 两个 datastore(extent list 里它出现 2 次),`mapDiskUsage` 把同盘多 datastore 的 used/free 累加:**实际 UsedBytes ≈ samsung_1t(1.4 GiB)+ OSDATA(用 5.1 GiB)= 约 6.5 GiB**,`Datastores=["samsung_1t","OSDATA-..."]`。

如果某 datastore 有**多 extent 跨多盘**(本机没有),则:
- 不写 used/free 到任何单盘(无可靠拆分规则);
- 只把 datastore 名追加到所有相关盘的 `Datastores`。

### 7.6 温度分级(`classifyDisk`)

| 类型 | warning | critical |
|---|---|---|
| NVMe | ≥ 70°C | ≥ 80°C |
| SSD | ≥ 60°C | ≥ 70°C |
| HDD / ATA | ≥ 50°C | ≥ 55°C |

本机:Samsung 870=38°C → ok;WD HC550=49°C → ok(HDD warning 阈值 50);NVMe=49°C → ok。

---

## 8. USBState — 控制器 / 仲裁器 / 直通设备

### 8.1 控制器 — `lspci | grep -i usb`

```bash
lspci | grep -i usb
```

实测输出:

```
0000:00:14.0 USB controller: Intel Corporation Cannon Lake PCH USB 3.1 xHCI Host Controller
```

- 正则 `^(\S+)\s+USB\s+controller:\s+(.+)$` → `PCIAddr="0000:00:14.0"`, `Name="Intel Corporation Cannon Lake PCH USB 3.1 xHCI Host Controller"`。
- Validator:解析出 ≥ 1 个控制器。

### 8.2 USB Arbitrator — `/etc/init.d/usbarbitrator status`

```bash
/etc/init.d/usbarbitrator status
```

实测输出:

```
usbarbitrator is running
```

- 看输出含 `running` 或 `stopped`,本机 → `ArbitratorRunning=true`。

### 8.3 可直通设备 — `localcli hardware usb passthrough device list`

```bash
localcli hardware usb passthrough device list
```

实测输出:

```
Bus  Dev  VendorId  ProductId  Enabled  Can Connect to VM  Name
---  ---  --------  ---------  -------  -----------------  ----
1    2    152d      a576       true     yes                JMicron Technology Corp. / JMicron USA Technology Corp.
```

- 列固定:`Bus / Dev / VID / PID / Enabled / Can Connect / Name`。
- 跳过表头(`Bus`)和分隔线(`---`)。
- VID/PID 已是 4 位 hex(`152d:a576`)。
- 最后 `filterVMOwnedUSB`:删掉已被 VM 持有的设备(VID:PID 命中),避免双计。本机的 JMicron 设备被 fnOS 直通,所以**会被 filter 掉**,不出现在最终 `AvailableForPassthrough` 里。

### 8.4 VM 持有的直通 USB

#### 合批策略 — `===VM===` 分段

```bash
printf '===VM===%d\n' 130; vim-cmd vmsvc/device.getdevices 130
printf '===VM===%d\n' 102; vim-cmd vmsvc/device.getdevices 102
# ... 每个 VM 一段
```

合批理由:每次 vim-cmd 启动 ~1s,14 个 VM 各起一次 session 会被 ESXi sshd `MaxStartups` 限速。

#### VM 130(fnOS)中的 VirtualUSB 块

实测输出(`vim-cmd vmsvc/device.getdevices 130`):

```
      (vim.vm.device.VirtualUSBXHCIController) {     ← 虚拟控制器,不算直通
         key = 14000,
         deviceInfo = (vim.Description) {
            label = "USB xHCI controller ",
            summary = "USB xHCI controller"
         },
         backing = (vim.vm.device.VirtualDevice.BackingInfo) null,
         ...
      }

      (vim.vm.device.VirtualUSB) {                   ← 物理直通
         key = 41000,
         deviceInfo = (vim.Description) {
            label = "USB 41001",
            summary = "JMicron / JMicron USA External"
         },
         backing = (vim.vm.device.VirtualUSB.USBBackingInfo) {
            deviceName = "path:0/1/20",              ← 关键标记
            useAutoDetect = <unset>
         },
         connectable = (vim.vm.device.VirtualDevice.ConnectInfo) null,
         slotInfo = (vim.vm.device.VirtualDevice.BusSlotInfo) null,
         controllerKey = 14000,
         unitNumber = 41000,
         numaNode = <unset>,
         connected = true,
         vendor = 5421,                              ← 十进制
         product = 42358,                            ← 十进制
```

#### 识别规则

- 找 `(vim.vm.device.VirtualUSB)` 标记 → `advanceBalancedBlock` 用花括号配对找块结束。
- `deviceName` 必须以 `path:` 起首才视为"物理直通"(过滤掉 xHCI 虚拟控制器)。
- `label` / `summary` 用 `quotedRe = "([^"]*)"` 取双引号内容。
- `vendor=5421` → hex `0x152d`(`fmt.Sprintf("%04x")`);`product=42358` → hex `0xa576`。
- 结果:`VMID=130, VMName=fnOS, Label="USB 41001", Summary="JMicron / JMicron USA External", Path="0/1/20", VID="152d", PID="a576"`。
- VID:PID 与 8.3 步骤里的可直通设备命中 → 从 `AvailableForPassthrough` 删除。

合批超时 35s,2 次尝试;缺台的 VM 单独补跑(10s × 2)。

---

## 9. VM — 虚拟机列表 / Guest OS / 电源态

### 9.1 `vim-cmd vmsvc/getallvms`

```bash
vim-cmd vmsvc/getallvms
```

实测输出(取前几行):

```
Vmid       Name                                   File                                   Guest OS       Version   Annotation
102    GitLabRunner    [samsung_4t] virtualMachine/GitLabRunner/GitLabRunner.vmx     rhel9_64Guest      vmx-19
103    Docker          [samsung_4t] virtualMachine/Docker/Docker.vmx                 rhel9_64Guest      vmx-19
104    Nps             [samsung_4t] virtualMachine/Nps/Nps.vmx                       rhel9_64Guest      vmx-19
106    Npc             [samsung_4t] virtualMachine/Npc/Npc.vmx                       rhel9_64Guest      vmx-19
116    WAF             [samsung_4t] virtualMachine/WAF/WAF.vmx                       rhel9_64Guest      vmx-19
122    StreamGateway   [samsung_4t] virtualMachine/StreamGateway/StreamGateway.vmx   rhel9_64Guest      vmx-19
130    fnOS            [samsung_4t] virtualMachine/fnOS/fnOS.vmx                     debian11_64Guest   vmx-19
131    GitLabEE        [samsung_4t] GitLabEE/GitLabEE.vmx                            rhel9_64Guest      vmx-19
139    Snapshot        [samsung_4t] Snapshot/Snapshot.vmx                            rhel9_64Guest      vmx-19
```

#### `parseVMListShallow`(VMShallow = id + name)
- 第一列必是数字(`Vmid`)。
- Name 列可能含空格,以 `[` 起首字段(`[samsung_4t]`)即路径开始 → name 取到此为止。
- 例:VM 130 → `VMShallow{ID:130, Name:"fnOS"}`。

#### `parseVMGuestOS`
- 正则:`(?m)^(\d+)\s+.+\.vmx\]?\s+(.+?)\s+vmx-\d+`。
- 锚点:`.vmx]` 之后到 `vmx-NN` 之前的串。
- 例:VM 130 → `guestOS[130]="debian11_64Guest"`;VM 102 → `"rhel9_64Guest"`。

### 9.2 单 VM 电源态 — `vim-cmd vmsvc/power.getstate <id>`

```bash
vim-cmd vmsvc/power.getstate 130
```

实测输出:

```
Retrieved runtime info
Powered on
```

- 状态映射(`mapVMPowerState`):`Powered on / Powered off / Suspended` → `powered_on / powered_off / suspended`。
- VM id 错的话:`(vim.fault.NotFound) ... Unable to find a VM corresponding to "1"`,映射成 `unknown`。

### 9.3 电源态合批 — `===VM===` 分段

```bash
printf '===VM===%d\n' 102; vim-cmd vmsvc/power.getstate 102
printf '===VM===%d\n' 103; vim-cmd vmsvc/power.getstate 103
# ...
```

- 超时 35s(14 VM × ~1s),2 次尝试。
- Validator:`已知态(非 unknown 非空) == 总 VM 数`。
- 缺台兜底:对未知态 VM 单独再跑(8s × 2)。

> 关于"每台 VM 功耗":vSphere 性能计数器只有主机级 `power.power.average`,**没有每 VM 真实功耗读数**。常见做法是按 CPU/内存使用份额估算分摊主机功耗,本项目未实现。

---

## 10. HostBoot — 启动信息 / 内核 crash dump

### 一次远端 shell 合批

```bash
u=$(esxcli system stats uptime get 2>/dev/null)
n=$(date +%s)
c=$(ls /var/core/vmkernel-zdump.* 2>/dev/null | wc -l)
m=$(ls /var/core/vmkernel-zdump.* 2>/dev/null | xargs -n1 stat -c '%Y' 2>/dev/null | sort -n | tail -1)
printf 'UPTIME_US=%s\nNOW_EPOCH=%s\nZDUMP_COUNT=%s\nZDUMP_LATEST=%s\n' "$u" "$n" "$c" "$m"
```

实测输出:

```
UPTIME_US=42825414367
NOW_EPOCH=1781193356
ZDUMP_COUNT=2
ZDUMP_LATEST=1781022240
```

- 解析(`parseHostBoot`):
  - `esxcli system stats uptime get` 单位是**微秒**(实测,易踩坑)。
  - `UptimeSeconds = 42825414367 / 1_000_000 = 42825 秒 ≈ 11.9 小时`。
  - `BootedAt = time.Unix(1781193356 - 42825, 0).UTC()` —— 用远端 `date +%s`,**不依赖远端时区**,直出 UTC。
  - `CrashDumpCount = 2`(本机有 2 个老 zdump)。
  - `ZDUMP_LATEST=1781022240` → `LastCrashAt = time.Unix(1781022240, 0).UTC()`(最近一次 crash)。
- 无 zdump 时 `LATEST` 留空,`LastCrashAt` 保留零值。
- 失败兜底:`UptimeSeconds = -1`。

---

## 11. NIC — 物理网卡列表 / 链路状态 / 收发计数

### 11.1 `esxcli network nic list`

```bash
esxcli network nic list
```

实测输出:

```
Name    PCI Device    Driver         Admin Status  Link Status  Speed  Duplex  MAC Address         MTU  Description
------  ------------  -------------  ------------  -----------  -----  ------  -----------------  ----  -----------
vmnic0  0000:00:1f.6  ne1000         Up            Up            1000  Full    XX:XX:XX:XX:XX:XX  1500  Intel Corporation Ethernet Connection (7) I219-LM
vmnic1  0000:01:00.0  igc-community  Up            Up            2500  Full    XX:XX:XX:XX:XX:XX  1500  Intel Corporation Ethernet Controller I226-V
```

- `parseNICList`:`vmnic0/vmnic1` 起首,按空白切;`Description` 列可能含空格(`Intel Corporation Ethernet ...`),取 `fields[9:]` join。
- 例(vmnic0):`Driver=ne1000, AdminStatus=Up, LinkStatus=Up, SpeedMbps=1000, Duplex=Full, MAC=XX:XX:XX:XX:XX:XX, MTU=1500`。
- Validator:输出含 `vmnic`。
- 缺数据兜底:`SpeedMbps=-1`/`Duplex=""`(链路 Down 或缺字段)。

### 11.2 `esxcli network nic stats get -n <vmnic>`

```bash
esxcli network nic stats get -n vmnic0
```

实测输出(取头部):

```
NIC statistics for vmnic0
   Packets received: 149901
   Packets sent: 96968
   Bytes received: 91835685
   Bytes sent: 21252409
   Receive packets dropped: 0
   Transmit packets dropped: 0
   Multicast packets received: 29569
   Broadcast packets received: 1959
   Multicast packets sent: 6776
   Broadcast packets sent: 1613
   Total receive errors: 0
   Receive length errors: 0
   ...
   Total transmit errors: 0
   Transmit aborted errors: 0
```

`fillNICStats` 取的字段:

| NIC 字段 | esxcli 行 |
|---|---|
| `RxBytes` | `Bytes received` → 91835685 |
| `TxBytes` | `Bytes sent` → 21252409 |
| `RxDropped` | `Receive packets dropped` → 0 |
| `TxDropped` | `Transmit packets dropped` → 0 |
| `RxErrors` | `Total receive errors` → 0 |
| `TxErrors` | `Total transmit errors` → 0 |

- vmnic 数量个位数,串行调用即可。

---

## 12. NetTopology — 网络拓扑(vSwitch / Portgroup / VM vNIC)

### 12.1 标准 vSwitch — `esxcli network vswitch standard list`

```bash
esxcli network vswitch standard list
```

实测输出(取首块):

```
vSwitch0
   Name: vSwitch0
   Class: cswitch
   Num Ports: 2560
   Used Ports: 17
   Configured Ports: 128
   MTU: 1500
   CDP Status: listen
   Beacon Enabled: false
   Beacon Interval: 1
   Beacon Threshold: 3
   Beacon Required By:
   Uplinks: vmnic0
   Portgroups: VM Network, Management Network

vSwitch1
   Name: vSwitch1
   ...
   Uplinks: vmnic1
   Portgroups: VM Network1, Management Network1
```

- `parseVSwitchList`:块按"顶格行(无前导空格、无冒号)"分隔;`splitCSV` 拆 `Uplinks: vmnic0` / `Portgroups: VM Network, Management Network`。
- 结果:
  - `vSwitch0`:`Uplinks=[vmnic0]`,`Portgroups=["VM Network","Management Network"]`。
  - `vSwitch1`:`Uplinks=[vmnic1]`,`Portgroups=["VM Network1","Management Network1"]`。

### 12.2 开机 VM 的 World ID — `esxcli network vm list`(重试 3 次)

```bash
esxcli network vm list
```

实测输出:

```
World ID  Name           Num Ports  Networks
--------  -------------  ---------  --------
 2270220  fnOS_2                 2  VM Network, VM Network1
 2270066  fnOS                   2  VM Network, VM Network1
 2100861  Docker                 2  VM Network, VM Network1
 2104084  Snapshot               2  VM Network, VM Network1
 2100702  StreamGateway          2  VM Network, VM Network1
 2101155  Componet               2  VM Network, VM Network1
 2100440  WAF                    2  VM Network, VM Network1
 2101723  GitLabEE               2  VM Network, VM Network1
 2103142  Nps                    2  VM Network, VM Network1
 2340597  fnOS_3                 2  VM Network, VM Network1
 2102248  GitLabRunner           2  VM Network, VM Network1
 2225304  Npc                    2  VM Network, VM Network1
 2104531  OVH_US                 2  VM Network, VM Network1
```

- 这里返回的是 **World ID**(`fnOS` 是 `2270066`),与 `VMShallow.ID`(vim-cmd 的 `Vmid=130`)**不是同一个值**!
- `parseVMNetList`:VM 名可能含空格,从后往前找第一个不含逗号的纯数字字段作为 `Num Ports` 切分锚点。
- Validator(`vmNetListCovers`):传入"已知开机 VM 名单"(`poweredOnVMNames(snap.VMs)`),要求解析出的行覆盖全部期望 VM。这条命令偶发输出截断,用名单做交叉验证让 `runEsxiRetry` 能感知并重试。
- 故意 3 次尝试(而不是 2 次):截断问题发生率比其他命令高。

### 12.3 每 VM vNIC 边 — `esxcli network vm port list -w <world_id>`

```bash
esxcli network vm port list -w 2270220
```

实测输出(fnOS_2 的两张 vNIC):

```
   Port ID: 67108881
   vSwitch: vSwitch0
   Portgroup: VM Network
   DVPort ID:
   MAC Address: 00:0c:29:72:3f:e3
   IP Address: 0.0.0.0
   Team Uplink: vmnic0
   Uplink Port ID: 2214592517
   Active Filters:

   Port ID: 100663312
   vSwitch: vSwitch1
   Portgroup: VM Network1
   DVPort ID:
   MAC Address: 00:0c:29:72:3f:ed
   IP Address: 0.0.0.0
   Team Uplink: vmnic1
   Uplink Port ID: 2248146952
   Active Filters:
```

- 解析(`parseVMPortList`):空行 `flush`,`Port ID` 起新块。
- `IP Address: 0.0.0.0` 视为无 IP,留空(VMware Tools 没汇报 IP)。
- `TeamUplink` 是当前 active 的 pNIC(标准 vSwitch + 默认 team 策略下稳定)。
- 结果(fnOS_2):两条 edge → `(VSwitch0, VM Network, vmnic0)` + `(VSwitch1, VM Network1, vmnic1)`。
- 前端按 `pNIC → vSwitch → Portgroup → VMs` 四列渲染(`@xyflow/react`)。

---

## 命令清单(去重)

| 命令 | 用途 | 超时 | 重试 |
|---|---|---|---|
| `esxcli hardware platform get` | 厂商/型号/UUID | 8s | 2 |
| `vmware -v; esxcli system version get` | ESXi 版本/build | 8s | 2 |
| `esxcli hardware cpu list` | CPU 静态信息 | 8s | 2 |
| `esxcli hardware cpu global get` | CPU 核心数 | 8s | 2 |
| `smbiosDump \| awk '/Processor Info \(Type 4\)/{p=NR+30} NR<=p'` | 真实 CPU 型号/频率/核心 | 12s | 2 |
| `vsish -e get /hardware/msr/pcpu/0/addr/0x1A2` | TjMax | 8s | 2 |
| `esxcli hardware memory get` | 内存总量 | 8s | 2 |
| `vsish -e cat /memory/comprehensive` | 可用内存 + 总量兜底 | 8s | 2 |
| `vim-cmd hostsvc/hostsummary` | CPU/内存使用率 | 12s | 2 |
| 16 核循环 `vsish 0x1A2 / 0x19C` | 每核温度 | 15s | 3 |
| `vsish -e cat /hardware/health/mce` | MCE 健康 | 8s | 2 |
| `esxcli storage core device list` | 盘清单 | 15s | 2 |
| 合批 `esxcli storage core device smart get -d <id>` | SMART 属性 | 30s | 2 |
| 合批 `vsish -e get /storage/scsifw/devices/<id>/smart/valuesBuffer` | ATA Raw 6 字节修正 | 20s | 2 |
| 单盘补跑 `esxcli storage core device smart get -d <id>` | SMART 单盘补救 | 10s | 2 |
| `esxcli storage filesystem list` | datastore 列表 | 8s | 2 |
| `esxcli storage vmfs extent list` | datastore→device 关联 | 8s | 2 |
| `lspci \| grep -i usb` | USB 控制器 | 8s | 2 |
| `/etc/init.d/usbarbitrator status` | arbitrator 状态 | 8s | 2 |
| `localcli hardware usb passthrough device list` | 可直通 USB | 8s | 2 |
| `vim-cmd vmsvc/getallvms` | VM 列表 + Guest OS | 12s | 2 |
| 合批 `vim-cmd vmsvc/device.getdevices <id>` | VM 持有的 USB | 35s | 2 |
| 单 VM 补跑 `vim-cmd vmsvc/device.getdevices <id>` | VM USB 补救 | 10s | 2 |
| 合批 `vim-cmd vmsvc/power.getstate <id>` | VM 电源态 | 35s | 2 |
| 单 VM 补跑 `vim-cmd vmsvc/power.getstate <id>` | 电源态补救 | 8s | 2 |
| 合批 `uptime + date + zdump 统计` | 主机启动信息 | 8s | 2 |
| `esxcli network nic list` | 物理网卡清单 | 8s | 2 |
| `esxcli network nic stats get -n <vmnic>` | 收发计数 | 8s | 2 |
| `esxcli network vswitch standard list` | vSwitch 拓扑 | 8s | 2 |
| `esxcli network vm list` | 开机 VM World ID | 8s | **3** |
| `esxcli network vm port list -w <wid>` | VM vNIC 边 | 8s | 2 |

---

## 整轮重试与完整性兜底(sampler 层)

`internal/esximon/sampler.go` 的 `probeOne` 调用 `CollectAll` 一次,如果关键指标(`Platform / CPU / 温度 / 磁盘 / 内存 / 运行时用量 / VM / NIC / 拓扑`)缺失,会触发"整轮再跑一次并合并"(`mergeHostMetrics`):后采字段填补先采的缺漏,**不覆盖已成功的值**。任一字段两轮都失败 → 本轮 sample 不入库,`esxi_state` 保留上一轮有效值。

完整性判定与告警差集(新增超阈值才推送,持续超阈值不重复)在 `internal/esximon/alert.go`,不在数据采集本身范围。

---

## 设计取舍小结

1. **不在远端拼 JSON**:ESXi busybox 转义陷阱多,直接把纯文本拉回本地用 Go 解析最稳。
2. **合批 + 分段标志(`===VM===` / `===DEV===`)**:压缩 SSH session 数,扛 `MaxStartups` 限速;每 vim-cmd 启动 ~1s 是大头。
3. **Raw 优先,Value 兜底**:SMART 三家差异 + NVMe Raw=N/A,统一规则避免特例分支。
4. **vsish 6-byte raw 覆盖 esxcli 1-byte raw**:ATA Power-on Hours 这类大数必须靠 vsish 才不被截断到 0-255。
5. **TjMax − DRO 算核温**:Intel CPU 没有内嵌温度寄存器,只能算"距 Tjunction 多少度"。
6. **多 extent datastore 不分摊到单盘**:无可靠归一规则,只记 datastore 名,不写用量(避免误导)。
7. **uptime 用远端 `date +%s` 算 booted_at**:不依赖远端时区,直接 UTC。
8. **Validator + 重试**:`runEsxiRetry` 的 ok 回调让"语义上空"(解析得到 0 项)也能触发重试,比纯 exit code 判定鲁棒。
9. **`{ cmd; true; } 2>/dev/null` 包裹**:扛 stderr banner + 非零 exit code 把已有 stdout 丢弃的双重风险。
10. **`esxcli network vm list` 留 3 次重试**:这条命令实测最易输出截断,Validator 用"覆盖期望 VM 名单"做语义校验,比"行数 > 0"更严格。
