# 数据库重构计划

> **状态：已实施完成（2026-05-17）**。所有 12 阶段（Phase 1-5 +5×子任务）落地，`go build ./...` + `go test ./...` 全绿，迁移程序 `cmd/migrate-db-v2/` 已就位并通过空源 smoke 测试。本文档保留作为决策与实施过程的归档；后续可在迁移程序运行验证完毕后整体删除。
>
> 目标：**提升效率** + **为未来扩展留够空间** + **让整个数据库更统一**。
>
> 范围：内部 schema + 流量管理策略一并审视。Side-by-side 新库迁移（Cloudreve `drive`/`drive_v2` 模式），旧库永久 backup。

---

## 1. 背景

当前 18 张表，存在四类问题：

| 类别 | 具体表现 |
|---|---|
| **配置散乱** | `ui_settings`（30+ 字段宽表）/ `mail_settings` / `saml_config` / `oidc_config` 四张配置表，phpMyAdmin 侧栏里散在 14 张业务表中间 |
| **命名不统一** | `*_config` vs `*_settings` 混用；`xui_clients` 字面误导（实际是 ownership 表，不是 client 缓存）|
| **数据冗余** | `panel_name` 在 `nodes` / `xui_clients` / `client_traffic_snapshots` 三张表都冗余存，panel 改名后不会同步 |
| **死代码** | `rule_sets` 表存在但生产用的是 yaml 文件实现，DB 表完全空跑 |
| **数据增长无控** | `traffic_snapshots` / `client_traffic_snapshots` / `node_traffic_snapshots` 三张快照表**无 retention 清理**，按 50 用户 × 3 panel × 3 inbound × 5 分钟轮询估算，`client_traffic_snapshots` 一年 ~4700 万行 |

参考：phpMyAdmin / DataGrip 打开后，配置类和业务类混杂排序，admin 找设置要从一堆业务表里翻。

---

## 2. 目标三原则

### 2.1 提升效率
- 写入量减少：删除冗余列、空 delta 不写 snapshot
- 查询路径优化：history bucketing 下沉到 DB；period baseline 用字段直存而非随机点查
- 长期可维护：snapshot 表自动清理，永远不爆炸

### 2.2 未来扩展留够空间
- 加新设置零 schema 变更（KV 表）
- 多发件账号（mail）/ 多 SSO provider（saml/oidc）的扩展路径预留：保留独立单行表 → 未来变多行表时不破坏 KV 模型
- 业务命名传递语义（`user_xui_clients` 暗示 join/mapping 表）

### 2.3 统一
- 所有"配置类"表带 `_settings` 后缀
- 每个事实只在一个地方存（无冗余列）
- 命名风格一致（snake_case，业务实体名复数）

---

## 3. 数据库改动清单

### 3.1 表层面

| 操作 | 表 | 说明 |
|---|---|---|
| **新增** | `settings` | KV 主配置表，统一管理 7 个 type 分组 |
| **删除** | `ui_settings` | 30+ 字段宽表，全部并入 `settings` |
| **删除** | `rule_sets` | 死代码（生产用 yaml 文件实现，见 [internal/adapters/yaml/ruleset_repo.go](../internal/adapters/yaml/ruleset_repo.go)）|
| **改名** | `saml_config → saml_settings` | 统一 `_settings` 后缀 |
| **改名** | `oidc_config → oidc_settings` | 同上 |
| **改名** | `xui_clients → user_xui_clients` | 字面表达"user 的 xui clients 占有"，符合 join 表惯例 |
| **瘦身** | `mail_settings` | 删除 `expire_before_days` / `traffic_remain_percent`（搬到 settings.type=notify）|
| **删列** | `nodes.panel_name` | 冗余，按 panel_id 查 pool。**完整清理**：DB 列 + Go domain 字段 + handler DTO + 前端 9 个 TS/locale 文件 |
| **删列** | `user_xui_clients.panel_name` | 同上 |
| **删列** | `client_traffic_snapshots.panel_name` | 同上 |

> **panel_name 删除影响面实测**（grep 验证）：Go 层 64 处 / 16 文件 + handler DTO JSON 3 处 + 前端 9 文件（types.ts / NodesView / UsersView / TrafficView / DashboardView / api/traffic.ts / api/reconcile.ts / locales/en-US / locales/zh-CN）。`fillPanelName()` 仅 4 个调用点，整体删除干净。

