package sshlike

import (
	"errors"
	"fmt"
)

// BastionLoader 用闭包注入,sshlike 不知道 bastion 落在哪张表。
// 返回的 *Target 必须是已经 Normalize 过的(各模块适配器内部走 ParseXxxTarget)。
type BastionLoader func(id int64) (*Target, error)

// UpstreamFinder 反查"有没有人把这个 id 当 bastion 用",
// 返回第一个引用者的 name 用于错误消息。
type UpstreamFinder func(id int64) (string, bool, error)

// ValidateBastion 校验 Target 的 bastion 设置:
//   - bastion_id <= 0: 没设跳板机,直接通过
//   - bastion 不能是自己
//   - 本机若已被人引用作跳板机,不能再为自己设跳板机(避免成链)
//   - bastion 自身不能再有 bastion(单跳约束)
//
// selfLabel 进错误消息,如 "当前实例" / "本机"。
func ValidateBastion(loader BastionLoader, finder UpstreamFinder, t Target, selfLabel string) error {
	if t.BastionID <= 0 {
		return nil
	}
	if t.BastionID == t.ID {
		return errors.New("跳板机不能是自己")
	}
	if t.ID > 0 && finder != nil {
		name, ok, err := finder(t.ID)
		if err != nil {
			return err
		}
		if ok {
			if selfLabel == "" {
				selfLabel = "本机"
			}
			return fmt.Errorf("%s已被 %s 设为跳板机，不能再为自己设置跳板机", selfLabel, name)
		}
	}
	if loader == nil {
		return errors.New("跳板机模式未注入 BastionLoader")
	}
	b, err := loader(t.BastionID)
	if err != nil {
		return err
	}
	if b.BastionID > 0 {
		return errors.New("所选跳板机已经设置了自己的跳板机，单跳模式不支持跳板机链")
	}
	return nil
}
