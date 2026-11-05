package application

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/errorx"
)

var (
	unitNameRe    = regexp.MustCompile(`^[A-Za-z0-9:._@\\-]+$`)
	processNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// BuildRestartCmd 仅允许固定 allowlist 的 systemd 重启，拒绝任意 shell。
func BuildRestartCmd(unit string) (string, error) {
	if err := validateUnit(unit); err != nil {
		return "", err
	}
	return "systemctl restart " + unit, nil
}

func BuildUnitStatusCmd(unit string) (string, error) {
	if err := validateUnit(unit); err != nil {
		return "", err
	}
	return "systemctl show " + unit + " --property=ActiveState,SubState,MainPID,NRestarts,ActiveEnterTimestamp", nil
}

func BuildProcessStatusCmd(name string) (string, error) {
	if !processNameRe.MatchString(name) {
		return "", errorx.NewBizI(context.Background(), imsg.ErrProcessMatchInvalid)
	}
	return "pgrep -x " + name, nil
}

func validateUnit(unit string) error {
	unit = strings.TrimSpace(unit)
	if unit == "" || !unitNameRe.MatchString(unit) || strings.ContainsAny(unit, " \t\n;&|$`") {
		return errorx.NewBizI(context.Background(), imsg.ErrRestartUnitInvalid)
	}
	return nil
}

func AssertRestartAllowed(inst *entity.OctopInstance) error {
	if inst == nil {
		return errorx.NewBizI(context.Background(), imsg.ErrInstanceNotFound)
	}
	if !inst.IsSystemd() {
		return errorx.NewBizI(context.Background(), imsg.ErrRestartNotSystemd)
	}
	return validateUnit(inst.SystemdUnit)
}

func parseSystemdShow(out string) (active string, pid int64, restarts int64, uptimeSec int64) {
	now := time.Now()
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "ActiveState":
			active = v
		case "MainPID":
			fmt.Sscanf(v, "%d", &pid)
		case "NRestarts":
			fmt.Sscanf(v, "%d", &restarts)
		case "ActiveEnterTimestamp":
			if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", v); err == nil && !t.IsZero() {
				uptimeSec = int64(now.Sub(t).Seconds())
			}
		}
	}
	return
}
