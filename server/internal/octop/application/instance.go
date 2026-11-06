package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	dbapp "mayfly-go/internal/db/application"
	machineapp "mayfly-go/internal/machine/application"
	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/octopapi"
	"mayfly-go/internal/pkg/utils"
	tagapp "mayfly-go/internal/tag/application"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/logx"
	"mayfly-go/pkg/model"
	"mayfly-go/pkg/utils/stringx"
)

type Instance interface {
	base.App[*entity.OctopInstance]
	SaveInstance(ctx context.Context, inst *entity.OctopInstance, apiToken string) error
	DeleteInstance(ctx context.Context, id uint64) error
	PageVisible(ctx context.Context, q *entity.InstanceQuery) (*model.PageResult[*dto.InstanceOverview], error)
	Overview(ctx context.Context) ([]*dto.InstanceOverview, error)
	Detail(ctx context.Context, id uint64) (*dto.InstanceDetail, error)
	TestConn(ctx context.Context, inst *entity.OctopInstance) (*dto.ConnTestResult, error)
	Restart(ctx context.Context, id uint64) (*dto.RestartResult, error)
	BindCasdoorApp(ctx context.Context, instanceId, appSnapshotId uint64, confirmOverwrite bool) error
	ListBindMatches(ctx context.Context, connectionId uint64) ([]*dto.AppBindMatch, error)
	ListOctopUsers(ctx context.Context, q *entity.OctopUserQuery) (*dto.OctopUserPage, error)
	VisibleInstanceIDs(ctx context.Context) ([]uint64, error)
	AssertInstanceAccess(ctx context.Context, inst *entity.OctopInstance) error
	CollectOne(ctx context.Context, inst *entity.OctopInstance) error
	MetricRange(ctx context.Context, instanceId uint64, start, end time.Time) ([]*entity.OctopMetricSample, error)
	InitMetricCron()
	CollectAll(ctx context.Context) error
}

type instanceAppImpl struct {
	base.AppImpl[*entity.OctopInstance, repository.Instance]

	machineApp       machineapp.Machine       `inject:"T"`
	machineMetricApp machineapp.MachineMetric `inject:"T"`
	dbApp            dbapp.Db                 `inject:"T"`
	tagApp           tagapp.TagTreeChecker    `inject:"T"`
	metricRepo       repository.MetricSample  `inject:"T"`
	checkpointRepo   repository.Checkpoint    `inject:"T"`
	appRepo          repository.CasdoorApp    `inject:"T"`
	runRepo          repository.CollectorRun  `inject:"T"`
	fetchUsers       func(context.Context, string, string) octopapi.UsersResult
}

var _ Instance = (*instanceAppImpl)(nil)

func (a *instanceAppImpl) SaveInstance(ctx context.Context, inst *entity.OctopInstance, apiToken string) error {
	if strings.TrimSpace(inst.Name) == "" {
		return errorx.NewBizI(ctx, imsg.ErrInstanceNameRequired)
	}
	if inst.MachineId == 0 {
		return errorx.NewBizI(ctx, imsg.ErrInstanceMachineRequired)
	}
	if inst.StartMode == "" {
		inst.StartMode = entity.StartModeCLI
	}
	if inst.StartMode == entity.StartModeSystemd {
		if err := validateUnit(inst.SystemdUnit); err != nil {
			return err
		}
	}
	if inst.ProcessMatch != "" && !processNameRe.MatchString(inst.ProcessMatch) {
		return errorx.NewBizI(ctx, imsg.ErrProcessMatchInvalid)
	}
	fillThresholdDefaults(inst)
	if inst.Id == 0 && inst.Code == "" {
		inst.Code = stringx.Rand(12)
	}
	if inst.Id > 0 {
		old, err := a.mustGetVisible(ctx, inst.Id)
		if err != nil {
			return err
		}
		preserveInstanceOnUpdate(inst, old)
	}
	if strings.TrimSpace(apiToken) != "" {
		enc, err := utils.PwdAesEncrypt(strings.TrimSpace(apiToken))
		if err != nil {
			return err
		}
		inst.ApiTokenEnc = enc
		inst.ApiTokenConfigured = 1
	}
	return a.Save(ctx, inst)
}

