package config

import "testing"

func TestEnvInt(t *testing.T) {
	t.Setenv("X_INT", "42")
	if got := envInt("X_INT", 7); got != 42 {
		t.Fatalf("got %d", got)
	}
	// 缺省 / 非法 / 非正数 → 默认值
	if got := envInt("X_INT_MISSING", 7); got != 7 {
		t.Fatalf("missing = %d", got)
	}
	t.Setenv("X_INT_BAD", "abc")
	if got := envInt("X_INT_BAD", 7); got != 7 {
		t.Fatalf("bad = %d", got)
	}
	t.Setenv("X_INT_ZERO", "0")
	if got := envInt("X_INT_ZERO", 7); got != 7 {
		t.Fatalf("zero must fall back, got %d", got)
	}
	t.Setenv("X_INT_NEG", "-3")
	if got := envInt("X_INT_NEG", 7); got != 7 {
		t.Fatalf("negative must fall back, got %d", got)
	}
	t.Setenv("X_INT_WS", "  15  ")
	if got := envInt("X_INT_WS", 7); got != 15 {
		t.Fatalf("whitespace trim = %d", got)
	}
}

func TestEnvIntAllowZero(t *testing.T) {
	// 与 envInt 的差别:0 是合法值(用于"关闭慢日志"这类语义)
	t.Setenv("X_AZ", "0")
	if got := envIntAllowZero("X_AZ", 1500); got != 0 {
		t.Fatalf("zero must be accepted, got %d", got)
	}
	t.Setenv("X_AZ_NEG", "-1")
	if got := envIntAllowZero("X_AZ_NEG", 1500); got != 1500 {
		t.Fatalf("negative must fall back, got %d", got)
	}
	if got := envIntAllowZero("X_AZ_MISSING", 1500); got != 1500 {
		t.Fatalf("missing = %d", got)
	}
}

func TestEnv(t *testing.T) {
	t.Setenv("X_STR", "value")
	if got := env("X_STR", "def"); got != "value" {
		t.Fatalf("got %q", got)
	}
	if got := env("X_STR_MISSING", "def"); got != "def" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizePort(t *testing.T) {
	cases := []struct{ in, want string }{
		{"8080", "8080"},
		{":8080", "8080"},
		{" :8080 ", "8080"},
		{"", "8081"},
		{"  ", "8081"},
		{":", "8081"},
	}
	for _, c := range cases {
		if got := normalizePort(c.in); got != c.want {
			t.Errorf("normalizePort(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDSNAndListenAddr(t *testing.T) {
	c := &Config{
		DBUser: "root", DBPassword: "pw", DBHost: "10.0.0.1", DBPort: "3306",
		DBName: "homer", DBCharset: "utf8mb4", ServerPort: ":9090",
	}
	want := "root:pw@tcp(10.0.0.1:3306)/homer?charset=utf8mb4&parseTime=True&loc=Local"
	if got := c.DSN(); got != want {
		t.Fatalf("DSN = %q", got)
	}
	if got := c.ListenAddr(); got != ":9090" {
		t.Fatalf("ListenAddr = %q", got)
	}
	if got := c.ListenURL(); got != "http://0.0.0.0:9090" {
		t.Fatalf("ListenURL = %q", got)
	}
}
