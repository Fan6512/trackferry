# 参与贡献

欢迎提交 Issue 和 Pull Request。报告问题前请先看[问题排查](docs/troubleshooting.md)，并去掉日志里的家庭地址、设备标识和对话内容。较大的改动建议先开 Issue 讨论。

## 目录

```
cmd/trackferry     程序入口
internal/          后端，各包职责见 docs/architecture.md
web/               管理页（React + TypeScript + Vite），构建后嵌入程序
build/             发版镜像
scripts/           检查、发版与开发部署
scripts/hooks/     Git 钩子
docs/              文档与截图
```

## 本地运行

需要 Go（版本见 `go.mod`）和 Node 22。

```sh
(cd web && npm ci && npm run build)
```

```sh
go run ./cmd/trackferry
```

默认数据目录是 `/data`，本地用 `TRACKFERRY_CONFIG` 指向自己的配置文件来更改。改前端时另开 `cd web && npm run dev`，接口会转到 18561。

## 测试

```sh
go vet ./... && go test -race ./...
```

```sh
cd web && npm test && npm run build
```

测试只用本地假服务，不得连真实的小米、飞牛或音箱。

## 提交与推送规则

克隆后执行一次，启用检查：

```sh
scripts/setup-hooks.sh
```

提交说明写成：

```
修复：金鱼在速度变化时抖动

游动相位原来由时间乘以当前速度得到，速度一变相位就跳。
改为逐帧累加，并给速度和幅度加了缓动。

在流转页连续回放十条记录，没有再出现跳变。
```

| 规则 | 要求 |
| --- | --- |
| 语言 | 标题和正文都用中文 |
| 标题 | `类型：内容`，32 个汉字以内，末尾不加标点 |
| 类型 | 新增、修复、优化、重构、文档、测试、构建、维护、发布 |
| 正文 | 必须有。写清改了什么、为什么、怎么验证 |
| 换行 | 标题后空一行；每行 40 个汉字以内；用真正的换行，不能出现字面的 `\n` |
| 署名 | 不带工具自动生成的协作署名 |

多行说明建议写进文件再提交：

```sh
git commit -F 说明.txt
```

本地钩子会检查：

| 时机 | 检查 |
| --- | --- |
| 提交前 `pre-commit` | 私有数据、疑似密钥、大文件、行尾空格、Go 格式、版本一致 |
| 写说明 `commit-msg` | 上表全部规则 |
| 推送前 `pre-push` | 每条待推送的说明、Go 与管理页测试；`main` 不能强推或删除；版本标签须与 `VERSION` 一致，不能移动或删除 |

Pull Request 的标题和说明遵循同样的语言、换行与署名规则。

## 不进仓库的东西

凭据、真实对话、家庭地址、设备标识、音频地址。放在 `.private/` 或 `data/`，两者都被忽略。截图与测试样例一律用虚构数据。

## 部署到 NAS 调试

```sh
TRACKFERRY_SSH_HOST=<别名> TRACKFERRY_DEPLOY_DIR=<目录> scripts/deploy.sh
```

在 NAS 上构建 `trackferry:local` 并重启。不覆盖 NAS 上的 `compose.yaml`、`.env` 和 `data/`，会留下旧镜像 `rollback.tar.gz` 和旧数据库 `data/trackferry.db.before`。

## 版本

采用[语义化版本](https://semver.org/lang/zh-CN/)。升级时需要手动处理的改动进大版本，新功能进小版本，修复进修订版。面向用户的改动请写进 `CHANGELOG.md` 的“未发布”。
