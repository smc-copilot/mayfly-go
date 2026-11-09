# Casdoor 独立管理模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.
>
> **版本 R3：保留 R2 的 Casdoor 独立模块边界，补齐页面 Spec、页面接口与验收场景。**
>
> **实施准备复核补充：明确命令目标、批量结果归并、写入准入和阶段交付条件。**
>
> **状态：实施中。** 用户已明确要求按实施计划与页面 Spec 开始功能实施；生产部署和生产应用接管仍按下方发布边界执行。

**Goal:** 在 Mayfly-Go 建立独立的 Casdoor 管理模块，通过组织级管理连接管理 Casdoor 应用、用户、角色与访问权限；业务系统可选接入关联，管理功能不要求存在任何 Octop 实例。

**Architecture:** 后端独立于 server/internal/casdoor，前端独立于 frontend/src/views/casdoor，API 统一 /casdoor，数据使用 t_casdoor_*。Casdoor 是用户与授权事实源，Mayfly 保存投影和管理操作记录。Octop 作为可选集成，通过独立适配器消费 Casdoor 公共应用服务；Casdoor 核心不依赖 Octop。

**Tech Stack:** Go / Gin / GORM / 现有 IOC；Vue 3 / TypeScript / Element Plus / Pinia / Api 封装；Casdoor REST API。沿用仓库锁定版本；当前 server/go.mod 声明 go 1.27，不自行降级。

**Spec:** 本文件“需求与模块边界”“接口及数据契约”，以及 [Casdoor 管理中心页面 Spec P1](../specs/2026-11-09-casdoor-management-center-pages.md)。页面字段、状态、交互及验收以 P1 为准；后端范围及分层以本计划为准。

## Global Constraints

- 编写时工作分支 feature/ver1.0、HEAD cfcb4d2c；实施前重新读取工作区、版本和用户修改。
- Casdoor 连接、应用、用户、角色、群组、权限、同步、审计均归 Casdoor 模块所有；新增核心代码不得放入其他业务模块。
- server/internal/casdoor 不导入 internal/octop；frontend/src/views/casdoor 不导入 views/octop；不得通过同义别名绕过依赖检查。
- 后端遵循 api → application → domain → infra；application/domain/infra 返回 error，不能使用 biz.ErrIsNil；所有错误必须处理。
- 前端所有展示文本使用 t() / $t()，中英文同步；通过 Api 封装调用，前端不持有管理密钥。
- 页面必须覆盖 P1 的 UI-01 至 UI-15；禁止将五个菜单名和组件清单作为完整页面规格。复杂变更不能套用普通表单“HTTP 成功即关闭”的默认成功流程。
- 不修改 pkg/ 或 common/ 公共接口；确需修改时另行列明影响请求批准。跨模块契约定义在 Casdoor 模块自身，无需改公共包。
- 迁移以 server/internal/pkg/config/app.go 当前 Version 为准。编写时为 v1.12.3，仅追加 server/migration/migrations/v1_12/v1_12_3.go，ID 前缀 v1.12.3-，不改历史迁移 ID、顺序及定义。
- 旧目录数据迁入通用表时保留连接/快照 ID、密钥密文和稳定源 ID；既有业务关联不得丢失。旧表保留作恢复依据，切换后不再作为运行时事实源。
- 权限由 Casdoor 管理动作权限及组织连接范围决定；Casdoor 用户/授权操作不以 SSH、主机或 Octop 实例权限为前提。
- 可选业务关联操作额外检查目标系统资源权限；这类检查只约束关联变更及资源信息展示，不替代 Casdoor 管理权限。
- 自动同步只读；迁移和启动不创建远端用户、应用或授权。
- 实际环境兼容性和隔离测试结果必须按阶段记录；生产发布和应用接管按下方发布边界执行。

## Review Focus

1. 不注册 Octop 适配器、不存在业务实例时，目录、用户和授权管理仍可运行；非 Octop 应用不能被过滤（任务 1、3、5、9）。
2. 历史数据迁入通用表后原 ID、密钥和业务关联保持，重跑迁移不覆盖新数据或重复建菜单（任务 2、8、9）。
3. 已保存密钥可用于原目标测试，目标变化时不可复用旧密钥；新旧路由都执行组织范围检查（任务 3、8）。
4. HTTP 200 业务错误、分页中断、写超时及进程中断不得假成功、清空成功快照或重复写入（任务 3、4、5）。
5. 角色成员变更、有效策略和业务系统已有会话是不同结果；外部权限、默认允许、管理员豁免及并发修改必须明确呈现（任务 5、9）。

---

## 需求与模块边界

### 1. 产品定位

Casdoor 管理中心是 Mayfly 的独立管理模块，管理对象来自 Casdoor：

- 一个服务/组织管理连接读取该组织的全部可管理应用和用户。
- 管理用户资料、启停、角色、群组及应用访问权限。
- 应用可以是 Octop，也可以是其他接入 Casdoor 的业务系统；无需登记业务实例即可进行 Casdoor 管理。
- “应用关联业务资源”是可选集成能力，不能变成应用存在、用户授权或目录同步的前置条件。
- 后续中央任务层和 Agent 复用 Casdoor 管理服务，不通过业务应用模块中转。

### 2. 本次范围

包含：

- 连接与组织范围管理，读取能力测试，定时/手动同步，多连接切换。
- 所有受管业务应用的列表/详情与必要字段编辑，包括未关联任何业务实例的应用。
- 用户列表/详情、新建、资料编辑、组织账号启停；新建需存在已验证的无明文密码激活/外部认证路径。
- 角色、群组、权限读取和来源展示；按用户授予/撤销受管应用访问资格。
- 逐应用初始化 Mayfly 专用角色/权限；变更预览、幂等执行、回读和审计。
- 现有目录数据与入口升级，已有关联保留。
- 一个 Octop 可选适配器，负责其资源选择、应用关联和身份关联的既有能力；该适配器不是 Casdoor 核心依赖。

后续单独扩展：

- 新建业务应用及自动生成/分发完整 OIDC 配置，删除应用/用户、密码重置。
- 群组成员写入、部门批量授权、任意模型编辑、跨组织共享应用授权。
- 各业务系统即时注销本地会话、修改其本地登录入口或运行配置。
- 新增 Casdoor 服务端接口、MCP/Agent 编排、多级审批。发现现有接口无法安全实施时先报告，再审核调整。

