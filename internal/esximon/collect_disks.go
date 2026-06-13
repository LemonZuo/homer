package esximon

// ESXi 磁盘、容量和 SMART 采集。

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LemonZuo/homer/internal/logx"
	"github.com/LemonZuo/homer/internal/sshx"
	"golang.org/x/crypto/ssh"
)

var deviceIDRe = regexp.MustCompile(`(?m)^(t10\.\S+|naa\.\S+|mpx\.\S+|eui\.\S+)`)

type diskDeviceInfo struct {
	Model         string
	Type          string
	CapacityBytes int64
}

type storageFilesystem struct {
	Name      string
	UUID      string
	Type      string
	SizeBytes int64
	FreeBytes int64
}

type vmfsExtent struct {
	VolumeName string
	UUID       string
	Device     string
	Partition  int
}

type diskUsage struct {
	Known      bool
	UsedBytes  int64
	FreeBytes  int64
	Datastores []string
}

func collectDisks(client *ssh.Client, opts CollectOptions) []DiskHealth {
	// list 命令本身在多盘机器上偶发 5-10s(要走 SCSI inquiry),给 15s 留余量。
	listOut, err := runEsxiRetry(client, "disk device list", "esxcli storage core device list", 15*time.Second, 2, func(out string) bool {
		return len(deviceIDRe.FindAllString(out, -1)) > 0
	})
	if err != nil {
		logx.Warn("esxi collectDisks: list failed", "err", err.Error())
		return nil
	}
	rawIDs := deviceIDRe.FindAllString(listOut, -1)
	if len(rawIDs) == 0 {
		// list 跑通但 regex 没匹配 —— 通常是设备前缀不在已知的 t10./naa./mpx./eui. 里。
		// 截短一下输出方便看,512 字节就够看到几行了。
		head := listOut
		if len(head) > 512 {
			head = head[:512]
		}
		logx.Warn("esxi collectDisks: list parsed 0 devices", "bytes", len(listOut), "head", head)
		return nil
	}
	devInfo := parseDeviceInventory(listOut)
	usageByDevice := collectDiskUsage(client)

	// 去重保序
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(rawIDs))
	for _, id := range rawIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if !isDiskDevice(devInfo[id]) {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		logx.Warn("esxi collectDisks: no smart-capable disk devices", "n_raw", len(rawIDs))
		return nil
	}

	prevByDevice := diskHealthByDevice(opts.PreviousDisks)
	smartByID := map[string]string{}
	ataAttrsByID := map[string]ataSMARTAttrs{}
	smartFull := false
	if opts.SkipDiskSMART {
		logx.Debug("esxi disk smart collection skipped")
	} else {
		// 把所有盘的 SMART 合并到一次 SSH session,用 `===DEV===<id>` 行做分段标志。
		// 远端逐盘跑 esxcli smart get,stderr 已被外层 runEsxi 重定向丢掉。
		// SMART 单盘 1~2s,N 盘合批后总耗时随盘数线性增长(8 盘可能 12-15s),
		// 这里给到 25s 上限,远高于默认 8s 单命令超时,避免被截断成空结果。
		var b strings.Builder
		for _, id := range ids {
			b.WriteString(`printf '===DEV===%s\n' `)
			b.WriteString(sshx.ShellQuote(id))
			b.WriteString("; esxcli storage core device smart get -d ")
			b.WriteString(sshx.ShellQuote(id))
			b.WriteString("; ")
		}
		smartAll, err := runEsxiRetry(client, "disk smart batch", b.String(), 30*time.Second, 2, func(out string) bool {
			return strings.Count(out, "===DEV===") >= len(ids)
		})
		if err != nil {
			// 即便 ssh.Run 返回非零(NVMe 等盘的 smart get 偶尔以 status 2 退出),
			// stdout 里大概率已经吐出了大半 `===DEV===` 段,优先按 stdout 是否含分段判定。
			if !strings.Contains(smartAll, "===DEV===") {
				logx.Warn("esxi collectDisks: smart batch failed", "err", err.Error(), "n_dev", len(ids))
			} else {
				logx.Warn("esxi collectDisks: smart batch partial", "err", err.Error(), "n_dev", len(ids), "bytes", len(smartAll))
			}
		}
		smartByID = splitSMARTOutput(smartAll)
		retryMissingSMART(client, ids, smartByID)
		smartFull = diskSMARTComplete(ids, smartByID)
		// ATA 盘走 vsish 拿真实 6-byte raw,修正 esxcli 截断到 1 字节的 attribute 9/12 等。
		// NVMe 不支持 vsish smart 路径,这里直接跳过(返回 map 不包含 NVMe 盘)。
		ataAttrsByID = collectATASmartBuffers(client, ids)
		logx.Debug("esxi collectDisks", "n_dev", len(ids), "smart_bytes", len(smartAll), "smart_segments", len(smartByID), "ata_vsish", len(ataAttrsByID), "smart_full", smartFull)
	}

	var out []DiskHealth
	now := time.Now()
	for _, id := range ids {
		smart := smartByID[id]
		attrs := parseSMARTAttrs(smart)
		if ata, ok := ataAttrsByID[id]; ok {
			// vsish 给到非 -1 的字段优先覆盖 esxcli 解析结果(esxcli Raw 列截断到低 1 字节)
			if ata.PowerOnHours >= 0 {
				attrs.PowerOnHours = ata.PowerOnHours
			}
			if ata.PowerCycleCount >= 0 {
				attrs.PowerCycleCount = ata.PowerCycleCount
			}
			if ata.ReallocatedSectors >= 0 {
				attrs.ReallocatedSectors = ata.ReallocatedSectors
			}
			if ata.PendingSectorReallocation >= 0 {
				attrs.PendingSectorReallocation = ata.PendingSectorReallocation
			}
			if ata.UncorrectableErrors >= 0 {
				attrs.UncorrectableErrors = ata.UncorrectableErrors
			}
		}
		info := devInfo[id]
		usage := usageByDevice[id]
		usedBytes := int64(-1)
		freeBytes := int64(-1)
		if usage.Known {
			usedBytes = usage.UsedBytes
			freeBytes = usage.FreeBytes
		}
		d := DiskHealth{
			Device:                    id,
			Model:                     info.Model,
			Type:                      info.Type,
			CapacityBytes:             info.CapacityBytes,
			UsedBytes:                 usedBytes,
			FreeBytes:                 freeBytes,
			Datastores:                usage.Datastores,
			TempC:                     attrs.TempC,
			ThresholdC:                attrs.ThresholdC,
			Status:                    classifyDisk(info.Type, attrs.TempC),
			HealthStatus:              attrs.HealthStatus,
			PowerOnHours:              attrs.PowerOnHours,
			PowerCycleCount:           attrs.PowerCycleCount,
			ReallocatedSectors:        attrs.ReallocatedSectors,
			UncorrectableErrors:       attrs.UncorrectableErrors,
			MediaWearoutValue:         attrs.MediaWearoutValue,
			ReadErrorCount:            attrs.ReadErrorCount,
			PendingSectorReallocation: attrs.PendingSectorReallocation,
		}
		if smartFull {
			d.SMARTLastFullSuccessAt = now
		}
		if prev, ok := prevByDevice[id]; ok {
			fillMissingDiskSMARTFromPrev(&d, prev)
			if !smartFull {
				d.SMARTLastFullSuccessAt = prev.SMARTLastFullSuccessAt
			}
		}
		out = append(out, d)
	}
	return out
}

