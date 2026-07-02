package aliyun

import "testing"

// AK/SK 任一为空 → (nil, nil):上层据此判「未配置」,不能误报错误。
func TestNewClientsUnconfigured(t *testing.T) {
	for _, c := range []struct{ ak, sk string }{
		{"", ""}, {"ak", ""}, {"", "sk"},
	} {
		if got, err := NewCASClient(c.ak, c.sk); got != nil || err != nil {
			t.Fatalf("NewCASClient(%q,%q) = %v, %v", c.ak, c.sk, got, err)
		}
		if got, err := NewCDNClient(c.ak, c.sk); got != nil || err != nil {
			t.Fatalf("NewCDNClient(%q,%q) = %v, %v", c.ak, c.sk, got, err)
		}
	}
}

func TestNewClientsConfigured(t *testing.T) {
	// 只构造客户端,不发请求
	if got, err := NewCASClient("ak", "sk"); got == nil || err != nil {
		t.Fatalf("CAS = %v, %v", got, err)
	}
	if got, err := NewCDNClient("ak", "sk"); got == nil || err != nil {
		t.Fatalf("CDN = %v, %v", got, err)
	}
}