本期“应用管理”覆盖所有应用类型；“不新建应用”是操作范围限制，与业务系统类型无关。

### 3. 独立入口

Mayfly 主导航“Casdoor 管理中心”，同级于其他管理模块：

- /casdoor/connections：连接管理。
- /casdoor/applications：应用管理。
- /casdoor/users：用户管理。
- /casdoor/access：访问授权；角色、群组和权限作为标签或抽屉。
- /casdoor/operations：操作记录。

当前连接选择器贯穿各页面；切换后清空前一连接选择，忽略旧请求晚到结果。连接管理列表展示全部可见连接，其余页面按当前连接过滤。应用页默认显示组织内全部受管应用，不按是否存在本地实例筛选。五个页面的表格列、筛选、表单、详情、权限及异常表现见页面 Spec P1 第 1–10 节。

### 4. 代码归属与依赖方向

~~~text
Mayfly
├─ Casdoor 管理模块
│  ├─ 独立菜单、页面、API
│  ├─ 连接/组织/应用/用户
│  ├─ 角色/群组/权限/同步
│  └─ 预览/执行/审计
├─ 业务运维模块
│  └─ Octop 等业务系统
└─ 可选集成适配器
   └─ casdooroctop：消费 Casdoor 契约与 Octop 服务
~~~

目录责任：

- server/internal/casdoor/api：/casdoor API、表单、仅用于历史目录的兼容路由。
- application：连接、目录同步、管理命令、访问解释、范围检查。
- domain/entity、domain/repository：通用数据模型和仓储。
- domain/integration：可选集成契约，不含任何具体业务系统实体。
- infra/client、infra/persistence：Casdoor HTTP 适配与本地持久化。
- imsg、init：独立国际化和 IOC/调度注册。
- frontend/src/views/casdoor：独立 api.ts、types.ts、route.ts、页面及通用集成视图。
- server/internal/integrations/casdooroctop：唯一允许同时导入 Casdoor 与 Octop 服务的集成适配器。

Casdoor 公共应用服务返回自身 DTO；其他业务模块不直接查询 Casdoor 仓储。集成适配器在组合层注册，未注册时 Casdoor 照常启动，集成列表为空且页面不显示对应操作。

### 5. 身份、应用和权限规则

- 连接范围为 Casdoor 服务 + 组织；同一服务/组织最多启用一个写入主连接。
- 应用由连接、真实 owner/name 标识，组织独立保存；不把 owner 填成应用名称，不固定 owner 为某个值。
- 用户保留源 UUID/SourceId，另存 API 使用的 owner/name；名称、邮箱仅作查询和人工候选，不能自动合并身份。
- 组织用户只同步一份，通过角色/权限关联多个应用；不按应用重复创建用户。
- 每个受管应用可使用一个专用角色和一条应用权限；角色前缀 mayfly-casdoor-app-，不能含业务类型绑定。
- 权限按已核实的 Casdoor 语义使用 Application / Read、真实应用名称、可用模型及审批状态。
- 外部已有角色/权限不覆盖；初始化先预览现有规则与默认允许影响，无法解释的规则阻止接管，保留只读。
- 成员变更结果、策略 allowed/denied/unknown、真实登录验证和业务会话影响分别显示。
- 移除一个角色成员后仍有其他授权时，展示剩余来源；不得宣称已封禁全部访问。
- 应用被多个部署实例共用时，其 Casdoor 权限对这些实例共同生效。核心展示应用级影响；已注册适配器补充当前操作者可见的关联资源，不能承诺掌握全部外部部署。
- Casdoor 管理员可以管理应用授权而没有业务主机权限；业务运维人员可以看主机而没有 Casdoor 写权限，两者独立。

### 6. 独立权限

动作权限：

- casdoor:read、casdoor:sync。
- casdoor:connection:manage、casdoor:connection:scope。
- casdoor:user:manage、casdoor:user:status。
- casdoor:application:manage、casdoor:access:manage、casdoor:audit:read。
- casdoor:integration:manage：可选业务资源关联；还需适配器验证目标资源权限。

另设 Mayfly 账号 × connectionId × read/manage 范围；系统管理员使用 consts.AdminId，非管理员缺少范围或用户上下文时拒绝。新建连接和配置范围首期仅系统管理员执行。已保存连接的服务地址/组织固定，切换目标通过新建连接完成；原目标内可轮换 Client ID，但必须输入新密钥。调度器调用私有只读同步方法，不通过 nil 用户绕过公开方法检查。

旧目录读取权限仅映射新读取/菜单；旧保存权限仅映射原有连接维护和同步，不自动授予远端用户/应用/授权写权限。旧业务关联权限保留在集成兼容层。已有连接默认管理员可用，普通账号需显式范围分配。

新增写权限不能自动授给 COMMON；迁移使用现有 InsertResource / GrantRoleResource，不改公共助手。

---

## 接口及数据契约

### 1. 核心 API：不携带业务实例字段

相对于 Mayfly 当前 API 基址：

- GET/POST /casdoor/connections，POST /casdoor/connections/test。
- POST /casdoor/connections/:id/sync，GET/PUT /casdoor/connections/:id/scopes。
- GET /casdoor/applications、GET /casdoor/applications/:id。
- GET /casdoor/users、GET /casdoor/users/:id。
- GET /casdoor/objects?connectionId=&kind=role|group|permission。
- GET /casdoor/objects/:id?connectionId=：单个授权依据的白名单投影详情。
- GET /casdoor/access?connectionId=&appSnapshotId=&userSnapshotId=。
- GET /casdoor/access/subjects：应用侧用户分页或用户侧应用分页，appSnapshotId/userSnapshotId 必须且只能提供一个；筛选与响应见页面 Spec P1 第 11 节。
- POST /casdoor/changes/preview。
- POST /casdoor/changes/execute：{connectionId, previewId, idempotencyKey}。
- GET /casdoor/operations、GET /casdoor/operations/:id。
- POST /casdoor/operations/:id/reconcile：仅回读源端核实结果，不再次写入；需要原管理动作权限及当前连接管理范围，只有审计读取权限不可调用。

核心表单和 DTO 中不存在 instanceId、MachineId、OctopUserId 等字段。未关联业务资源的应用也可完成同一套管理流程。

