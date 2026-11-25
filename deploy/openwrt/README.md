# OpenWrt 原生部署（Draft；不要直接用于现有运行实例）

基于已实机运行的 upstream v1.5.0 / ARM64。此 PR 对实验脚本重新整理，未修改 Go、React、Docker 或上游签名公钥。新脚本尚未实机验证。

## 约束
- 首版仅 aarch64、ext4 挂载到 /mnt 的直接子目录、cgroup v2 memory controller；安装路径为该挂载点下 trackferry。记录实际设备名，设备名变化时拒绝启动；UUID 绑定、AMD64、已有部署迁移后续完成。
- LAN IPv4 必须由管理员明确指定且当前绑定本机；不得使用回环/通配地址；管理员负责确保它确为 LAN 而非 WAN，并检查防火墙。不会打开任何防火墙端口。
- 需预先存在专用 trackferry UID/GID 10001，且无其他附加组；不自动改写 /etc/passwd 或 /etc/group。现有实验账号可复用，但已有部署不能覆盖。确认账号无其他用途。
- 只接受固定官方 v1.5.0 ARM64 程序及签名，安装前后验证 SHA256 和 Ed25519。需 OpenSSL、BusyBox 工具、procd；不下载、不启停、不启用自启。管理脚本及配置 root 拥有，data 仅服务用户可写。
- 初始安装固定 v1.5.0；原版管理页检查官方新版本，由用户主动确认安装。沿用官方验签、数据库备份和 exec 交接，不自建 OTA。管理层升级、迁移、LuCI 另立 PR。
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
用实际 LAN IP 替换示例。确认 readiness、进程 UID/GID、cgroup、播放及无 OOM；测试 stop/restart、异常挂载、冷重启、24h。隔离测试机通过手动启动后可用 tf enable 验证自启与 USB/LAN 晚就绪，未验收前不要部署到生产。若安装拒绝既有路径，不要删除旧部署绕过检查。

`sh uninstall.sh` 仅停止/禁用并移除管理层；保留程序、数据、配置、账号、cgroup。此后重新安装会拒绝现存目录，需后续显式迁移/恢复流程。不会卸载实验版 LuCI。

## 接管音箱
`takeover_speakers` 默认为 `[]`，任何音箱都不能接管，管理页会提示先写名字。先在 `<MOUNT>/trackferry/config.yaml`（root 拥有，644）写入音箱名，再回管理页逐个接管。

## 官方更新（用户确认）
默认 update_check: true，仅检查版本；登录管理员主动点击更新才安装。上游 Linux 使用官方 Ed25519 验签、备份数据库，syscall.Exec 原地切换，保留 PID、UID 与 cgroup。不要拒绝 installed.json，也不要手动删除它。状态识别兼容 data/update/trackferry-X.Y.Z，并使用 procd main PID；这是状态检查，不替代上游签名验证。基础二进制保持官方 v1.5.0；管理页显示实际运行版本。若更新启动失败，无 respawn，管理员检查日志后手动启动，官方三次尝试后的回退机制仍由上游负责；不可自动重启三次。保留独立完整备份。64 MiB 内存限制及升级期间子进程、磁盘缓存、OOM/断电恢复需实机测试。

## 延迟挂载和 LAN
开机时创建 /var/run/trackferry.boot-pending（仅本次开机），检查挂载与 LAN；不就绪则等待 OpenWrt mount add 或 iface ifup/ifupdate 事件，无常驻轮询。就绪时消费标记并发起一次启动；重复事件、手动启动/停止、OOM 或进程退出不会自动重新拉起。依赖目标系统 blockd 挂载事件，需在 ImmortalWrt 实机核实；未收到事件时手动 tf start。安装时不启用自启；卸载移除本插件两个 hook。仅解决启动时的晚就绪，不支持运行中强拔 USB 自动恢复。

## 复审后的边界
- procd 注册始终提供稳定的实例定义，避免 guard 失败时提交空实例清单；实际 child 在 exec 之前检查挂载、身份和 cgroup。tf start/restart 额外检查真实程序 PID、UID、cgroup；仍需实机验证重复 start、直接 init start、stop/restart 和 procd PID 连续性。直接 init 的返回值不代表 readiness。
- 启动器开启根 cgroup memory controller 时会修改全局 subtree_control；卸载不会关闭它，避免影响其他已使用该 controller 的服务。拒绝 noexec/ro 挂载。OpenSSL 需支持 Ed25519 的 pkeyutl -rawin（例如 3.x）。
- 卸载比对原始管理入口文件，若已被替换则拒绝操作；保留数据和现有其他组件。
- 不允许同时手工启动另一实例并使用同一 data；尚未建立跨进程独占锁。安装/卸载并行及异常断电恢复仍需验证。
- 官方更新及晚挂载实现尚未实机验收，仍为发布门禁；安装/卸载使用 flock 串行化，尚需测试并发管理与断电恢复。禁止直接在生产实例部署此 Draft。
