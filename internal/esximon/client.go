// Package esximon 通过 SSH 远程执行 ESXi 的 esxcli / vsish / vim-cmd 命令,
// 采集平台、CPU 温度、磁盘 SMART、MCE、USB、VM 等信息。机器来源 esxi_host 表,
// 凭证来源 esxi_ssh_credential,与 UPS / ACME 完全解耦。
//
// 数据采集策略:每轮 SSH 连接复用,**串行**地跑 N 条命令(esxcli/vsish 都很轻),
// 不在远端拼 JSON —— 直接把每条命令的纯文本拉回本地用 Go 解析,避开 busybox
// 转义陷阱。详见 prompt/6_ESXI_SSH_MONITORING.md。
package esximon
