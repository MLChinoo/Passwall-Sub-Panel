# v3.9.0 共享 client 迁移 —— V4 清理指南

> 目的：把 v3.8(每节点一个 client / ownership 表)→ v3.9(每面板一个共享 client /
> `psp_client`)的**过渡期代码**集中登记,让将来 V4 移除遗留路径时**一处可查、不易漏**。
> 本文也顺手登记其它明确标注为「next major / v4.0.0 删除」的兼容代码。

## 背景

- **v3.8(遗留)**:`user_xui_clients` 表(GORM `ownershipRow` / `domain.XUIClientEntry`),
  每个 (user, node) 一行,经 `ports.OwnershipRepo` 访问。
- **v3.9(当前)**:`psp_client` + `psp_client_inbound`(`ports.PSPClientRepo`),每个用户每面板
  一个共享 client,跨多入站;渲染从 UUID 推导每节点凭据。
- **运行时**:迁移完成后 `user_xui_clients` 会被 `DropIfMigrated` **物理删除**;此后
  `ownership_repo` 的查询**静默返回空**(不报错)。

## 状态拆分迁移记录

v4 前已经把「账号是否能登录面板」和「代理/订阅服务是否可用」拆成两层:

- **账号状态**:`enabled` + `auto_disabled_reason` + `disable_detail`,只负责面板登录 /
  审批 / 删除 / 邮箱验证等账号级状态。
- **服务状态**:`service_disabled_reason` + `service_disable_detail` +
  `service_disabled_at`,只负责代理访问、订阅输出和 3X-UI client enable。

本次不做历史禁用账号的自动搬迁:

- 旧数据里已经 `enabled=false` 的账号保持原样,不会在启动时自动改成服务暂停。
- `service_disabled_*` 列只承接新逻辑之后产生的到期、流量超限、客户端封禁、手动暂停等服务状态。
- 如需恢复历史账号,由管理员在后台按实际情况手动启用账号或调整服务状态。

后续清理注意:

- 自动封禁客户端只暂停服务,不再禁用账号。
- 客户端封禁恢复时通过 `ResumeServiceAndSync` 自动清空 `block_violation_count`。
- 管理员后台应展示和管理服务状态,不能再把「禁用账号」当作所有停用场景的唯一入口。
- `service_disabled_*` 是新状态模型的长期字段,不是 V4 临时迁移字段,不要清理。

## AutoMigrate 边界

`AutoMigrate(schemaModels...)` 只负责创建/补齐当前模型需要的表和列,**不会自动删除**旧表、
旧列、旧索引或旧数据。V4 清理必须显式执行:

- `DropTable` / `DropColumn` / `DropIndex`,或者
- 把 v3 期间的 `cleanupLegacyState` 幂等清理块搬进 v4 的 `psp migrate` 流程。

因此,删除 `ownershipRow` 或从 `schemaModels` 移除某个模型,只代表新装不再创建它;升级库里已经存在的
表/列仍要靠明确的迁移或清理代码处理。

## 约定:`MIGRATION(v3→v4)` 标记

每一处**只为过渡期存在、V4 该删/该简化**的代码,都带注释标记:

```go
// MIGRATION(v3→v4): <说明 + 移除动作>
```

V4 清理第一步就是:

```bash
grep -rn "MIGRATION(v3→v4)" internal
```

标记是**唯一事实来源**(不维护行号清单,避免漂移)。下面只列**类别 + 移除配方**。

## V4 移除配方(按类别)

### 1. 遗留 ownership 表 + 仓库(靠编译器兜底)

删除 `ports.OwnershipRepo` 接口 + 适配器 `internal/adapters/sqlstore/ownership_repo.go`
+ `ownershipRow`/`user_xui_clients` schema(`schema.go`)+ `domain.XUIClientEntry`。
**删掉接口后,编译器会逐个报出所有调用点** —— 它们全是遗留每节点逻辑,挨个删即可。
这类**不需要**手动标记(类型系统就是登记表)。

涉及的遗留 sync 原语随之一起删:`DelAllOwnedForUser` / `DelAllOwnedForInbound` /
`ClaimClient` 的 `ownership.Add` / 每节点 `RotateClientUUID` / 每节点 `AddClient`·`UpdateClient`。

### 2. 一次性 shared-client 迁移逻辑(带标记)

- `user.Service`:`EnqueueSharedMigration`、`BackfillPSPClients`、`SharedMigrationComplete`。
- `domain.SyncTaskUserMigrate` 任务类型 + 其处理分支(`runUserTask` / `ProcessDueTasks`)。
- `sharedclient.Service`:`DeleteLegacyForUser`、`SetOwnershipRepo` 及其仅为删 legacy per-node
  client 服务的 ownership 依赖。(原 `MigrateUser` 一次性 helper 已在 v3.9.0 删除——迁移由
  `user.ResyncMembership` 的 provision→lifecycle→删 legacy 安全顺序驱动,不再有独立 helper。)
