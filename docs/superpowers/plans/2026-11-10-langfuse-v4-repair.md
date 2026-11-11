# Langfuse v4 会话追踪修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 恢复当前 Octop 用户会话的 Langfuse 4.70.1 Trace、节点及受控正文读取，修正连接验证和项目外链。

**Architecture:** 在现有 Go HTTP 客户端内按健康接口的已验证主版本选择 v4 Observation 或经典协议。应用层统一验证 Octop 会话所有权、Langfuse 项目和结果范围，前端保持现有抽屉并接入真实游标及异步上下文守卫。

**Tech Stack:** Go 1.27、现有 Gin/IOC/GORM；Vue Composition API、TypeScript、Api 封装、Vitest。

**Spec:** 本会话已批准的六项修复方案；现场 `/api/openapi.yaml`（2026-11-10）及 `docs/octop/2026-11-10-instance-function-management-prd.md` 的隔离/脱敏边界。

## Global Constraints

- 继续当前 `feature/ver1.0` 分支；保留既有未跟踪文件，不部署、重启或 push。2026-11-11 按用户要求提交本次修复。
- 不修改 `pkg/` 或 `common/` 公共接口；后端返回 error，`biz.ErrIsNil` 仅限 API 层。
- 新前端文案使用中英文 i18n；所有 HTTP 请求经已有 Api 封装。
- 不改变 Octop/Langfuse 业务数据、凭据或部署模式，不存会话正文或认证材料。
- 保持动作权限、实例/数据库 ACL；正文仍需 `octop:content:read`。
- 无需数据库迁移；迁移唯一版本真源仍为 `server/internal/pkg/config/app.go`。

## Review Focus

- 健康 200、认证 401 时绝不标记连接就绪；项目名称不能替代真实 ID。
- 高级过滤覆盖普通参数时，项目、用户、会话、Trace 和时间边界不能丢失。
- 带游标的空页、短页、重复节点和不存在总数不能造成提前结束或假零。
- Trace/节点/正文 ID 被替换时，不能跨实例、会话或用户返回内容。
- 关闭/切换抽屉和快速切换 Trace 后，旧异步响应不得污染当前内容或 loading。

## Task 1: 认证、版本和项目验证

**Files:** `server/internal/octop/infra/langfuse/client.go`、新增 `verification.go`/对应测试；`application/read_connection.go`、DTO、模块 imsg。

**Interfaces:** 保留 `Probe(ctx) (string,error)`，新增详细验证结果（version/apiMode/projectId/projectName/checks）；配置增加预期项目 ID。前端测试结果四项 check 名称为 `health/auth/project/read`。

- [x] 先验证公开 health 成功而项目接口拒绝认证的失败测试。
- [x] 认证项目接口必须返回可识别项目；预期 ID 不匹配拒绝，未填写且凭据唯一绑定一个项目时可返回该项目供配置确认，不静默保存。
- [x] 仅明确版本选择协议，v4 探测新版读取能力；401 不回退，解析错误不吞掉。
- [x] 修改连接测试接线并保持凭据 revision 并发保护。

## Task 2: v4 读取与范围完整性

**Files:** 新增 `infra/langfuse/v4.go`、测试；更新 `application/trace.go`/测试、`domain/entity/trace.go`。

**Interfaces:** 现有 TracePage 保留；v4 Total=-1，真实 nextCursor/hasMore。ThreadTraceMeta 增加 connection/version/projectName，原 association 不再因健康成功而置 ready。

- [x] 先验证旧客户端请求 v4 被 404 拒绝的回归测试。
- [x] 根 Observation 按 traceId 合并成 Trace；节点和正文使用同一项目/会话/用户/时间范围查询并逐行复核。
- [x] 使用 fromStartTime/toStartTime、fields 和 cursor；列表不请求 io。原生游标不转页码或伪造总数。
- [x] 直接按目标 Trace 查询归属，消除“只查前100条”导致末页 Trace 无法打开的问题。
- [x] 项目共用但没有实例隔离依据时拒绝关联；节点/正文必须精确复核 ID 与范围。
- [x] 缺少会话创建时间时明确显示有限时间覆盖；查询时间非法时返回请求错误，不扩大查询。
- [x] 外链包含经过验证的真实 projectId 和编码后的 sessionId。

## Task 3: 前端追踪抽屉

