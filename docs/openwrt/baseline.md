# 实验环境验收（2026-11-25）

用户提供的脱敏终端记录，非当前 PR 的自动测试。

- ImmortalWrt 24.10 系列，Linux 6.6.133，ARM64，约 484 MiB RAM，USB ext4 外置存储。
- 官方 v1.3.0 → v1.5.0：SHA256 与 Ed25519 签名均通过，版本输出 1.5.0 (75550c9)，旧程序及数据完整备份。
- /healthz/ready: initialised、music_read_only、database、data_writable 均 true。
- procd running、开机自启已配置、LuCI RPC 状态可读、进程 UID 10001。
- 14:19（用户提供的设备终端记录，设备本地时间），`ubus call luci.trackferry status`：PID 18081，RSS 20700 KiB，cgroup current 9437184 bytes，peak 16826368 bytes。
- 14:23（同一设备本地时间），同一 RPC：PID 18081，RSS 23648 KiB，current 16257024 bytes，peak 17313792 bytes，available 136004 KiB，OOM=0。
- memory.high 50331648、memory.max 67108864、memory.swap.max 0。
- RPC 输出为顺序读取的不同统计量，不是原子快照。RSS 统计进程驻留映射，cgroup current 统计归属该 cgroup 的内存收费（含内核内存及文件缓存）；共享/已在其他 cgroup 收费的页可能计入 RSS 而不计入本 cgroup current。不存在 current 必须大于 RSS 的恒等式。参见 [Linux cgroup v2 Memory Ownership](https://docs.kernel.org/admin-guide/cgroup-v2.html#memory-ownership)。峰值跨启动可能保留；不能相加或据此证明内存更省。待实机追加 /proc/PID/status、smaps_rollup（如可用）、/proc/PID/cgroup、memory.stat、memory.current/peak 的同轮采样；本记录保留原始 RPC 数字而不推断具体差异原因。
- 已安装的临时 LuCI 与服务脚本并不等于仓库新版实现已通过测试。
- 待验证：新版实际音乐播放、网易云会员授权、LuCI 启停与 ACL、USB 延迟挂载、冷重启、24h 稳定性。

不公开真实 LAN 地址、账号、设备标识、原始日志、数据库、凭据或备份。
