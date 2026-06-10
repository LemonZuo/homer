// Package sshlike 提供模块无关的 SSH 目标抽象:Target/Credential/Bastion/Conn。
// 不导入 internal/acme | internal/upsmon | internal/esximon | internal/model,
// 由各模块自己写一个适配器把 GORM 行转成 sshlike.Target、提供 BastionLoader 闭包、
// 在凭证库上挂 CredentialResolver 实现。
package sshlike