func preserveInstanceOnUpdate(inst, old *entity.OctopInstance) {
	inst.Code = old.Code
	inst.CasdoorAppSnapshotId = old.CasdoorAppSnapshotId
	inst.CasdoorConnectionId = old.CasdoorConnectionId
	inst.ApiTokenEnc = old.ApiTokenEnc
	inst.ApiTokenConfigured = old.ApiTokenConfigured
	inst.ExtraData = old.ExtraData
	inst.LastHealthStatus = old.LastHealthStatus
	inst.LastHealthReason = old.LastHealthReason
	inst.LastCollectAt = old.LastCollectAt
	inst.LastCollectError = old.LastCollectError
	inst.ConsecutiveHttpFails = old.ConsecutiveHttpFails
}

func (a *instanceAppImpl) DeleteInstance(ctx context.Context, id uint64) error {
	if _, err := a.mustGetVisible(ctx, id); err != nil {
		return err
	}
	return a.DeleteById(ctx, id)
}

func (a *instanceAppImpl) PageVisible(ctx context.Context, q *entity.InstanceQuery) (*model.PageResult[*dto.InstanceOverview], error) {
	list, err := a.Overview(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]*dto.InstanceOverview, 0, len(list))
	for _, item := range list {
		if q.Name != "" && !strings.Contains(item.Name, q.Name) {
			continue
		}
		if q.Center != "" && item.Center != q.Center {
			continue
		}
		if q.Status != "" && item.HealthStatus != q.Status {
			continue
		}
		if q.Keyword != "" && !strings.Contains(strings.ToLower(item.Name+item.Code+item.Center), strings.ToLower(q.Keyword)) {
			continue
		}
		filtered = append(filtered, item)
	}
	pageNum, pageSize := q.Normalize()
	start := (pageNum - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return &model.PageResult[*dto.InstanceOverview]{
		Total: int64(len(filtered)), List: filtered[start:end], PageNum: pageNum, PageSize: pageSize,
	}, nil
}

func (a *instanceAppImpl) Overview(ctx context.Context) ([]*dto.InstanceOverview, error) {
	visible, err := a.ListVisible(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.InstanceOverview, 0, len(visible))
	for _, inst := range visible {
		out = append(out, a.toOverview(ctx, inst))
	}
	return out, nil
}

func (a *instanceAppImpl) Detail(ctx context.Context, id uint64) (*dto.InstanceDetail, error) {
	inst, err := a.mustGetVisible(ctx, id)
	if err != nil {
		return nil, err
	}
	ov := a.toOverview(ctx, inst)
	latest, err := a.metricRepo.Latest(inst.Id)
	if err != nil {
		return nil, err
	}
	cps, err := a.checkpointRepo.SelectByCond(model.NewCond().Eq("instance_id", inst.Id))
	if err != nil {
		return nil, err
	}
	canRestart := inst.IsSystemd()
	hint := ""
	if !canRestart {
		hint = "命令行启动的实例只监测，不提供重启"
	}
	return &dto.InstanceDetail{
		Overview:     ov,
		Probes:       overviewProbes(ov),
		Thresholds:   inst,
		CanRestart:   canRestart,
		RestartHint:  hint,
		Checkpoints:  cps,
		LatestSample: latest,
	}, nil
}

