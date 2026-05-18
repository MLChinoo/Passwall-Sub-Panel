# 升级到 v3.0.0：数据库重构

> v3.0.0 是一次破坏性的数据库 schema 重构（KV `settings` 主表、`xui_clients` 改名为 `user_xui_clients`、`panel_name` 冗余列删除、新增 `lifetime/last_raw/period_baseline` 字段等）。**v3.0.0 主程序只识别新 schema**，不会原地升级旧库。
>
> 升级方式：**side-by-side 新库**（参考 Cloudreve `drive → drive_v2` 升级模式）—— 旧库**永远不被新版主程序触碰**，可作为永久 backup；迁移由独立的 `cmd/migrate-db-v2/` 一次性程序完成；admin 在 `config.yaml` 改一行 `database` 字段切换。

适用版本：**当前 ≤ v2.5.x → v3.0.0+**

---

## 0. 升级前必读

1. **AES-GCM 加密字段**（SMTP 密码、SAML SP 私钥、OIDC client secret、3X-UI panel API token / 密码）的密文会原样搬到新库。新版主程序使用 `config.yaml` 的 `jwt_secret`（或 `PSP_JWT_SECRET` 环境变量）作为 secret key material 解密。**新 `jwt_secret` 必须和旧版完全一致**，否则解密会失败、SMTP / SSO / panel 接管全部断掉。

2. **旧库零修改**。迁移程序只读旧库 → 写新库。旧库一行都不动，理论上随时可以切回旧版主程序。

3. **迁移程序跑完就删**。`cmd/migrate-db-v2/` 是一次性工具，git history 是它的审计轨迹。

4. **新库名 admin 自己定**。下面示例用 `psp_v3`（或 SQLite `panel_v3.db`），你可以叫任何名字；迁移程序通过 `--src` / `--dst` 参数接收。

---

## 1. 通用前置步骤（任何部署方式都做）

### 1.1 备份旧库

**MySQL**：
```bash
mysqldump -u root -p --single-transaction --routines --triggers passwall \
  > passwall-backup-$(date +%F).sql
```

**SQLite**：
```bash
cp /opt/psp/data/panel.db /opt/psp/data/panel-backup-$(date +%F).db
# 或 Docker 命名卷：
docker run --rm -v psp-data:/data -v "$PWD":/backup alpine \
  cp /data/panel.db /backup/panel-backup-$(date +%F).db
```

旧库本身是永久 backup，但额外离线 dump 一份仍推荐（防 mysqldump 漏掉触发器等极端场景）。

### 1.2 记录当前 `jwt_secret`

```bash
grep '^jwt_secret' /opt/psp/config/config.yaml
# 或
docker exec psp grep '^jwt_secret' /app/config/config.yaml
```

把这个值原样复制下来 —— 新版 `config.yaml` 必须用同一个值。

---

## 2. 二进制部署升级

适用：直接跑 `psp` 二进制（systemd 或裸跑），数据存 SQLite 或 MySQL。

### 2.1 编译新二进制 + 迁移工具

在 v3.0.0 代码 checkout 出来的工作机上：

```bash
cd /path/to/Passwall-Sub-Panel
go build -o psp ./cmd/panel/
go build -o migrate-db-v2 ./cmd/migrate-db-v2/
```

把这两个二进制 `scp` 到生产机。

### 2.2 停旧版主程序

```bash
sudo systemctl stop psp
# 或裸跑：kill 掉对应进程
```

### 2.3 跑迁移

**SQLite 场景**：

```bash
cd /opt/psp
./migrate-db-v2 --driver=sqlite \
  --src=./data/panel.db \
  --dst=./data/panel_v3.db
```

**MySQL 场景**：先创建空目标库：

```bash
mysql -u root -p -e 'CREATE DATABASE psp_v3 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'
```

然后跑迁移：

```bash
./migrate-db-v2 --driver=mysql \
  --src='user:pw@tcp(127.0.0.1:3306)/passwall?charset=utf8mb4&parseTime=true' \
  --dst='user:pw@tcp(127.0.0.1:3306)/psp_v3?charset=utf8mb4&parseTime=true'
```