**Files:** `frontend/src/views/octop/user/SessionTraceDrawer.vue`、相关 composable/测试、`types.ts`、中英文 `octop.ts`、`ReadConnectionForm.vue`。

**Interfaces:** API 端点不变。下一页发送 cursor 和首次 coverageStart/coverageEnd；ReadConnectionTestResult 接收真实项目和四项验证结果。

- [x] 挂载/交互测试先复现状态优先级、分页、晚响应、循环树问题。
- [x] 分开显示连接能力与会话状态；当前读取结果优先，失败不冒充无 Trace。
- [x] Trace 与 Observation 分别继续加载、按 ID 去重、标注未完整；关闭和上下文变化清空内容。
- [x] 上下文序号保护每个 await 与 finally，树结构处理缺父节点和循环。
- [x] 展示配置验证结果，不自动改写项目或保存密钥。

## Task 4: 集成验证与审阅

- [x] 后端：`go test -p 1 ./internal/octop/... -count=1`。
- [x] 前端：新增交互测试、菜单/i18n 回归、type-check、lint、build；报告与本改动无关的既有错误。
- [x] 独立只读代码审阅，修正重要问题后复跑对应测试。
- [x] 核对 Git 改动范围，报告未部署和现场认证数据验收限制。

## 执行记录

- 基线：`8caaa58c`；开始时仅有交接文档和 `frontend/.vitest/` 未跟踪。
- 现场：Langfuse 4.70.1，旧 traces/observations 返回 events_only 404；新 v2 observations 路由存在。
- 前端任务交由独立子任务处理，后端由主任务处理，文件所有权分离。

## 最终实现与验证记录

- 后端按已验证主版本选择协议；v4 使用 `/api/public/v2/observations`，实际部署 OpenAPI 已复核字段组、root 筛选及高级 filter 列。保留明确 v2/v3 经典读取路径。
- 会话列表和节点读取传递真实 cursor，缺省窗口有界；节点、正文继承首次查询窗口。正文按需读取，保留权限与脱敏；未取得总数使用 -1。
- 会话归属按目标 Trace 精确查询，空游标页继续核验，设 32 页上限、超时和重复游标保护。
- 同地址下其他实例必须有当前 revision 的新认证/项目验证证明。证明复用 `ExtraJSON`，只含协议号、真实项目 ID 与 revision；旧 health-only 就绪标记不作为项目独立性的依据。无需迁移。
- 独立审阅发现的四项 P2（跨页 Trace 聚合、明文认证头脱敏、未验证 peer 项目、归属查询空游标页）均补失败回归并修复；复审未发现剩余确定性 P1/P2。结构化 Proxy-Authorization/Set-Cookie 同类脱敏已补测。
- 最终后端 `go test -p 1 ./internal/octop/... -count=1` 通过；前端四文件 47/47 测试通过，9 个改动文件定向 ESLint 通过，`pnpm build` 通过（产物 `frontend/dist`），Git 差异空白检查通过。
- 全项目 `pnpm type-check` 仍报既有 `PropSettingDrawer.vue:120` 与 `InstanceEdit.vue` 的类型错误，本次改动文件没有类型错误。扩展组件事件契约守卫存在既有的 Windows 路径豁免不匹配，未修改相关三个组件。
- 后端完整入口构建首次被本地缺少非 Octop 模块依赖的 go.sum 校验记录阻塞；使用临时 modfile 补齐依赖后 `go build main.go` 编译通过，未修改项目依赖文件。

## 部署后验收顺序

1. 用完整源码构建流程重新打包前端并嵌入后端，发布新二进制/镜像；不可复用旧前端包。
2. 在实例读取连接中使用真实 Langfuse 项目 ID；若此前填的是项目名称，可先测试读取返回的真实 ID，手工填入后保存。
3. 保存后再执行一次“测试读取”，确认健康、认证、项目、读取能力四项通过；同一 Langfuse 地址下的其他实例也应完成此验证，并使用各自独立项目。候选配置测试不生成当前已保存 revision 的证明。
4. 对已知用户与会话检查 Trace 列表、节点树、继续加载和按需正文；外链应包含真实 projectId。上报映射须满足 `sessionId=thread_id`、`userId=Octop 用户 ID`，无法证明时不扩大查询。
5. 本次未部署、重启或 push；2026-11-11 按用户要求提交本次修复。未使用现场密钥读取真实会话，现场 session/user 映射与真实数据验收仍需部署后完成。