func (a *instanceAppImpl) TestConn(ctx context.Context, inst *entity.OctopInstance) (*dto.ConnTestResult, error) {
	now := time.Now()
	res := &dto.ConnTestResult{
		SSH:  dto.ProbeItem{Name: "ssh", Status: entity.ProbeUnknown, At: &now},
		HTTP: dto.ProbeItem{Name: "http", Status: entity.ProbeUnknown, At: &now},
		PG:   dto.ProbeItem{Name: "pg", Status: entity.ProbeUnknown, At: &now},
	}
	if inst.MachineId > 0 {
		if _, err := a.machineApp.GetCli(ctx, inst.MachineId); err != nil {
			res.SSH.Status = entity.ProbeFail
			res.SSH.Reason = err.Error()
		} else if stats, err := a.machineApp.GetMachineStats(inst.MachineId); err != nil {
			res.SSH.Status = entity.ProbeUnknown
			res.SSH.Reason = err.Error()
		} else if stats != nil {
			res.SSH.Status = entity.ProbeOK
		}
	} else {
		res.SSH.Status = entity.ProbeFail
		res.SSH.Reason = "未关联主机"
	}

	httpRes := octopapi.Check(ctx, inst.ApiURL, 5*time.Second)
	if inst.ApiURL == "" {
		res.HTTP.Status = entity.ProbeUnknown
		res.HTTP.Reason = "未配置 API 地址"
	} else if httpRes.OK {
		res.HTTP.Status = entity.ProbeOK
	} else {
		res.HTTP.Status = entity.ProbeFail
		res.HTTP.Reason = httpRes.Error
	}

	if inst.DbId == 0 || inst.DbName == "" {
		res.PG.Status = entity.ProbeUnknown
		res.PG.Reason = "未关联 PostgreSQL"
	} else {
		conn, err := a.dbApp.GetDbConn(ctx, inst.DbId, inst.DbName)
		if err != nil {
			res.PG.Status = entity.ProbeUnknown
			res.PG.Reason = err.Error()
		} else if err := conn.Ping(); err != nil {
			res.PG.Status = entity.ProbeFail
			res.PG.Reason = err.Error()
		} else {
			res.PG.Status = entity.ProbeOK
		}
	}

	if res.SSH.Status == entity.ProbeOK && res.HTTP.Status == entity.ProbeOK && res.PG.Status == entity.ProbeOK {
		res.Saved = "ok"
	} else {
		res.Saved = "pending_fix"
	}
	return res, nil
}

