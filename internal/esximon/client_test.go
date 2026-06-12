package esximon

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseEsxiCommandDiagnostics(t *testing.T) {
	stderr := "__HOMER_ESXI_BEGIN__\nwarning from esxcli\n__HOMER_ESXI_END__ rc=2\n"
	clean, diag := parseEsxiCommandDiagnostics(stderr)

	if clean != "warning from esxcli" {
		t.Fatalf("stderr = %q", clean)
	}
	if !diag.started || !diag.finished || !diag.exitCodeKnown || diag.exitCode != 2 {
		t.Fatalf("diag = %+v", diag)
	}
}

func TestParseDeviceInventory(t *testing.T) {
	out := `
t10.ATA_____Samsung_SSD_870_EVO_1TB_________________S6PVNX0T805261Z_____
   Display Name: Local ATA Disk (t10.ATA_____Samsung_SSD_870_EVO_1TB_________________S6PVNX0T805261Z_____)
   Size: 953869
   Device Type: Direct-Access
   Model: Samsung SSD 870 EVO 1TB
eui.002538b831b00000
   Display Name: Local NVMe Disk (eui.002538b831b00000)
   Size: 3815447
   Device Type: Direct-Access
   Model: Samsung SSD 990 PRO 4TB
`
	got := parseDeviceInventory(out)
	sata := got["t10.ATA_____Samsung_SSD_870_EVO_1TB_________________S6PVNX0T805261Z_____"]
	if sata.Model != "Samsung SSD 870 EVO 1TB" {
		t.Fatalf("sata model = %q", sata.Model)
	}
	if sata.CapacityBytes != 953869*1024*1024 {
		t.Fatalf("sata capacity = %d", sata.CapacityBytes)
	}
	nvme := got["eui.002538b831b00000"]
	if nvme.Model != "Samsung SSD 990 PRO 4TB" {
		t.Fatalf("nvme model = %q", nvme.Model)
	}
	if nvme.CapacityBytes != 3815447*1024*1024 {
		t.Fatalf("nvme capacity = %d", nvme.CapacityBytes)
	}
}

func TestMapDiskUsage(t *testing.T) {
	filesystemOut := `
Mount Point                                        Volume Name     UUID                                  Mounted  Type    Size           Free
-------------------------------------------------  --------------  ------------------------------------  -------  ------  -------------  ------------
/vmfs/volumes/651aaa                              datastore1      651aaa                                true     VMFS-6  999653638144   644046012416
/vmfs/volumes/652bbb                              data store      652bbb                                true     VMFS-6  2000           500
/vmfs/volumes/653ccc                              span            653ccc                                true     VMFS-6  3000           1000
/vmfs/volumes/BOOTBANK1                           BOOTBANK1       boot                                  true     vfat    4293591040     4262854656
`
	extentOut := `
Volume Name  VMFS UUID  Extent Number  Device Name  Partition
-----------  ---------  -------------  -----------  ---------
datastore1   651aaa     0              naa.disk1    3
data store   652bbb     0              t10.disk2    1
span         653ccc     0              naa.disk3    1
span         653ccc     1              naa.disk4    1
`
	fs := parseStorageFilesystems(filesystemOut)
	if len(fs) != 3 {
		t.Fatalf("filesystems len = %d", len(fs))
	}
	extents := parseVMFSExtents(extentOut)
	if len(extents) != 4 {
		t.Fatalf("extents len = %d", len(extents))
	}
	usage := mapDiskUsage(fs, extents)

	d1 := usage["naa.disk1"]
	if !d1.Known {
		t.Fatal("naa.disk1 usage should be known")
	}
	if d1.UsedBytes != 999653638144-644046012416 {
		t.Fatalf("naa.disk1 used = %d", d1.UsedBytes)
	}
	if d1.FreeBytes != 644046012416 {
		t.Fatalf("naa.disk1 free = %d", d1.FreeBytes)
	}
	if len(d1.Datastores) != 1 || d1.Datastores[0] != "datastore1" {
		t.Fatalf("naa.disk1 datastores = %#v", d1.Datastores)
	}

	d2 := usage["t10.disk2"]
	if !d2.Known || d2.UsedBytes != 1500 || d2.FreeBytes != 500 {
		t.Fatalf("t10.disk2 usage = %#v", d2)
	}

	if usage["naa.disk3"].Known || usage["naa.disk4"].Known {
		t.Fatalf("multi-extent datastore should not be split: %#v %#v", usage["naa.disk3"], usage["naa.disk4"])
	}
}

