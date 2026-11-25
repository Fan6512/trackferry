# OpenWrt 原生部署（Draft；不要直接用于现有运行实例）

基于已实机运行的 upstream v1.5.0 / ARM64。此 PR 对实验脚本重新整理，未修改 Go、React、Docker 或上游签名公钥。新脚本尚未实机验证。

## 约束
- 首版仅 aarch64、ext4 挂载到 /mnt 的直接子目录、cgroup v2 memory controller；安装路径为该挂载点下 trackferry。记录实际设备名，设备名变化时拒绝启动；UUID 绑定、延迟挂载热插拔、AMD64、已有部署迁移后续完成。
- LAN IPv4 必须由管理员明确指定且当前绑定本机；不得使用回环/通配地址；管理员负责确保它确为 LAN 而非 WAN，并检查防火墙。不会打开任何防火墙端口。
- 需预先存在专用 trackferry UID/GID 10001，且无其他附加组；不自动改写 /etc/passwd 或 /etc/group。现有实验账号可复用，但已有部署不能覆盖。确认账号无其他用途。
- 只接受固定官方 v1.5.0 ARM64 程序及签名，安装前后验证 SHA256 和 Ed25519。需 OpenSSL、BusyBox 工具、procd；不下载、不启停、不启用自启。管理脚本及配置 root 拥有，data 仅服务用户可写。
- 禁止内置更新与已安装内置更新记录；签名升级、迁移、回滚、LuCI 不属于此 PR。
- 不启用 respawn；挂载、身份、cgroup 或地址不符即失败。默认不自动接管音箱，请在原版 UI 先观察再接管。
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
用实际 LAN IP 替换示例。确认 readiness、进程 UID/GID、cgroup、播放及无 OOM 后再 `tf enable`。测试 stop/restart、异常挂载、冷重启、24h。若安装拒绝既有路径，不要删除旧部署绕过检查。

`sh uninstall.sh` 仅停止/禁用并移除管理层；保留程序、数据、配置、账号、cgroup。此后重新安装会拒绝现存目录，需后续显式迁移/恢复流程。不会卸载实验版 LuCI。