迁移程序会打印 plan + 每张表的进度 + `Migration complete`。如果 mid-way 失败，按提示 DROP 目标库重跑（迁移程序首步会写一个 `_migration` sentinel 行，重跑会被 guard 拦住直到 DROP）。

> 也可以加 `--dry-run` 看每张表行数，不实际写入。

### 2.4 替换二进制 + 改 config.yaml

```bash
# 备份旧二进制（保留以备紧急回滚）
mv /opt/psp/psp /opt/psp/psp.bak
cp ./psp /opt/psp/psp

# 修改 config.yaml 指向新库
sudo -u psp vi /opt/psp/config/config.yaml
```

**SQLite**：把 `mysql.dsn` 一段改成（或新增）：

```yaml
mysql:
  dsn: "sqlite:./data/panel_v3.db"
```

**MySQL**：把 `mysql.database` 改成新库名：

```yaml
mysql:
  host: "127.0.0.1"
  port: 3306
  user: "psp"
  password: "your_password"
  database: "psp_v3"        # ← 从 passwall 改成 psp_v3
```

**`jwt_secret` 保持不变**（必须和旧版一致）。

### 2.5 启动 v3.0.0

```bash
sudo systemctl start psp
sudo journalctl -u psp -f
```

观察日志，确认：
- `traffic poll start` 周期性出现，无 decrypt 错误
- 管理后台能登录（SSO 不报 `decrypt secret` 错误）
- 节点 / 用户列表显示正常 panel 名

---

## 3. Docker 部署升级

适用：通过 `docker compose up -d` 跑（official `ghcr.io/kazuhahub/passwall-sub-panel:latest` image）。

### 3.1 拉取新 image

```bash
cd /opt/Passwall-Sub-Panel
docker compose pull
```

确认 image 已是 v3.0.0（标签 `latest` 或 `v3.0.0` / `v3.0.0-beta.X`）。

### 3.2 停容器

```bash
docker compose stop psp
```

### 3.3 跑迁移

**迁移工具不包含在 release image 里**（设计上 release image 只含新版主程序）。最简单的做法是在 host 上跑 Go 二进制：

```bash
cd /path/to/Passwall-Sub-Panel
go build -o migrate-db-v2 ./cmd/migrate-db-v2/
```

然后按 DB 类型操作：

**SQLite + named volume（默认 compose 配置）**：

SQLite 文件在 Docker 命名卷 `psp-data` 里。容器停了之后，把卷挂出来跑迁移：

```bash
# 创建临时挂载容器复制 SQLite 文件到 host 工作目录
docker run --rm -v psp-data:/data -v "$PWD":/work alpine \
  cp /data/panel.db /work/panel.db

# host 上跑迁移
./migrate-db-v2 --driver=sqlite \
  --src=./panel.db --dst=./panel_v3.db

# 把新库写回命名卷
docker run --rm -v psp-data:/data -v "$PWD":/work alpine \
  cp /work/panel_v3.db /data/panel_v3.db
```

**MySQL（外部数据库）**：

跟二进制场景一样，先建空库：

```bash
mysql -u root -p -e 'CREATE DATABASE psp_v3 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'
```

host 上跑迁移（DSN 用 host 视角的连接串）：

```bash
./migrate-db-v2 --driver=mysql \
  --src='user:pw@tcp(127.0.0.1:3306)/passwall?charset=utf8mb4&parseTime=true' \
  --dst='user:pw@tcp(127.0.0.1:3306)/psp_v3?charset=utf8mb4&parseTime=true'
```

### 3.4 改 config.yaml 指向新库

```bash
sudo vi /opt/Passwall-Sub-Panel/config/config.yaml
```

**SQLite**：
```yaml
mysql:
  dsn: "sqlite:/app/data/panel_v3.db"
```

注意路径是容器内路径 `/app/data/panel_v3.db`（对应 `psp-data` 卷的 `/data/panel_v3.db`）。

**MySQL**：把 `database` 字段从 `passwall` 改成 `psp_v3`。`host` 通常是 `127.0.0.1`（compose 用了 `network_mode: host`）。

### 3.5 启动 v3.0.0

```bash
docker compose up -d psp
docker compose logs -f psp
```

观察日志，同 2.5。

---

## 4. 升级后验证清单

按顺序过一遍：