列表支持 P1 各页面指定的关键词、状态和分页条件；操作记录支持时间/动作/状态/操作者/目标查询。连接响应补充凭证配置标记、可见范围、分项同步和按动作能力，用户/应用/对象详情补充页面规定的白名单资料。所有新写命令最多包含 100 个显式用户，前后端同时限制。操作列表默认 SubmittedAt 非空，未提交/过期预览另行查看。

写命令 action：user.create、user.update、user.set-status、application.update、access.initialize、access.grant、access.revoke。禁止任意 URL/字段透传。

白名单：

- user.create：name、displayName、email；组织服务端注入，不传管理员标记/密码；激活路径未核实则阻止创建。
- user.update：displayName、email；不改 owner/name/UUID、管理员或 signupApplication。
- user.set-status：isForbidden，单独权限，预览其组织全部应用影响。
- application.update：displayName、homepageUrl、redirectUris、disableSignin；适配实际字段名，不改 owner/name/organization/Client ID/Secret。
- access.*：appSnapshotId、userSnapshotIds；角色/权限模型由服务端确定，不要求本地业务实例 ID。

**命令目标及字段规则：**

- user.create：AppSnapshotId=0、UserSnapshotIds 为空，只允许 UserCreate；name 必须在该组织可创建。
- user.update / user.set-status：AppSnapshotId=0，恰好一个 UserSnapshotId，只允许对应 UserPatch / UserStatus。
- application.update：恰好一个 AppSnapshotId，UserSnapshotIds 为空，只允许 ApplicationPatch。
- access.initialize：恰好一个 AppSnapshotId，0–100 个显式初始用户；空初始集合必须在预览突出说明可能形成空白名单，不自动把组织用户补入。
- access.grant / access.revoke：恰好一个 AppSnapshotId，1–100 个显式用户。用户 ID 规范化排序去重，但输入数组长度也不能超过 100。
- 所有非零目标均须属于当前连接；请求携带无关字段、跨连接目标或错误数量时在预览前拒绝，Execute 再次校验已保存规范命令。
- Patch 使用可选字段区分“未提交”和“显式清空”；邮箱、首页、回调地址集合允许按部署端契约清空，未提交字段不更新，显示名不能清空。bool 字段需区分未提交与显式 false。

### 2. 独立服务契约

在 server/internal/casdoor/application 中定义：

~~~go
type Directory interface {
    GetApplicationRef(ctx context.Context, connectionId, appSnapshotId uint64) (*dto.ApplicationRef, error)
    GetUserRef(ctx context.Context, connectionId, userSnapshotId uint64) (*dto.UserRef, error)
    GetUserRefBySource(ctx context.Context, connectionId uint64, sourceId string) (*dto.UserRef, error)
    GetObject(ctx context.Context, connectionId, objectSnapshotId uint64) (*dto.DirectoryObject, error)
}
type Management interface {
    Preview(ctx context.Context, cmd dto.ChangeCommand) (*dto.ChangePreview, error)
    Execute(ctx context.Context, connectionId, previewId uint64, idempotencyKey string) (*dto.OperationResult, error)
    GetAccess(ctx context.Context, connectionId, appSnapshotId, userSnapshotId uint64) (*dto.AccessResult, error)
    ListAccessSubjects(ctx context.Context, query dto.AccessSubjectQuery) (*model.PageResult[*dto.AccessSubjectRow], error)
    Reconcile(ctx context.Context, operationId uint64) (*dto.OperationResult, error)
}
~~~

ApplicationRef：Id、ConnectionId、SourceId、Owner、Name、Organization、DisplayName、ClientId、Present。UserRef：Id、ConnectionId、SourceId、ObjectId、Organization。均为 Casdoor DTO，不引入业务系统类型。

dto/management.go：

- ChangeCommand：ConnectionId、Action、AppSnapshotId、UserSnapshotIds，以及对应 action 的互斥白名单字段。
- ChangePreview：Id、ActorId、ConnectionId、Action、TargetIds、脱敏 Diff、Warnings、BlockingReasons、ExpiresAt。有效期 5 分钟，规范化命令与源对象指纹服务端保存。
- OperationResult：OperationId、Status、MemberChangeVerified、AccessDecision、ReasonCodes、Replayed、VerifiedAt、分项结果；Status 为 pending/verified/noop/failed/unknown。
- AccessResult：Decision（allowed/denied/unknown）、EvaluationScope（policy）、Sources、ReasonCodes、EvaluatedAt、SessionNotice；unknown 必须给原因。
- DirectoryObject：Id、ConnectionId、Kind、SourceId、白名单投影、LastSuccessAt 和 Present；不得提供完整原始响应。
- AccessSubjectQuery：ConnectionId、AppSnapshotId/UserSnapshotId（二选一）、Keyword、Membership、PageNum、PageSize。AccessSubjectRow：Application/User 引用、ManagedMembership（member/not_member/unknown）、Decision、ReasonCodes、EvaluatedAt，未完整评估时 Decision=unknown。

**操作结果归并：**

- 分项结果包含目标 ID、步骤、状态、Changed、是否核实、ReasonCode、核实时间；已核实成功的分项不能因其他分项失败而丢失。
- 操作尚在执行/核对时整体 pending。执行结束后：任何分项结果不确定则整体 unknown；否则任何已知失败则 failed；全部无需变更则 noop；其余全部已核实或无需变更才为 verified。
- failed 同时存在成功分项时，页面显示“部分完成”，不是回滚成功。保留成功/失败/未知目标清单，不自动重跑整个批次。
- MemberChangeVerified 为可空布尔：非成员操作返回 null，不能把默认 false 误渲染成成员变更失败；非授权操作的 AccessDecision 也不渲染为访问拒绝。
- Reconcile 只核对已记录目标和目标状态；不创建角色、用户或权限，不补发缺失步骤。需要修复的已知失败必须重新预览，源漂移不能强制覆盖。

infra/client/management.go 的 ManagementClient：

~~~go
type ManagementClient interface {
    GetUser(ctx context.Context, objectID string) (*UserDetail, error)
    GetApplication(ctx context.Context, objectID string) (*ApplicationDetail, error)
    ListObjects(ctx context.Context, kind string, page, pageSize int) (*ObjectPage, error)
    GetRole(ctx context.Context, objectID string) (*RoleDetail, error)
    GetPermission(ctx context.Context, objectID string) (*PermissionDetail, error)
    CreateUser(ctx context.Context, input CreateUserInput) error
    PatchUser(ctx context.Context, objectID string, patch UserPatch) error
    PatchApplication(ctx context.Context, objectID string, patch ApplicationPatch) error
    CreateRole(ctx context.Context, input RoleDetail) error
    ReplaceRoleUsers(ctx context.Context, objectID string, users []string) error
    CreatePermission(ctx context.Context, input PermissionDetail) error
}
~~~

