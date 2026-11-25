# 实验环境验收（2026-11-25）

用户提供的脱敏终端记录，非当前 PR 的自动测试。

- ImmortalWrt 24.10 系列，Linux 6.6.133，ARM64，约 484 MiB RAM，USB ext4 外置存储。
- 官方 v1.3.0 → v1.5.0：SHA256 与 Ed25519 签名均通过，版本输出 1.5.0 (75550c9)，旧程序及数据完整备份。
- /healthz/ready: initialised、music_read_only、database、data_writable 均 true。
- procd running、开机自启已配置、LuCI RPC 状态可读、进程 UID 10001。
- v1.5.0 启动采样：RSS 20700 KiB，cgroup current 9437184 bytes。
- 后续采样：RSS 23648 KiB，current 16257024，peak 17313792，available 136004 KiB，OOM=0。
- memory.high 50331648、memory.max 67108864、memory.swap.max 0。
- 峰值跨服务启动可能保留；RSS 与 cgroup current 不可相加。
- 已安装的临时 LuCI 与服务脚本并不等于仓库新版实现已通过测试。
- 待验证：新版实际音乐播放、网易云会员授权、LuCI 启停与 ACL、USB 延迟挂载、冷重启、24h 稳定性。

不公开真实 LAN 地址、账号、设备标识、原始日志、数据库、凭据或备份。
