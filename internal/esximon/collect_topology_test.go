package esximon

import "testing"

func TestParseVMNetList(t *testing.T) {
	out := `World ID  Name                            Num Ports  Networks
--------  ------------------------------  ---------  --------
 2100697  fnOS                                     1  VM Network
 2100710  Windows Server 2022, with GPU            2  VM Network, Internal
`
	got := parseVMNetList(out)
	if len(got) != 2 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	if got[0].worldID != 2100697 || got[0].name != "fnOS" {
		t.Fatalf("first = %+v", got[0])
	}
	// 名字含空格和逗号,靠"从后往前第一个不含逗号的纯数字"锚定 Num Ports
	if got[1].worldID != 2100710 || got[1].name != "Windows Server 2022, with GPU" {
		t.Fatalf("second = %+v", got[1])
	}
}

func TestParseVMNetListSkipsGarbage(t *testing.T) {
	out := "not a number here at all\n12345\n"
	if got := parseVMNetList(out); got != nil {
		t.Fatalf("garbage must yield nil: %v", got)
	}
}

func TestParseVSwitchList(t *testing.T) {
	out := `vSwitch0
   Name: vSwitch0
   Class: cswitch
   Uplinks: vmnic0, vmnic1
   Portgroups: VM Network, Management Network

vSwitch1
   Uplinks:
   Portgroups: Internal
`
	got := parseVSwitchList(out)
	if len(got) != 2 {
		t.Fatalf("len = %d: %v", len(got), got)
	}
	v0 := got[0]
	if v0.Name != "vSwitch0" || len(v0.Uplinks) != 2 || v0.Uplinks[1] != "vmnic1" {
		t.Fatalf("vSwitch0 = %+v", v0)
	}
	if len(v0.Portgroups) != 2 || v0.Portgroups[0] != "VM Network" {
		t.Fatalf("portgroups = %v", v0.Portgroups)
	}
	// 无上联的内部交换机
	v1 := got[1]
	if v1.Name != "vSwitch1" || len(v1.Uplinks) != 0 || len(v1.Portgroups) != 1 {
		t.Fatalf("vSwitch1 = %+v", v1)
	}
}

func TestVMNetListCovers(t *testing.T) {
	entries := []vmNetEntry{{worldID: 1, name: "a"}, {worldID: 2, name: "b"}}
	if !vmNetListCovers(entries, nil) {
		t.Fatal("no expectation must be covered")
	}
	if !vmNetListCovers(entries, []string{"a", "b"}) {
		t.Fatal("all present must be covered")
	}
	if vmNetListCovers(entries, []string{"a", "c"}) {
		t.Fatal("missing vm must not be covered")
	}
}