类型放同目录 management_types.go，按白名单映射。RoleDetail 保留 Users、Roles 及管理标记；PermissionDetail 保留主体/资源/动作/模型/审批/启停。不存在返回 (nil,nil)，权限和传输失败返回 error。

### 3. 可选集成契约

在 domain/integration/contracts.go 定义，不依赖任何业务模块：

~~~go
type Adapter interface {
    Key() string
    ListResources(ctx context.Context, query ResourceQuery) (*ResourcePage, error)
    ListBindings(ctx context.Context, connectionId, appSnapshotId uint64) ([]ResourceRef, error)
    ChangeBinding(ctx context.Context, command BindingCommand) error
}
~~~

- ResourceQuery：Keyword、PageNum、PageSize；ResourceRef：System、ResourceId（不透明字符串）、DisplayName、URL；ResourcePage：List、Total。
- BindingCommand：ConnectionId、AppSnapshotId、ResourceId、Action（bind/unbind），没有某个业务的具体字段。
- application/IntegrationRegistry.Register(adapter integration.Adapter) error：由组合层注册；重复 Key 使启动报错。Casdoor 内部只依赖契约。
- API：GET /casdoor/integrations；GET /casdoor/integrations/:key/resources；GET /casdoor/applications/:id/related-resources；POST /casdoor/integrations/:key/bindings。
- 适配器不存在时，核心用户/应用/授权接口不受影响；指定不存在适配器的关联请求返回 unsupported。
- 修改关联需 Casdoor 读取/集成动作权限及目标系统操作权限；适配器只能修改自己拥有的业务数据。
- 授权预览可以补充可见关联资源；适配器不可用时声明“关联资源范围未核实”，不能冒充完整资源清单，也不能因此阻止合法的 Casdoor 应用级授权。
- 现有 Octop 身份映射仍归其业务模块，用 Casdoor Directory 验证用户引用；不迁入 Casdoor 核心。

### 4. 数据归属与升级策略

独立核心表：

- t_casdoor_connection：服务/组织/加密凭证、同步状态、managementEnabled（默认关闭）、能力信息。
- t_casdoor_application：应用投影、SourceId、真实 Owner、ClientId 和必要字段；不缓存 Secret。
- t_casdoor_user：用户投影、SourceId 与 ObjectId。
- t_casdoor_auth_relation：关系与来源证据，confirmed 不等于完整登录允许。
- t_casdoor_object：role/group/permission 白名单投影、指纹、present、完整同步时间；唯一 (connectionId,kind,sourceId)。
- t_casdoor_scope：账号/连接/read或manage；唯一 (accountId,connectionId)。
- t_casdoor_managed_app：应用与远端专用角色/权限 ID、管理标记、指纹和接管状态；唯一 (connectionId,applicationSourceId)。
- t_casdoor_change：预览、操作者、源指纹、幂等键、SubmittedAt、状态及分项结果；SubmittedAt 区分预览与已提交操作，不能把未提交预览算作执行中。幂等键按连接唯一，同键不同命令/操作者返回冲突。

连接、应用、用户和关系的旧四张目录表复制到对应通用表，保留原主键、密钥密文、时间及稳定 ID；目标已存在且不一致时停止迁移，不盲覆盖。ID 回填后校正数据库自增序列，后续新建不能撞 ID。

源表映射固定为：

- t_octop_casdoor_connection → t_casdoor_connection。
- t_octop_casdoor_app → t_casdoor_application。
- t_octop_casdoor_user → t_casdoor_user。
- t_octop_casdoor_auth → t_casdoor_auth_relation。

旧表保持原样作为恢复依据；切换后只有新表参与运行。原业务表中的连接/应用 ID 无须改写；其他业务专属身份表继续归原模块。禁止双写旧/新目录表。升级失败时不启动新模块，不容忍一半切换。

新增迁移只做本地数据和菜单调整，不调用远端管理 API。新权限不批量赋给普通角色。

---

## 实施准备结论与阶段门

**文档结论：用户已要求开始实施；实际部署兼容性仍需验证。** 不应把“文档可实施”“功能已通过测试”“可以生产上线”写成同一个状态。

开始任务 4/5 中的具体写能力前，任务 1 必须产出：

- 实际 Casdoor 版本或可核实构建来源，管理认证方式、组织/Owner 规则和完整分页契约。
- 对用户/应用/角色/权限相关 API 的字段、部分更新、响应及权限模型核查；同版本隔离环境的写入和回读证据。
- 创建用户的可用激活/认证路径；缺失时明确该分支无法交付，不能静默当作完成。
- 可用隔离环境及测试数据、当前 Go/前端测试基线。不得借生产测试连接创建探针对象。

**写入准入不构成循环要求：**

- 按动作分别记录 ProtocolCapability（supported/unsupported/unverified）、ReadProbe（passed/failed/unverified）及 WriteAuthorization（allowed/denied/unverified），不能把读取成功当作写权限通过。
- 协议/安全更新契约已在相同版本隔离环境验证、原目标读取测试通过、操作者有动作/范围权限且明确开启写入后，可执行该操作者预览并确认的业务变更；不要求先在生产做无业务意义的写探针。
- 生产 WriteAuthorization 尚未实测时页面明确显示“写权限未实测”，不宣称已经验证。首次批准的真实业务请求若被 401/403 拒绝，记录失败并阻止同凭证对应动作继续提交，直到凭证或权限更新后重新检查；不静默换认证方式或扩大权限。
- 必要协议/模型不支持时停止对应写能力，报告缺口和调整方案供审核；只读交付只能记为阶段结果，不能代替完整用户/授权管理验收。

**阶段交付：**

- A（任务 1–3）：独立契约、数据迁移与读取服务的测试证据；不是生产菜单已经可用。
- B（任务 4–5）：隔离环境的真实写入、回读、幂等、失败与权限测试证据。
- C（任务 6）：五页及 P1 交互通过组件/接口联调；核心不依赖业务实例。
- D（任务 7–8）：可选业务适配、启动切换和旧入口兼容，在隔离/预发布环境验证。
- E（任务 9）：完整验收、部署/恢复证据；生产发布仍按下文审核边界执行。

