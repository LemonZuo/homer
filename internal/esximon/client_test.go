package esximon

import (
	"fmt"
	"strings"
	"testing"
)

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