func diskHealthByDevice(disks []DiskHealth) map[string]DiskHealth {
	m := make(map[string]DiskHealth, len(disks))
	for _, d := range disks {
		if d.Device != "" {
			m[d.Device] = d
		}
	}
	return m
}

func diskSMARTComplete(ids []string, smartByID map[string]string) bool {
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if parseSMARTAttrs(smartByID[id]).TempC < 0 {
			return false
		}
	}
	return true
}

func fillMissingDiskSMARTFromPrev(d *DiskHealth, prev DiskHealth) {
	if d.TempC < 0 {
		d.TempC = prev.TempC
	}
	if d.ThresholdC < 0 {
		d.ThresholdC = prev.ThresholdC
	}
	if d.Status == "" || d.Status == "unknown" {
		d.Status = prev.Status
	}
	if d.HealthStatus == "" {
		d.HealthStatus = prev.HealthStatus
	}
	if d.PowerOnHours < 0 {
		d.PowerOnHours = prev.PowerOnHours
	}
	if d.PowerCycleCount < 0 {
		d.PowerCycleCount = prev.PowerCycleCount
	}
	if d.ReallocatedSectors < 0 {
		d.ReallocatedSectors = prev.ReallocatedSectors
	}
	if d.UncorrectableErrors < 0 {
		d.UncorrectableErrors = prev.UncorrectableErrors
	}
	if d.MediaWearoutValue < 0 {
		d.MediaWearoutValue = prev.MediaWearoutValue
	}
	if d.ReadErrorCount < 0 {
		d.ReadErrorCount = prev.ReadErrorCount
	}
	if d.PendingSectorReallocation < 0 {
		d.PendingSectorReallocation = prev.PendingSectorReallocation
	}
}

