# MaDOAXVV

基于 MaaFramework 的《DEAD OR ALIVE Xtreme Venus Vacation》自动化项目。

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
- [.NET Desktop Runtime 10.0（x64）](https://dotnet.microsoft.com/zh-cn/download/dotnet/10.0)。
- [Microsoft Visual C++ 2015–2022 Redistributable（x64）](https://aka.ms/vs/17/release/vc_redist.x64.exe)。

首次运行前请确认游戏能够正常启动，并保持游戏窗口可见，窗口标题应为 `DOAX VenusVacation`。如果缺少 .NET 或 Visual C++ 运行库，可以管理员身份运行发布包内的 `DependencySetup_依赖库安装_win.bat` 自动安装依赖。

## 目录结构

```text
assets/interface.json              # MaaFramework 项目入口配置
assets/resource/pipeline/          # 任务流程定义
assets/resource/image/             # 图像识别模板
assets/resource/model/ocr/         # 本地 OCR 模型
tools/                             # 资源安装、OCR 配置和 schema 校验脚本
```

## 开发注意

- `assets/resource/model/ocr/` 体积较大，作为本地依赖保留即可。
- 新增任务后需要同步更新 `assets/interface.json` 的 `task` 列表。
- 图像模板应放在 `assets/resource/image/` 下，并在 pipeline 中使用相对文件名引用。

## 鸣谢

本项目由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 驱动。