---

## 实施任务

顺序 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9。每项先写行为测试并确认失败，再实现、执行验证，建议按任务提交；任务 1 已开始，尚未完成。

### Task 1: 建立独立模块骨架与实际接口契约

**Files:**
- Create: server/internal/casdoor/init/init.go、application/application.go
- Create: server/internal/casdoor/application/dto/directory.go、management.go
- Create: server/internal/casdoor/domain/entity/connection.go、application.go、user.go、relation.go、management.go
- Create: server/internal/casdoor/domain/repository/repository.go
- Create: server/internal/casdoor/domain/integration/contracts.go
- Create: server/internal/casdoor/imsg/imsg.go、zh_cn.go、en.go
- Create: server/internal/casdoor/architecture_test.go
- Create: docs/casdoor/deployment-compatibility.md

**Consumes/Produces:** 消费项目分层规范；产出独立 DTO/实体/接口及兼容性报告，后续任务不再从业务模块取类型。

- [ ] 核对实际部署 Casdoor 版本/提交，组织应用、用户身份、分页、角色/权限模型、部分字段更新和用户激活路径；生产仅只读检查。
- [ ] 写 TestCasdoorCoreDoesNotDependOnBusinessModules，断言核心无业务模块导入，启动无适配器时可用。
- [ ] 建立本文契约与独立初始化定义，暂不在 main.go 切换启动；迁入可复用算法但类型/依赖归独立目录。
- [ ] 文档记录已核实、待验证、不支持的能力；master/Swagger 不能代替实际部署证据。
- [ ] 按“实施准备结论与阶段门”输出实际版本、隔离写契约、用户激活方式、协议/读取/写权限三类能力状态；必要缺项不向后假定成功。
- [ ] 记录后端/前端测试基线，缺工具链/数据库时明确说明。

**Run:** server 中 go test ./internal/casdoor/...；预期架构/契约测试通过，不需要 Octop 服务或数据库。
**建议提交:** feat(casdoor): establish independent module contracts

### Task 2: 独立仓储与无损数据迁移

**Files:**
- Create: server/internal/casdoor/infra/persistence/persistence.go、directory.go、management.go
- Create: server/internal/casdoor/infra/persistence/persistence_test.go
- Modify: server/migration/migrations/v1_12/v1_12_3.go
- Modify: server/migration/migrations/v1_12/v1_12_3_test.go

**Consumes/Produces:** 使用任务 1 实体/仓储契约；产出 t_casdoor_*、保留旧 ID 的数据及菜单/动作权限。新应用 ID 与已有关联可直接对应。

- [ ] 先写 TestCasdoorCopiesLegacyRowsWithoutChangingIDs、TestCasdoorMigrationIsIdempotent、TestCasdoorMigrationRefusesConflictingTarget、TestCasdoorNewIDsDoNotCollide、TestCasdoorLegacyRolesDoNotGainRemoteWrite。
- [ ] 追加 v1.12.3-casdoor-core-schema、v1.12.3-casdoor-directory-data、v1.12.3-casdoor-management-resources 三条迁移；再次读取当前 Version，不能自行升级版本。
- [ ] 创建通用表，事务复制旧四类目录数据、检查数量/主键/密文/业务引用，校正自增序列；重复执行不覆盖新数据。
- [ ] 复用旧目录菜单 ID 1750000004 迁成顶级 /casdoor，新增五个页面；更新父链/ui_path，旧书签由兼容路由承接。保留旧资源授权，不修改原迁移 seed。
- [ ] 只映射原本的读取/连接/同步能力；新增远端写权限不自动授给 COMMON，范围默认管理员。
- [ ] 仓储使用显式列更新零值；投影不保存完整响应/凭证。

**Run:** server 中 go test ./internal/casdoor/infra/persistence ./migration/migrations/v1_12。
**预期:** 全新库、已有库、重复执行和目标冲突用例符合约定，原业务引用不变。
**建议提交:** feat(casdoor): migrate directory into independent tables

### Task 3: 独立连接、范围检查和全应用同步

**Files:**
- Create: server/internal/casdoor/application/connection.go、access.go、directory.go、sync.go
- Create: server/internal/casdoor/application/connection_test.go、sync_test.go、access_test.go
- Create: server/internal/casdoor/infra/client/client.go、error.go、client_test.go
- Create: server/internal/casdoor/api/connection.go、directory.go、api.go
- Create: server/internal/casdoor/api/form/connection.go、query.go

**Consumes/Produces:** 使用新仓储；实现 Directory、连接服务和独立只读 API；公开方法检查账号范围，私有 syncConnectionInternal 供调度器读取已配置连接。

- [ ] 先写 TestDirectoryIncludesUnboundApplications、TestSavedProbeUsesStoredCredential、TestChangedTargetRequiresNewCredential、TestScopeRejectsMissingActor、TestPartialSyncKeepsLastCompleteData。
- [ ] 修复按连接 ID 恢复保存密钥；已保存服务地址/组织不可改，切换目标创建新连接，原目标内换 Client ID 要求新密钥。测试新目标不复用旧密钥，禁止跨目标重定向携带 Authorization。
- [ ] 连接字段校验与 P1 一致：超时 1–120 秒、同步周期至少 60 秒、默认 3600 秒；新建/轮换密钥规则前后端一致。显式 Owner 筛选显示范围受限，不当作组织完整目录。
- [ ] 真实 API 测试区分连接、认证、权限、路由与筛选为空；不做远端写探针，不自动开启管理写入。
- [ ] 管理开关按动作协议支持和读取测试结果校验；生产写权限未实测可显式显示，但不能伪标已验证，也不能要求生产探针作为循环前提。
- [ ] 用已核实的管理列表接口读取组织全应用，不能用调用者“允许登录的应用列表”代替管理目录；无实例或无适配器时也读完整。
- [ ] 分页完整读取后按类别事务提交；失败保留最后成功快照，成功清除旧错误；移除 application.users 推断，改读角色/群组/权限证据。
- [ ] 调度器每 60 秒检查到期连接，最小同步周期 60 秒、默认 3600 秒；连接内锁统一协调同步和后续写操作。
- [ ] 绑定目录 DTO 的权限完全来自连接范围，不引用任何机器/实例权限。

