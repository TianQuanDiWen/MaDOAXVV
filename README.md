# MaDOAXVV

[![Release](https://img.shields.io/github/v/release/TianQuanDiWen/MaDOAXVV?style=flat-square&color=blue)](https://github.com/TianQuanDiWen/MaDOAXVV/releases/latest)
[![Platform](https://img.shields.io/badge/Platform-Windows%2010%2B%20x64-informational?style=flat-square)](https://github.com/TianQuanDiWen/MaDOAXVV)
[![Framework](https://img.shields.io/badge/Powered%20by-MaaFramework-blueviolet?style=flat-square)](https://github.com/MaaXYZ/MaaFramework)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

> 🎮 **基于 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 的《DEAD OR ALIVE Xtreme Venus Vacation》（死或生：沙滩排球女神假期）全自动日常减负辅助工具。**  
> 一键启动，全套流程跑完正好能做完每日日常，减少枯燥重复劳作，轻松护肝。

---

## ⚠️ 免责声明

* 本项目为**非官方的开源自动化工具**，与游戏开发商、运营商无关。
* 使用自动化脚本可能违反游戏的用户协议或运营规则，并可能导致账号受到限制、暂停或永久封禁。
* 使用者应在使用前自行了解并遵守相关规则，充分评估风险。使用本项目所产生的一切后果由使用者自行承担，项目作者及贡献者不对账号损失或其他直接、间接损失承担任何责任。

---

## 🎯 项目定位与初衷

* **完全免费开源**：本项目为非盈利的开源辅助工具，**完全免费，严禁倒卖，请勿付费购买**。
* **专注日常减负**：本项目纯粹为**自动化清理每日日常**而生，按顺序跑完全套流程正好能做完当天的日常任务。
* **克制与轻量**：设立初衷仅为减少重复枯燥的鼠标点击打卡负担。因此，本项目**不提供、也不打算提供**任何形式的无限刷分、挂机清体力等功利性功能。

---

## 📥 下载与快速开始

### 1. 下载安装
前往 [GitHub Releases 页面](https://github.com/TianQuanDiWen/MaDOAXVV/releases/latest) 下载最新的 `MaDOAXVV-win-x86_64.zip` 发布包，解压到任意非系统敏感目录即可直接使用。

### 2. 运行环境
* **操作系统**：Windows 10 / Windows 11 x64
* **游戏客户端**：Steam 版《DEAD OR ALIVE Xtreme Venus Vacation》
* **基础依赖**：
  * [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/)（Win11 自带）
  * [Microsoft Visual C++ 2015–2022 Redistributable x64](https://aka.ms/vs/17/release/vc_redist.x64.exe)

> [!NOTE]
> 发布包内已集成编译好的原生 Go Agent，普通用户**无需安装 Go、Python 或 Node.js**。

### 3. 开始使用
1. 打开解压目录中的 `MaDOAXVV.exe`（自维护 MXU 前端）。
2. 在任务列表中确认开启的日常任务（默认已配置推荐预设）。
3. 点击右下角 **“开始”** 按钮，工具将自动拉起 Steam 启动器并开始自动化流程。

---

## 📋 功能清单

| 任务名称 | 功能覆盖说明 | 自动化特性 / 异常处理 | 默认状态 |
| :--- | :--- | :--- | :---: |
| **🚀 启动游戏** | Steam URI 拉起启动器，OCR 识别并点击“开始游戏” | 游戏若已在运行则自动跳过启动，无缝接管 | `开启` |
| **🔑 登录游戏** | 点击进入游戏，自动处理登录确认与活动公告关闭 | 自动跳过开屏动画与活动 `SKIP` 弹窗 | `开启` |
| **🎁 抽免费券** | 自动定位并抽取所有可用的免费扭蛋 | 自动识别扭蛋旁“免费”标记，处理抽卡动画 | `开启` |
| **♨️ 岛主房间** | 自动收发温泉与工作，自动补充温泉剂 | 工作完成自动收菜并无缝继续指派 | `开启` |
| **⚔️ 自动打新比赛** | 自动检查并挑战未通关的新比赛 | 支持新剧情跳过、结算退出；无新比赛时自动流转 | `开启` |
| **🎫 每日自动挑战券**| 自动领取每日挑战券并进行挑战 | 适配 SS 级自动挑战券流程 | `开启` |
| **🔥 每日活动挑战赛**| 自动参与当期活动挑战赛并消耗挑战券 | 自动识别 SSS+ 流程；未识别到活动时安全退出 | `开启` |
| **🏆 自动排位** | 自动消耗排位赛挑战次数 | 遇游戏验证弹窗自动停止，保护账号安全 | `开启` |
| **📬 领取奖励** | 自动领取邮箱奖励、每日任务奖励及成就奖励 | 具备道具达到持有上限时的异常防卡死处理 | `开启` |
| **🎰 赌场** | 自动挂机赌场项目（测试中），自动打二十一点挂机 | 目前以累计达成约 24000 金筹码为完成依据 | `开启` |
| **🚪 退出游戏** | 通过游戏内置菜单正常退出：`主页 → 选项 → 结束游戏` | 专为无人值守设计，日常跑完自动关闭游戏省电 | `默认关闭` |

---

## ✨ 核心特性与工程设计

### 1. 前台拟真模式（防检测更友好）
* 项目默认采用 **Win32 前台拟真控制模式**。相较于后台直接发消息注入事件，前台模式模拟真实外设交互，防检测更加友好，能显著降低游戏触发无限金球验证的概率。
* 建议运行时保持游戏窗口可见，分辨率推荐使用默认匹配的 `1280x720`（窗口标题：`DOAX VenusVacation`）。

### 2. 全局防遮挡退避机制 (SafeRecognition)
前台模式下，鼠标点击后停留在按钮上极易触发游戏的 **悬停（Hover）高亮特效**，导致文字变色或图标被遮盖，造成下一次识别失败。为此我们在底层实现了透明的全局防遮挡：
* **零侵入注入**：编译期自动为所有底层识别节点包裹包装层，无需在 Pipeline JSON 中手动编写冗余的移开鼠标逻辑。
* **智能退避**：当任意节点连续识别未命中达到阈值时，自动向识别目标（ROI）外缘安全随机漂移，消除 Hover 遮挡。
* **坐标钳位保护 (Clamp)**：内置窗口安全边界限制，严格杜绝退避位移超出 1280x720 边缘导致底层动作崩溃的问题。

### 3. 独立自动更新
* 前端采用自维护的 [MXU_tqdw](https://github.com/TianQuanDiWen/MXU_tqdw) Fork。
* 直接通过 **GitHub Release** 检查并获取 MaDOAXVV 的版本更新，发布节奏自主，不依赖第三方镜像渠道。

---

## 🛠️ 开发者指南

### 项目目录结构

```text
MaDOAXVV/
├─ assets/
│  ├─ interface.json           # MaaFramework / MXU 前端入口与参数配置
│  └─ resource/
│     ├─ pipeline/             # 自动化任务管线 (JSON)
│     ├─ image/                # 图像特征模板 (.png)
│     └─ model/ocr/            # OCR 识别模型
├─ agent/
│  ├─ cmd/madoaxvv-agent/      # Go Agent 入口
│  └─ internal/
│     ├─ agentserver/          # MaaFramework AgentServer 自定义扩展能力
│     ├─ clickaway/            # SafeRecognition 全局防遮挡与状态队列
│     └─ launcher/             # Steam 游戏启动拉起逻辑
├─ tools/                      # 资源转译、Schema 校验与安装脚本
├─ build.ps1                   # 本地一键构筑脚本
└─ build.config.json           # 构筑与依赖版本配置
```

### 本地编译构建

**环境要求**：
* PowerShell 5.1 或 PowerShell 7+
* Go 1.24+
* Python 包管理工具 [uv](https://github.com/astral-sh/uv)
* Node.js（仅运行 Maa 资源检查时需要）

```powershell
# 交互式构筑
.\build.ps1

# 直接执行本地构建（输出至 install/ 目录）
.\build.ps1 -Action Build

# 清理构筑缓存
.\build.ps1 -Action Clean

# 若遇 PowerShell 脚本权限受限，可通过 Bypass 执行：
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

---

## 🙏 鸣谢

* 核心自动化引擎由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 强力驱动。
* 通用前端基于 [MistEO/MXU](https://github.com/MistEO/MXU) 进行扩展与适配。
* 感谢所有为 MaaFramework 及自动化开源生态做出贡献的开发者！\n