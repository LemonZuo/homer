package esximon

import "testing"

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