func isDiskDevice(info diskDeviceInfo) bool {
	model := strings.ToLower(info.Model)
	devType := strings.ToLower(info.Type)
	if strings.Contains(model, "dvd") || strings.Contains(model, "cd-rom") || strings.Contains(model, "cdrom") {
		return false
	}
	if strings.Contains(devType, "cd-rom") || strings.Contains(devType, "cdrom") || strings.Contains(devType, "optical") {
		return false
	}
	return true
}

func retryMissingSMART(client *ssh.Client, ids []string, smartByID map[string]string) {
	for _, id := range ids {
		if parseSMARTAttrs(smartByID[id]).TempC >= 0 {
			continue
		}
		cmd := "esxcli storage core device smart get -d " + sshx.ShellQuote(id)
		out, err := runEsxiRetry(client, "disk smart "+id, cmd, 10*time.Second, 2, func(out string) bool {
			return parseSMARTAttrs(out).TempC >= 0
		})
		if err != nil {
			logx.Warn("esxi collectDisks: smart retry failed", "device", id, "err", err.Error())
			continue
		}
		smartByID[id] = out
	}
}

func collectDiskUsage(client *ssh.Client) map[string]diskUsage {
	fsOut, fsErr := runEsxiRetry(client, "storage filesystem list", "esxcli storage filesystem list", defaultCmdTimeout, 2, func(out string) bool {
		return len(parseStorageFilesystems(out)) > 0
	})
	extentOut, extentErr := runEsxiRetry(client, "storage vmfs extent list", "esxcli storage vmfs extent list", defaultCmdTimeout, 2, func(out string) bool {
		return len(parseVMFSExtents(out)) > 0
	})
	if fsErr != nil || extentErr != nil {
		if fsErr != nil {
			logx.Warn("esxi collectDisks: filesystem list failed", "err", fsErr.Error())
		}
		if extentErr != nil {
			logx.Warn("esxi collectDisks: vmfs extent list failed", "err", extentErr.Error())
		}
		return nil
	}
	return mapDiskUsage(parseStorageFilesystems(fsOut), parseVMFSExtents(extentOut))
}

// splitSMARTOutput 按 `===DEV===<id>` 标记切合批后的 SMART 输出,返回 id→该盘原始 SMART 文本。
func splitSMARTOutput(out string) map[string]string {
	res := map[string]string{}
	var currentID string
	var buf strings.Builder
	flush := func() {
		if currentID != "" {
			res[currentID] = buf.String()
		}
		buf.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "===DEV===") {
			flush()
			currentID = strings.TrimPrefix(line, "===DEV===")
			currentID = strings.TrimRight(currentID, "\r")
			continue
		}
		if currentID == "" {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return res
}

// parseDeviceInventory 扫 `esxcli storage core device list` 输出,
// 把每个设备的 Model / Type / Size 映射出来(同一段以设备 id 起首,后续若干行缩进键值)。
func parseDeviceInventory(out string) map[string]diskDeviceInfo {
	devices := map[string]diskDeviceInfo{}
	var current string
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if isStorageDeviceID(trim) {
			current = strings.Fields(trim)[0]
			if _, ok := devices[current]; !ok {
				devices[current] = diskDeviceInfo{}
			}
			continue
		}
		if current == "" {
			continue
		}
		idx := strings.IndexByte(trim, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trim[:idx]))
		val := strings.TrimSpace(trim[idx+1:])
		info := devices[current]
		switch key {
		case "model":
			if val != "" {
				info.Model = val
			}
		case "device type":
			if val != "" {
				info.Type = val
			}
		case "size":
			info.CapacityBytes = parseESXiDeviceSize(val)
		}
		devices[current] = info
	}
	return devices
}

func isStorageDeviceID(s string) bool {
	return strings.HasPrefix(s, "t10.") ||
		strings.HasPrefix(s, "naa.") ||
		strings.HasPrefix(s, "mpx.") ||
		strings.HasPrefix(s, "eui.")
}

// parseESXiDeviceSize 解 `esxcli storage core device list` 的 Size 字段。
// 该字段裸数字单位是 MiB;如果输出带 GB/Bytes 等单位,交给 parseBytes。
func parseESXiDeviceSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	low := strings.ToLower(s)
	if strings.Contains(low, "b") {
		return parseBytes(s)
	}
	num := extractLeadingFloat(s)
	if num == "" {
		return 0
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	return int64(f * 1024 * 1024)
}

