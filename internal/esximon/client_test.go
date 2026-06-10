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
