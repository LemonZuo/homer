package email

import "testing"

func TestResendEnabled(t *testing.T) {
	var nilClient *ResendClient
	if nilClient.Enabled() {
		t.Fatal("nil client must be disabled")
	}
	if NewResend("", "from@x.com").Enabled() {
		t.Fatal("missing apiKey must be disabled")
	}
	if NewResend("re_key", "").Enabled() {
		t.Fatal("missing from must be disabled")
	}
	if !NewResend("re_key", "from@x.com").Enabled() {
		t.Fatal("both set must be enabled")
	}
}

func TestSendTextNotConfigured(t *testing.T) {
	if err := NewResend("", "").SendText("to@x.com", "s", "t"); err == nil {
		t.Fatal("unconfigured SendText must error without HTTP call")
	}
}
