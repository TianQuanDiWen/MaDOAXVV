# MaDOAXVV

基于 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 的《DEAD OR ALIVE Xtreme Venus Vacation》自动化项目。

项目使用自维护的 MXU 前端，并通过 MaaFramework Pipeline 与 Go Agent 实现游戏启动、登录、日常任务执行以及退出游戏等自动化流程。

## 免责声明

本项目为非官方自动化工具，与游戏开发商、运营商无关。

使用自动化脚本可能违反游戏的用户协议或运营规则，并可能导致账号限制、暂停或永久封禁。

使用者应在使用前自行了解并遵守相关规则，充分评估风险。使用本项目所产生的一切后果由使用者自行承担，项目作者及贡献者不对账号损失或其他直接、间接损失承担责任。

## 项目定位

本项目纯粹为**自动化清理每日日常**而生，依次执行全套任务流程能正好覆盖游戏内的各项每日任务指标。

设立本项目的初衷仅为**减少游戏内枯燥的重复劳作，降低玩家的每日打卡负担**。因此，本项目**不提供、也不打算提供**任何形式的无限刷分、挂机清体力等功利性功能，旨在以最克制、轻量的方式辅助玩家。

## 下载

最新版本：

https://github.com/TianQuanDiWen/MaDOAXVV/releases/latest


## 功能

当前已维护以下任务：

* 启动游戏
* 登录游戏
* 抽免费券
* 岛主房间
* 自动打新比赛
* 每日自动挑战券
* 每日活动挑战赛
* 自动排位
* 领取邮件与任务奖励
* 赌场（测试中）
* 退出游戏

项目使用 Win32 控制器连接游戏窗口，默认匹配：

```text
DOAX VenusVacation
```

### 启动游戏

默认任务中的“启动游戏”会：

1. 通过 Steam URI 启动游戏。
2. 检测 `DOAX_VV_Launcher.exe` 启动器。
3. 使用 MaaFramework OCR 识别并点击“开始游戏”。
4. 等待 `DOAX VenusVacation` 游戏窗口就绪。
5. 连接控制器并继续后续任务。

如果游戏已经运行，会直接跳过启动步骤。

### 登录游戏

自动处理启动后的登录流程，包括：

* 登录界面点击。
* 确认窗口。
* 公告关闭。
* 宣传动画 / `SKIP` 判断。
* 等待进入游戏主页。

### 退出游戏

可通过游戏自身的退出选项正常关闭游戏：

```text
主页 → 选项 → 结束游戏 → 确认
```

“退出游戏”默认不启用，可根据无人值守任务需求自行加入任务队列。

因此可以组成完整流程：

```text
启动游戏
  ↓
登录游戏
  ↓
执行日常任务
  ↓
退出游戏
```

## 自动更新

项目目前使用自行维护的 MXU Fork：

https://github.com/TianQuanDiWen/MXU_tqdw

相较于直接使用官方 MXU，本项目维护的版本主要用于适配 MaDOAXVV 的发布与更新需求，使前端能够通过 **GitHub Release 检查并获取 MaDOAXVV 的新版本**。

项目更新以 GitHub Release 为主要发布渠道，不依赖 MirrorChyan。

构筑时同样会从配置指定的 GitHub Release 获取 MXU 与 MaaFramework。

## 运行环境

* Windows 10 / Windows 11 x64
* Steam 版《DEAD OR ALIVE Xtreme Venus Vacation》
* Microsoft Edge WebView2 Runtime
* Microsoft Visual C++ 2015–2022 Redistributable x64

发布包中的 Go Agent 已编译为原生可执行文件。

普通用户运行发布版时 **不需要安装 Go、Python 或 Node.js**。

## 运行控制模式与防遮挡机制

**前台模式（推荐）**：
项目当前默认使用前台模式（Win32 控制器）。前台模式模拟真实外设交互，防检测更友好，能有效降低游戏内置的安全验证弹出概率。建议运行时尽量保持游戏窗口可见。
虽然部分流程理论上支持后台运行，但后台运行注入事件容易触发游戏行为验证。如在自动比赛等流程中检测到验证场景，程序会自动停止，请手动完成验证。

**全局防遮挡机制 (SafeRecognition)**：
由于前台模式会产生真实的鼠标轨迹并在点击后停留在原位，极易触发游戏按钮的悬停（Hover）特效，从而遮盖文字或图像导致后续识别失败。为此，项目在底层专门针对前台模式引入了全局防遮挡机制：
* **零侵入设计**：编写流水线 JSON 时无需关心该问题。编译打包时（`build.ps1`）会自动向所有的底层识别节点注入该包装层。
* **智能退避**：当任意节点连续识别失败时，会自动根据其 ROI 框计算周边安全的防遮挡坐标，自动移开鼠标以消除 Hover UI 态。
* **安全边界**：自带坐标钳位（Clamp）保护，严格限制鼠标退避操作在 1280x720 窗口分辨率内，防止触发超出边框的底层运行错误。

