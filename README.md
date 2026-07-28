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

首次运行前请确认游戏能够正常启动，并保持游戏窗口可见，窗口标题应为 `DOAX VenusVacation`。发布包使用 MXU 读取 `interface.json`，用户配置会由 MXU 保存在运行目录的 `config/` 下。

## 目录结构

```text
assets/interface.json              # MaaFramework 项目入口配置
assets/resource/pipeline/          # 任务流程定义
assets/resource/image/             # 图像识别模板
assets/resource/model/ocr/         # 本地 OCR 模型
build.ps1                          # 本地构筑与清理入口
build.config.json                  # 本地与线上共享的构筑配置
tools/                             # MXU 安装、OCR 配置和 schema 校验脚本
```

发布目录以 MXU 官方结构为准：`MaDOAXVV.exe` 位于根目录，MaaFramework 运行库位于 `maafw/`，项目入口和资源分别为 `interface.json` 与 `resource/`。

## 本地构筑

运行 `build.ps1` 后，可使用方向键选择构筑或清理：

需要预先安装 PowerShell 5.1 或 PowerShell 7、[uv](https://docs.astral.sh/uv/)；Node.js 仅用于 Maa 资源检查。

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

## 开发注意

- `assets/resource/model/ocr/` 体积较大，作为本地依赖保留即可。
- 新增任务后需要同步更新 `assets/interface.json` 的 `task` 列表。
- 图像模板应放在 `assets/resource/image/` 下，并在 pipeline 中使用相对文件名引用。

## 鸣谢

本项目由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 驱动，并使用 [MXU](https://github.com/MistEO/MXU) 作为通用前端。
