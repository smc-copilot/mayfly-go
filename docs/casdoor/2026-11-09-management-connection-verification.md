# Casdoor 后端管理连接认证核查

日期：2026-11-09。状态：连接认证方案已有同镜像隔离验证，生产管理身份尚未创建，Mayfly 新认证方式尚未实现。

## 结论

本次验证确认：可以在 Casdoor 的 `built-in` 组织登记专供 Mayfly 使用的管理应用，通过该应用的 `client_credentials` 取得管理令牌，再使用 Bearer 令牌调用组织、应用和用户管理 API。

这是 Casdoor 原生的机器身份认证机制。Client ID / Secret 仍属于这个专用管理应用；Mayfly 的服务连接行仍代表整套 Casdoor 后端，业务组织、应用、用户不归属于该管理应用。不能复用业务组织中的 Octop 应用凭证并假定它具有全局权限。

## 证据与验证环境

- 现场 Swagger：`http://192.168.102.247:8000/swagger/swagger.json`。认证定义包含 `Authorization`，组织、应用、用户 API 可核对。
- 只读查看了 `/data/casdoor` 的认证过滤、应用虚拟用户和组织管理校验实现。该源码提交与生产镜像提交的对应关系未完全确认，因此最终结论采用实际镜像副本测试。
- 隔离 Casdoor 镜像副本：`sha256:db5f97d608e02aa8c0d0ce972082822cadecd2c7434539583ac266e270545fb4`；与先前确认的现场镜像 RootFS 层和 Config 哈希一致，详见 [兼容性记录](deployment-compatibility.md)。
- 隔离环境使用独立 MySQL、Docker 内部网络、随机生成的测试凭证，不挂载生产配置或数据库，不发布宿主端口。
- 管理身份为隔离环境中的专用管理应用，归属 `built-in`，启用 `client_credentials`；另设普通业务组织及其应用作对照。

## 实际结果

最终运行共 22 项检查通过，其中包括原有目录、用户、角色检查，以及本次新增的管理身份检查：

- 普通业务组织应用凭证未能发现 `built-in`。
- 专用管理应用 Basic 凭证能发现内置组织与测试业务组织。
- 专用管理应用能读取上述两个组织的应用和用户。
- `POST /api/login/oauth/access_token` 使用 `client_credentials` 成功返回令牌及 `expires_in`。
- 使用返回的 Bearer 令牌能发现上述两个组织。
- 使用 Bearer 令牌成功创建和修改测试业务组织用户。
- 使用 Bearer 令牌成功修改隔离环境已有内置用户的显示名。
- 使用 Bearer 令牌成功修改内置组织、业务组织中测试应用的显示名，回读确认 Client ID / Secret 保持不变。
- 默认向 `built-in` 创建新用户被 Casdoor 拒绝，且没有创建目标用户。这是产品保护规则，不能将它解释为整个管理连接认证失败。
- Key 列表接口在隔离环境中可访问，但本次未验证 Key 创建、认证或轮换，不将它作为已验证方案。

中间测试曾把新增内置用户当作应成功动作而失败；尝试通过初始化数据新增内置用户同样被产品规则阻止。最终保留默认保护设置，并将拒绝创建作为预期结果，另外验证已有内置用户的修改。没有为使测试通过而关闭该保护。

每次隔离运行结束均清理本次测试容器、内部网络和临时文件。输出不记录实际凭证、令牌或用户资料。

## 连接建立流程

1. Casdoor 现有管理员在原生管理界面登记专供 Mayfly 使用的管理应用，组织选择 `built-in`，配置 `client_credentials` 及有效令牌签发配置。名称可为 `mayfly-management`，不得误用某个 Octop 业务应用。
2. Mayfly 登记一套服务，例如名称 `Casdoor-102.247`、地址 `http://192.168.102.247:8000`；其管理身份配置引用该管理应用凭证，密文保存在后端。登记管理应用不在本次生产环境自动执行。
3. Mayfly 后端调用令牌端点：

   ```http
   POST /api/login/oauth/access_token
   Content-Type: application/json

   {
     "grant_type": "client_credentials",
     "client_id": "<management-client-id>",
     "client_secret": "<management-client-secret>"
   }
   ```

4. 后端缓存令牌及实际返回的有效期，后续请求使用 `Authorization: Bearer <access_token>`。到期前使用同一管理身份重新获取令牌；不要求人员定期粘贴令牌，不假设存在 refresh_token。
5. 使用 `GET /api/get-organizations?owner=admin` 发现组织，再按每个真实组织名称调用 `GET /api/get-organization-applications?organization=...` 和 `GET /api/get-users?owner=...`。完整列表分页行为须进一步核实，不能以本次少量测试替代全量验收。
6. 页面展示“服务 → 组织 → 组织原生对象”；不增加管理应用作为业务目录父节点，不添加业务类型 Owner 筛选，不按 Octop 实例数量复制服务连接。

## Mayfly 需要修正的内容

- 将服务身份、管理认证配置和组织目录拆开；现有单组织 Connection 实体不能直接改个名称充当完成。
- 增加后端自动获取与缓存令牌、到期换取新令牌、并发换令牌协调和凭证轮换。
- 发现权限不足时明确报告，不将组织级凭证标成全服务管理身份；单次成功列举仍需结合身份来源和实际接口校验。
- 查询、同步和对象引用加入服务与真实组织上下文；不另造 Casdoor 组织启用名单或业务实例授权模型。
- 保留 Mayfly 现有账号与动作权限、操作审计及远端错误处理。服务端原有的特殊规则仍以 Casdoor 决定，管理身份不是绕过规则的开关。

## 仍需验证的范围

### 生产只读认证复核

2026-11-09 用户提供一组凭证后，通过生产令牌端点验证了 `client_credentials`。来源应用经 API 核对为 `app-built-in`，所属组织为 `built-in`；它是内置控制台应用，不是另建的 Mayfly 专用身份。

令牌端点返回 Bearer 令牌及有效期；组织列表返回成功并包含 3 条记录，其中包含 `built-in` 和 `smartcore`。这两个组织的应用、用户首页只读探测均返回成功。只记录认证结果和范围，不保存或输出实际凭证、令牌及用户资料；没有执行用户、应用或权限写入，没有修改内置应用配置。

本次证明现场生产端点可用该方式认证及读取上述范围，不代表所有生产管理写操作已验证，也不代表“专用身份”已经配置。正式连接所需的独立管理应用和 Mayfly 自动令牌管理仍需实施。

本次不声称所有 CRUD 或授权行为已通过。用户启停、用户激活、其他应用字段、角色/权限完整语义、完整分页、令牌有效期单位与边界、并发续取、撤销后的既有令牌行为及生产配置均需后续核实。

当前 Mayfly 线上版本仍使用旧 Basic 应用凭证客户端，不具备上述服务级组织发现和自动令牌管理。此核查没有在生产 Casdoor 新增管理应用、用户或凭证，也没有修改生产保护配置。

复现入口：仓库 `scripts/casdoor/isolated_api_probe.py`，在具备所需本地测试镜像的隔离测试主机设置 `CASDOOR_IMAGE`、`MYSQL_IMAGE` 和 `PROBE_MANAGEMENT_AUTH=1`，输入 base64 编码的空初始化模板。不可传入生产配置或生产导出数据。
