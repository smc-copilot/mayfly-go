package v112

import (
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
			// Octop 实例回调地址 / API Token，Casdoor 应用 redirectUris 白名单，用户列表菜单。
			ID: "v1.12.3-octop-bind-and-users",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(
					&octopentity.OctopInstance{},
					&octopentity.CasdoorApplicationSnapshot{},
				); err != nil {
					return err
				}
				return migrationx.InsertResourcesWithRole(tx, octopBindUserResources)
			},
			Rollback: migrationx.NoopRollback,
		},
	}
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

var octopBindUserResources = []migrationx.ResourceDef{
	migrationx.Resource(1750000006, 1750000001, "Octop001/User0001/", 1, "menu.octopUsers", "octop-users", 35000000, `{"component":"octop/users/UserList","icon":"UserFilled","isKeepAlive":true,"routeName":"OctopUsers"}`),
	migrationx.Resource(1750000021, 1750000006, "Octop001/User0001/Us00002/", 2, "menu.octopUsersBase", "octop:users:base", 10000000, ""),
}
