package esximon

import "testing"

func TestParseUSBControllers(t *testing.T) {
	out := `0000:00:14.0 USB controller: Intel Cannon Lake PCH USB 3.1 xHCI Host Controller
some unrelated line
0000:02:00.0 USB controller: ASMedia ASM3142 USB 3.1 Host Controller`
	got := parseUSBControllers(out)
	if len(got) != 2 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	if got[0].PCIAddr != "0000:00:14.0" || got[0].Name != "Intel Cannon Lake PCH USB 3.1 xHCI Host Controller" {
		t.Fatalf("first = %+v", got[0])
	}
	if parseUSBControllers("nothing here") != nil {
		t.Fatal("no match must yield nil")
	}
}

func TestParseUSBPassthrough(t *testing.T) {
	out := `Bus  Dev  VendorId  ProductId  Enabled  Can Connect to VM  Name
---  ---  --------  ---------  -------  -----------------  ----
  1    4  764       501        true     yes                Cyber Power System UT1050EGC
  1    5  152d      a576       false    no                 JMicron JMS576
`
	got := parseUSBPassthrough(out)
	if len(got) != 2 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	d := got[0]
	if d.Bus != 1 || d.Dev != 4 || d.VID != "764" || d.PID != "501" || !d.Enabled {
		t.Fatalf("first = %+v", d)
	}
	// 名字含空格,拼回完整串
	if d.Name != "Cyber Power System UT1050EGC" {
		t.Fatalf("name = %q", d.Name)
	}
	if got[1].Enabled {
		t.Fatal("second must be disabled")
	}
}

func TestParseBatchVMDevices(t *testing.T) {
	out := "===VM===3\ndevice A\nline2\n===VM===7\r\ndevice B\n===VM===bad\nignored\n"
	got := parseBatchVMDevices(out)
	if len(got) != 2 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	if got[3] != "device A\nline2\n" {
		t.Fatalf("vm3 = %q", got[3])
	}
	// \r 结尾的标记行也能解析
	if got[7] != "device B\n" {
		t.Fatalf("vm7 = %q", got[7])
	}
	// 空输入
	if len(parseBatchVMDevices("")) != 0 {
		t.Fatal("empty input must yield empty map")
	}
}