// parseStorageFilesystems 解 `esxcli storage filesystem list`。
// 只保留已挂载 VMFS datastore;Size/Free 在该命令里是 bytes。
func parseStorageFilesystems(out string) []storageFilesystem {
	var list []storageFilesystem
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 7 {
			continue
		}
		if fields[0] == "Mount" || strings.HasPrefix(fields[0], "---") {
			continue
		}
		freeBytes := parseBytes(fields[len(fields)-1])
		sizeBytes := parseBytes(fields[len(fields)-2])
		fsType := fields[len(fields)-3]
		mounted := fields[len(fields)-4]
		uuid := fields[len(fields)-5]
		if !strings.EqualFold(mounted, "true") || !strings.HasPrefix(strings.ToLower(fsType), "vmfs") {
			continue
		}
		name := strings.Join(fields[1:len(fields)-5], " ")
		if name == "" || uuid == "" || sizeBytes <= 0 {
			continue
		}
		list = append(list, storageFilesystem{
			Name:      name,
			UUID:      uuid,
			Type:      fsType,
			SizeBytes: sizeBytes,
			FreeBytes: freeBytes,
		})
	}
	return list
}

// parseVMFSExtents 解 `esxcli storage vmfs extent list`,用于把 datastore 关联回 device。
func parseVMFSExtents(out string) []vmfsExtent {
	var list []vmfsExtent
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 {
			continue
		}
		if fields[0] == "Volume" || strings.HasPrefix(fields[0], "---") {
			continue
		}
		partition := parseIntDefault(fields[len(fields)-1], -1)
		device := fields[len(fields)-2]
		uuid := fields[len(fields)-4]
		name := strings.Join(fields[:len(fields)-4], " ")
		if name == "" || uuid == "" || device == "" {
			continue
		}
		list = append(list, vmfsExtent{
			VolumeName: name,
			UUID:       uuid,
			Device:     device,
			Partition:  partition,
		})
	}
	return list
}

func mapDiskUsage(filesystems []storageFilesystem, extents []vmfsExtent) map[string]diskUsage {
	fsByUUID := map[string]storageFilesystem{}
	fsByName := map[string]storageFilesystem{}
	for _, fs := range filesystems {
		fsByUUID[strings.ToLower(fs.UUID)] = fs
		fsByName[fs.Name] = fs
	}

	type datastoreExtent struct {
		fs      storageFilesystem
		devices map[string]struct{}
	}
	byDatastore := map[string]datastoreExtent{}
	for _, ext := range extents {
		fs, ok := fsByUUID[strings.ToLower(ext.UUID)]
		if !ok {
			fs, ok = fsByName[ext.VolumeName]
		}
		if !ok {
			continue
		}
		key := strings.ToLower(fs.UUID)
		if key == "" {
			key = "name:" + fs.Name
		}
		item := byDatastore[key]
		if item.devices == nil {
			item.fs = fs
			item.devices = map[string]struct{}{}
		}
		item.devices[ext.Device] = struct{}{}
		byDatastore[key] = item
	}

	usageByDevice := map[string]diskUsage{}
	for _, item := range byDatastore {
		usedBytes := item.fs.SizeBytes - item.fs.FreeBytes
		if usedBytes < 0 {
			usedBytes = 0
		}
		if len(item.devices) != 1 {
			// 多 extent datastore 不能可靠拆分到单盘,只记录关联名称,不写用量。
			for dev := range item.devices {
				u := usageByDevice[dev]
				u.Datastores = appendUniqueString(u.Datastores, item.fs.Name)
				usageByDevice[dev] = u
			}
			continue
		}
		for dev := range item.devices {
			u := usageByDevice[dev]
			u.Known = true
			u.UsedBytes += usedBytes
			u.FreeBytes += item.fs.FreeBytes
			u.Datastores = appendUniqueString(u.Datastores, item.fs.Name)
			usageByDevice[dev] = u
		}
	}
	return usageByDevice
}

func appendUniqueString(list []string, item string) []string {
	if item == "" {
		return list
	}
	for _, v := range list {
		if v == item {
			return list
		}
	}
	return append(list, item)
}

// classifyDisk 给磁盘按温度评 ok/warning/critical(基于 prompt 表格)。
func classifyDisk(devType string, temp int) string {
	if temp < 0 {
		return "unknown"
	}
	t := strings.ToUpper(devType)
	switch {
	case strings.Contains(t, "NVME"):
		switch {
		case temp >= 80:
			return "critical"
		case temp >= 70:
			return "warning"
		}
	case strings.Contains(t, "SSD"):
		switch {
		case temp >= 70:
			return "critical"
		case temp >= 60:
			return "warning"
		}
	case strings.Contains(t, "HDD"), strings.Contains(t, "ATA"):
		switch {
		case temp >= 55:
			return "critical"
		case temp >= 50:
			return "warning"
		}
	}
	return "ok"
}
