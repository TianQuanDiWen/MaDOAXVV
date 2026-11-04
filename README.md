<!-- markdownlint-disable MD033 MD041 -->
<p align="center">
  <img alt="LOGO" src="assets/logo.png" width="256" height="256" />
</p>
<div align="center">

# MaDOAXVV

<!-- prettier-ignore-start -->
<!-- markdownlint-disable-next-line MD036 -->
_✨ 基于 MaaFramework 的《DEAD OR ALIVE Xtreme Venus Vacation》全自动日常减负辅助工具 ✨_
<!-- prettier-ignore-end -->

</div>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go">
  <img alt="platform" src="https://img.shields.io/badge/platform-Windows%2010%2B%20x64-informational">
  <a href="https://github.com/MaaXYZ/MaaFramework" target="_blank"><img alt="framework" src="https://img.shields.io/badge/Powered%20by-MaaFramework-blueviolet"></a>
  <br>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/github/license/TianQuanDiWen/MaDOAXVV?color=blue"></a>
  <img alt="activity" src="https://img.shields.io/github/commit-activity/m/TianQuanDiWen/MaDOAXVV?color=%231890ff">
  <a href="https://github.com/TianQuanDiWen/MaDOAXVV/stargazers"><img alt="stars" src="https://img.shields.io/github/stars/TianQuanDiWen/MaDOAXVV?style=social"></a>
</p>

## 简介