**Run:** server 中 go test ./internal/casdoor/application ./internal/casdoor/infra/client。
**预期:** 超 200 个应用不截断；同名不同组织隔离；nil 用户不绕过公开接口；后台私有只读同步正常。
**建议提交:** feat(casdoor): add independent connection and directory services

### Task 4: 独立远端管理 API 适配

**Files:**
- Create: server/internal/casdoor/infra/client/management.go、management_types.go、management_test.go
- Create: server/internal/casdoor/infra/client/testdata/management_contracts.json

**Consumes/Produces:** 实现本文 ManagementClient，复用独立客户端传输，接口不含业务资源类型。

- [ ] 先写 TestBusinessErrorIsFailure、TestRoleUsersPatchPreservesOtherFields、TestWriteTimeoutIsUnknown、TestCredentialsAreRedacted、TestUserCreateRequiresSupportedActivation。
- [ ] 用 httptest.Server 验证认证头、实际字段、owner/name 编码、分页和响应；真实写契约只在隔离环境确认。
- [ ] 实现白名单的用户/应用更新、角色/权限创建及成员更新；不支持安全部分更新时阻止对应功能，不回退完整对象覆盖。
- [ ] HTTP 200 但 status=error 或 affected=false 不成功；写超时结果不确定，交业务服务回读，不盲重试。
- [ ] 日志脱敏，不缓存响应中的密码、令牌或 Client Secret。

**Run:** server 中 go test ./internal/casdoor/infra/client。
**建议提交:** feat(casdoor): implement typed management API client

### Task 5: 通用用户与应用授权管理闭环

**Files:**
- Create: server/internal/casdoor/application/management.go、change.go、policy.go
- Create: server/internal/casdoor/application/management_test.go、policy_test.go
- Create: server/internal/casdoor/api/management.go、management_test.go
- Create: server/internal/casdoor/api/form/management.go
- Modify: server/internal/casdoor/application/application.go、api/api.go
- Modify: server/internal/casdoor/imsg/imsg.go、zh_cn.go、en.go

**Consumes/Produces:** 实现 Management 和 Change DTO；不查询业务实例表、不要求业务适配器存在。

- [ ] 先写 TestUnboundApplicationCanBeManaged、TestPreviewMakesNoRemoteWrite、TestExpiredPreviewCannotExecute、TestIdempotencyReplayDoesNotWriteTwice、TestSourceDriftRejectsExecution、TestRevokeShowsRemainingGrant、TestIntentFailurePreventsRemoteWrite、TestReconcileMakesNoWrite、TestAccessSubjectsRequireExactlyOneSide、TestGrantRejectsMoreThan100Users、TestCommandTargetCardinality、TestBatchResultAggregation。
- [ ] Preview 计算 Casdoor 对象级差异，记录动作、操作者和指纹，5 分钟过期；应用没有实例也能预览。
- [ ] 执行本文命令目标/互斥字段规则；补未提交字段不更新、显式空值更新和 false 更新的行为测试。
- [ ] Execute 重查权限/范围、管理开关、预览/指纹，先落操作意图再写远端；同键同命令重放已有结果，不同命令冲突。
- [ ] 用户资料、组织账号禁用和应用配置分别鉴权。没有已验证激活方式时阻止 user.create；不伪造可登录用户。
- [ ] 初始化专用 mayfly-casdoor-app- 角色与权限，核对远端管理标记、审批和模型；同名外部对象不接管。部分失败逐项记录，未完整成功不得标为就绪。
- [ ] grant/revoke 只改受管角色成员；专用角色首期由 Mayfly 单一写入，发现外部漂移拒绝覆盖；不承诺跨系统 CAS。
- [ ] 写后回读，超时或重启后的 pending 先核对现状，未确认记 unknown，不盲重发。
- [ ] 按结果归并规则生成整体和分项状态；混合成功/失败显示部分完成，保留已生效证据，不声称批次整体回滚。
- [ ] 实现仅回读的 Reconcile，复用源状态核实逻辑，检查原动作权限与连接范围；实现应用/用户双向访问分页与对象详情，覆盖 P1 的数据闭环。
- [ ] AccessResult 展示其他角色/群组/直接授权、管理员及默认允许情况；无法完整判断则 unknown。
- [ ] 适配器可选补充关联资源，但其故障不阻止有权管理员执行 Casdoor 自身授权；核心 API 不暴露其无权看到的资源。

**Run:** server 中 go test ./internal/casdoor/...。
**预期:** 零业务实例也能完成用户管理和两应用授权；操作审计保存失败不发远端请求；未确认结果不报成功。
**建议提交:** feat(casdoor): add application-neutral management workflows

### Task 6: 独立前端与菜单页面

**Files:**
- Create: frontend/src/views/casdoor/api.ts、types.ts、route.ts
- Create: frontend/src/views/casdoor/connections/ConnectionList.vue
- Create: frontend/src/views/casdoor/applications/ApplicationList.vue
- Create: frontend/src/views/casdoor/users/UserList.vue
- Create: frontend/src/views/casdoor/access/AccessManagement.vue
- Create: frontend/src/views/casdoor/operations/OperationList.vue
- Create: frontend/src/views/casdoor/components/ConnectionSelector.vue、ConnectionEdit.vue、ConnectionScopes.vue
- Create: frontend/src/views/casdoor/components/ApplicationDetail.vue、ApplicationEdit.vue、UserDetail.vue、UserEdit.vue
- Create: frontend/src/views/casdoor/components/ChangeWorkflow.vue、ChangePreview.vue、OperationDetail.vue、ObjectDetail.vue、RelatedResources.vue
- Create: frontend/src/views/casdoor/composables/useCasdoorConnection.ts
- Create: frontend/src/views/casdoor/__tests__/management.test.ts
- Create: frontend/src/views/casdoor/__tests__/connections.test.ts、applications.test.ts、users.test.ts、access.test.ts、operations.test.ts、changeWorkflow.test.ts
- Create: frontend/src/i18n/zh-cn/casdoor.ts、frontend/src/i18n/en/casdoor.ts
- Modify: frontend/src/i18n/zh-cn/menu.ts、frontend/src/i18n/en/menu.ts
- Modify: frontend/src/__tests__/menuRouteResolver.test.ts

**Consumes/Produces:** 仅消费 /casdoor 核心和可选集成 API；不从旧业务目录导入 API/DTO。实现页面 Spec P1：连接 §3、应用 §4、用户 §5、授权 §6、共享变更 §7、操作 §8、可选关联 §9；公共状态遵守 §1–2/10。

