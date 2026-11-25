# TrackFerry OpenWrt 增强版（开发中）

上游：[madrays/trackferry](https://github.com/madrays/trackferry)，MIT 许可；保留原作者署名、原始 LICENSE、Docker 部署与音乐功能。独立维护 OpenWrt 原生部署，不代表上游官方发布。

## 边界
- 第一阶段使用上游官方签名的 Linux ARM64 二进制；不改 Go/React、SQLite schema、上游 VERSION。
- OpenWrt 管理层使用 procd、cgroup v2、LuCI/rpcd、短命令 tf；不增加常驻服务或 Docker 依赖。
- 默认 LAN 监听，音箱回调使用实际可达的 LAN IP，不能为 localhost、127.0.0.1、0.0.0.0；不主动开放 WAN。
- USB 外置分区未挂载时拒绝启动，不写入路由器根分区的替代目录。
- 默认非 root UID/GID 10001，GOMEMLIMIT=32MiB、GOGC=75、GOMAXPROCS=1、memory.high=48MiB、memory.max=64MiB、memory.swap.max=0；无法建立保护则拒绝启动，不自动无限重启。不同硬件需实测。
- 音乐源扩展是独立后续任务，仅接入用户有权使用的服务，不将第三方解析器作为默认依赖。

## 分支
main 只同步上游（fast-forward；分叉时人工处理），openwrt 为长期维护分支。功能分支经独立 PR 合入 openwrt；同步上游经单独 PR，避免重写公共历史。普通台账更新可直接提交 openwrt；代码、权限、发布、迁移变更走 PR。仓库默认分支切换须在治理 PR 合并后进行。
发布使用独立 OpenWrt 版本命名，不覆盖上游 tag 或签名。修改主程序后须单独设计签名和更新渠道，禁止用官方更新覆盖自编译版本。

## 入口
- [开发台账](roadmap.md)
- [实机验收基线](baseline.md)
- [安全约束与验收门禁](security.md)
