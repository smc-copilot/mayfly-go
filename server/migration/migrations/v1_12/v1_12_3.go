package v112

import (
	"fmt"
	"strings"
	"time"

	casdoorentity "mayfly-go/internal/casdoor/domain/entity"
	dbentity "mayfly-go/internal/db/domain/entity"
	octopentity "mayfly-go/internal/octop/domain/entity"
	"mayfly-go/migration/migrations/migrationx"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// V1_12_3 v1.12.3 批次迁移。
//
// 批次号跟随应用版本（internal/pkg/config/app.go 的 Version），ID 前缀与批次版本一致。
// 本批次新增迁移统一收口在本文件，不按功能拆散；所有迁移自身幂等，
// 重复执行安全（由全链直调重跑用例覆盖）。
func v1123() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			// 数据库实例 TLS 证书材料列。
			//
			// 档位/主机名/是否用系统 CA/指纹是短标量，归实例 extra（与机器 SOCKS5 代理
			// 同样的存法：可读、可随列表回显）；而 CA/客户端证书/私钥合起来可达数 KB，
			// extra 是 varchar(2000) 装不下——与 db_transfer.go 中「表名清单独立成表
			// 而非进 Extra」是同一个判据。材料以 AES 密文整体存储，实体上带 json:"-"，
			// 既不随详情接口下发，也不参与任何列表查询。
			ID: "v1.12.3-db-instance-tls-material",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&dbentity.DbInstance{}, "tls_material") {
					return nil
				}
				return tx.Migrator().AddColumn(&dbentity.DbInstance{}, "TLSMaterialEnc")
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// SQL 执行记录的语句列从 varchar(5000) 扩为 mediumtext。
			//
			// 单条语句超过 5000 字符（批量 INSERT、长 DDL）时，整条审计记录会写不进去，
			// 而失败只落一条 error 日志——表现是「SQL 确实执行了，审计里却查不到」。
			// AutoMigrate 对已有列按类型比对执行改列、对全新库直接建出目标结构，天然幂等；
			// sqlite 侧 mediumtext 属 TEXT 亲和性，不引入方言差异（与 v1.12.1 扩 old_value 同一手法）。
			ID: "v1.12.3-db-sql-exec-sql-mediumtext",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&dbentity.DbSQLExec{})
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// 表访问控制表。
			//
			// 规则按 (库, 表, 角色集合) 存一行：角色集合进 json 列而不是拆成关系表，
			// 是因为它只参与「是否命中」的一次性判定，没有按角色反查表的需求。
			// AutoMigrate 对已有表按列比对补齐、对全新库直接建出目标结构，天然幂等。
			ID: "v1.12.3-db-table-acl",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&dbentity.DbTableAcl{})
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// DbmsConfig 补「指标采样周期 / 保留天数」两个配置项。
			//
			// 只追加字段定义、不动已有取值；按 prop 去重，重复执行安全。
			ID: "v1.12.3-dbms-config-metric-sampling",
			Migrate: func(tx *gorm.DB) error {
				return migrationx.AppendConfigFields(tx, "DbmsConfig", []map[string]any{
					{
						"prop":    "metricSampleMinutes",
						"label":   "system.sysconf.metricSampleMinutes",
						"tooltip": "system.sysconf.metricSampleMinutesPlaceholder",
					},
					{
						"prop":    "metricRetainDays",
						"label":   "system.sysconf.metricRetainDays",
						"tooltip": "system.sysconf.metricRetainDaysPlaceholder",
					},
				}, nil)
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// 实例指标采样点表（趋势看板）。
			//
			// 与告警评估器分开落库：评估按规则现算、图表按固定周期落点，
			// 两边的采集频率与保留期不同，共用一张表会互相绑架。
			ID: "v1.12.3-db-metric-sample",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&dbentity.DbMetricSample{})
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// 表访问控制补「规则类型」列：1 禁止（默认）/ 2 允许。
			//
			// 老数据没有这一列，按默认值 1（黑名单）解释，语义不变；
			// 只有管理员显式加了 allow 规则，该库才切换到「白名单 + 默认拒绝」
			ID: "v1.12.3-db-table-acl-effect",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&dbentity.DbTableAcl{}, "effect") {
					return nil
				}
				return tx.Migrator().AddColumn(&dbentity.DbTableAcl{}, "Effect")
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// 列级访问控制表（与表级同构，粒度到列）。
			ID: "v1.12.3-db-column-acl",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&dbentity.DbColumnAcl{})
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// DbmsConfig 补「执行记录保留天数」配置项。
			//
			// 默认不清理（字段缺省即为 0）：到期自动删除审计记录属于销毁证据，必须由管理员显式设值。
			// 只在 params 里追加字段定义，不动已有取值；按 prop 去重，重复执行安全。
			ID: "v1.12.3-dbms-config-sql-exec-save-days",
			Migrate: func(tx *gorm.DB) error {
				return migrationx.AppendConfigFields(tx, "DbmsConfig", []map[string]any{
					{
						"prop":    "sqlExecSaveDays",
						"label":   "system.sysconf.sqlExecSaveDays",
						"tooltip": "system.sysconf.sqlExecSaveDaysPlaceholder",
					},
				}, nil)
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// AiAgentConfig 配置行的权限归一与去重。
			//
			// 该行由 AI 设置页首次保存时创建，早期插入没带 permission（空串），而系统
			// 配置列表接口按 permission 过滤（all 或含当前用户名的清单），空权限的行对
			// 任何账号都不可见 —— 设置页因此每次都判为「配置不存在」再插入一行：重复行
			// 无限增长，且后端按 key 取首行永远读到最旧那行，管理员改的 Agent 配置静默
			// 不生效（保存成功、日志无异常，只有真机改配置才能发现）。
			//
			// 处置：保留最早一行（后端按 key 读取的稳定目标，并继承最新一行的取值，不丢
			// 故障期间改过的内容），权限归一到 admin,（与 Oauth2Login / DbmsConfig 同口径：
			// 管理员设置页可见可改、公开 value 接口不回显），其余重复行软删；行不存在时
			// 按同一口径补建，让全新库直接可用。
			ID: "v1.12.3-ai-agent-config-permission-dedupe",
			Migrate: func(tx *gorm.DB) error {
				return migrationx.NormalizeSingleConfigRow(tx, migrationx.ConfigRowSpec{
					Key:          "AiAgentConfig",
					Name:         "ai.settings.agentConfig",
					Permission:   "admin,",
					DefaultValue: "{}",
				})
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// AiModelConfig 权限从 all 收紧到 admin,。
			//
			// 该配置行的值里含 LLM 的 apiKey，而 permission=all 会被免鉴权的
			// GET /sys/configs/value 原样下发 —— 未登录即可读到密钥。前端探测
			// 「AI 是否已配置」已改走只回布尔值的 GET /ai/status，不再需要公开原文；
			// admin, 与 Oauth2Login / DbmsConfig 等管理类配置同口径：管理员设置页
			// 可见可改，公开 value 接口不回显。重复行的收敛规则与 AiAgentConfig 一致。
			ID: "v1.12.3-ai-model-config-permission-restrict",
			Migrate: func(tx *gorm.DB) error {
				return migrationx.NormalizeSingleConfigRow(tx, migrationx.ConfigRowSpec{
					Key:          "AiModelConfig",
					Name:         "system.sysconf.aiModelConf",
					Permission:   "admin,",
					DefaultValue: "{}",
				})
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// Octop 运维、Casdoor 目录与业务事件采集表。
			// AutoMigrate 对已有表按列比对补齐、对全新库直接建出目标结构，天然幂等。
			ID: "v1.12.3-octop-tables",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&octopentity.OctopInstance{},
					&octopentity.CasdoorConnection{},
					&octopentity.CasdoorApplicationSnapshot{},
					&octopentity.CasdoorUserSnapshot{},
					&octopentity.CasdoorAuthRelation{},
					&octopentity.OctopIdentityLink{},
					&octopentity.OctopEvent{},
					&octopentity.CollectorCheckpoint{},
					&octopentity.CollectorRun{},
					&octopentity.OctopMetricSample{},
					&octopentity.OctopSettings{},
				)
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// Octop 管理菜单与权限码。权限码 fail-closed：路由声明了 RequiredPermissionCode
			// 就必须同时写入 t_sys_resource，否则除已授权角色外全员 403。
			ID: "v1.12.3-octop-resources",
			Migrate: func(tx *gorm.DB) error {
				return migrationx.InsertResourcesWithRole(tx, octopResources)
			},
			Rollback: migrationx.NoopRollback,
		},

		{
			// Casdoor 目录：独立应用 owner、分项同步状态、关系核实字段。
			// AutoMigrate 对已有连接行按列比对补齐，新列为空/0，不改写 ClientSecretEnc 与旧快照。
			ID: "v1.12.3-casdoor-directory-sync-fields",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&octopentity.CasdoorConnection{},
					&octopentity.CasdoorApplicationSnapshot{},
					&octopentity.CasdoorUserSnapshot{},
					&octopentity.CasdoorAuthRelation{},
				)
			},
			Rollback: migrationx.NoopRollback,
		},
		{
			ID: "v1.12.3-casdoor-core-schema",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&casdoorentity.Connection{},
					&casdoorentity.Application{},
					&casdoorentity.User{},
					&casdoorentity.AuthRelation{},
					&casdoorentity.DirectoryObject{},
					&casdoorentity.ConnectionScope{},
					&casdoorentity.ManagedApplication{},
					&casdoorentity.Change{},
				)
			},
			Rollback: migrationx.NoopRollback,
		},
		{
			ID: "v1.12.3-casdoor-directory-data",
			Migrate: func(tx *gorm.DB) error {
				return tx.Transaction(copyCasdoorDirectory)
			},
			Rollback: migrationx.NoopRollback,
		},
		{
			ID: "v1.12.3-casdoor-management-resources",
			Migrate: func(tx *gorm.DB) error {
				return tx.Transaction(migrateCasdoorResources)
			},
			Rollback: migrationx.NoopRollback,
		},
		{
			ID:       "v1.12.3-casdoor-object-sync-state",
			Migrate:  func(tx *gorm.DB) error { return tx.AutoMigrate(&casdoorentity.ObjectSyncState{}) },
			Rollback: migrationx.NoopRollback,
		},
	}
}

