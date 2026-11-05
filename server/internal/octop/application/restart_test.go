package application

import (
	"context"
	"testing"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/i18n"
	"mayfly-go/pkg/model"

	"github.com/stretchr/testify/require"
)

func init() {
	i18n.AppendLangMsg(i18n.Zh_CN, imsg.Zh_CN)
}

func TestBuildRestartCmdAllowlist(t *testing.T) {
	cmd, err := BuildRestartCmd("octop.service")
	require.NoError(t, err)
	require.Equal(t, "systemctl restart octop.service", cmd)

	_, err = BuildRestartCmd("octop.service; rm -rf /")
	require.Error(t, err)
	_, err = BuildRestartCmd("octop && reboot")
	require.Error(t, err)
	_, err = BuildRestartCmd("")
	require.Error(t, err)
}

func TestAssertRestartAllowed(t *testing.T) {
	cli := &entity.OctopInstance{StartMode: entity.StartModeCLI, ProcessMatch: "octop"}
	err := AssertRestartAllowed(cli)
	require.Error(t, err)
	var be *errorx.BizError
	require.ErrorAs(t, err, &be)

	sys := &entity.OctopInstance{StartMode: entity.StartModeSystemd, SystemdUnit: "octop.service"}
	require.NoError(t, AssertRestartAllowed(sys))

	bad := &entity.OctopInstance{StartMode: entity.StartModeSystemd, SystemdUnit: "octop;reboot"}
	require.Error(t, AssertRestartAllowed(bad))
}

func TestProcessMatchRejectsShell(t *testing.T) {
	_, err := BuildProcessStatusCmd("octop;id")
	require.Error(t, err)
	cmd, err := BuildProcessStatusCmd("octop-api")
	require.NoError(t, err)
	require.Equal(t, "pgrep -x octop-api", cmd)
}

func TestRestartUnauthorizedDenied(t *testing.T) {
	a := &instanceAppImpl{}
	inst := &entity.OctopInstance{StartMode: entity.StartModeSystemd, SystemdUnit: "octop.service"}
	inst.Id = 8
	inst.MachineId = 0

	opsCtx := contextx.NewLoginAccount(&model.LoginAccount{Id: 99, Username: "ops"})
	err := a.AssertInstanceAccess(opsCtx, inst)
	require.Error(t, err)
	var be *errorx.BizError
	require.ErrorAs(t, err, &be)

	// 无登录账号按管理端内部任务放行；非管理员且未绑定主机必须拒绝重启入口
	require.NoError(t, a.AssertInstanceAccess(context.Background(), inst))
	adminCtx := contextx.NewLoginAccount(&model.LoginAccount{Id: 1, Username: "admin"})
	require.NoError(t, a.AssertInstanceAccess(adminCtx, inst))
}