- [ ] 主程序日志没有 `decrypt secret` 错误
- [ ] 管理员能登录后台
- [ ] 节点列表 / 用户列表渲染正常，每个 panel 显示正确 name
- [ ] 改一个 panel 的 name → 立即在所有视图生效（旧版是冗余列要等下次轮询才更新）
- [ ] 流量轮询日志正常 (`traffic poll start ... users=X panels=Y`)
- [ ] 全局设置页面打开，数值都还在（SMTP / SSO / 限流 / 站点品牌 / 通知阈值 / 流量快照保留天数）
- [ ] Admin → 设置 → 通用 找到新增字段：**流量快照保留天数**（默认 180）+ **到期前 N 天提醒** + **剩余流量 < N% 时提醒**（后两个从 mail_settings 搬过来的）
- [ ] 跑一次 SSO 登录测试（如启用），确认 SAML SP 私钥 / OIDC client secret 解密 OK
- [ ] 跑一次邮件测试（如启用 SMTP）
- [ ] 订阅 URL 拉取正常
- [ ] 等一个 cron 周期（5 分钟），用户流量看板数字正常更新

---

## 5. 失败回滚

**主程序启动失败 / 数据明显出问题**：

二进制：
```bash
sudo systemctl stop psp
mv /opt/psp/psp.bak /opt/psp/psp           # 恢复旧二进制
sudo vi /opt/psp/config/config.yaml         # 把 database 改回旧库名（passwall）
sudo systemctl start psp
```

Docker：
```bash
docker compose down
# 把 compose 文件里的 image 标签固定回旧版本（如 :v2.5.5）
# 改回 config.yaml 的 database 字段
docker compose up -d
```

旧库完全没动过，回滚后状态等于升级前。

**迁移程序中途崩溃**：

```bash
# MySQL
mysql -u root -p -e 'DROP DATABASE psp_v3; CREATE DATABASE psp_v3 CHARACTER SET utf8mb4;'

# SQLite
rm /opt/psp/data/panel_v3.db
```

然后重跑迁移即可。`_migration` sentinel 会确保半成品状态被检测到。

---

## 6. 清理旧库（升级稳定一周后）

观察一周确认 v3.0.0 稳定后，可以释放旧库占用的空间。

**MySQL**：
```bash
mysql -u root -p -e 'DROP DATABASE passwall;'
```

**SQLite**：
```bash
rm /opt/psp/data/panel.db
# Docker 命名卷场景
docker run --rm -v psp-data:/data alpine rm /data/panel.db
```

**保留** `passwall-backup-*.sql` 一段时间（建议至少 30 天），万一发现某个边界数据不对还能挖回来。

---

## 7. 常见问题

**Q: 迁移程序报 "dst has 2 rows in `settings` already"**

A: dst 库不是全新的（含 `_migration` sentinel 或其他数据）。按 §5 的 DROP+CREATE 重来即可。

**Q: 启动新版报 `decrypt secret` 错误**

A: `config.yaml` 的 `jwt_secret` 跟旧版不一致。回去对一遍（包括 `PSP_JWT_SECRET` 环境变量覆盖）。

**Q: 管理后台节点列表 panel 名为空**

A: 新版从 in-memory pool 现查 panel name，pool 在主程序启动时初始化。如果 panel pool init 失败（比如 3X-UI 不可达），name 会暂时为空。检查 `xui_panels` 表的 panel 是否能 ping 通。

**Q: 流量曲线在升级时刻有断点**

A: 正常 —— 迁移会保留旧的 `traffic_snapshots` 历史 + 用户的 `lifetime_total_bytes`，但 `client_traffic_snapshots`（per-client 级历史）不复制（新版把它的 raw counter 信息收纳进了 `user_xui_clients.last_raw_*_bytes`）。所以**节点 / 用户级**累计曲线连续，**per-client** 级看板从升级时刻起重建。

**Q: 升级前后 `cmd/migrate-db-v2/` 删除时机**

A: 你升级完、稳定运行一段时间确认无问题后，在下一个 commit 里把 `cmd/migrate-db-v2/` 整个目录删了 + commit message 注明 "v3.0.0 migration ran on YYYY-MM-DD, removing one-shot tool"。git history 是这次重构的审计记录。
