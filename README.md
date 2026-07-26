# MaDOAXVV

基于 MaaFramework 的《DEAD OR ALIVE Xtreme Venus Vacation》自动化项目。

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
- Chrome 111、Edge 111、Firefox 114、Safari 16.4 或更高版本的浏览器。

首次运行前请确认游戏能够正常启动，并保持游戏窗口可见，窗口标题应为 `DOAX VenusVacation`。运行发布包内的 `MWU.exe` 后，程序会在浏览器中打开操作界面。

## 目录结构

```text
assets/interface.json              # MaaFramework 项目入口配置
assets/resource/base/pipeline/     # 任务流程定义
assets/resource/base/image/        # 图像识别模板
agent/                             # 自定义 Action 和 Recognition
requirements.txt                   # Agent 第三方依赖
build.config.json                 # 本地与线上共享的构筑配置
build.ps1                         # MWU 本地构筑入口
tools/                             # Schema 校验脚本
```

## 本地构筑

本地构筑与 GitHub Actions 共用 `build.config.json`，并调用配置中指定版本的 MWU 官方资源复制和依赖下载脚本。

需要预先安装：

- PowerShell 5.1 或 PowerShell 7。
- [uv](https://docs.astral.sh/uv/)：MWU 官方脚本使用的 Python 环境与依赖执行器。
- Windows 自带的 `tar.exe`：用于解压 MWU 官方发布的 `.7z` 运行时，命令和参数由 `build.config.json` 配置。
- Node.js；仅在执行资源检查时需要。

```powershell
# 打开交互菜单，使用方向键选择“构筑”或“清理”
.\build.ps1

# 检查最终解析出的构筑参数，不下载文件
.\build.ps1 -DryRun

# 跳过菜单，使用 defaultTarget 构筑到 build 目录
.\build.ps1 -Action Build

# 构筑配置中声明的其他目标
.\build.ps1 -Action Build -Target win-aarch64

# 只清理 build 目录和本地下载缓存
.\build.ps1 -Action Clean

# 清理下载缓存后重新构筑
.\build.ps1 -Action Build -Clean
```

无参数运行时可使用上下或左右方向键选择，按 Enter 确认，也可直接按数字键 `1` 或 `2`。可使用 `-Version` 临时覆盖产物版本，使用 `-SkipChecks` 跳过 Maa 资源检查。构筑失败时脚本会显示错误并等待按键；自动化调用可使用 `-Action` 指定操作，并通过 `-NoPause` 禁用错误等待。本地构筑只生成可直接运行的 `build` 目录；GitHub Release 流程仍会生成压缩包。

## 开发注意

- 发布流程使用 MWU 官方脚本组装资源和 OCR 模型。
- 新增任务后需要同步更新 `assets/interface.json` 的 `task` 列表。
- 图像模板应放在 `assets/resource/base/image/` 下，并在 pipeline 中使用相对文件名引用。
- Agent 使用第三方库时，应将依赖添加到根目录 `requirements.txt`。

## 鸣谢

本项目由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 和 [MWU](https://github.com/ravizhan/MWU) 驱动。