// copyCasdoorDirectory 保留旧目录的主键及全部公共列。目标行不一致时停止，
// 不覆盖目标库中的数据；SQL 只绑定主键，避免将凭证材料写入日志。
func copyCasdoorDirectory(tx *gorm.DB) error {
	tables := [][2]string{
		{"t_octop_casdoor_connection", "t_casdoor_connection"},
		{"t_octop_casdoor_app", "t_casdoor_application"},
		{"t_octop_casdoor_user", "t_casdoor_user"},
		{"t_octop_casdoor_auth", "t_casdoor_auth_relation"},
	}
	for _, table := range tables {
		if err := copyCasdoorTable(tx, table[0], table[1]); err != nil {
			return err
		}
	}
	return nil
}

func copyCasdoorTable(tx *gorm.DB, source, target string) error {
	if !tx.Migrator().HasTable(source) || !tx.Migrator().HasTable(target) {
		return fmt.Errorf("casdoor directory migration requires tables %s and %s", source, target)
	}
	sourceTypes, err := tx.Migrator().ColumnTypes(source)
	if err != nil {
		return err
	}
	targetTypes, err := tx.Migrator().ColumnTypes(target)
	if err != nil {
		return err
	}
	targetColumns := make(map[string]bool, len(targetTypes))
	for _, column := range targetTypes {
		targetColumns[column.Name()] = true
	}
	columns := make([]string, 0, len(sourceTypes))
	quoted := make([]string, 0, len(sourceTypes))
	for _, column := range sourceTypes {
		name := column.Name()
		if !targetColumns[name] {
			return fmt.Errorf("casdoor directory target %s lacks column %s", target, name)
		}
		columns = append(columns, name)
		quoted = append(quoted, tx.Statement.Quote(name))
	}
	var sourceRows []map[string]any
	if err := tx.Table(source).Find(&sourceRows).Error; err != nil {
		return err
	}
	for _, row := range sourceRows {
		id, ok := row["id"]
		if !ok || id == nil {
			return fmt.Errorf("casdoor directory source %s has a row without id", source)
		}
		var existing []map[string]any
		if err := tx.Table(target).Where("id = ?", id).Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) > 0 {
			for _, name := range columns {
				if casdoorMigrationValue(row[name]) != casdoorMigrationValue(existing[0][name]) {
					return fmt.Errorf("casdoor directory target %s id %v conflicts with source", target, id)
				}
			}
			continue
		}
		columnSQL := strings.Join(quoted, ", ")
		query := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s WHERE %s = ?",
			tx.Statement.Quote(target), columnSQL, columnSQL, tx.Statement.Quote(source), tx.Statement.Quote("id"))
		if err := tx.Exec(query, id).Error; err != nil {
			return err
		}
	}
	if tx.Dialector.Name() == "postgres" {
		quotedTarget := tx.Statement.Quote(target)
		query := fmt.Sprintf("SELECT setval(pg_get_serial_sequence(?, 'id'), COALESCE(MAX(id), 1), COUNT(*) > 0) FROM %s", quotedTarget)
		if err := tx.Exec(query, target).Error; err != nil {
			return err
		}
	}
	return nil
}

func casdoorMigrationValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null:"
	case []byte:
		return "value:" + string(v)
	case time.Time:
		return "time:" + v.UTC().Format(time.RFC3339Nano)
	default:
		return "value:" + fmt.Sprint(v)
	}
}

// migrateCasdoorResources 迁移旧入口及既有读取角色；新增远端写权限不自动授权。
func migrateCasdoorResources(tx *gorm.DB) error {
	const rootID int64 = 1750000004
	const oldReadID int64 = 1750000015
	const oldSaveID int64 = 1750000016
	var count int64
	if err := tx.Table("t_sys_resource").Where("id = ? AND is_deleted = 0", rootID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("existing casdoor menu is missing or deleted")
	}
	if err := tx.Table("t_sys_resource").Where("code = ? AND id <> ? AND is_deleted = 0", "/casdoor", rootID).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("casdoor menu path already exists")
	}
	var descendants []struct {
		Id     int64
		UiPath string
	}
	const oldPrefix = "Octop001/Casd0001/"
	const newPrefix = "Casd0001/"
	if err := tx.Table("t_sys_resource").Select("id, ui_path").
		Where("ui_path LIKE ? AND id <> ? AND is_deleted = 0", oldPrefix+"%", rootID).Find(&descendants).Error; err != nil {
		return err
	}
	for _, row := range descendants {
		path := newPrefix + strings.TrimPrefix(row.UiPath, oldPrefix)
		if err := tx.Table("t_sys_resource").Where("id = ?", row.Id).Update("ui_path", path).Error; err != nil {
			return err
		}
	}
	if err := tx.Table("t_sys_resource").Where("id = ?", rootID).Updates(map[string]any{
		"pid": 0, "ui_path": newPrefix, "code": "/casdoor", "name": "menu.casdoor",
		"meta": `{"icon":"User","routeName":"CasdoorCenter","redirect":"/casdoor/applications"}`,
	}).Error; err != nil {
		return err
	}
	resources := []migrationx.ResourceDef{
		migrationx.Resource(1750010001, rootID, "Casd0001/Conn0001/", 1, "menu.casdoorConnections", "connections", 10000000, `{"component":"casdoor/connections/ConnectionList","routeName":"CasdoorConnectionList"}`),
		migrationx.Resource(1750010002, rootID, "Casd0001/Apps0001/", 1, "menu.casdoorApplications", "applications", 20000000, `{"component":"casdoor/applications/ApplicationList","routeName":"CasdoorApplicationList"}`),
		migrationx.Resource(1750010003, rootID, "Casd0001/Users001/", 1, "menu.casdoorUsers", "users", 30000000, `{"component":"casdoor/users/UserList","routeName":"CasdoorUserList"}`),
		migrationx.Resource(1750010004, rootID, "Casd0001/Access01/", 1, "menu.casdoorAccess", "access", 40000000, `{"component":"casdoor/access/AccessManagement","routeName":"CasdoorAccessManagement"}`),
		migrationx.Resource(1750010005, rootID, "Casd0001/Ops00001/", 1, "menu.casdoorOperations", "operations", 50000000, `{"component":"casdoor/operations/OperationList","routeName":"CasdoorOperationList"}`),
		migrationx.Resource(1750010010, rootID, "Casd0001/Perm0010/", 2, "menu.casdoorRead", "casdoor:read", 10000000, ""),
		migrationx.Resource(1750010011, rootID, "Casd0001/Perm0011/", 2, "menu.casdoorConnectionManage", "casdoor:connection:manage", 11000000, ""),
		migrationx.Resource(1750010012, rootID, "Casd0001/Perm0012/", 2, "menu.casdoorConnectionScope", "casdoor:connection:scope", 12000000, ""),
		migrationx.Resource(1750010013, rootID, "Casd0001/Perm0013/", 2, "menu.casdoorSync", "casdoor:sync", 13000000, ""),
		migrationx.Resource(1750010014, rootID, "Casd0001/Perm0014/", 2, "menu.casdoorUserManage", "casdoor:user:manage", 14000000, ""),
		migrationx.Resource(1750010015, rootID, "Casd0001/Perm0015/", 2, "menu.casdoorUserStatus", "casdoor:user:status", 15000000, ""),
		migrationx.Resource(1750010016, rootID, "Casd0001/Perm0016/", 2, "menu.casdoorApplicationManage", "casdoor:application:manage", 16000000, ""),
		migrationx.Resource(1750010017, rootID, "Casd0001/Perm0017/", 2, "menu.casdoorAccessManage", "casdoor:access:manage", 17000000, ""),
		migrationx.Resource(1750010018, rootID, "Casd0001/Perm0018/", 2, "menu.casdoorAuditRead", "casdoor:audit:read", 18000000, ""),
		migrationx.Resource(1750010019, rootID, "Casd0001/Perm0019/", 2, "menu.casdoorIntegrationManage", "casdoor:integration:manage", 19000000, ""),
	}
	for _, resource := range resources {
		var existing []struct {
			Id   int64
			Pid  int64
			Type int8
			Code string
		}
		if err := tx.Table("t_sys_resource").Select("id, pid, type, code").
			Where("id = ?", resource.Id).Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) > 0 && (existing[0].Pid != resource.Pid || existing[0].Type != resource.Typ || existing[0].Code != resource.Code) {
			return fmt.Errorf("casdoor resource id %d already belongs to a different resource", resource.Id)
		}
		query := tx.Table("t_sys_resource").Where("code = ? AND id <> ? AND type = ? AND is_deleted = 0", resource.Code, resource.Id, resource.Typ)
		if resource.Typ == 1 {
			query = query.Where("pid = ?", resource.Pid)
		}
		if err := query.Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("casdoor resource code %s already exists", resource.Code)
		}
		if err := migrationx.InsertResource(tx, resource); err != nil {
			return err
		}
	}
	for _, resourceID := range []int64{rootID, 1750010001, 1750010002, 1750010003, 1750010004, 1750010010} {
		if err := migrationx.GrantResourceToParentRoles(tx, oldReadID, resourceID); err != nil {
			return err
		}
	}
	for _, resourceID := range []int64{1750010011, 1750010013} {
		if err := migrationx.GrantResourceToParentRoles(tx, oldSaveID, resourceID); err != nil {
			return err
		}
	}
	return nil
}