- [ ] 先按 P1 的 UI-01 至 UI-15 写行为测试：无适配器/无关联应用、连接切换晚响应、表单校验、权限/能力、预览过期、批量限制、仅回读核对及 unknown/部分失败展示。
- [ ] 实现五页面和共享连接选择；应用类型保持通用；未关联资源显示正常管理入口。
- [ ] 授权页分别展示来源、成员写入、策略判定和会话提示；全组织账号禁用与单应用撤权分开操作。
- [ ] 可选关联视图根据 /casdoor/integrations 能力渲染，不硬编码业务实例选择器；没有注册集成时省略关联操作。
- [ ] 全部文案中英文 i18n，写操作先预览再提交；结果以回读为准。
- [ ] 单独实现 ChangeWorkflow 状态机：编辑 → 预览 → 执行/核对 → 结果；结果保留抽屉，不让普通保存逻辑把 pending/unknown 当成功。操作 GET 每 3 秒轮询最多 60 秒，显式核对才调用 reconcile，禁止自动重复写。
- [ ] 覆盖连接范围、应用/用户/对象详情与双向访问列表、操作详情；页面字段/筛选/分页与后端契约一致，不只交付五个列表壳。
- [ ] 检查 1024px/1440px、明暗主题、中英文、键盘操作及错误/空态，按 P1 留存验收证据。
- [ ] 前端依赖检查断言 views/casdoor 不导入 views/octop；独立模块无原业务菜单也可直达已授权页面。

**Run:** frontend 中 pnpm test:run src/views/casdoor src/__tests__/menuRouteResolver.test.ts src/__tests__/i18nKeyExistence.test.ts；再执行 pnpm type-check、pnpm lint、pnpm build。
**建议提交:** feat(casdoor): add independent management center UI

### Task 7: 实现可选业务适配器（Octop 作为首个接入方）

**Files:**
- Create: server/internal/casdoor/application/integration.go、integration_test.go
- Create: server/internal/casdoor/api/integration.go
- Create: server/internal/integrations/casdooroctop/adapter.go、init.go、adapter_test.go
- Modify: server/internal/octop/application/instance.go、api/instance.go
- Create: server/internal/octop/application/identity.go、identity_test.go
- Modify: frontend/src/views/octop/instance/InstanceDetail.vue

**Consumes/Produces:** 适配器同时消费 Casdoor Directory 和现有业务服务；Casdoor 核心仅消费 Adapter 契约。业务身份表保持原归属。

- [ ] 先写 TestCasdoorWorksWithoutRegisteredAdapter、TestAdapterRejectsInvisibleResource、TestUnbindClearsBothReferences、TestCasdoorGrantDoesNotRequireMachinePermission。
- [ ] 实现通用注册/资源查询/关联入口；没有适配器返回能力未注册，其他管理功能不受影响。
- [ ] 首个适配器 Key=octop，负责自己的实例选择、绑定/解绑与资源可见性；绑定引用 Casdoor Directory，禁止核心反向依赖。
- [ ] 原业务应用查询改用 Directory 公共服务，不注入 Casdoor 仓储；绑定/解绑明确更新两个引用字段，零值必须落库。
- [ ] 把现有业务身份关联服务留在业务模块，调用 Directory.GetUserRefBySource 按连接及原稳定 SourceId 验证；不能为兼容旧身份记录直接查 Casdoor 仓储，不能改为按邮箱/用户名合并。补迁移前后同一 SourceId 仍可解析的测试。
- [ ] 业务实例页显示/跳转应用，Casdoor 页面通过通用资源契约展示可见关联；其他业务系统可实现相同契约，不在本期创建空壳适配器。

**Run:** server 中 go test ./internal/casdoor/... ./internal/integrations/casdooroctop/... ./internal/octop/...。
**建议提交:** feat(integrations): connect Octop through optional Casdoor adapter

### Task 8: 启动切换与旧入口退役兼容

**Files:**
- Modify: server/main.go
- Modify: server/internal/octop/application/application.go、api/api.go、infra/persistence/persistence.go
- Modify: server/internal/octop/application/casdoor.go、casdoor_sync.go、application/dto/dto.go
- Modify: server/internal/octop/domain/repository/repository.go、infra/persistence/repos.go
- Modify: server/internal/octop/api/casdoor.go、api/form/form.go
- Create: server/internal/casdoor/api/legacy.go、legacy_test.go
- Modify: frontend/src/router/staticRouter.ts
- Modify: frontend/src/views/octop/casdoor/CasdoorDirectory.vue、route.ts、api.ts、types.ts

**Consumes/Produces:** 独立模块接管运行时所有 Casdoor 能力；旧路径仅保留委托或跳转。以上旧业务路径只属于迁出/兼容清理，不承担新增管理功能。

- [ ] 注册独立 Casdoor 初始化与可选适配器，取消旧目录仓储/同步任务/IOC 注册；确认只有一个 Casdoor 同步调度器。
- [ ] 移除旧业务模块中的 Casdoor 连接、同步、目录业务实现及 HTTP 客户端；相应测试迁到独立模块并保留原回归断言，不复制双份运行逻辑。
- [ ] 旧四类实体若仍被已发布迁移引用，冻结为历史迁移兼容定义并明确标注“仅历史迁移使用”；运行时不得再注册对应仓储。不能为清理命名改写旧迁移。
- [ ] /octop/casdoor 的历史目录请求由独立 api/legacy.go 委托新服务，执行新范围检查；历史业务身份/绑定请求仍由对应业务服务处理。
- [ ] 前端旧目录仅登录后跳转 /casdoor/applications；旧书签兼容但不加入免登录白名单。
- [ ] 清除旧业务 API/types 中已迁出的核心定义；必要导出由新模块向旧引用提供兼容，依赖方向不得反转。
- [ ] 架构测试扫描运行时导入与注册：没有核心新代码留在旧模块，无重复 cron，无旧表双写。

**Run:** server 中 go test ./internal/casdoor/... ./internal/integrations/casdooroctop/... ./internal/octop/... ./migration/migrations/v1_12；frontend 中运行相关测试、类型检查和构建。
**预期:** 旧数据正常展示，旧入口转入独立模块，业务采集/运维回归通过。
**建议提交:** refactor(casdoor): retire legacy module ownership

### Task 9: 独立性、真实授权和升级验收

