// Package modulecontrol 管理单进程内业务模块的持久化开关、执行准入和状态订阅。
package modulecontrol

type Definition struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Group       string   `json:"group"`
	Description string   `json:"description"`
	Impact      string   `json:"impact"`
	Hint        string   `json:"hint,omitempty"`
	Jobs        []string `json:"jobs"`
}

var definitions = []Definition{
	{"ups", "UPS 状态", "设备监控", "市电、电池、低电状态与历史曲线", "暂停采样、告警和历史清理，关闭监控页面与接口。", "重新启用后从下一轮采样恢复，保留原有设备设置。", []string{"ups-sample", "ups-cleanup"}},
	{"esxi", "ESXi 状态", "设备监控", "主机资源、硬件健康与网络拓扑", "暂停采样、告警和历史清理，关闭监控页面与接口。", "重新采样并建立告警基线，停用期间不会补采数据。", []string{"esxi-sample", "esxi-cleanup"}},
	{"acme", "证书签发", "证书与云服务", "ACME 签发、自动续期与多目标部署", "暂停新签发、续期、部署和重试，关闭证书签发页面与接口。", "已开始的操作安全收尾；排队任务暂停，重新启用后可手动重试。", []string{"acme-renew", "acme-deploy-retry"}},
	{"certstore", "CAS 证书", "证书与云服务", "阿里云证书库存与部署到 CDN", "关闭证书库存查看、删除和从此模块发起的 CDN 部署。", "保留云端证书；ACME 自己的 CAS 上传由证书签发模块控制。", []string{}},
	{"cdnops", "CDN 域名", "证书与云服务", "阿里云 CDN 加速域名只读查询", "关闭域名查询页面与接口。", "仅控制域名查询，不影响 CAS 模块的证书部署。", []string{}},
	{"birthday", "生日提醒", "提醒与消息", "农历生日与提前提醒", "暂停自动提醒，关闭名单管理与手动推送。", "保留名单，恢复后按正常提醒日期运行。", []string{"birthday"}},
	{"event", "事项提醒", "提醒与消息", "续费、体检与办事截止日期", "暂停自动提醒，关闭事项管理与手动推送。", "保留日期、提前天数和去重记录。", []string{"event"}},
	{"sms", "短信转发", "提醒与消息", "SmsForwarder 设备与短信收发", "关闭设备管理、短信查询和发送接口。", "手机上的 SmsForwarder 仍独立运行。", []string{}},
	{"bypass", "12306Bypass 中转", "提醒与消息", "分流抢票助手通知转发", "停止处理和转发 webhook，向调用方返回模块已禁用。", "通知渠道和绑定关系继续保留。", []string{}},
}

func Known(key string) bool {
	for _, d := range definitions {
		if d.Key == key {
			return true
		}
	}
	return false
}

func JobModule(name string) string {
	for _, d := range definitions {
		for _, job := range d.Jobs {
			if job == name {
				return d.Key
			}
		}
	}
	return ""
}