func TestParseRuntimeUsage(t *testing.T) {
	out := `
      quickStats = (vim.host.Summary.QuickStats) {
         overallCpuUsage = 875,
         overallMemoryUsage = 32768,
      },
      hardware = (vim.host.Summary.HardwareSummary) {
         cpuMhz = 3500,
         numCpuCores = 4,
         memorySize = 137438953472,
      },
`
	got := parseRuntimeUsage(out, CPUStatic{}, MemoryInfo{})
	if got.CPUUsedMHz != 875 || got.CPUCapacityMHz != 14000 || got.CPUUsagePercent != 6 {
		t.Fatalf("cpu runtime = %#v", got)
	}
	if got.MemoryUsedBytes != 32768*1024*1024 || got.MemoryTotalBytes != 137438953472 || got.MemoryUsagePercent != 25 {
		t.Fatalf("memory runtime = %#v", got)
	}
}

func TestExtractVirtualUSBMultipleDevices(t *testing.T) {
	out := `
      (vim.vm.device.VirtualUSBXHCIController) {
         key = 14000,
         deviceInfo = (vim.Description) {
            label = "USB xHCI controller ",
            summary = "USB xHCI controller"
         },
         device = (int) [
            41000,
            41001
         ],
         autoConnectDevices = false
      },
      (vim.vm.device.VirtualUSB) {
         key = 41000,
         deviceInfo = (vim.Description) {
            label = "USB 41001",
            summary = "Cyber Power System UT1050EGC"
         },
         backing = (vim.vm.device.VirtualUSB.USBBackingInfo) {
            deviceName = "path:0/1/6",
            useAutoDetect = <unset>
         },
         connected = true,
         vendor = 1892,
         product = 1281,
         family = (string) [
            "hid"
         ],
         speed = (string) [
            "low"
         ]
      },
      (vim.vm.device.VirtualUSB) {
         key = 41001,
         deviceInfo = (vim.Description) {
            label = "USB 41002",
            summary = "JMicron / JMicron USA External"
         },
         backing = (vim.vm.device.VirtualUSB.USBBackingInfo) {
            deviceName = "path:0/1/20",
            useAutoDetect = <unset>
         },
         connected = true,
         vendor = 5421,
         product = 42358,
         family = (string) [
            "storage"
         ],
         speed = (string) [
            "superSpeed"
         ]
      },
`
	got := extractVirtualUSB(130, "fnOS", out)
	if len(got) != 2 {
		t.Fatalf("owned len = %d, %#v", len(got), got)
	}
	if got[0].Summary != "Cyber Power System UT1050EGC" || got[0].Path != "0/1/6" || got[0].VID != "0764" || got[0].PID != "0501" {
		t.Fatalf("first device = %#v", got[0])
	}
	if got[1].Summary != "JMicron / JMicron USA External" || got[1].Path != "0/1/20" || got[1].VID != "152d" || got[1].PID != "a576" {
		t.Fatalf("second device = %#v", got[1])
	}
}

func TestFilterVMOwnedUSB(t *testing.T) {
	avail := []USBPassthroughDevice{
		{Bus: 1, Dev: 3, VID: "152d", PID: "a576", Name: "JMicron", Enabled: true},
		{Bus: 1, Dev: 9, VID: "abcd", PID: "0001", Name: "Other", Enabled: true},
	}
	owned := []USBVMOwned{
		{VMID: 130, VMName: "fnOS", VID: "152d", PID: "a576", Summary: "JMicron"},
	}
	got := filterVMOwnedUSB(avail, owned)
	if len(got) != 1 || got[0].VID != "abcd" {
		t.Fatalf("filtered = %#v", got)
	}
}

