package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"mayfly-go/internal/machine/mcm"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/infra/isolate"
	"mayfly-go/internal/octop/infra/octopapi"
	"mayfly-go/pkg/logx"
	"mayfly-go/pkg/taskx"
)

const metricCronKey = "octop-metric-collect"

func (a *instanceAppImpl) InitMetricCron() {
	_ = taskx.BindCronTaskWithLock(metricCronKey, "0 */2 * * * *", 2*time.Minute, true, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := a.CollectAll(ctx); err != nil {
			logx.ErrorTrace("octop metric collect", err)
		}
	})
}

func (a *instanceAppImpl) CollectAll(ctx context.Context) error {
	settings := GetSettingsApp().MustGet(ctx)
	list, err := a.GetRepo().ListEnabled(ctx)
	if err != nil {
		return err
	}
	isolate.Run(ctx, list, settings.CollectConcurrency, time.Duration(settings.CollectTimeoutSec)*time.Second, func(c context.Context, inst *entity.OctopInstance) error {
		return a.CollectOne(c, inst)
	})
	return nil
}

func (a *instanceAppImpl) CollectOne(ctx context.Context, inst *entity.OctopInstance) error {
	now := time.Now().UTC()
	sample := &entity.OctopMetricSample{InstanceId: inst.Id, CollectedAt: now}
	facts := probeFacts{maintenance: inst.IsMaintenance()}

	hostErr := a.fillHost(ctx, inst, sample, &facts)
	procErr := a.fillOctopProcess(ctx, inst, sample, &facts)
	httpErr := a.fillHTTP(ctx, inst, sample, &facts)
	pgErr := a.fillPG(ctx, inst, sample, &facts)

	if hostErr != nil && procErr != nil && httpErr != nil && pgErr != nil {
		facts.collectFailed = true
		facts.collectError = "采集失败"
		sample.CollectError = "采集失败"
	}

	status, reason := aggregateHealth(inst, facts)
	sample.HealthStatus = status
	sample.HealthReason = reason
	if err := a.metricRepo.Insert(ctx, sample); err != nil {
		return err
	}
	inst.LastHealthStatus = status
	inst.LastHealthReason = reason
	inst.LastCollectAt = &now
	inst.LastCollectError = sample.CollectError
	inst.ConsecutiveHttpFails = facts.httpFails
	return a.UpdateById(ctx, inst)
}

func (a *instanceAppImpl) fillHost(ctx context.Context, inst *entity.OctopInstance, sample *entity.OctopMetricSample, facts *probeFacts) error {
	_ = ctx
	stats, err := a.machineApp.GetMachineStats(inst.MachineId)
	if err != nil {
		sample.HostOnline = nil // 未知，不写 0
		facts.hostOnline = nil
		return err
	}
	online := int8(1)
	sample.HostOnline = &online
	facts.hostOnline = &online
	cpu := float64(100 - stats.CPU.Idle)
	mem := memUsage(stats)
	disk, freeGB := diskUsage(stats)
	sample.HostCpu = &cpu
	sample.HostMem = &mem
	sample.HostDisk = &disk
	sample.HostDiskFreeGB = &freeGB
	if load, err := strconv.ParseFloat(stats.Load1, 64); err == nil {
		sample.HostLoad1 = &load
	}
	facts.hostCpu = sample.HostCpu
	facts.hostMem = sample.HostMem
	facts.hostDisk = sample.HostDisk

	if hist, err := a.machineMetricApp.GetRange(inst.MachineId, time.Now().Add(-10*time.Minute), time.Now()); err == nil {
		cpus := make([]float64, 0, len(hist))
		mems := make([]float64, 0, len(hist))
		for _, p := range hist {
			cpus = append(cpus, p.CpuUsage)
			mems = append(mems, p.MemUsage)
		}
		interval := inst.Interval()
		facts.cpuSustainWarn = sustainedOver(cpus, float64(inst.CpuWarn), 5*time.Minute, interval)
		facts.cpuSustainCrit = sustainedOver(cpus, float64(inst.CpuCrit), 5*time.Minute, interval)
		facts.memSustainWarn = sustainedOver(mems, float64(inst.MemWarn), 5*time.Minute, interval)
		facts.memSustainCrit = sustainedOver(mems, float64(inst.MemCrit), 5*time.Minute, interval)
	}
	return nil
}

