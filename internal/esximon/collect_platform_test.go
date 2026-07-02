package esximon

import "testing"

func TestParsePlatform(t *testing.T) {
	out := `
   Vendor Name: LENOVO
   Product Name: ThinkStation P340
   Serial Number: PC1ABCDE
   UUID: 01234567-89ab-cdef-0123-456789abcdef
   IPMI Supported: false
`
	p := parsePlatform(out)
	if p.Vendor != "LENOVO" || p.Product != "ThinkStation P340" || p.Serial != "PC1ABCDE" {
		t.Fatalf("platform = %+v", p)
	}
	if p.IPMISupported {
		t.Fatal("ipmi must be false")
	}

	// 字段名变体兜底:Vendor / Product
	p2 := parsePlatform("Vendor: Dell\nProduct: R730")
	if p2.Vendor != "Dell" || p2.Product != "R730" {
		t.Fatalf("fallback = %+v", p2)
	}
}

func TestParseVersionInto(t *testing.T) {
	var p PlatformInfo
	parseVersionInto(&p, "VMware ESXi 7.0.3 build-24784741")
	if p.ESXiVersion != "7.0.3" || p.ESXiBuild != 24784741 {
		t.Fatalf("version = %+v", p)
	}
	// 不匹配时不动字段
	p2 := PlatformInfo{ESXiVersion: "keep"}
	parseVersionInto(&p2, "garbage output")
	if p2.ESXiVersion != "keep" || p2.ESXiBuild != 0 {
		t.Fatalf("unmatched = %+v", p2)
	}
}

func TestParseCPUStatic(t *testing.T) {
	// L2/L3 Cache Size 单位是 byte;Core Speed 是 Hz
	out := `
   Brand: GenuineIntel
   Family: 6
   Model: 158
   Stepping: 13
   Core Speed: 3792874464
   L2 Cache Size: 262144
   L2 Cache Line Size: 64
   L3 Cache Size: 8388608
   Number of CPU Cores: 4
`
	c := parseCPUStatic(out)
	if c.Brand != "GenuineIntel" || c.Family != 6 || c.ModelID != 158 || c.Stepping != 13 {
		t.Fatalf("cpu = %+v", c)
	}
	if c.FreqMHz != 3792 {
		t.Fatalf("freq = %d", c.FreqMHz)
	}
	if c.L2KB != 256 || c.L3KB != 8192 {
		t.Fatalf("cache = L2 %d / L3 %d", c.L2KB, c.L3KB)
	}
	if c.Cores != 4 {
		t.Fatalf("cores = %d", c.Cores)
	}
	if c.TjMaxC != -1 {
		t.Fatalf("tjmax default = %d", c.TjMaxC)
	}
}

func TestFillCoresFromGlobal(t *testing.T) {
	c := CPUStatic{Cores: 0}
	fillCoresFromGlobal(&c, "   CPU Packages: 1\n   CPU Cores: 4\n   CPU Threads: 4")
	if c.Cores != 4 {
		t.Fatalf("cores = %d", c.Cores)
	}
	// 无效值不覆盖
	c2 := CPUStatic{Cores: 8}
	fillCoresFromGlobal(&c2, "CPU Cores: 0")
	if c2.Cores != 8 {
		t.Fatalf("cores overwritten = %d", c2.Cores)
	}
}

func TestFillFromSmbios(t *testing.T) {
	out := `Processor Info (Type 4): #74
  Socket: "U3E1"
  Version: "Intel(R) Xeon(R) E-2244G CPU @ 3.80GHz"
  Current Speed: 3800 MHz
  Core Count: 4
Cache Info (Type 7): #75
  Version: "should not reach here"
`
	c := CPUStatic{Brand: "GenuineIntel", FreqMHz: 100, Cores: 1}
	fillFromSmbios(&c, out)
	if c.Brand != "Intel(R) Xeon(R) E-2244G CPU @ 3.80GHz" {
		t.Fatalf("brand = %q", c.Brand)
	}
	if c.FreqMHz != 3800 || c.Cores != 4 {
		t.Fatalf("freq/cores = %d/%d", c.FreqMHz, c.Cores)
	}
}

func TestFillFromSmbiosOEMPlaceholderIgnored(t *testing.T) {
	out := `Processor Info (Type 4): #74
  Version: "To Be Filled By O.E.M."
  Core Count: 6
`
	c := CPUStatic{Brand: "keep"}
	fillFromSmbios(&c, out)
	if c.Brand != "keep" {
		t.Fatalf("OEM placeholder must not overwrite brand: %q", c.Brand)
	}
	if c.Cores != 6 {
		t.Fatalf("cores = %d", c.Cores)
	}
}

func TestFillFromSmbiosNoBlock(t *testing.T) {
	c := CPUStatic{Brand: "keep", Cores: 2}
	fillFromSmbios(&c, "no processor info here")
	if c.Brand != "keep" || c.Cores != 2 {
		t.Fatalf("must stay unchanged: %+v", c)
	}
}

func TestDecodeTjMax(t *testing.T) {
	// MSR 0x1A2 bits 23:16;0x640000 >> 16 = 0x64 = 100
	cases := []struct {
		in   string
		want int
	}{
		{"0x640000", 100},
		{"6553600", 100},
		{" 0x5A0000 ", 90},
		{"", -1},
		{"garbage", -1},
	}
	for _, c := range cases {
		if got := decodeTjMax(c.in); got != c.want {
			t.Errorf("decodeTjMax(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseMemory(t *testing.T) {
	m := parseMemory("   Physical Memory: 137262243840 Bytes\n   Reliable Memory: 0 Bytes")
	if m.TotalBytes != 137262243840 {
		t.Fatalf("total = %d", m.TotalBytes)
	}
	if m2 := parseMemory("no memory field"); m2.TotalBytes != 0 {
		t.Fatalf("empty = %d", m2.TotalBytes)
	}
}

func TestFillMemoryFromVsish(t *testing.T) {
	out := `Comprehensive {
   Physical memory estimate:134045160 KB
   Free:75142516 KB
}`
	// esxcli 已有 total → 只补 free
	m := MemoryInfo{TotalBytes: 999}
	fillMemoryFromVsish(&m, out)
	if m.FreeBytes != 75142516*1024 {
		t.Fatalf("free = %d", m.FreeBytes)
	}
	if m.TotalBytes != 999 {
		t.Fatalf("total must not be overwritten: %d", m.TotalBytes)
	}
	// esxcli 没拿到 total → 用 vsish 兜底
	var m2 MemoryInfo
	fillMemoryFromVsish(&m2, out)
	if m2.TotalBytes != 134045160*1024 {
		t.Fatalf("total fallback = %d", m2.TotalBytes)
	}
}
