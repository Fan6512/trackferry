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
默认 update_check: true，仅检查版本；登录管理员主动点击更新才安装。上游在打开数据库之前执行 Launch：按 installed.json 核对版本文件的 sha256 与 Ed25519 签名，然后 syscall.Exec 原地切换。PID、UID/GID、cgroup 都不变，procd 只按 PID 跟踪子进程，所以更新后 running/status 仍有效；tf 同时接受 data/update/trackferry-X.Y.Z 作为实际运行路径（只做状态识别，不替代上游验签）。不要拒绝 installed.json，也不要手动删除它。基础二进制保持官方 v1.5.0，管理页显示实际运行版本。

新版本要连续运行 20 秒才写 confirmed。在此之前每次启动（包括 tf restart 和“更新后自动重启”本身）都会让上游的 tries 加一；tries 到 3 后再启动一次，上游会还原更新前的数据库备份并退回旧版本。所以更新完成后 20 秒内不要反复 stop/restart。更新启动失败时无 respawn，管理员看日志后手动 tf start，回退由上游负责，管理层不自行重启。保留独立完整备份。分档内存限制、升级期间的 version 探测子进程与数据库备份、OOM/断电恢复仍需实机测试。

## 延迟挂载和 LAN
开机时（S99，procd 以 boot 动作执行 /etc/rc.d/S*）创建 /var/run/trackferry.boot-pending，仅本次开机有效；同一次 boot 调用里立即检查挂载与 LAN。不就绪则保留标记，等待 blockd 的 mount add 或 netifd 的 iface ifup/ifupdate 事件，由 /etc/hotplug.d/{mount,iface}/95-trackferry 调 /etc/init.d/trackferry boot_ready；无常驻轮询。就绪时消费标记并 start 一次；重复事件、手动 start/stop、OOM 或进程退出都不会自动重新拉起。stop 与 boot_ready 都持有 procd_lock，标记的检查与消费因此是串行的，不会在 stop 之后被事件拉起。标记用 O_EXCL 创建：tmpfs 可被普通用户写入，不能跟随预先放置的符号链接。依赖目标系统 blockd 挂载事件，需在 ImmortalWrt 实机核实；未收到事件时手动 tf start。安装时不启用自启；卸载移除本插件两个 hook。仅解决启动时的晚就绪，不支持运行中强拔 USB 自动恢复。挂载点存在但设备与安装记录不同时只记一条日志，不会启动。

## 复审后的边界
- procd 注册始终提供稳定的实例定义，避免 guard 失败时提交空实例清单；实际 child 在 exec 之前检查挂载、身份和 cgroup。procd 对相同定义的 service set 只更新数据、不重启实例，所以重复 tf start/boot_ready 不会产生第二个进程。run.sh 另在 `<app>/.instance.lock` 上持有 flock 直到进程退出（fd 跨 start-stop-daemon 的 exec 保留），第二个启动器直接退出并记日志；手工直接运行二进制绕过该锁，端口冲突不保证数据库独占，不要这样做。tf start/restart 额外检查真实程序 PID、UID、cgroup；直接 init 的返回值不代表 readiness。
- tf start/restart 与安装/卸载共用 /etc/trackferry-openwrt.lock（前者共享、后者独占），start 不会插进卸载的 stop 与删除之间。
- 启动器开启根 cgroup memory controller 时会修改全局 subtree_control；卸载不会关闭它，避免影响其他已使用该 controller 的服务。拒绝 noexec/ro 挂载。OpenSSL 需支持 Ed25519 的 pkeyutl -rawin（例如 3.x）。启动要求内核提供 memory.swap.max（CONFIG_MEMCG_SWAP），缺失即失败；这是有意的 fail-closed，需实机确认。
- 卸载比对原始管理入口文件，若已被替换则拒绝操作；保留数据和现有其他组件。
- 安装/卸载由 flock 串行化，trap 覆盖普通失败与 INT/TERM。SIGKILL 或断电可能留下半套管理文件：此时 install 会拒绝现存路径，uninstall 又需要 common.sh 才能运行。恢复办法是先确认没有进程，再手工删除 /usr/libexec/trackferry-openwrt、/etc/init.d/trackferry、/etc/trackferry-openwrt.conf、/usr/bin/tf 和两个 hotplug 文件后重装；程序目录与数据不动。
- 官方更新及晚挂载实现尚未实机验收，仍为发布门禁。禁止直接在生产实例部署此 Draft。

## 自动内存保护（OP-002 / OP-008）

