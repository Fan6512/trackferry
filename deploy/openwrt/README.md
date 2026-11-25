# OpenWrt 原生部署（Draft；不要直接用于现有运行实例）

基于已实机运行的 upstream v1.5.0 / ARM64。此 PR 对实验脚本重新整理，未修改 Go、React、Docker 或上游签名公钥。新脚本尚未实机验证。

## 约束
- 首版仅 aarch64、ext4 挂载到 /mnt 的直接子目录、cgroup v2 memory controller；安装路径为该挂载点下 trackferry。记录实际设备名，设备名变化时拒绝启动；UUID 绑定、延迟挂载热插拔、AMD64、已有部署迁移后续完成。
- LAN IPv4 必须由管理员明确指定且当前绑定本机；不得使用回环/通配地址；管理员负责确保它确为 LAN 而非 WAN，并检查防火墙。不会打开任何防火墙端口。
- 需预先存在专用 trackferry UID/GID 10001，且无其他附加组；不自动改写 /etc/passwd 或 /etc/group。现有实验账号可复用，但已有部署不能覆盖。确认账号无其他用途。
- 只接受固定官方 v1.5.0 ARM64 程序及签名，安装前后验证 SHA256 和 Ed25519。需 OpenSSL、BusyBox 工具、procd；不下载、不启停、不启用自启。管理脚本及配置 root 拥有，data 仅服务用户可写。
- 不支持内置更新与已安装内置更新记录（见下方残留风险）；签名升级、迁移、回滚、LuCI 不属于此 PR。
- 不启用 respawn；挂载、身份、cgroup 或地址不符即失败。默认不自动接管音箱，见下方接管说明。
- 必须在可信 root 控制的 ext4 挂载上安装；不要让普通用户写入挂载根、程序目录或管理脚本。

## 测试命令
先在非路由器环境执行 `sh deploy/openwrt/test.sh`。其只验证语法和纯函数输入，不触碰系统。
在隔离的 ARM64 OpenWrt 测试机（非已有部署）准备官方 release 文件及专用账户，进入本目录：
```sh
sh install.sh /mnt/usb 192.168.1.1 /tmp/trackferry-linux-arm64 /tmp/trackferry-linux-arm64.sig
tf start
tf status
tf mem
```
用实际 LAN IP 替换示例。确认 readiness、进程 UID/GID、cgroup、播放及无 OOM；测试 stop/restart、异常挂载、冷重启、24h。此 Draft 禁止通过 tf enable 开启自启，也不要直接调用 init enable；必须先实现并验证 USB 晚挂载恢复。若安装拒绝既有路径，不要删除旧部署绕过检查。

`sh uninstall.sh` 仅停止/禁用并移除管理层；保留程序、数据、配置、账号、cgroup。此后重新安装会拒绝现存目录，需后续显式迁移/恢复流程。不会卸载实验版 LuCI。

## 接管音箱
`takeover_speakers` 默认为 `[]`，任何音箱都不能接管，管理页会提示先写名字。先在 `<MOUNT>/trackferry/config.yaml`（root 拥有，644）写入音箱名，再回管理页逐个接管。

## 内置更新（残留风险）
上游 Linux 构建总会注入更新安装器，`POST /api/update` 与 `/api/update/install` 不读 `update_check`，已登录的管理员仍可从管理页触发。安装后 `<MOUNT>/trackferry/data/update/installed.json` 存在时，`tf start` 会被门禁拒绝（启动器日志提示检查更新记录）；不要直接删除该记录：新程序可能已经迁移数据库。先停止服务，备份当前状态，再根据已验证的旧版完整备份（程序、签名和数据）恢复；没有可靠备份时不要盲目降级。彻底禁用需上游加开关，另立 PR。

## 复审后的边界
- procd 注册始终提供稳定的实例定义，避免 guard 失败时提交空实例清单；实际 child 在 exec 之前检查挂载、身份和 cgroup。tf start/restart 额外检查真实程序 PID、UID、cgroup；仍需实机验证重复 start、直接 init start、stop/restart 和 procd PID 连续性。直接 init 的返回值不代表 readiness。
- 启动器开启根 cgroup memory controller 时会修改全局 subtree_control；卸载不会关闭它，避免影响其他已使用该 controller 的服务。拒绝 noexec/ro 挂载。OpenSSL 需支持 Ed25519 的 pkeyutl -rawin（例如 3.x）。
- 卸载比对原始管理入口文件，若已被替换则拒绝操作；保留数据和现有其他组件。
- 不允许同时手工启动另一实例并使用同一 data；尚未建立跨进程独占锁。安装/卸载并行及异常断电恢复仍需验证。
- 内置更新仍是发布阻塞，不接受以仅关闭 update_check 或提示用户不点击作为正式解决方案。需独立设计可验证的封堵方案；禁止直接在生产实例部署此 Draft。