最终 **17 张表**，按职责分 4 类：

```
─── 配置 (4) ────────────────────────────────
settings              -- KV 主配置
mail_settings         -- SMTP 连接（瘦身后）
saml_settings         -- SAML SSO（单 IdP）
oidc_settings         -- OIDC SSO（单 provider）

─── 业务实体 (6) ────────────────────────────
users                 -- 用户身份 + 配置 + 状态
groups_               -- 用户组（"groups" 是 MySQL 关键字）
xui_panels            -- 下游 3X-UI 面板
user_xui_clients      -- user ↔ panel client 占有映射
nodes                 -- 3X-UI inbound 暴露给用户的节点
mail_templates        -- 邮件模板（按 kind 主键）

─── 时序快照 (3) ────────────────────────────
traffic_snapshots         -- user 级流量
client_traffic_snapshots  -- user × panel × inbound × client 级
node_traffic_snapshots    -- node 级

─── 日志/事件 (4) ───────────────────────────
audit_log             -- admin 操作审计
sub_logs              -- 订阅访问日志
mail_sent             -- 发件历史 + 幂等键 (unique user_id,kind,window_key)
sync_tasks            -- 同步任务（带状态/重试）
```

预览文件：`local-build/schema_final_v2.db`

### 3.2 `settings` 表分组（type）

| type | 字段数 | 内容 |
|---|---|---|
| `site` | 8 | 站点品牌：标题 / icon / logo / footer / 主题色 / 邮箱域名 |
| `auth` | 6 | JWT TTL / login_mode / 用户本地登录策略 |
| `sub` | 10 | 订阅渲染：base_url / path / 客户端规则 / 自动禁用 |
| `security` | 9 | 限流 / 留存（含 traffic snapshot retention）/ 应急访问 |
| `runtime` | 5 | Cron / 并发 / 时区 / 个人规则开关 |
| `notice` | 2 | 用户视图：公告 + 快捷链接 |
| `notify` | 2 | 邮件触发阈值（过期 / 流量预警）|

加密字段（`encrypted=1`，AES-GCM 透明 enc/dec）：
- `mail_settings.smtp_password`
- `saml_settings.sp_key_pem`
- `oidc_settings.client_secret`
- `xui_panels.api_token` / `xui_panels.password`

### 3.3 未来扩展暗示（这次不做）

预留路径，不破坏本次结构：

- **多发件账号**：`mail_settings` 单行 → `mail_accounts`（多行）+ 路由策略入 `settings.type=mail_policy`
- **多 SAML IdP**：`saml_settings` → `settings.type=saml_sp`（SP 自身身份单例）+ `saml_idps`（多行）
- **多 OIDC provider**：`oidc_settings` → `oidc_providers`（多行）

---

## 4. 流量管理优化

### 4.1 现状评估

