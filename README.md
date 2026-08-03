# MaDOAXVV

基于 MaaFramework、使用 MXU 前端的《DEAD OR ALIVE Xtreme Venus Vacation》自动化项目。

## 免责声明

本项目为非官方自动化工具，与游戏开发商、运营商无关。使用自动化脚本可能违反游戏的用户协议或运营规则，并可能导致账号限制、暂停或永久封禁。

使用者应在使用前自行了解并遵守相关规则，充分评估风险。使用本项目所产生的一切后果由使用者自行承担，项目作者及贡献者不对账号损失或其他直接、间接损失承担责任。

当前已维护以下任务：

- 自动打新比赛
- 自动排位五次
- 每日自动挑战券
- 每日活动挑战赛
- 抽免费券
- 岛主房间
- 领取邮件与任务奖励
- 赌场(测试中)

项目使用 Win32 控制器连接游戏窗口，默认匹配标题 `DOAX VenusVacation`。

## 下载

[下载最新版本](https://github.com/TianQuanDiWen/MaDOAXVV/releases/latest)

## 运行环境

- Windows 10 或 Windows 11（x64）。
- Steam 版《DEAD OR ALIVE Xtreme Venus Vacation》，运行区域需与当前图像模板匹配。
- [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)（Windows 10/11 通常已内置）。
- [Microsoft Visual C++ 2015–2022 Redistributable（x64）](https://aka.ms/vs/17/release/vc_redist.x64.exe)。

首次运行前请确认 Steam 已登录且游戏能够正常启动。默认任务列表中的“启动游戏”会通过 `steam://rungameid/958260` 启动 Steam 版游戏，按进程名 `DOAX_VV_Launcher.exe` 定位启动器，使用 MaaFramework OCR 识别并点击“开始游戏”，然后等待标题为 `DOAX VenusVacation` 的游戏窗口就绪。游戏已经运行时会直接跳过启动步骤。

发布包使用 MXU 读取 `interface.json`，用户配置会由 MXU 保存在运行目录的 `config/` 下。不需要自动启动时，可在 MXU 任务列表中取消勾选“启动游戏”。

虽然项目支持后台运行，但是建议保持游戏在前台，后台运行会加大弹出金球验证的概率，在弹出金球验证后会直接关闭挑战选项不继续执行挑战

## 目录结构

```text
assets/interface.json              # MaaFramework 项目入口配置
assets/resource/pipeline/          # 任务流程定义
assets/resource/image/             # 图像识别模板
assets/resource/model/ocr/         # 本地 OCR 模型
agent/                             # Go Agent、启动器任务及自定义识别扩展
go.mod / go.sum                    # Go 依赖及可复现版本锁定
build.ps1                          # 本地构筑与清理入口
build.config.json                  # 本地与线上共享的构筑配置
tools/                             # MXU 安装、OCR 配置和 schema 校验脚本
```

发布目录以 MXU 官方结构为准：`MaDOAXVV.exe` 位于根目录，MaaFramework 运行库位于 `maafw/`，项目入口和资源分别为 `interface.json` 与 `resource/`。

## 本地构筑

运行 `build.ps1` 后，可使用方向键选择构筑或清理：

需要预先安装 PowerShell 5.1 或 PowerShell 7、[Go](https://go.dev/dl/) 1.24、[uv](https://docs.astral.sh/uv/)；Node.js 仅用于 Maa 资源检查。Go 只在构筑时使用，发布包中的 Agent 为 AOT 编译的原生可执行文件，运行时不要求用户安装 Go、Python 或额外依赖。

```powershell
# 交互选择
.\build.ps1

# 直接构筑
.\build.ps1 -Action Build

# 清理配置指定的输出、downloads 和下载缓存
.\build.ps1 -Action Clean

# 当前 PowerShell 执行策略禁止直接运行脚本时
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

本地与 GitHub Actions 共用 `build.config.json`，均从配置指定的 MXU 和 MaaFramework GitHub Release 获取前端及运行库。需要固定版本或调整目标平台时直接修改构筑配置，脚本不维护额外的前端下载规则。可使用 `-SkipChecks` 跳过 Node 与 Schema 检查。本地构筑只生成 `install/` 目录，不生成发布压缩包；GitHub Release 流程仍会按配置中的 `packageName` 生成 ZIP。

## Go Agent 扩展

发布包只包含一个 `agent/MaDOAXVV.Agent.exe`。MXU 在 Controller 连接前以 `launch-game` 模式调用它完成 Steam 启动、启动器 OCR 点击和游戏窗口等待；MXU 同时可按 Project Interface 的 `agent` 配置，以 `agent` 模式启动 MaaFramework AgentServer。

使用 MaaTools 本地插件调试时，`assets/interface.json` 会通过 `go run` 直接启动 Agent 源码，并自动使用 `deps/bin` 和 `assets/resource`，无需预先生成 EXE。构筑发布包时，安装脚本会将这两处命令改写为编译后的 `agent/MaDOAXVV.Agent.exe`，运行时不依赖 Go 工具链。

后续的分数识别、场景判断或特殊操作应分别实现为 MaaFramework 自定义 Recognition 或 Action，并集中在 `agent/internal/agentserver` 注册。模式入口、框架生命周期、启动器流程与具体识别算法相互分离，增加新能力时不需要修改 MXU，也不需要再增加一种运行时。

## 开发注意

- `assets/resource/model/ocr/` 体积较大，作为本地依赖保留即可。
- 新增任务后需要同步更新 `assets/interface.json` 的 `task` 列表。
- 图像模板应放在 `assets/resource/image/` 下，并在 pipeline 中使用相对文件名引用。
- Go 自定义能力在 `agent/internal/agentserver.BuildRegistry` 统一装配；通用算法可继续拆分为独立 package。

## 鸣谢

本项目由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 驱动，并使用 [MXU](https://github.com/MistEO/MXU) 作为通用前端。