- `app.go`:`sharedMigratorAdapter`、`SetOwnershipRepo`、`SetSharedMigrator`、开机迁移入队、
  `DropIfMigrated` 轮询、开机 heal;reconcile 循环里的 `migrationComplete` 分支 +
  `shouldRunSharedHeal` 的「migrating → 每 tick」分支(改为永远走 backstop 节奏)+
  `sharedHealBackstopEvery`。
- `ownership_repo.go`:`DropIfMigrated`、`gone` 标记、`isMissingTableErr`。

### 3. 读路径里的「psp_client 否则 ownership」回落分支(⚠️ 最易漏,务必带标记)

这些分支**删掉 ownership 仓库后不会编译报错**(它们还有 psp_client 分支会留下),所以**必须**
靠 `MIGRATION(v3→v4)` 标记找到,把 ownership 回落删掉、只留 psp_client 路径:

- `sync.go` `ensureInboundDeletable` —— ownership.Exists 否则 psp。
- `node.go` `ListClientsOfInbound` —— ownership 否则 psp 解析 owner。
- `admin_node.go` `ClaimClient` —— `preExistingOwned` 同时数 ownership + psp。
- `traffic.go` `PollOnce` 的 `panelsToFetch`(ownership ∪ psp 面板并集);`UserServerUsage`
  的「无 psp_client 时回落到 ownership 聚合」分支。

### 4. 已经清理干净的(无需动作)

`user_me.go` `ServerStatus`(B5 已改为走 group 选择器,不再读 ownership)。
渲染 / 订阅 Userinfo 头 / SSO / 仪表盘 / top-users / rollup —— 本就不依赖 ownership。

### 5. 旧订阅客户端设置兼容(非 shared-client,但也是 V4 清理)

v3.3.0 把旧的 `sub_client_rules` + `sub_import_clients` 合并为 `sub_clients`。为了升级兼容,
当前仍保留一次性折叠逻辑:

- `internal/adapters/sqlstore/sub_clients_legacy.go` 整个文件。
- `ports.UISettings.SubClientRules` / `ports.UISettings.SubImportClients` 两个 deprecated 字段。
- `settings_kv_repo.go` 中 `sub_client_rules` / `sub_import_clients` 的 legacy KV descriptor。
- 对应测试:`internal/adapters/sqlstore/sub_clients_legacy_test.go` 以及 settings 默认值测试中对
  deprecated 字段的断言。
- 文档引用:`docs/ARCHITECTURE.md` 中 sub settings 的 v3.3.0 兼容说明。

删除前确认所有 v3 安装至少启动过一次新版本,让旧 KV 被折叠进 `sub_clients`。V4 若提供
`psp migrate` 自动升级,也可以在 migrate 阶段先执行一次同等折叠,再删除运行时兼容代码。

### 6. `cleanupLegacyState` 移交

`cleanupLegacyState` 里的块是 v3 同 major 内部演进留下的幂等清理,不是 AutoMigrate 能替代的东西。
V4 发版时按 `docs/ARCHITECTURE.md §17.4` 的规则处理:

- 把仍需覆盖 v3→v4 升级的清理块搬进 v4 的 `psp migrate` 前序步骤。
- 从 v4 运行时的 `cleanupLegacyState` 删除这些旧块,避免主程序长期携带上个 major 的清理逻辑。
- 当前已登记的块包括 separator 旧行/旧列清理、`sub_logs` 旧单列索引清理、`users.idx_users_email`
  清理等;以当时 `schema.go cleanupLegacyState` 的实际内容为准。

## V4 清理 checklist

1. `grep -rn "MIGRATION(v3→v4)" internal` —— 处理每一处(类别 2、3)。
2. `grep -rn "Remove in the next major\\|v4.0.0\\|Removed at V4\\|V3-ONLY" internal docs`
   —— 处理未挂 `MIGRATION(v3→v4)` 标记的 next-major 兼容代码(类别 5 等)。
3. 删 `ports.OwnershipRepo` + 适配器 + schema + `XUIClientEntry`,跟着编译错误删干净(类别 1)。
4. 把 `cleanupLegacyState` 里仍需要的幂等清理搬进 v4 `psp migrate`,再清空/简化运行时 cleanup(类别 6)。
5. 删 `internal/migrate` 里仅服务 v2→v3 的部分,替换为 v3→v4 迁移逻辑。
6. 更新/删除相应测试与文档:`ownership_repo_test`、shared migration 测试、legacy sub-client 测试等。
7. `go build ./... && go vet ./... && go test ./...` 全绿;前端若改到类型/API,跑 `npm run build`。
8. 删除本文件。