func (a *instanceAppImpl) fillOctopProcess(ctx context.Context, inst *entity.OctopInstance, sample *entity.OctopMetricSample, facts *probeFacts) error {
	if inst.IsSystemd() {
		cmd, err := BuildUnitStatusCmd(inst.SystemdUnit)
		if err != nil {
			sample.OctopProcess = entity.ProbeUnknown
			facts.octopProcess = entity.ProbeUnknown
			return err
		}
		out, _, err := a.runGoverned(ctx, inst.MachineId, cmd, 15*time.Second)
		if err != nil {
			sample.OctopProcess = entity.ProbeUnknown
			facts.octopProcess = entity.ProbeUnknown
			return err
		}
		active, pid, restarts, uptime := parseSystemdShow(out)
		if active == "active" {
			sample.OctopProcess = entity.ProbeOK
			facts.octopProcess = entity.ProbeOK
		} else {
			sample.OctopProcess = entity.ProbeFail
			facts.octopProcess = entity.ProbeFail
		}
		if pid > 0 {
			sample.OctopPid = &pid
		}
		sample.OctopRestarts = &restarts
		if uptime > 0 {
			sample.OctopUptimeSec = &uptime
		}
		return nil
	}
	if inst.ProcessMatch == "" {
		sample.OctopProcess = entity.ProbeUnknown
		facts.octopProcess = entity.ProbeUnknown
		return nil
	}
	cmd, err := BuildProcessStatusCmd(inst.ProcessMatch)
	if err != nil {
		sample.OctopProcess = entity.ProbeUnknown
		facts.octopProcess = entity.ProbeUnknown
		return err
	}
	out, _, err := a.runGoverned(ctx, inst.MachineId, cmd, 10*time.Second)
	if err != nil {
		// pgrep 无匹配时常返回 exit 1，与采集失败区分：输出空视为进程停止
		if strings.TrimSpace(out) == "" && !isCredentialErr(err) {
			sample.OctopProcess = entity.ProbeFail
			facts.octopProcess = entity.ProbeFail
			return nil
		}
		sample.OctopProcess = entity.ProbeUnknown
		facts.octopProcess = entity.ProbeUnknown
		return err
	}
	if strings.TrimSpace(out) == "" {
		sample.OctopProcess = entity.ProbeFail
		facts.octopProcess = entity.ProbeFail
		return nil
	}
	sample.OctopProcess = entity.ProbeOK
	facts.octopProcess = entity.ProbeOK
	if pid := parseInt64(strings.Fields(out)[0]); pid > 0 {
		sample.OctopPid = &pid
	}
	return nil
}

func (a *instanceAppImpl) fillHTTP(ctx context.Context, inst *entity.OctopInstance, sample *entity.OctopMetricSample, facts *probeFacts) error {
	if inst.ApiURL == "" {
		sample.HttpHealth = entity.ProbeUnknown
		facts.httpHealth = entity.ProbeUnknown
		return nil
	}
	res := octopapi.Check(ctx, inst.ApiURL, 5*time.Second)
	sample.HttpCode = res.StatusCode
	if res.OK {
		sample.HttpHealth = entity.ProbeOK
		facts.httpHealth = entity.ProbeOK
		facts.httpFails = 0
		return nil
	}
	sample.HttpHealth = entity.ProbeFail
	facts.httpHealth = entity.ProbeFail
	facts.httpFails = inst.ConsecutiveHttpFails + 1
	return nil
}

func (a *instanceAppImpl) fillPG(ctx context.Context, inst *entity.OctopInstance, sample *entity.OctopMetricSample, facts *probeFacts) error {
	if inst.DbId == 0 || inst.DbName == "" {
		sample.PgConnect = entity.ProbeUnknown
		facts.pgConnect = entity.ProbeUnknown
		return nil
	}
	conn, err := a.dbApp.GetDbConn(ctx, inst.DbId, inst.DbName)
	if err != nil {
		sample.PgConnect = entity.ProbeUnknown // 凭据/ACL 失败：未知
		facts.pgConnect = entity.ProbeUnknown
		return err
	}
	if err := conn.Ping(); err != nil {
		sample.PgConnect = entity.ProbeFail
		facts.pgConnect = entity.ProbeFail
		return nil
	}
	sample.PgConnect = entity.ProbeOK
	facts.pgConnect = entity.ProbeOK

	if _, rows, err := conn.QueryContext(ctx, `SELECT count(*) AS total, count(*) FILTER (WHERE state = 'active') AS active FROM pg_stat_activity`); err == nil && len(rows) > 0 {
		if n := asInt64(rows[0]["total"]); n >= 0 {
			sample.PgConns = &n
		}
		if n := asInt64(rows[0]["active"]); n >= 0 {
			sample.PgActiveConns = &n
		}
	}
	if _, rows, err := conn.QueryContext(ctx, `SELECT pg_database_size(current_database()) AS sz`); err == nil && len(rows) > 0 {
		if n := asInt64(rows[0]["sz"]); n > 0 {
			mb := float64(n) / 1024 / 1024
			sample.PgSizeMB = &mb
		}
	}
	if locks, err := a.dbApp.LockWaits(ctx, inst.DbId, inst.DbName); err != nil {
		sample.PgLockAvail = -1 // 不支持/无权限，不写 0
	} else {
		sample.PgLockAvail = 1
		n := int64(len(locks))
		sample.PgLockWaits = &n
	}
	return nil
}

func memUsage(stats *mcm.Stats) float64 {
	if stats.MemInfo.Total == 0 {
		return 0
	}
	used := stats.MemInfo.Total - stats.MemInfo.Available
	return float64(used) * 100 / float64(stats.MemInfo.Total)
}

func diskUsage(stats *mcm.Stats) (usage float64, freeGB float64) {
	var bestUsed, bestTotal, bestFree uint64
	for _, fs := range stats.FSInfos {
		total := fs.Used + fs.Free
		if fs.MountPoint == "/" || total > bestTotal {
			bestUsed, bestTotal, bestFree = fs.Used, total, fs.Free
			if fs.MountPoint == "/" {
				break
			}
		}
	}
	if bestTotal > 0 {
		usage = float64(bestUsed) * 100 / float64(bestTotal)
	}
	freeGB = float64(bestFree) / 1024 / 1024 / 1024
	return
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		i, _ := strconv.ParseInt(string(n), 10, 64)
		return i
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		return -1
	}
}

func isCredentialErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "auth") || strings.Contains(msg, "permission") || strings.Contains(msg, "denied") || strings.Contains(msg, "credential")
}

func (a *instanceAppImpl) MetricRange(ctx context.Context, instanceId uint64, start, end time.Time) ([]*entity.OctopMetricSample, error) {
	if _, err := a.mustGetVisible(ctx, instanceId); err != nil {
		return nil, err
	}
	return a.metricRepo.GetRange(instanceId, start, end)
}
