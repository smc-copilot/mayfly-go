package application

import (
	"fmt"
	"time"

	"mayfly-go/internal/octop/domain/entity"
)

type probeFacts struct {
	collectFailed bool
	collectError  string
	maintenance   bool

	hostOnline *int8
	hostCpu    *float64
	hostMem    *float64
	hostDisk   *float64

	octopProcess string // ok/fail/unknown
	httpHealth   string
	httpFails    int
	pgConnect    string

	cpuSustainWarn bool
	cpuSustainCrit bool
	memSustainWarn bool
	memSustainCrit bool
}

func aggregateHealth(inst *entity.OctopInstance, f probeFacts) (status, reason string) {
	if inst != nil && inst.IsMaintenance() || f.maintenance {
		return entity.HealthMaintenance, "维护中"
	}
	if f.collectFailed {
		// 采集器自己故障或凭据失效：未知，不误作服务宕机
		if f.collectError == "" {
			f.collectError = "采集失败"
		}
		return entity.HealthUnknown, f.collectError
	}

	crit := make([]string, 0, 4)
	warn := make([]string, 0, 4)

	if f.octopProcess == entity.ProbeFail {
		crit = append(crit, "Octop 进程停止")
	}
	httpThreshold := entity.DefaultHttpFailThreshold
	if inst != nil && inst.HttpFailThreshold > 0 {
		httpThreshold = inst.HttpFailThreshold
	}
	if f.httpHealth == entity.ProbeFail && f.httpFails >= httpThreshold {
		crit = append(crit, fmt.Sprintf("HTTP 健康连续失败 %d 次", f.httpFails))
	} else if f.httpHealth == entity.ProbeFail {
		warn = append(warn, "HTTP 健康失败")
	}
	if f.pgConnect == entity.ProbeFail {
		crit = append(crit, "PostgreSQL 连接失败")
	}
	if f.hostOnline != nil && *f.hostOnline == 0 {
		crit = append(crit, "主机离线")
	}

	if f.cpuSustainCrit || over(f.hostCpu, threshold(inst, true, true)) {
		crit = append(crit, "主机 CPU 严重")
	} else if f.cpuSustainWarn || over(f.hostCpu, threshold(inst, true, false)) {
		warn = append(warn, "主机 CPU 告警")
	}
	if f.memSustainCrit || over(f.hostMem, threshold(inst, false, true)) {
		crit = append(crit, "主机内存严重")
	} else if f.memSustainWarn || over(f.hostMem, threshold(inst, false, false)) {
		warn = append(warn, "主机内存告警")
	}
	if over(f.hostDisk, diskThreshold(inst, true)) {
		crit = append(crit, "磁盘严重")
	} else if over(f.hostDisk, diskThreshold(inst, false)) {
		warn = append(warn, "磁盘告警")
	}

	if len(crit) > 0 {
		return entity.HealthCritical, joinReasons(crit)
	}
	if f.octopProcess == entity.ProbeUnknown || f.httpHealth == entity.ProbeUnknown || f.pgConnect == entity.ProbeUnknown {
		return entity.HealthUnknown, firstUnknown(f)
	}
	if len(warn) > 0 {
		return entity.HealthWarning, joinReasons(warn)
	}
	return entity.HealthNormal, "正常"
}

func threshold(inst *entity.OctopInstance, cpu bool, crit bool) int {
	if inst == nil {
		if cpu && crit {
			return entity.DefaultCpuCrit
		}
		if cpu {
			return entity.DefaultCpuWarn
		}
		if crit {
			return entity.DefaultMemCrit
		}
		return entity.DefaultMemWarn
	}
	if cpu && crit {
		return inst.CpuCrit
	}
	if cpu {
		return inst.CpuWarn
	}
	if crit {
		return inst.MemCrit
	}
	return inst.MemWarn
}

func diskThreshold(inst *entity.OctopInstance, crit bool) int {
	if inst == nil {
		if crit {
			return entity.DefaultDiskCrit
		}
		return entity.DefaultDiskWarn
	}
	if crit {
		return inst.DiskCrit
	}
	return inst.DiskWarn
}

func over(v *float64, limit int) bool {
	return v != nil && *v >= float64(limit)
}

func joinReasons(rs []string) string {
	out := ""
	for i, r := range rs {
		if i > 0 {
			out += "；"
		}
		out += r
	}
	return out
}

func firstUnknown(f probeFacts) string {
	switch {
	case f.octopProcess == entity.ProbeUnknown:
		return "Octop 进程状态未知/采集失败"
	case f.httpHealth == entity.ProbeUnknown:
		return "HTTP 健康未知/采集失败"
	case f.pgConnect == entity.ProbeUnknown:
		return "PostgreSQL 状态未知/采集失败"
	default:
		return "未知/采集失败"
	}
}

func sustainedOver(points []float64, limit float64, window time.Duration, interval time.Duration) bool {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	need := int((window + interval - 1) / interval)
	if need < 2 {
		need = 2
	}
	if len(points) < need {
		return false
	}
	tail := points[len(points)-need:]
	for _, p := range tail {
		if p < limit {
			return false
		}
	}
	return true
}