var octopResources = []migrationx.ResourceDef{
	migrationx.Resource(1750000001, 0, "Octop001/", 1, "menu.octop", "/octop", 49999990, `{"icon":"Monitor","isKeepAlive":true,"routeName":"Octop"}`),
	migrationx.Resource(1750000002, 1750000001, "Octop001/Ov00001/", 1, "menu.octopOverview", "octop-overview", 10000000, `{"component":"octop/overview/Overview","icon":"DataBoard","isKeepAlive":true,"routeName":"OctopOverview"}`),
	migrationx.Resource(1750000003, 1750000001, "Octop001/Inst0001/", 1, "menu.octopInstance", "octop-instance", 20000000, `{"component":"octop/instance/InstanceDetail","icon":"OfficeBuilding","isKeepAlive":true,"routeName":"OctopInstanceDetail"}`),
	migrationx.Resource(1750000004, 1750000001, "Octop001/Casd0001/", 1, "menu.octopCasdoor", "octop-casdoor", 30000000, `{"component":"octop/casdoor/CasdoorDirectory","icon":"User","isKeepAlive":true,"routeName":"OctopCasdoor"}`),
	migrationx.Resource(1750000005, 1750000001, "Octop001/Evnt0001/", 1, "menu.octopEvent", "octop-event", 40000000, `{"component":"octop/event/EventList","icon":"Document","isKeepAlive":true,"routeName":"OctopEvent"}`),

	migrationx.Resource(1750000010, 1750000002, "Octop001/Ov00001/Ov00002/", 2, "menu.octopOverviewBase", "octop:overview:base", 10000000, ""),
	migrationx.Resource(1750000011, 1750000003, "Octop001/Inst0001/In00002/", 2, "menu.octopInstanceBase", "octop:instance:base", 10000000, ""),
	migrationx.Resource(1750000012, 1750000003, "Octop001/Inst0001/In00003/", 2, "menu.octopInstanceSave", "octop:instance:save", 20000000, ""),
	migrationx.Resource(1750000013, 1750000003, "Octop001/Inst0001/In00004/", 2, "menu.octopInstanceDel", "octop:instance:del", 30000000, ""),
	migrationx.Resource(1750000014, 1750000003, "Octop001/Inst0001/In00005/", 2, "menu.octopInstanceRestart", "octop:instance:restart", 40000000, ""),
	migrationx.Resource(1750000015, 1750000004, "Octop001/Casd0001/Ca00002/", 2, "menu.octopCasdoorBase", "octop:casdoor:base", 10000000, ""),
	migrationx.Resource(1750000016, 1750000004, "Octop001/Casd0001/Ca00003/", 2, "menu.octopCasdoorSave", "octop:casdoor:save", 20000000, ""),
	migrationx.Resource(1750000017, 1750000005, "Octop001/Evnt0001/Ev00002/", 2, "menu.octopEventBase", "octop:event:base", 10000000, ""),
	migrationx.Resource(1750000018, 1750000005, "Octop001/Evnt0001/Ev00003/", 2, "menu.octopEventExport", "octop:event:export", 20000000, ""),
	migrationx.Resource(1750000019, 1750000005, "Octop001/Evnt0001/Ev00004/", 2, "menu.octopCollectRecollect", "octop:collect:recollect", 30000000, ""),
	migrationx.Resource(1750000020, 1750000002, "Octop001/Ov00001/Ov00003/", 2, "menu.octopConfigSave", "octop:config:save", 20000000, ""),
}