func (a *instanceAppImpl) Restart(ctx context.Context, id uint64) (*dto.RestartResult, error) {
	inst, err := a.mustGetVisible(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := AssertRestartAllowed(inst); err != nil {
		return nil, err
	}
	cmd, err := BuildRestartCmd(inst.SystemdUnit)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	output, notice, err := a.runGoverned(ctx, inst.MachineId, cmd, 60*time.Second)
	result := &dto.RestartResult{Output: trimOut(output), Notice: notice, DurationMs: time.Since(start).Milliseconds()}
	if err != nil {
		return result, err
	}
	statusCmd, serr := BuildUnitStatusCmd(inst.SystemdUnit)
	if serr == nil {
		stOut, _, _ := a.runGoverned(ctx, inst.MachineId, statusCmd, 15*time.Second)
		active, _, _, _ := parseSystemdShow(stOut)
		result.UnitActive = active
	}
	httpRes := octopapi.Check(ctx, inst.ApiURL, 5*time.Second)
	if httpRes.OK {
		result.HttpHealth = entity.ProbeOK
	} else {
		result.HttpHealth = entity.ProbeFail
	}
	return result, nil
}

func (a *instanceAppImpl) toOverview(ctx context.Context, inst *entity.OctopInstance) *dto.InstanceOverview {
	ov := &dto.InstanceOverview{
		OctopInstance: inst,
		HealthStatus:  inst.LastHealthStatus,
		HealthReason:  inst.LastHealthReason,
		LastCollectAt: inst.LastCollectAt,
	}
	if ov.HealthStatus == "" {
		ov.HealthStatus = entity.HealthUnknown
		ov.HealthReason = "尚未采集"
	}
	if machine, err := a.machineApp.GetById(inst.MachineId); err == nil && machine != nil {
		ov.MachineName = machine.Name
		ov.MachineIp = machine.Ip
	}
	if latest, err := a.metricRepo.Latest(inst.Id); err == nil && latest != nil {
		ov.OctopProcess = latest.OctopProcess
		ov.HttpHealth = latest.HttpHealth
		ov.PgConnect = latest.PgConnect
		if latest.HostOnline != nil {
			if *latest.HostOnline == 1 {
				ov.MachineOnline = entity.ProbeOK
			} else {
				ov.MachineOnline = entity.ProbeFail
			}
		} else {
			ov.MachineOnline = entity.ProbeUnknown
		}
		ov.HealthStatus = latest.HealthStatus
		ov.HealthReason = latest.HealthReason
		t := latest.CollectedAt
		ov.LastCollectAt = &t
	} else {
		ov.MachineOnline = entity.ProbeUnknown
		ov.OctopProcess = entity.ProbeUnknown
		ov.HttpHealth = entity.ProbeUnknown
		ov.PgConnect = entity.ProbeUnknown
	}
	if inst.CasdoorAppSnapshotId > 0 {
		if app, err := a.appRepo.GetById(inst.CasdoorAppSnapshotId); err == nil && app != nil {
			ov.CasdoorAppName = firstNonEmpty(app.DisplayName, app.Name)
		}
	}
	_ = ctx
	return ov
}

func overviewProbes(ov *dto.InstanceOverview) []dto.ProbeItem {
	return []dto.ProbeItem{
		{Name: "host", Status: ov.MachineOnline, Reason: ""},
		{Name: "octop", Status: ov.OctopProcess, Reason: ""},
		{Name: "http", Status: ov.HttpHealth, Reason: ""},
		{Name: "pg", Status: ov.PgConnect, Reason: ""},
	}
}

func fillThresholdDefaults(inst *entity.OctopInstance) {
	if inst.SampleIntervalSec <= 0 {
		inst.SampleIntervalSec = entity.DefaultSampleIntervalSec
	}
	if inst.CpuWarn <= 0 {
		inst.CpuWarn = entity.DefaultCpuWarn
	}
	if inst.CpuCrit <= 0 {
		inst.CpuCrit = entity.DefaultCpuCrit
	}
	if inst.MemWarn <= 0 {
		inst.MemWarn = entity.DefaultMemWarn
	}
	if inst.MemCrit <= 0 {
		inst.MemCrit = entity.DefaultMemCrit
	}
	if inst.DiskWarn <= 0 {
		inst.DiskWarn = entity.DefaultDiskWarn
	}
	if inst.DiskCrit <= 0 {
		inst.DiskCrit = entity.DefaultDiskCrit
	}
	if inst.HttpFailThreshold <= 0 {
		inst.HttpFailThreshold = entity.DefaultHttpFailThreshold
	}
	if inst.CollectEnabled == 0 {
		inst.CollectEnabled = 1
	}
	if inst.Maintenance == 0 {
		inst.Maintenance = -1
	}
}

func (a *instanceAppImpl) runGoverned(ctx context.Context, machineId uint64, cmd string, timeout time.Duration) (string, string, error) {
	cli, err := a.machineApp.GetCli(ctx, machineId)
	if err != nil {
		return "", "", err
	}
	notice, err := machineapp.CheckMachineCmd(ctx, cli.Info.CodePath, cmd)
	if err != nil {
		return "", notice, err
	}
	out, err := cli.RunWithTimeout(timeout, cmd)
	return out, notice, err
}

func trimOut(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func ptrInt8(v int8) *int8      { return &v }
func ptrF64(v float64) *float64 { return &v }
func ptrI64(v int64) *int64     { return &v }

func logCollectErr(inst *entity.OctopInstance, err error) {
	if err == nil {
		return
	}
	logx.Errorf("octop collect instance=%s err=%v", inst.Code, err)
}