## 当前任务说明

### 抽免费券

自动识别可用的免费扭蛋并完成抽取，同时处理抽卡动画与 `SKIP`。

### 岛主房间

自动处理温泉、温泉剂以及工作奖励领取等流程。

### 自动打新比赛

存在新比赛时自动执行比赛。

支持：

* 推荐入口识别。
* 新剧情跳过。
* 比赛结算。
* 无新比赛退出。
* 部分异常界面恢复。

遇到无法继续挑战或游戏验证时停止执行。

### 每日自动挑战券

自动领取并使用每日挑战券，目前主要适配 SS 级自动挑战券流程。

### 每日活动挑战赛

自动进入活动挑战赛并使用自动挑战券。

目前主要针对 SSS+ 活动流程，未识别到支持的活动时不会强制继续执行。

### 自动排位

自动进行排位比赛，直到当前次数完成或无法继续。

遇到游戏验证时提前结束。

### 领取邮件与任务奖励

自动领取：

* 信箱奖励。
* 每日任务奖励。
* 其他已支持的任务奖励。

包含道具达到持有上限时的异常处理。

### 赌场

目前仍处于测试阶段。

当前主要以累计获得约 `24000` 金筹码作为流程完成判断。

## 项目结构

```text
assets/
├─ interface.json              # MaaFramework / MXU 项目入口配置
└─ resource/
   ├─ pipeline/                # Pipeline 自动化流程
   ├─ image/                   # 图像识别模板
   └─ model/ocr/               # OCR 模型

agent/
├─ cmd/
│  └─ madoaxvv-agent/          # Agent 程序入口
└─ internal/
   ├─ agentserver/             # MaaFramework AgentServer 与自定义能力
   └─ launcher/                # Steam / 游戏启动流程

build.ps1                      # Windows 本地构筑脚本
build.config.json              # 本地与 GitHub Actions 共用构筑配置
tools/                         # 安装、资源处理与检查脚本
go.mod / go.sum                # Go 模块与依赖
```

发布包结构大致为：

```text
MaDOAXVV.exe
interface.json
resource/
maafw/
agent/
config/
```

其中：

* `MaDOAXVV.exe`：MXU 前端
* `maafw/`：MaaFramework 运行库
* `resource/`：Pipeline 与识别资源
* `agent/`：MaDOAXVV Agent
* `config/`：用户配置

## 本地构筑

开发环境需要：

* PowerShell 5.1 或 PowerShell 7
* Go 1.24
* uv
* Node.js（仅 Maa 资源检查时需要）

### 交互构筑

```powershell
.\build.ps1
```

### 直接构筑

```powershell
.\build.ps1 -Action Build
```

### 清理构筑目录

```powershell
.\build.ps1 -Action Clean
```

如果 PowerShell 执行策略禁止脚本运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

本地构筑与 GitHub Actions 共用：

```text
build.config.json
```

当前配置会从 GitHub Release 获取：

* MaaFramework
* TianQuanDiWen/MXU_tqdw

本地构筑默认生成：

```text
install/
```

GitHub Release 构筑则会进一步生成发布 ZIP。

## Go Agent

项目使用一个统一的：

```text
MaDOAXVV.Agent.exe
```

承担扩展功能。

当前主要包含两种运行模式：

### launch-game

用于：

* 启动 Steam 游戏。
* 等待启动器。
* OCR 识别“开始游戏”。
* 等待游戏窗口就绪。

### agent

启动 MaaFramework AgentServer，用于注册自定义 Recognition、Action 以及后续扩展能力。

本地使用 MaaTools 开发时，可以通过：

```text
go run
```

直接运行 Agent 源码。

正式构筑时会自动编译为：

```text
agent/MaDOAXVV.Agent.exe
```

因此发布版本运行时不依赖 Go 工具链。

## 开发说明

* Pipeline 位于 `assets/resource/pipeline/`。
* 图像模板位于 `assets/resource/image/`。
* 新增任务后需要同步修改 `assets/interface.json`。
* Go 自定义能力统一在 AgentServer 中注册。
* 通用算法建议拆分为独立 package。
* 本地与 GitHub Actions 应尽量保持相同的 `build.config.json` 构筑配置。

## 鸣谢

本项目由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 驱动。

通用前端基于 [MXU](https://github.com/MistEO/MXU)，项目使用自行维护的 [MXU_tqdw](https://github.com/TianQuanDiWen/MXU_tqdw) Fork 进行适配与扩展。

感谢 MaaFramework、MXU 及相关开源项目的开发者与贡献者。
