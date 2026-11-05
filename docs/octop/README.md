# Octop 运维二开（Mayfly-Go）

本期在 Mayfly-Go 中增加 **Octop 管理** 分组，复用现有主机、数据库连接、资源 ACL 与主机/DB 指标，不平行再造一套 SSH/登录/写库系统。

Mayfly-Go 只做运维管理端：

- 不创建/删除 Casdoor 用户，不改应用授权，不做 Casdoor SSO 替换
- 不修改 Octop 核心业务，不共享其控制库
- 不存储聊天全文、prompt、工具参数/结果、密码、令牌
- 不提供 PostgreSQL 停启、任意远程命令、跨实例批量重启或自动修复

## 配置（无密钥）

参考 `server/config-octop.example.yml`。8 套实例的主机、API、systemd unit、PostgreSQL 只读账号、Casdoor 凭据都是**环境值**，必须按实例填写，不能假设全部相同。

凭据保存位置：

- SSH / 数据库：沿用 Mayfly 已有资源授权凭证（AES）
- Casdoor Client Secret：仅后端加密列 `client_secret_enc`，接口只回显「已配置/未配置」和轮换时间

## 迁移与回滚

- 应用版本真源：`server/internal/pkg/config/app.go` 的 `Version`（实施时为 `v1.12.3`）
- 本批迁移：`v1.12.3-octop-tables`、`v1.12.3-octop-resources`
- 回滚：gormigrate 对本批 `Rollback` 为 noop。需要撤回时：
  1. 停用 Octop 菜单（或从角色收回权限码）
  2. 停采集 cron（停进程即可）
  3. 表可保留；若必须删除，先导出再 `DROP TABLE t_octop_*`（会丢掉集中库事件，源端不受影响）

## 留存

- 集中库默认 90 天，管理员可在总览页「采集与留存」修改
- 源端留存不由本后台改写；若源端更短，页面会标「源端历史已清理」
- 采集失败显示「采集失败」，不会把零事件当成零活动

## 权限码

| 码 | 用途 |
| --- | --- |
| `octop:overview:base` | 运行总览 |
| `octop:instance:base` | 实例详情 |
| `octop:instance:save` | 注册/编辑实例 |
| `octop:instance:del` | 删除实例 |
| `octop:instance:restart` | systemd 重启 |
| `octop:casdoor:base` | 目录只读 |
| `octop:casdoor:save` | 连接/同步/绑定/身份确认 |
| `octop:event:base` | 日志查询 |
| `octop:event:export` | 导出 |
| `octop:collect:recollect` | 手工补采 |
| `octop:config:save` | 留存与并发配置 |

实例可见性在服务端按操作者对**关联 Mayfly 主机**的资源 ACL 收窄后再分页。管理员（id=1）可见全部。

## 采集与并发

- 指标与事件默认每 2 分钟一轮
- 每轮并发默认 3、单实例超时默认 45s，失败指数退避
- 一台异常不会阻塞其他实例刷新

## 兼容性清单模板

对每套实例单独填写，不以一台推断全部。见 `docs/octop/compatibility-checklist.md`。
