package acmealicas

import (
	"strings"
	"testing"

	"github.com/LemonZuo/homer/internal/model"
)

func TestValidateTarget(t *testing.T) {
	d := NewDriver()
	// 完整配置 + 字段两侧空白会被 normalize 掉
	ok := model.ACMEDeployTarget{
		ID:       1,
		Name:     "  cas  ",
		AuthJSON: `{"access_key_id":" AK ","access_key_secret":" SK "}`,
	}
	if err := d.ValidateTarget(ok); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		tgt  model.ACMEDeployTarget
		want string
	}{
		{"empty name", model.ACMEDeployTarget{AuthJSON: `{"access_key_id":"a","access_key_secret":"s"}`}, "名称"},
		{"missing ak", model.ACMEDeployTarget{Name: "n", AuthJSON: `{"access_key_secret":"s"}`}, "AccessKeyId"},
		{"missing sk", model.ACMEDeployTarget{Name: "n", AuthJSON: `{"access_key_id":"a"}`}, "AccessKeySecret"},
		{"bad json", model.ACMEDeployTarget{Name: "n", AuthJSON: `{bad`}, "解析"},
	}
	for _, c := range cases {
		if err := d.ValidateTarget(c.tgt); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want contains %q", c.name, err, c.want)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	d := NewDriver()
	// state_json 为空 → 按 {} 处理不报错
	if err := d.ValidateConfig(model.ACMEDeployTarget{}, model.ACMEDeployConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := d.ValidateConfig(model.ACMEDeployTarget{}, model.ACMEDeployConfig{StateJSON: `{"cert_id":7}`}); err != nil {
		t.Fatal(err)
	}
	if err := d.ValidateConfig(model.ACMEDeployTarget{}, model.ACMEDeployConfig{StateJSON: `{bad`}); err == nil {
		t.Fatal("bad state json must error")
	}
}
