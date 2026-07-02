package upsmon

import (
	"testing"

	"github.com/LemonZuo/homer/internal/model"
)

func TestStateKeyAndHostKey(t *testing.T) {
	if stateKey("ups", 3, "apc") != "ups/3/apc" {
		t.Fatalf("stateKey = %q", stateKey("ups", 3, "apc"))
	}
	if hostKey("ups", 3) != "ups/3" {
		t.Fatalf("hostKey = %q", hostKey("ups", 3))
	}
	// UPS 名含 "/" 时 key 仍应唯一可分(靠前缀区分,不冲突即可)
	a := stateKey("ups", 1, "a/b")
	b := stateKey("ups", 1, "a") + "/b"
	if a != b {
		t.Fatalf("expected identical string %q vs %q", a, b)
	}
}

func TestIndexStates(t *testing.T) {
	states := []model.UPSState{
		{HostKind: "ups", HostID: 1, UPSName: "apc"},
		{HostKind: "ups", HostID: 2, UPSName: "cyber"},
	}
	idx := indexStates(states)
	if len(idx) != 2 {
		t.Fatalf("len = %d", len(idx))
	}
	if got, ok := idx["ups/1/apc"]; !ok || got.UPSName != "apc" {
		t.Fatalf("missing ups/1/apc: %+v", idx)
	}
	if got, ok := idx["ups/2/cyber"]; !ok || got.HostID != 2 {
		t.Fatalf("missing ups/2/cyber: %+v", idx)
	}
	// 空输入
	if len(indexStates(nil)) != 0 {
		t.Fatal("nil input must yield empty map")
	}
}