func TestIsDiskDeviceExcludesOpticalDevices(t *testing.T) {
	if isDiskDevice(diskDeviceInfo{Model: "DVD-RW DA8AESH", Type: "CD-ROM"}) {
		t.Fatal("optical device should be excluded")
	}
	if !isDiskDevice(diskDeviceInfo{Model: "Samsung SSD 870 EVO 1TB", Type: "Direct-Access"}) {
		t.Fatal("direct-access disk should be included")
	}
}

func TestParseBatchVMPowerState(t *testing.T) {
	out := `
===VM===130
Retrieved runtime info
Powered on
===VM===77
Retrieved runtime info
Powered off
===VM===96
`
	got := parseBatchVMPowerState(out)
	if got[130] != "powered_on" {
		t.Fatalf("vm 130 state = %q", got[130])
	}
	if got[77] != "powered_off" {
		t.Fatalf("vm 77 state = %q", got[77])
	}
	if got[96] != "unknown" {
		t.Fatalf("vm 96 state = %q", got[96])
	}
	if knownPowerCount(got) != 2 {
		t.Fatalf("known count = %d", knownPowerCount(got))
	}
}

// fixture 直接复制自 192.168.31.138 上 esxcli storage core device smart get 的真实输出,
// 覆盖三种盘:SATA SSD(Samsung 870 EVO) / SATA HDD(WDC) / NVMe(Samsung 990 PRO)。
func TestParseSMARTAttrs(t *testing.T) {
	sataSSD := `Parameter                  Value  Threshold  Worst  Raw
-------------------------  -----  ---------  -----  ---
Health Status              OK     N/A        N/A    N/A
Media Wearout Indicator    99     0          99     24
Write Error Count          100    10         100    0
Power-on Hours             93     0          93     209
Power Cycle Count          99     0          99     160
Reallocated Sector Count   100    10         100    0
Drive Temperature          64     0          49     36
Write Sectors TOT Count    99     0          99     81
Initial Bad Block Count    100    10         100    0
Program Fail Count         100    10         100    0
Erase Fail Count           100    10         100    0
Uncorrectable Error Count  100    0          100    0
`
	sataHDD := `Parameter                          Value  Threshold  Worst  Raw
---------------------------------  -----  ---------  -----  ---
Health Status                      OK     N/A        N/A    N/A
Read Error Count                   0      16         N/A    0
Power-on Hours                     96     0          96     34
Power Cycle Count                  192    0          N/A    192
Reallocated Sector Count           0      5          N/A    0
Drive Temperature                  49     0          N/A    49
Sector Reallocation Event Count    0      0          N/A    0
Pending Sector Reallocation Count  0      0          N/A    0
Uncorrectable Sector Count         0      0          N/A    0
`
	nvme := `Parameter                 Value  Threshold  Worst  Raw
------------------------  -----  ---------  -----  ---
Health Status             OK     N/A        N/A    N/A
Power-on Hours            15296  N/A        N/A    N/A
Power Cycle Count         125    N/A        N/A    N/A
Reallocated Sector Count  0      90         N/A    N/A
Drive Temperature         47     82         N/A    N/A
`

	t.Run("sata_ssd", func(t *testing.T) {
		a := parseSMARTAttrs(sataSSD)
		if a.HealthStatus != "OK" {
			t.Fatalf("health=%q", a.HealthStatus)
		}
		if a.PowerOnHours != 209 {
			t.Fatalf("power_on=%d", a.PowerOnHours)
		}
		if a.PowerCycleCount != 160 {
			t.Fatalf("power_cycle=%d", a.PowerCycleCount)
		}
		if a.ReallocatedSectors != 0 {
			t.Fatalf("realloc=%d", a.ReallocatedSectors)
		}
		if a.UncorrectableErrors != 0 {
			t.Fatalf("uncorr=%d", a.UncorrectableErrors)
		}
		if a.MediaWearoutValue != 99 {
			t.Fatalf("wearout=%d (want 99 normalized)", a.MediaWearoutValue)
		}
		if a.TempC != 36 {
			t.Fatalf("temp=%d", a.TempC)
		}
		// HDD 独有字段在 SSD 上应为 -1
		if a.ReadErrorCount != -1 {
			t.Fatalf("read_err=%d (want -1)", a.ReadErrorCount)
		}
		if a.PendingSectorReallocation != -1 {
			t.Fatalf("pending=%d (want -1)", a.PendingSectorReallocation)
		}
	})

	t.Run("sata_hdd", func(t *testing.T) {
		a := parseSMARTAttrs(sataHDD)
		if a.HealthStatus != "OK" {
			t.Fatalf("health=%q", a.HealthStatus)
		}
		if a.PowerOnHours != 34 {
			t.Fatalf("power_on=%d", a.PowerOnHours)
		}
		if a.PowerCycleCount != 192 {
			t.Fatalf("power_cycle=%d", a.PowerCycleCount)
		}
		if a.ReallocatedSectors != 0 {
			t.Fatalf("realloc=%d", a.ReallocatedSectors)
		}
		if a.UncorrectableErrors != 0 { // 来自 "Uncorrectable Sector Count"
			t.Fatalf("uncorr=%d", a.UncorrectableErrors)
		}
		if a.ReadErrorCount != 0 {
			t.Fatalf("read_err=%d", a.ReadErrorCount)
		}
		if a.PendingSectorReallocation != 0 {
			t.Fatalf("pending=%d", a.PendingSectorReallocation)
		}
		if a.TempC != 49 {
			t.Fatalf("temp=%d", a.TempC)
		}
		// SSD 独有
		if a.MediaWearoutValue != -1 {
			t.Fatalf("wearout=%d (want -1)", a.MediaWearoutValue)
		}
	})

	t.Run("nvme_raw_fallback_to_value", func(t *testing.T) {
		a := parseSMARTAttrs(nvme)
		if a.HealthStatus != "OK" {
			t.Fatalf("health=%q", a.HealthStatus)
		}
		// Raw=N/A,必须回退到 Value
		if a.PowerOnHours != 15296 {
			t.Fatalf("power_on=%d (raw N/A 必须回退 Value=15296)", a.PowerOnHours)
		}
		if a.PowerCycleCount != 125 {
			t.Fatalf("power_cycle=%d", a.PowerCycleCount)
		}
		if a.TempC != 47 {
			t.Fatalf("temp=%d", a.TempC)
		}
		// NVMe 没暴露的字段应为 -1
		if a.UncorrectableErrors != -1 {
			t.Fatalf("uncorr=%d (want -1)", a.UncorrectableErrors)
		}
		if a.MediaWearoutValue != -1 {
			t.Fatalf("wearout=%d (want -1)", a.MediaWearoutValue)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		a := parseSMARTAttrs("")
		if a.TempC != -1 || a.PowerOnHours != -1 || a.HealthStatus != "" {
			t.Fatalf("empty input not all defaults: %+v", a)
		}
	})
}

// renderVsishBuffer 把 [512]byte 还原成 vsish 输出格式(每行 `[N]: 0xXX`),
// 用于在测试里合成 ATA SMART data buffer。
func renderVsishBuffer(buf [512]byte) string {
	var sb strings.Builder
	for i, b := range buf {
		fmt.Fprintf(&sb, "[%d]: 0x%02x \n", i, b)
	}
	return sb.String()
}

// putAttr 在 buf 的指定 attribute 槽位写入 id + 6 字节 LE raw。
// slot 取值 0..29,对应 ATA SMART data 第 1..30 个 entry。
func putAttr(buf *[512]byte, slot int, id byte, raw int64) {
	off := 2 + slot*12
	buf[off] = id
	for j := 0; j < 6; j++ {
		buf[off+5+j] = byte(raw >> (8 * j))
	}
}

func TestParseATASMARTBuffer(t *testing.T) {
	t.Run("decodes_full_6byte_raw", func(t *testing.T) {
		// 关键场景:Power-on Hours 真值 30425 (0x76D9),低 1 字节 0xD9=217,
		// 这正是 esxcli 截断时会显示的错值。vsish 必须给出 30425。
		var buf [512]byte
		buf[0] = 0x01 // revision lo
		buf[1] = 0x00
		putAttr(&buf, 0, 5, 0)     // Reallocated Sector Count
		putAttr(&buf, 1, 9, 30425) // Power-on Hours
		putAttr(&buf, 2, 12, 160)  // Power Cycle Count
		putAttr(&buf, 3, 197, 0)   // Pending Sector Reallocation
		putAttr(&buf, 4, 198, 0)   // Offline Uncorrectable
		// 故意塞个无关 attribute,确认不影响关注字段
		putAttr(&buf, 5, 194, 35) // Temperature (不在覆盖列表)

		attrs, ok := parseATASMARTBuffer(renderVsishBuffer(buf))
		if !ok {
			t.Fatalf("parse failed (got<50?)")
		}
		if attrs.PowerOnHours != 30425 {
			t.Fatalf("power-on hours = %d (want 30425)", attrs.PowerOnHours)
		}
		if attrs.PowerCycleCount != 160 {
			t.Fatalf("power cycle = %d (want 160)", attrs.PowerCycleCount)
		}
		if attrs.ReallocatedSectors != 0 {
			t.Fatalf("reallocated = %d (want 0)", attrs.ReallocatedSectors)
		}
		if attrs.PendingSectorReallocation != 0 {
			t.Fatalf("pending = %d (want 0)", attrs.PendingSectorReallocation)
		}
		if attrs.UncorrectableErrors != 0 {
			t.Fatalf("uncorr = %d (want 0)", attrs.UncorrectableErrors)
		}
	})

	t.Run("nvme_not_supported", func(t *testing.T) {
		attrs, ok := parseATASMARTBuffer("VSISHCmdGetInt():Get failed: Not supported\n")
		if ok {
			t.Fatalf("expected parse to bail on Not supported, got %+v", attrs)
		}
		if attrs.PowerOnHours != -1 {
			t.Fatalf("attrs not reset to -1 on failure: %+v", attrs)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		_, ok := parseATASMARTBuffer("")
		if ok {
			t.Fatalf("expected !ok for empty input")
		}
	})
}

func TestParseHostBoot(t *testing.T) {
	t.Run("with_zdump", func(t *testing.T) {
		out := `UPTIME_US=123584921319
NOW_EPOCH=1781145786
ZDUMP_COUNT=2
ZDUMP_LATEST=1781022240
`
		b := parseHostBoot(out)
		if b.UptimeSeconds != 123584 {
			t.Fatalf("uptime = %d (want 123584)", b.UptimeSeconds)
		}
		// 1781145786 - 123584 = 1781022202
		if b.BootedAt.Unix() != 1781022202 {
			t.Fatalf("booted_at unix = %d (want 1781022202)", b.BootedAt.Unix())
		}
		if b.CrashDumpCount != 2 {
			t.Fatalf("crash count = %d (want 2)", b.CrashDumpCount)
		}
		if b.LastCrashAt.Unix() != 1781022240 {
			t.Fatalf("last_crash_at unix = %d (want 1781022240)", b.LastCrashAt.Unix())
		}
	})

	t.Run("no_zdump", func(t *testing.T) {
		out := `UPTIME_US=42000000
NOW_EPOCH=1781145786
ZDUMP_COUNT=0
ZDUMP_LATEST=
`
		b := parseHostBoot(out)
		if b.UptimeSeconds != 42 {
			t.Fatalf("uptime = %d (want 42)", b.UptimeSeconds)
		}
		if b.CrashDumpCount != 0 {
			t.Fatalf("crash count = %d (want 0)", b.CrashDumpCount)
		}
		if !b.LastCrashAt.IsZero() {
			t.Fatalf("last_crash_at should be zero, got %v", b.LastCrashAt)
		}
	})

	t.Run("empty_output_keeps_defaults", func(t *testing.T) {
		b := parseHostBoot("")
		if b.UptimeSeconds != -1 {
			t.Fatalf("expected uptime=-1 on empty, got %d", b.UptimeSeconds)
		}
	})
}

func TestParseNICList(t *testing.T) {
	out := `Name    PCI Device    Driver         Admin Status  Link Status  Speed  Duplex  MAC Address         MTU  Description
------  ------------  -------------  ------------  -----------  -----  ------  -----------------  ----  -----------
vmnic0  0000:00:1f.6  ne1000         Up            Up            1000  Full    d8:bb:c1:c1:b6:49  1500  Intel Corporation Ethernet Connection (7) I219-LM
vmnic1  0000:01:00.0  igc-community  Up            Up            2500  Full    1c:86:0b:2c:3f:1b  1500  Intel Corporation Ethernet Controller I226-V
`
	nics := parseNICList(out)
	if len(nics) != 2 {
		t.Fatalf("want 2 nics, got %d", len(nics))
	}
	if nics[0].Name != "vmnic0" || nics[0].Driver != "ne1000" || nics[0].SpeedMbps != 1000 || nics[0].MTU != 1500 {
		t.Fatalf("nic0 = %+v", nics[0])
	}
	if nics[0].Description != "Intel Corporation Ethernet Connection (7) I219-LM" {
		t.Fatalf("nic0 desc = %q", nics[0].Description)
	}
	if nics[1].SpeedMbps != 2500 || nics[1].MAC != "1c:86:0b:2c:3f:1b" {
		t.Fatalf("nic1 = %+v", nics[1])
	}
}

func TestParseIPInterfaceList(t *testing.T) {
	out := `vmk0
   Name: vmk0
   MAC Address: 00:50:56:6a:11:22
   Enabled: true
   Portset: vSwitch0
   Portgroup: Management Network
   Netstack Instance: defaultTcpipStack
   VDS Name: N/A

vmk1
   Name: vmk1
   MAC Address: 00:50:56:6a:33:44
   Enabled: true
   Portset: vSwitch0
   Portgroup: vMotion Network
   Netstack Instance: defaultTcpipStack
`
	got := parseIPInterfaceList(out)
	if len(got) != 2 {
		t.Fatalf("want 2 vmkernel ports, got %d: %#v", len(got), got)
	}
	if got[0].Name != "vmk0" || got[0].VSwitch != "vSwitch0" || got[0].Portgroup != "Management Network" || got[0].MAC != "00:50:56:6a:11:22" || !got[0].Enabled {
		t.Fatalf("vmk0 = %+v", got[0])
	}
	if got[1].Name != "vmk1" || got[1].Portgroup != "vMotion Network" || !got[1].Enabled {
		t.Fatalf("vmk1 = %+v", got[1])
	}
}

func TestParseIPv4Get(t *testing.T) {
	out := `Name  IPv4 Address   IPv4 Netmask   IPv4 Broadcast  Address Type  Gateway      DHCP DNS
----  -------------  -------------  --------------  ------------  -----------  --------
vmk0  192.168.8.138  255.255.255.0  192.168.8.255   STATIC        192.168.8.1  false
`
	if got := parseIPv4Get(out, "vmk0"); got != "192.168.8.138" {
		t.Fatalf("ipv4 = %q", got)
	}
}

func TestParseVMPortListIncludesIP(t *testing.T) {
	out := `Port ID: 33554442
vSwitch: vSwitch0
Portgroup: VM Network
MAC Address: 00:0c:29:86:f2:a0
IP Address: 192.168.8.21
Team Uplink: vmnic0

Port ID: 33554443
vSwitch: vSwitch0
Portgroup: VM Network
MAC Address: 00:0c:29:72:3f:e3
IP Address: 0.0.0.0
Team Uplink: vmnic0
`
	got := parseVMPortList(out)
	if len(got) != 2 {
		t.Fatalf("links = %#v", got)
	}
	if got[0].IP != "192.168.8.21" {
		t.Fatalf("first ip = %q", got[0].IP)
	}
	if got[1].IP != "" {
		t.Fatalf("0.0.0.0 should be ignored, got %q", got[1].IP)
	}
}

func TestParseBatchVMGuestNICs(t *testing.T) {
	out := `===VM===130
(vim.vm.GuestInfo) {
   guestState = "running",
   net = (vim.vm.GuestInfo.NicInfo) [
      (vim.vm.GuestInfo.NicInfo) {
         network = "VM Network",
         ipAddress = (string) [
            "fe80::250:56ff:fe86:f2a0",
            "192.168.8.21"
         ],
         macAddress = "00:0c:29:86:f2:a0",
         connected = true
      }
   ],
}
===VM===131
(vim.vm.GuestInfo) {
   net = (vim.vm.GuestInfo.NicInfo) [
      (vim.vm.GuestInfo.NicInfo) {
         ipAddress = (string) [
            "10.0.0.5"
         ],
         macAddress = "00:0c:29:72:3f:e3",
      }
   ],
}
`
	got := parseBatchVMGuestNICs(out)
	if got[130][0].MAC != "00:0c:29:86:f2:a0" || got[130][0].IP != "192.168.8.21" {
		t.Fatalf("vm 130 guest nics = %#v", got[130])
	}
	if got[131][0].IP != "10.0.0.5" {
		t.Fatalf("vm 131 guest nics = %#v", got[131])
	}
}

func TestFillVMNICGuestIPs(t *testing.T) {
	links := []VMNICLink{
		{MAC: "00:0c:29:86:f2:a0"},
		{MAC: "00:0c:29:72:3f:e3", IP: "192.168.8.30"},
	}
	guest := []vmGuestNIC{
		{MAC: "00:0c:29:86:f2:a0", IP: "192.168.8.21"},
		{MAC: "00:0c:29:72:3f:e3", IP: "192.168.8.31"},
	}
	got := fillVMNICGuestIPs(links, guest)
	if got[0].IP != "192.168.8.21" {
		t.Fatalf("missing ip should be filled, got %#v", got[0])
	}
	if got[1].IP != "192.168.8.30" {
		t.Fatalf("existing ip should win, got %#v", got[1])
	}
}

func TestMergeTopologyFillsVMNICIP(t *testing.T) {
	base := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs: []VMNICLink{{
			VMName:    "fnOS",
			VSwitch:   "vSwitch0",
			Portgroup: "VM Network",
			MAC:       "00:0c:29:86:f2:a0",
		}},
	}
	next := NetTopology{
		VMNICs: []VMNICLink{{
			VMName:     "fnOS",
			VSwitch:    "vSwitch0",
			Portgroup:  "VM Network",
			MAC:        "00:0C:29:86:F2:A0",
			IP:         "192.168.8.21",
			TeamUplink: "vmnic0",
		}},
	}
	got := mergeTopology(base, next)
	if len(got.VMNICs) != 1 {
		t.Fatalf("vm_nics = %#v", got.VMNICs)
	}
	if got.VMNICs[0].IP != "192.168.8.21" || got.VMNICs[0].TeamUplink != "vmnic0" {
		t.Fatalf("merged vmnic = %#v", got.VMNICs[0])
	}
}

func TestTopologyCompleteDoesNotRequireVMKPorts(t *testing.T) {
	m := HostMetrics{
		VMs: []VM{{Name: "fnOS", State: "powered_on"}},
		Topology: NetTopology{
			VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
			VMNICs:    []VMNICLink{{VMName: "fnOS", MAC: "00:0c:29:00:00:01"}},
		},
	}
	if !topologyComplete(m) {
		t.Fatal("topology should be complete when VM vNICs are collected, even if VMkernel ports are absent")
	}
}

func TestTopologyFullyCollectedRequiresAllTopologyParts(t *testing.T) {
	m := HostMetrics{
		VMs: []VM{{Name: "fnOS", State: "powered_on"}},
		Topology: NetTopology{
			VSwitches:        []VSwitchInfo{{Name: "vSwitch0"}},
			VMNICs:           []VMNICLink{{VMName: "fnOS", MAC: "00:0c:29:00:00:01"}},
			Collected:        true,
			VSwitchCollected: true,
			VMNetCollected:   true,
			VMPortsCollected: true,
			VMKCollected:     true,
			VMKFullCollected: true,
		},
	}
	if !topologyFullyCollected(m) {
		t.Fatal("topology should be fully collected when every topology command succeeded")
	}
	m.Topology.VMKFullCollected = false
	if topologyFullyCollected(m) {
		t.Fatal("topology should not be fully collected when vmkernel ipv4 fetch is incomplete")
	}
}

func TestMergeTopologyMergesVMKPorts(t *testing.T) {
	base := NetTopology{
		VMKPorts: []VMKPort{{
			Name:      "vmk0",
			VSwitch:   "vSwitch0",
			Portgroup: "Management Network",
			MAC:       "00:50:56:6a:11:22",
			Enabled:   true,
		}},
	}
	next := NetTopology{
		VMKCollected: true,
		VMKPorts: []VMKPort{{
			Name:      "vmk0",
			VSwitch:   "vSwitch0",
			Portgroup: "Management Network",
			MAC:       "00:50:56:6a:11:22",
			IPv4:      "192.168.8.138",
			Enabled:   true,
		}},
	}
	got := mergeTopology(base, next)
	if len(got.VMKPorts) != 1 {
		t.Fatalf("vmk ports = %#v", got.VMKPorts)
	}
	vmk := got.VMKPorts[0]
	if vmk.VSwitch != "vSwitch0" || vmk.Portgroup != "Management Network" || vmk.MAC == "" || vmk.IPv4 != "192.168.8.138" || !vmk.Enabled {
		t.Fatalf("merged vmk = %+v", vmk)
	}
}

func TestMergeTopologySkipsVMKPortsWhenCollectionFailed(t *testing.T) {
	base := NetTopology{
		VMKCollected: true,
		VMKPorts: []VMKPort{{
			Name:      "vmk0",
			VSwitch:   "vSwitch0",
			Portgroup: "Management Network",
			IPv4:      "192.168.8.138",
			Enabled:   true,
		}},
	}
	next := NetTopology{
		VSwitches: []VSwitchInfo{{Name: "vSwitch0"}},
		VMNICs:    []VMNICLink{{VMName: "fnOS", MAC: "00:0c:29:86:f2:a0"}},
	}
	got := mergeTopology(base, next)
	if len(got.VMKPorts) != 1 || got.VMKPorts[0].IPv4 != "192.168.8.138" {
		t.Fatalf("vmk ports should remain from previous collected attempt: %#v", got.VMKPorts)
	}
	if !got.VMKCollected {
		t.Fatal("vmk collected flag should remain true from base")
	}
}

func TestFillNICStats(t *testing.T) {
	out := `NIC statistics for vmnic0
   Packets received: 638456
   Packets sent: 516978
   Bytes received: 614164771
   Bytes sent: 64981576
   Receive packets dropped: 0
   Transmit packets dropped: 0
   Total receive errors: 0
   Total transmit errors: 0
`
	n := newNIC()
	fillNICStats(&n, out)
	if n.RxBytes != 614164771 || n.TxBytes != 64981576 {
		t.Fatalf("bytes = %d / %d", n.RxBytes, n.TxBytes)
	}
	if n.RxErrors != 0 || n.TxErrors != 0 {
		t.Fatalf("errors should be 0, got %d / %d", n.RxErrors, n.TxErrors)
	}
	if n.RxDropped != 0 || n.TxDropped != 0 {
		t.Fatalf("dropped should be 0, got %d / %d", n.RxDropped, n.TxDropped)
	}
}