只在安装预检和实际启动前读取一次 /proc/meminfo，不增加监控进程；重启时按旧进程退出后的可用内存重新计算。Go 目标不是硬上限，memory.high/max 由 cgroup v2 执行，swap=0，GOGC=75、GOMAXPROCS=1 不变。运行期间不自动调档，不因 OOM 放宽限制。

| 实际 MemTotal | Go 目标 (MiB) | memory.high (MiB) | memory.max (MiB) |
|---|---:|---:|---:|
| <=128 MiB | 不支持 | - | - |
| >128 至 256 MiB | 24 | 36 | 48 |
| >256 至 512 MiB | 32 | 48 | 64 |
| >512 至 1024 MiB | 48 | 72 | 96 |
| >1024 MiB | 64 | 96 | 128 |

计算以 KiB 原值判断档位，不按厂商标称内存或 RSS。系统保留预算 = max(64 MiB, 向上取整的实际 MemTotal × 12.5%)；仅当 MemAvailable >= (保留预算 + 硬上限) 时允许新启动。缺失、格式错误或不支持时 fail-closed。此保留预算是保守准入条件，不是对系统内存的实际预留或保证；已有 cgroup 缓存仍可能使判断更保守。安装时预算不足仅警告（不启动），不支持的设备拒绝安装。启动失败不自动重试，用户释放资源后手动启动。tf mem 显示“下一次启动”的建议与当前 cgroup 实测值，运行中的可用内存不可当作未运行时的基线。

已有 495484 KiB 设备样本：cgroup 运行 16257024 B、峰值 17313792 B（约 16.51 MiB）；保留原 32/48/64 档，初版预算 73+64=137 MiB，实机迁移时启动前 146448 KiB、实际启动器 136028 KiB，因原门槛拒启；自动回滚成功且 OOM=0。修订保留预算为 64 MiB，准入 64+64=128 MiB；不改变 Go/cgroup 限制。仅此档位具备历史实机证据；其他档位为初始实验策略，连续播放、多音箱、在线升级和低内存场景仍需实机验收。手动资源参数与 LuCI 配置入口留待后续，不接受动态增大限制。

## 实机验收（隔离测试机）
1. `tf mem` 先显示下一次启动的分档（state/Go/high/max/预留），再显示当前 cgroup 的 memory.current/high/max/swap.max（5.19 以前没有 memory.peak，会跳过）。
2. `tf start` 后 `tf status` 为 running；`/proc/<pid>/cgroup` 为 `0::/trackferry`，`/proc/<pid>/status` 的 Uid/Gid 四列都是 10001；`readlink /proc/<pid>/exe` 指向基础二进制或 data/update/trackferry-X.Y.Z。
3. 冷启动内存门槛：本机 MemAvailable 必须达到“预留 + memory.max”（495484 KiB 样本为 128 MiB）；同时记录 boot 时的实际 MemAvailable，确认不会因为开机占用导致服务不启动。
4. 更新：确认安装前后 PID 不变（`tf status`、`pidof trackferry`），20 秒后 installed.json 的 confirmed 为 true；再 `tf restart`，仍运行新版本（Launch 会再次 exec）。
5. 回退（仅隔离测试机的可丢弃数据副本，不在生产数据库上操作）：把 installed.json 的 confirmed 置 false 并连续启动到 tries=3，下一次启动应还原更新前的数据库并回到旧版本。
6. 晚挂载（仅独立数据盘测试环境；禁止拔除承载系统根分区或运行中数据的 USB）：`tf enable` 后冷启动且数据盘未挂载，服务不启动、标记存在；数据盘挂载后只有一次启动，重复事件不再启动。
7. 晚 LAN：LAN 地址就绪前不启动；恢复后只启动一次。
8. 手动 `tf stop` 后插拔 USB 不自动启动；`tf disable` 后重启不创建标记。
9. 并发：两个终端同时 `tf start` 不产生第二个进程；卸载过程中执行 `tf start` 应等待或失败，不产生第三个实例。

管理锁位于 root 控制的 /etc/trackferry-openwrt.lock，避免 OpenWrt 的 /var -> /tmp 与 1777 /tmp/lock 中的可预置符号链接。锁文件为空，仅内核 flock 状态有意义；卸载时保留锁文件，防止并发进程锁定不同 inode。无需更改系统目录权限。

修订后的 12.5% 是基于首次实机拒启样本调整的保守经验值，不是内核预留保证；128 MiB 门槛仍需实机复验。实际启动器始终独立采样，不绕过预算；不增加 Swap、不放宽 cgroup，不清理系统缓存或停止其他服务。实例锁文件可以在退出后保留；不要删除可能正在持锁的 inode。