**MaDOAXVV** 是基于 **[MaaFramework](https://github.com/MaaXYZ/MaaFramework)** 深度定制的《DEAD OR ALIVE Xtreme Venus Vacation》（死或生：沙滩排球女神假期 / DOAXVV）全自动日常减负辅助工具。

采用纯 Go 原生常驻 Agent 驱动与自维护现代化桌面端 UI（[MXU](https://github.com/TianQuanDiWen/MXU_tqdw)），专注一键自动拉起 Steam 启动器、登录游戏并按序跑完全套日常，大幅减少重复枯燥的打卡点击，为岛主打造极速、稳定、低占用的自动化减负体验。

> [!NOTE]
> 🚧 **开发状态提示**：本项目当前处于积极开发与迭代维护中。

---

## 🎯 定位与免责声明

* **专注日常减负**：本工具纯粹用于自动化执行游戏内重复性的日常打卡（自动启动与登录、岛主房间收发、抽免费扭蛋、新比赛与活动挑战赛、排位赛、领取邮件与日常奖励等）。不包含任何破坏游戏平衡、逆向解密或篡改游戏内存的功能，亦不提供任何形式的无限刷分、挂机清体力等功利性外挂。
* **完全开源免费**：本项目为非营利开源软件，遵循 AGPL-3.0 开源协议，**完全免费，严禁任何形式的倒卖、转售与商业牟利**。
* **免责声明**：本项目为第三方非官方自动化工具，与《DEAD OR ALIVE Xtreme Venus Vacation》官方开发商与运营商（光荣特库摩 KOEI TECMO）无关。使用自动化辅助可能违反游戏用户协议，使用风险由使用者自行评估承担，项目作者及贡献者不对账号损失或其他直接、间接损失承担任何责任。

---

## 📥 下载与快速开始

### 1. 下载发布包
前往 [GitHub Releases 页面](https://github.com/TianQuanDiWen/MaDOAXVV/releases/latest) 下载最新的 `MaDOAXVV-win-x86_64.zip` 发布包，解压到任意非系统敏感目录（如 `D:\Tools\MaDOAXVV\`）即可直接使用。

### 2. 运行环境要求
* **操作系统**：Windows 10 / Windows 11 x64
* **游戏客户端**：Steam 版《DEAD OR ALIVE Xtreme Venus Vacation》
* **基础运行库**：
  * [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/zh-cn/microsoft-edge/webview2/)（Win11 已内置）
  * [Microsoft Visual C++ 2015–2022 Redistributable x64](https://aka.ms/vs/17/release/vc_redist.x64.exe)

> [!NOTE]
> 发布包内已集成编译好的原生 Go Agent，普通用户**无需安装 Go、Python 或 Node.js** 环境。

### 3. 开始使用
1. 打开解压目录中的 `MaDOAXVV.exe`（自维护 MXU 前端）；
2. 在任务列表中确认需要执行的日常任务（默认已配置推荐预设）；
3. 点击界面右下角 **“开始”** 按钮，工具将自动拉起 Steam 启动器并开始自动化日常流程。

---

## 📋 功能清单

| 任务模块 | 功能说明 |
| :--- | :--- |
| **🚀 启动游戏** | 通过 Steam URI（`steam://rungameid/958260`）自动拉起启动器，OCR 识别“开始游戏”，检测游戏主窗口就绪并自动连接接管（已在运行时自动跳过） |
| **🔑 登录游戏** | 自动处理点击进入游戏、登录确认、自动跳过开屏动画与活动公告弹窗，平稳进入主页 |
| **🎁 抽免费券** | 自动定位扭蛋界面，通过 OCR 智能识别“免费”标记并完成抽取，自动跳过抽卡动画 |
| **♨️ 岛主房间** | 自动领取温泉收益并补充温泉剂；自动收取工作报酬并无缝继续指派工作 |
| **⚔️ 自动打新比赛** | 自动检测并挑战未通关的新比赛，支持战术指示、主攻击手与目标选择，直到通关或遇到验证 |
| **🎫 每日自动挑战券** | 自动领取每日挑战券并消耗次数，适配 SS 级自动挑战券全套流程 |
| **🔥 每日活动挑战赛** | 自动参与当期活动挑战赛并消耗挑战券，精准识别 SSS+ 关卡，支持自动战斗流程 |
| **🏆 自动排位** | 自动参与排位赛消耗挑战次数，支持 ALLSKIP 快速跳过对战画面，遇验证弹窗安全熔断退出 |
| **📬 领取邮件与任务奖励** | 自动领取系统邮箱奖励、每日任务奖励及成就奖励，内置道具持有上限防卡死处理 |
| **🎰 赌场（测试中）** | 自动挂机赌场项目（二十一点），目前以累计达成约 24000 金筹码为自动完成阈值 |
| **🚪 退出游戏** | 通过游戏内正规菜单路径安全退出（`主页 → 选项 → 结束游戏`），专为无人值守日常设计 |

---

## 📖 使用说明

### 1. 游戏画面与窗口设置
* **分辨率与比例**：游戏画面保持标准的 **`16:9`** 比例即可，底层已做等比自适应，支持任意 `16:9` 分辨率。
* **前台拟真控制**：自动化运行期间请保持游戏窗口可见且未被最小化（窗口标题：`DOAX VenusVacation`），以确保前台拟真点击正常交互。

### 2. 比赛作战配置
在 MXU 的任务选项中，可对排位赛与新比赛的攻防战术进行针对性配置：

| 选项名称 | 对应任务 | 配置与行为说明 |
| :--- | :--- | :--- |
| **跳过对战** | **🏆 自动排位** | 开启后在排位赛中自动点击 ALLSKIP 跳过对战动画，极大缩短比赛耗时（默认开启）。 |
| **作战指示** | **⚔️ 自动打新比赛** | 是否在比赛准备界面设置针对性作战方针（默认关闭，可手动开启）。 |
| **攻击方法** | **⚔️ 自动打新比赛** | 选择进攻倾向偏好：`杀球` 或 `佯攻`。 |
| **主攻击手** | **⚔️ 自动打新比赛** | 指定主力得分选手站位：`一号位` 或 `二号位`。 |
| **攻击目标** | **⚔️ 自动打新比赛** | 指定针对对方防守薄弱选手：`一号位` 或 `二号位`。 |

### 3. 可选开关与进阶配置
* **🎰 赌场 ➔ 自动挂机（测试中）**：  
  自动前往赌场进行二十一点自动下注挂机，当筹码累计达成约 24000 金筹码时平稳结束（可在任务队列中按需启闭）。
* **🚪 退出游戏 ➔ 无人值守自动关机/省电**：  
  默认开启。全部日常流程跑完后会自动通过游戏内置选项安全退出游戏客户端，省心省电（如需保持游戏运行可在任务队列中取消勾选）。

---

## ✨ 核心架构与技术亮点

本项目采用 MaaFramework 图像识别推理引擎与 Go 原生常驻 Agent 深度协同，兼具精准感知、前台防检测与极致性能。

### 1. 前台拟真模式与 EvasionSink 防遮挡退避
* **前台拟真交互（防检测更友好）**：  
  区别于后台粗暴的消息注入，采用 Win32 前台拟真输入，模拟真实键鼠操作轨迹，防检测机制更友好，能有效降低触发游戏无限金球验证的几率。
* **EvasionSink 旁路事件监听与避让**：  
  解决前台模式下鼠标点击后停留在按钮引发 **Hover 悬停高亮特效** 遮挡文字/图标的业界难题：
  * **非侵入式旁路挂载**：通过 `AgentServerAddContextSink` 在 Agent 底层注册，零改写管线且无运行时性能损耗；
  * **智能退避与漂移**：当检测到同一节点连续识别未命中时，自动就近移出目标 ROI 区域或在视口中心随机微漂移；
  * **贴边钳位保底 (Clamp)**：内置视口安全边界保护，严格防止位移坐标溢出游戏窗口边缘。

### 2. 状态机韧性设计与 LoopGuard 防死循环守卫
* **公共流程脱离与锚点恢复**：  
  在 `公共流程.json` 中抽离通用异常处理逻辑，支持安全空白区域点击唤醒、长时间游玩防沉迷弹窗拦截、金球验证安全熔断，并通过 MaaFramework `[Anchor]` 与 `[JumpBack]` 实现跨任务入口的平滑自愈。
* **LoopGuard 状态守卫**：  
  Go Agent 底层内置循环安全守卫，对连续反复进入的异常节点进行滑动窗口追踪，超时或超次时自动熔断，杜绝死循环卡死。
* **单节点闭环 OCR 动态日志 (OCRLog)**：  
  原创 `OCRLog` 动态文本清洗与日志替换组件，按自然阅读顺序自动拼接关卡与活动识别结果，单节点闭环原生输出高亮运行信息。

### 3. 纯 Go 原生极速 Agent（超轻量、免环境折腾）
* 基于 MaaFramework 官方 Go 绑定开发，原生编译为二进制可执行文件。
* 内存占用仅 **10MB 左右**，毫秒级冷启动，发布包即开即用，**普通用户无需安装 Go、Python 或 Node.js 环境**。
* 通过本地安全 Socket 与底层 C++ 视觉推理引擎双向通信，任务事件调度与状态反馈几乎零延迟。

---

## 🛠️ 本地开发与构建

### 运行环境要求
* **操作系统**：Windows 10 / 11 x64
* **游戏客户端**：Steam 版《DEAD OR ALIVE Xtreme Venus Vacation》（游戏画面保持 `16:9` 比例即可）
* **基础运行库**：[WebView2 Runtime](https://developer.microsoft.com/zh-cn/microsoft-edge/webview2/)、[VC++ 2015-2022 x64](https://aka.ms/vs/17/release/vc_redist.x64.exe)
* **编译工具链**：PowerShell 5.1+、Go 1.24+、[uv](https://github.com/astral-sh/uv) 0.5+、Node.js 22+

### 一键构建步骤

```powershell
# 1. 克隆代码仓库及子模块
git clone --recurse-submodules https://github.com/TianQuanDiWen/MaDOAXVV.git
cd MaDOAXVV

# 2. 执行自动化构建脚本（自动拉取依赖、编译 Go Agent、注入程序图标并打包输出至 install/ 目录）
.\build.ps1 -Action Build

# 3. 运行调试
# 进入 install 目录，启动 MaDOAXVV.exe 即可直接测试
cd install
.\MaDOAXVV.exe
```

---

## 📂 项目结构概览

```text
MaDOAXVV/
├── agent/                       # 原生 Go Agent 核心代码
│   ├── cmd/madoaxvv-agent/      # Agent 主入口程序 (launch-game / agent 双模式分发)
│   └── internal/
│       ├── agentserver/         # Agent 生命周期服务、管线交互与 Action 注册
│       ├── clickaway/           # EvasionSink 旁路事件防遮挡智能退避
│       ├── gameclose/           # 优雅退出游戏流程
│       ├── launcher/            # Steam 启动器自动化拉起与窗口检测
│       ├── loopguard/           # 状态机防死循环守卫
│       ├── ocrlog/              # OCRLog 动态日志替换与单节点闭环输出
│       ├── runtimepath/         # 开发态与发布态运行路径自适应解析
│       └── safeclick/           # Win32 前台拟真鼠标交互
├── assets/                      # 资源与任务管线定义
│   ├── icon.ico                 # 6 规格多层级 Windows 程序图标
│   ├── icon.png                 # 高清应用图标 (256x256)
│   ├── logo.png                 # 项目官方透明 Logo (1024x1024)
│   ├── interface.json           # MXU 界面入口配置与任务 Schema
│   └── resource/
│       ├── image/               # 模板匹配特征图
│       ├── model/ocr/           # OCR 识别模型
│       └── pipeline/            # MaaFramework 任务管线
│           ├── 公共流程.json      # 通用脱离卡死、空白唤醒与锚点流转
│           ├── 启动游戏.json
│           ├── 登陆游戏.json
│           ├── 抽免费券.json
│           ├── 岛主房间.json
│           ├── 自动打新比赛.json
│           ├── 每日自动挑战券.json
│           ├── 每日活动挑战赛.json
│           ├── 自动排位.json
│           ├── 领取邮件与任务奖励.json
│           ├── 赌场.json
│           └── 退出游戏.json
├── build.ps1                    # 全自动跨组件构建脚本
├── build.config.json            # 构建与依赖版本配置
└── README.md
```

---

## 🙏 鸣谢与生态

* 核心底层图像处理与自动化控制引擎由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 强力驱动。
* 桌面端 UI 基于 [MistEO/MXU](https://github.com/MistEO/MXU) 及其自维护定制分支 [TianQuanDiWen/MXU_tqdw](https://github.com/TianQuanDiWen/MXU_tqdw)。
* OCR 基础模型与字典资源来自 [MaaCommonAssets](https://github.com/MaaXYZ/MaaCommonAssets)。

---

## 📄 开源许可证

本项目基于 [GNU Affero General Public License v3.0 (AGPL-3.0)](LICENSE) 协议开源。

Copyright (c) 2024-2026 TianQuanDiWen <https://github.com/TianQuanDiWen>  
Copyright (c) 2024 MaaXYZ