**Files:**
- Create: server/internal/casdoor/infra/client/management_integration_test.go
- Create: docs/casdoor/management-guide.md、management-acceptance.md、management-release.md

**Consumes/Produces:** 使用独立模块和隔离测试环境，输出可复现验收证据，不默认连接生产 IP。

- [ ] 在不注册任何业务适配器、不配置业务实例时，验证连接、全应用/用户同步、资料管理、两个普通测试应用的授予/撤销。
- [ ] 使用两个不同业务类型的测试应用验证用户属于组织，权限按应用分配；目录不把非 Octop 应用漏掉。
- [ ] 有效策略/真实登录测试覆盖默认允许、其他角色/群组、管理员、账号/应用禁用；撤权后验证新登录，已有会话表现单独记录。
- [ ] 注册首个适配器后验收资源权限、关联/解绑和既有身份映射；Casdoor 管理者没有主机权限仍可管理授权，运维人员没有 Casdoor 权限不能改授权。
- [ ] 老库副本验证复制/主键/密钥/引用/自增、重跑迁移、启动与回退；确认未自动调用远端写接口。
- [ ] 验证分页中断、远端业务错误、源漂移、并发、超时及重启恢复；不确定结果显示 unknown。
- [ ] server 运行 go test ./internal/casdoor/... ./internal/integrations/casdooroctop/... ./internal/octop/... ./migration/migrations/v1_12，再运行 go test ./...；frontend 运行 pnpm test:run、pnpm type-check、pnpm lint、pnpm build。
- [ ] 集成测试缺少隔离配置时明确 skip，不把 skip 当通过；全量测试外部依赖失败单独记录，不用局部通过冒充全量通过。
- [ ] 按页面 Spec P1 的 UI-01 至 UI-15 逐条登记通过/失败及证据；页面菜单存在不等于 Spec 已完成。
- [ ] 逐项核对 A–E 阶段证据；明确尚未实现/未通过/受外部条件阻断的能力，不能把条件禁用项静默标记为完整交付。
- [ ] 交付配置指南、部署/恢复步骤、实际 API 版本与局限；后续 Agent 接入复用 Casdoor 服务契约。

**验收结论:** 不能仅因菜单已独立就报告完成；需证明代码、数据、权限和运行依赖均已独立。
**建议提交:** test(casdoor): verify independent management and migration

---

## 发布、恢复与审核边界

1. 用户已明确要求按 R3 及页面 Spec P1 开始实施；目前任务 1 正在进行。
2. 审核通过后按九项任务开发和隔离测试；外部接口缺少必要能力时说明影响并调整计划，不能擅自改 Casdoor。
3. 开发验证不改生产授权；上线前提供测试/迁移证据、目标版本、备份和恢复步骤，再按用户发布授权执行。
4. 部署、迁移和同步不会自动接管应用；实际接管/授权由有权限管理员对具体预览提交。
5. 回退先关闭写入开关。新表切换后旧表不再同步，不能直接回旧镜像读取旧表，否则会看到过时目录；回退包须使用新表或先在停写条件下执行经过验证的回迁。
6. 镜像/数据库恢复不等于撤销 Casdoor 远端操作。远端恢复按审计差异核对当前状态，生成新的预览执行，不能盲恢复旧快照。
7. 完成标准包括核心独立运行、通用应用管理、真实授权验证、历史引用保持及已批准的部署结果。

## R3 修订摘要与用户审核重点

- 后端由“继续复用旧业务模块”改为完整独立的 internal/casdoor。
- 数据由旧业务前缀切换到 t_casdoor_*；通过迁移保留主键和已有引用。
- 核心 API/DTO/角色命名通用化，无业务实例字段，无实例前置条件。
- Casdoor 管理权限与业务机器权限分开；只在操作关联资源时检查对应业务权限。
- Octop 相关任务集中在可选适配和历史迁出两项，其余核心实现不依赖它。
- 新增独立页面 Spec P1，明确五页字段、抽屉、状态、权限、表单校验、预览/结果及 15 项 UI 验收。
- 补齐对象详情、用户/应用双向访问列表及只读结果核对 API；界面未知结果不能用普通保存成功流程。
- 已保存连接的服务地址/组织固定，切换目标新建连接；这是避免目录身份与旧关联混用的待审建议。
- 本期功能范围仍为连接/目录、已有应用资料、用户、角色/权限和访问管理；其他扩展另行推进。
- 用户已明确要求按本计划及页面 Spec 实施；生产发布和应用接管仍遵循上述边界。

## 自查与依据

- 独立性覆盖：任务 1/3/5/6/9；数据无损：任务 2/8/9；可选业务接入：任务 7；旧入口退役：任务 8。
- 未把应用资源绑定混入核心授权事务；核心无需机器权限或业务服务在线。
- 首期一个 Mayfly 服务实例，专用角色单一写入；多副本/外部并发编辑需额外锁或版本控制，不能依赖进程内锁宣称安全。
- 核心使用新类型和表；历史旧路径只用于数据来源、迁出和兼容，最终运行时所有者是 Casdoor 模块。
- 页面与接口一致性覆盖：P1 第 11 节映射任务 3/5；五页及交互映射任务 6；UI-01 至 UI-15 映射任务 6/9。
- 实施准备复核：命令目标和互斥字段明确，批量状态有确定归并规则，写能力准入不依赖生产探针；实际环境兼容性仍由任务 1 验证。
- 本次只修改计划；未完成任何产品实现、测试、迁移或部署。

本地依据：AGENTS.md、docs/server/architecture.md、docs/server/security.md、docs/frontend/api.md、现有目录/实例关联实现、server/main.go 及 v1_12_3.go。已核对现有目录功能确实处于业务模块内，必须安排迁出而非只改菜单名称。

外部接口依据（实施任务 1 仍需核对实际部署版本）：

- [Casdoor 管理 API 及认证](https://casdoor.ai/docs/basic/public-api/)
- [应用接口](https://github.com/smc-copilot/casdoor/blob/master/controllers/application.go)
- [用户接口](https://github.com/smc-copilot/casdoor/blob/master/controllers/user.go)
- [角色接口](https://github.com/smc-copilot/casdoor/blob/master/controllers/role.go)
- [权限接口](https://github.com/smc-copilot/casdoor/blob/master/controllers/permission.go)
- [应用登录权限检查](https://github.com/smc-copilot/casdoor/blob/master/object/check.go)
