# MTS 备份编排（可商用样例）

## 目标
- 周期性把 live `data_dir` 快照到 `backups/data-snapshot-*`
- 异地/跨主机拷贝
- 旁路 restore-drill 校验
- 保留策略与失败告警

## 脚本
仓库入口：`scripts/mts-backup.sh`

### 获取管理员 Token（示例）
```bash
# 用户登录获取 bearer token（若启用密码认证）
TOKEN="$(curl -sS -X POST "$MTS_BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"user_name":"admin","password":"***","ttl_seconds":3600}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",{}).get("token",""))')"
export MTS_ADMIN_TOKEN="$TOKEN"

# 或直接使用配置中的 admin_token：
# export MTS_ADMIN_TOKEN='static-admin-token'
```


```bash
export MTS_BASE_URL='https://mts.example.com'
export MTS_ADMIN_TOKEN='***'
export MTS_BACKUP_REMOTE='backup@backup-host:/var/backups/mts'
export MTS_BACKUP_DIR='/var/lib/mts/backups'   # 可选：本地清理
export MTS_BACKUP_KEEP=7

# 试跑
./scripts/mts-backup.sh --dry-run

# 生产
./scripts/mts-backup.sh
```

## cron 示例
```cron
15 * * * * MTS_BASE_URL=https://mts.example.com MTS_ADMIN_TOKEN=*** /opt/mts/scripts/mts-backup.sh >>/var/log/mts-backup.log 2>&1
```

## systemd timer 示例
`/etc/systemd/system/mts-backup.service`：
```ini
[Unit]
Description=MTS data_dir backup orchestration
[Service]
Type=oneshot
Environment=MTS_BASE_URL=https://mts.example.com
EnvironmentFile=-/etc/mts/backup.env
ExecStart=/opt/mts/scripts/mts-backup.sh
Nice=10
```

`/etc/systemd/system/mts-backup.timer`：
```ini
[Unit]
Description=Hourly MTS backup
[Timer]
OnCalendar=hourly
Persistent=true
[Install]
WantedBy=timers.target
```

## Dashboard 联动
- 就绪中心：`/ops/readiness` 备份编排清单
- 存储页：`/storage#data-restore` 一键 data-snapshot / restore-drill
- 自检：`make backup-script-check` 或 `./scripts/mts-backup-selfcheck.sh`

## 边界
- 脚本不替代真实边缘 HTTPS 证书验收
- 远程 rsync/凭据与告警通道由部署侧配置