**设计上做得对的地方**：
- `users.lifetime_*_bytes` 是 monotonic 累加 → Xray 重启不丢
- 并发拉 panel + `panelSem` 限流 → 50 panel 不至于 fan-out 风暴
- `pollSink` 批量 INSERT → poll 结束 3 个 batched insert（[traffic.go:336-350](../internal/service/traffic/traffic.go#L336-L350)）
- floor push 到 3X-UI inbound → panel 离线也能断流
- emergency access 配额检查 → 超额自动 end 窗口

**问题清单**（按优先级）：

| 优先级 | 问题 | 影响 |
|---|---|---|
| P0 | 三张 snapshot 表无 retention | `client_traffic_snapshots` 一年 ~4700 万行，索引随之膨胀，写入变慢 |
| P0 | snapshot.total_bytes 语义不一致 | `traffic_snapshots.total_bytes` 现在存的是 lifetime 累计，`client_traffic_snapshots.total_bytes` 仍是 raw 计数器。dump 一眼分不清，admin 直接看 DB 会困惑 |
| P1 | `periodUsage()` 走随机点查 | 每个 user 每次 PollOnce 都 `LastBefore(user_id, period_start)`。50 user 50 次 |
| P1 | `client_traffic_snapshots` 写入冗余 | 即使 client 上次到这次 delta=0（没人上线）也写一行。可删 |
| P1 | history bucket 在内存算 | `HistoryFor` 拉时间范围所有 snapshot 到 app 层 GROUP，month 视图一个用户一年 ~10 万行 |
| P2 | `PollOnce` 单函数 250+ 行 | 可读性差但功能正常，不阻塞 |
| P3 | `client_traffic_snapshots` 缺分区 | 万级用户后值得做 PARTITION BY RANGE(captured_at)；现规模不必 |

### 4.2 这次要做的（P0 + 部分 P1）

#### P0-1 — 加 retention
- 新增字段 `settings.type=security, name=traffic_snapshot_retention_days`，默认 180
- 在已有 cron 调度里挂 `traffic.Cleanup(ctx)` 函数：DELETE WHERE `captured_at < NOW() - INTERVAL N DAY`
- 三张表都加：`traffic_snapshots` / `client_traffic_snapshots` / `node_traffic_snapshots`
- 索引 `(user_id, captured_at)` / `idx_client_time` / `(node_id, captured_at)` 已就位，DELETE 走索引

#### P0-2 — 把 client-level lifetime 提升到 `user_xui_clients` 表

**当前语义不一致**：`traffic_snapshots.total_bytes` 存 lifetime 累计，`node_traffic_snapshots.total_bytes` 也存 lifetime，但 `client_traffic_snapshots.total_bytes` 存的是 raw 计数器（[traffic.go:438-440](../internal/service/traffic/traffic.go#L438-L440)）。dump 出来一眼分不清。

**方案**（已实施）：三张实体表（users / nodes / user_xui_clients）都持有 `lifetime_*_bytes` monotonic 累加字段；三张 snapshot 表统一存 **lifetime cumulative**（不是 raw）。`user_xui_clients` 额外持有 `last_raw_*_bytes` 作为下一轮 monotonicDelta 的 baseline，替代了 旧版"读上一条 client_traffic_snapshot.raw"路径。

| 实体表 | lifetime 字段 | 对应 snapshot 表（重构后均存 lifetime） |
|---|---|---|
| `users` | `lifetime_up/down/total_bytes`（已有）+ `period_baseline_bytes`（新增） | `traffic_snapshots` |
| `nodes` | `lifetime_up/down/total_bytes`（已有）| `node_traffic_snapshots` |
| `user_xui_clients` | `lifetime_up/down/total_bytes` **（新增）** + `last_raw_up/down/total_bytes` **（新增 baseline）** | `client_traffic_snapshots`（重构前是 raw，现改为 lifetime；migrate-db-v2 跳过历史 client snapshot 防止语义混淆） |

收益：
- snapshot 表语义统一为 lifetime —— admin dump 三张表看到的都是同一时刻的累计值
- per-client 历史曲线可直接由 snapshot 差值算出（lifetime 增量天然即是该窗口的实际流量）
- `recordClientStats` 简化：从 `ownership.LastRawXxx`（行内 baseline）→ 计算 delta → 累加到 `user_xui_clients.lifetime_*` + snapshot 写 lifetime。**0 次** snapshot SELECT 替代旧版的 `LatestForClient` 随机点查

#### P1-1 — period baseline 直存字段（**含 mailer.go 重复实现合并**）
- `users` 加列 `period_baseline_bytes int64`（period_start 时刻的 lifetime 快照）
- period rollover 时 `period_baseline_bytes = lifetime_total_bytes`
- `periodUsage()` 简化为 `lifetime_total_bytes - period_baseline_bytes`（O(1) 内存计算，0 个 DB 查询）
- 旧 LastBefore 查询路径可删

> **重复实现合并**（grep 发现）：[traffic.go:861](../internal/service/traffic/traffic.go#L861) 和 [mailer.go:977](../internal/service/mailer/mailer.go#L977) 各自有一份 `periodUsage`，**逻辑一致但代码重复**，都走 `LastBefore` 随机点查。本次改造同时：
> - 删除 mailer.go 那份 `periodUsage`
> - 在 `domain.User` 上加一个 `PeriodUsed() int64` getter（或在 traffic.Service 暴露为公共方法），mailer / traffic 共用
> - 一处真相，避免后续两份逻辑分叉

#### P1-2 — 空 delta 不写 client snapshot
- `recordClientStats` 里：如果 prev 存在且 `up == prev.up && down == prev.down`，跳过 INSERT
- 用户没上线时 PollOnce 不再产生垃圾行
- 估算：实际活跃用户大概 20-30%，写入量降到 1/3

### 4.3 这次不做（留 P1-3 / P2 / P3）

- history bucket 下沉到 DB：跨 MySQL/SQLite 兼容性麻烦，工程量大，先看 P0+P1 改完性能够不够
- `PollOnce` 拆函数：纯重构，无功能改进
- 分区表：现规模不需要

---

## 5. 实施步骤

**迁移策略：side-by-side 新库**（同 Cloudreve `drive` → `drive_v2` 升级模式）

- V3 主程序代码 **100% 只认新 schema**，旧表的知识不出现在主程序里
- `cmd/migrate-db-v2/` 是唯一同时知道两边 schema 的程序，跑完即删
- 旧库 **完全不被修改** —— 永久 backup，admin 验证 V3 稳定后自行 DROP

```
V2 主程序（现在）       V3 主程序（重构后）
  ↓                        ↓
旧库 panel              新库 panel_v3
  (旧 schema)              (新 schema)
       \                    /
        \                  /
         cmd/migrate-db-v2/   ← 只这一个程序同时知道两边
         （独立 binary, 跑完即删）
```

**实施顺序：先主程序（Phase 1-3），后迁移 cmd（Phase 4）**。理由：
- 迁移 cmd 的"目标 schema"由主程序定义，先有对岸才造桥
- 主程序可以在干净 DB 环境（新建空库 + GORM AutoMigrate）独立开发 + 测试
- 失败 rollback 简单：主程序代码 git revert；生产数据完全未碰
- 迁移 cmd 是 thin shim（~300 行 Go），最后一步成本最低

按依赖顺序：

### Phase 1 — Schema & 配置层（核心）
1. 新增 `settings` 表 + `internal/adapters/mysql/settings_kv_repo.go`（KV 通用 Load/Save，支持加密透明 enc/dec）
2. 删除 `ui_settings` 表 + [settings_repo.go](../internal/adapters/mysql/settings_repo.go) + `uiSettingsRow` struct
3. 改名 `saml_config → saml_settings`、`oidc_config → oidc_settings`（Go `TableName()` + 测试名 + 文档）
4. `mail_settings` 删除两个字段，迁移到 `settings.type=notify`
5. 服务层 `ports.UISettings` typed struct 保留，service 用 SettingsRepo Load/Save 一次 → 填进 struct
6. 删除 `rule_sets` 表 + [mysql/ruleset_repo.go](../internal/adapters/mysql/ruleset_repo.go) + `ruleSetRow` + [conn.go:89](../internal/adapters/mysql/conn.go#L89) 注入

### Phase 2 — 业务表清理
7. `xui_clients → user_xui_clients` 改名（GORM TableName + struct + repo 文件 + 测试）
8. 删除 `nodes.panel_name` / `user_xui_clients.panel_name` / `client_traffic_snapshots.panel_name` 列
9. 删除 [node.go:197-207](../internal/service/node/node.go#L197) `fillPanelName` 兜底逻辑，改为 service 层每次从 pool 查（pool 已经在 admin 改 panel 时 Remove+Add 自动刷新）
10. 删除 `domain.Node.PanelName` / `domain.XUIClientEntry.PanelName` / `domain.ClientTrafficSnapshot.PanelName` 字段
11. handler / DTO / 前端 API 类型相应清理（前端展示从 panel_id 查 panel pool 拿 name）

### Phase 3 — 流量管理优化
12. `traffic.Cleanup(ctx)` 函数 + cron 挂接（P0-1）
13. `client_traffic_snapshots` 语义统一为 lifetime + 维护 per-client lifetime 字段（P0-2）
14. `users.period_baseline_bytes` 列 + `periodUsage` 改 O(1)（P1-1）
15. `recordClientStats` 空 delta 跳过 INSERT（P1-2）

### Phase 4 — 迁移程序（最后一步）
16. 新建 `cmd/migrate-db-v2/`：`main.go` / `legacy_schema.go`（旧 GORM struct，独立定义不依赖主程序 adapters/mysql）/ `migrate.go` / `README.md`
17. 实现流程：
    - 命令行参数 `--src=panel --dst=panel_v3`（admin 自己决定库名）
    - 连旧库 + 连新库
    - 在新库跑 GORM AutoMigrate（复用主程序的 schema 注册）建出新 schema
    - 从旧库每张表读 → 转换 → INSERT 到新库
    - 行数对比 sanity check
    - 打印 "迁移完成。请把 config.yaml 的 database 改成 panel_v3 后启动主程序"
18. 在干净 SQLite 上模拟一遍旧 schema → 跑迁移 → 跑主程序 → 数据对得上
19. 在生产 MySQL 上真跑：旧库零修改，新库填好

### Phase 5 — 收尾
20. 单元测试全部跑通（[[feedback_tdd]] 改行为前跑现有测试，挂了对症修）
21. 集成测试：admin UI 改 panel 名 → nodes / user_xui_clients 显示立即跟着改（fillPanelName 删除后的行为验证）
22. 更新 [docs/ARCHITECTURE.md](ARCHITECTURE.md) 反映新 schema
23. 删除 `cmd/migrate-db-v2/` 目录（同 commit 写明"v2 migration ran successfully on $date"）
24. 删除本文档（重构完成即归档/删除）

---

## 6. 决策记录

### 6.1 已定
- 数据库类型：MySQL（生产）；本地预览用 SQLite
- 改名方案：`xui_clients → user_xui_clients`（[本次对话](.) 已确认）
- 配置表后缀：`_settings`
- KV 表分组 type：7 个（site / auth / sub / security / runtime / notice / notify）
- mail/saml/oidc 保留独立表（为未来多账号 / 多 IdP 留扩展位）

### 6.2 已定（开干前所有决策已闭环）

- **迁移策略**：**side-by-side 新库**（Cloudreve `drive` → `drive_v2` 模式）。V3 主程序只认新 schema，旧库零修改作永久 backup。迁移 cmd 是唯一同时知道两边 schema 的程序，跑完即删
- **库名约定**：admin 自己决定（cmd 接 `--src` `--dst` 参数）；推荐 `panel_v3`（或 `psp` 等）
- **旧库 DROP 时机**：admin 验证 V3 稳定一周后自行 DROP，cmd 不碰
- **panel_name 删除范围**：完整清理 —— DB 列 + Go domain 字段 + handler DTO + 前端 9 个文件 + `fillPanelName()` 兜底函数
- **snapshot 语义统一**：在 `user_xui_clients` 加 `lifetime_*_bytes` 字段；三张 snapshot 表统一存 raw 计数器
- **periodUsage 优化**：删除 mailer.go 那份重复实现，traffic + mailer 共用一处真相

---

## 7. 风险与回滚

### 风险
1. **迁移脚本 bug 导致新库数据错误** — 缓解：旧库零修改 + 迁移可无限重跑（`DROP DATABASE panel_v3; CREATE DATABASE panel_v3;` 重来）
2. **fillPanelName 删除后某处显示 panel_name 为空** — 缓解：测试覆盖 reconcile / 节点列表 / 流量表三个调用点
3. **lifetime 字段迁移后第一次 poll 数据跳变** — 缓解：迁移时把 `users.lifetime_total_bytes` 等字段原样搬到新库；第一次 poll 走 LifetimeBaselineAt 兜底逻辑

### 回滚
- **新库方案天然支持完美回滚**：admin 改 config.yaml 把 database 切回旧库名即可，V2 主程序无缝重启
- 主程序代码：每个 Phase 独立 commit，可单独 git revert
- 迁移失败：删新库重建重跑，零成本

---

## 8. 完成标准

- [ ] 所有单元测试通过
- [ ] phpMyAdmin / DataGrip 侧栏排序后，配置类 4 张表通过 `_settings` 后缀视觉聚集
- [ ] admin 改 panel 名，3 秒内所有 node / ownership / 流量 视图反映新名字
- [ ] `client_traffic_snapshots` 跑 retention 后行数收敛
- [ ] [docs/ARCHITECTURE.md](ARCHITECTURE.md) schema 章节同步更新
- [ ] 本文档归档或删除
