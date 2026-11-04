from pathlib import Path

import argparse
import json
import os
import shutil
import subprocess
import sys

try:
    import jsonc
except ModuleNotFoundError as e:
    raise ImportError(
        "Missing dependency 'json-with-comments' (imported as 'jsonc').\n"
        f"Install it with:\n  {sys.executable} -m pip install json-with-comments\n"
        "Or add it to your project's requirements."
    ) from e

from configure import configure_ocr_model


working_dir = Path(__file__).parent.parent.resolve()


def resolve_project_path(relative_path, field_name):
    path = Path(relative_path)
    if path.is_absolute():
        raise ValueError(f"{field_name} must be relative to the repository root")

    resolved_path = (working_dir / path).resolve()
    if not resolved_path.is_relative_to(working_dir):
        raise ValueError(f"{field_name} escapes the repository root")
    return resolved_path


parser = argparse.ArgumentParser()
parser.add_argument("version")
parser.add_argument("os_name", choices=("win", "linux", "macos"))
parser.add_argument("arch", choices=("x86_64", "aarch64"))
parser.add_argument("--config", default="build.config.json")
args = parser.parse_args()

config_path = Path(args.config)
if not config_path.is_absolute():
    config_path = working_dir / config_path
with config_path.resolve().open("r", encoding="utf-8") as file:
    build_config = json.load(file)

version = args.version
os_name = args.os_name
arch = args.arch
project_name = build_config["projectName"]
install_path = resolve_project_path(
    build_config["directories"]["output"],
    "directories.output",
)
dependencies_path = resolve_project_path(
    build_config["directories"]["dependencies"],
    "directories.dependencies",
)
downloads_path = resolve_project_path(
    build_config["directories"]["downloads"],
    "directories.downloads",
)


def expand_agent_output(template):
    executable_suffix = ".exe" if os_name == "win" else ""
    return template.replace("{exe}", executable_suffix)


def resolve_install_path(relative_path, field_name):
    path = Path(relative_path)
    if path.is_absolute():
        raise ValueError(f"{field_name} must be relative to the install root")

    resolved_path = (install_path / path).resolve()
    if not resolved_path.is_relative_to(install_path):
        raise ValueError(f"{field_name} escapes the install root")
    return resolved_path


def install_deps():
    if not (dependencies_path / "bin").exists():
        print('Please download the MaaFramework to "deps" first.')
        print('请先下载 MaaFramework 到 "deps"。')
        sys.exit(1)

    shutil.copytree(
        dependencies_path / "bin",
        install_path / "maafw",
        dirs_exist_ok=True,
    )

    agent_binary_path = dependencies_path / "share" / "MaaAgentBinary"
    if agent_binary_path.exists():
        shutil.copytree(
            agent_binary_path,
            install_path / "maafw" / "MaaAgentBinary",
            dirs_exist_ok=True,
        )


def set_windows_exe_icon(exe_path, ico_path):
    """
    Embed an .ico file into a Windows PE executable (.exe) using Windows Resource APIs.
    Sets both RT_ICON and RT_GROUP_ICON for resource IDs 32512 and 1 in languages 1033 (en-US) and 0 (neutral).
    Gracefully skips on non-Windows platforms.
    """
    if sys.platform != "win32":
        return

    exe_path = Path(exe_path)
    ico_path = Path(ico_path)
    if not exe_path.exists() or not ico_path.exists():
        return

    import ctypes
    from ctypes import wintypes
    import struct

    kernel32 = ctypes.windll.kernel32

    RT_ICON = 3
    RT_GROUP_ICON = 14

    kernel32.BeginUpdateResourceW.argtypes = [wintypes.LPCWSTR, wintypes.BOOL]
    kernel32.BeginUpdateResourceW.restype = wintypes.HANDLE

    kernel32.UpdateResourceW.argtypes = [
        wintypes.HANDLE,
        ctypes.c_void_p,
        ctypes.c_void_p,
        wintypes.WORD,
        ctypes.c_void_p,
        wintypes.DWORD,
    ]
    kernel32.UpdateResourceW.restype = wintypes.BOOL

    kernel32.EndUpdateResourceW.argtypes = [wintypes.HANDLE, wintypes.BOOL]
    kernel32.EndUpdateResourceW.restype = wintypes.BOOL

    try:
        with open(ico_path, "rb") as f:
            ico_data = f.read()

        reserved, ico_type, image_count = struct.unpack("<HHH", ico_data[:6])
        if reserved != 0 or ico_type != 1:
            return

        grp_header = ico_data[:6]
        grp_entries = bytearray()
        images = []
        offset = 6

        for i in range(1, image_count + 1):
            (
                width,
                height,
                color_count,
                reserved,
                planes,
                bit_count,
                bytes_in_res,
                image_offset,
            ) = struct.unpack("<BBBBHHII", ico_data[offset : offset + 16])
            offset += 16
            grp_entry = struct.pack(
                "<BBBBHHIH",
                width,
                height,
                color_count,
                reserved,
                planes,
                bit_count,
                bytes_in_res,
                i,
            )
            grp_entries.extend(grp_entry)
            images.append((i, ico_data[image_offset : image_offset + bytes_in_res]))

        grp_data = bytes(grp_header + grp_entries)

        h_update = kernel32.BeginUpdateResourceW(str(exe_path), False)
        if not h_update:
            err = kernel32.GetLastError()
            print(f"Notice: BeginUpdateResourceW failed for {exe_path.name} (error code: {err}).")
            return

        committed = False
        try:
            target_langs = [1033, 0]
            for icon_id, img_bytes in images:
                for lang in target_langs:
                    ok = kernel32.UpdateResourceW(
                        h_update,
                        RT_ICON,
                        icon_id,
                        lang,
                        img_bytes,
                        len(img_bytes),
                    )
                    if not ok:
                        err = kernel32.GetLastError()
                        print(f"Notice: Failed to write RT_ICON {icon_id} (lang {lang}) to {exe_path.name} (error code: {err}).")
                        return

            target_group_ids = [32512, 1]
            for grp_id in target_group_ids:
                for lang in target_langs:
                    ok = kernel32.UpdateResourceW(
                        h_update,
                        RT_GROUP_ICON,
                        grp_id,
                        lang,
                        grp_data,
                        len(grp_data),
                    )
                    if not ok:
                        err = kernel32.GetLastError()
                        print(f"Notice: Failed to write RT_GROUP_ICON {grp_id} (lang {lang}) to {exe_path.name} (error code: {err}).")
                        return

            success = kernel32.EndUpdateResourceW(h_update, False)
            committed = True
            if success:
                print(f"Embedded icon into {exe_path.name} successfully.")
            else:
                err = kernel32.GetLastError()
                print(f"Notice: EndUpdateResourceW returned False for {exe_path.name} (error code: {err}).")
        finally:
            if not committed:
                kernel32.EndUpdateResourceW(h_update, True)
    except Exception as e:
        print(f"Notice: Failed to embed icon into {exe_path.name}: {e}")


def install_mxu():
    mxu_path = downloads_path / "MXU"
    if not mxu_path.exists():
        raise FileNotFoundError(f"MXU directory not found: {mxu_path}")

    executable_name = "mxu.exe" if os_name == "win" else "mxu"
    executable_candidates = list(mxu_path.rglob(executable_name))
    if not executable_candidates:
        raise FileNotFoundError(
            f"MXU executable not found under: {mxu_path}"
        )

    install_path.mkdir(parents=True, exist_ok=True)
    original_executable = executable_candidates[0]
    executable_suffix = ".exe" if os_name == "win" else ""
    project_executable = install_path / f"{project_name}{executable_suffix}"
    shutil.copy2(original_executable, project_executable)

    if os_name == "win":
        set_windows_exe_icon(project_executable, working_dir / "assets" / "icon.ico")


def install_resource():

    configure_ocr_model()

    shutil.copytree(
        working_dir / "assets" / "resource",
        install_path / "resource",
        dirs_exist_ok=True,
    )

    shutil.copy2(
        working_dir / "assets" / "interface.json",
        install_path,
    )

    with open(install_path / "interface.json", "r", encoding="utf-8") as f:
        interface = jsonc.load(f)

    interface["version"] = version
    # assets/interface.json 面向 MaaTools 源码调试，直接通过 go run 启动 Agent。
    # 发布包不应依赖 Go 工具链，因此在复制后改写为已编译的原生 Agent。
    interface["pretask"]["exec"] = "./agent/MaDOAXVV.Agent.exe"
    interface["pretask"]["args"] = [
        "launch-game",
        "--root",
        ".",
        "--steam-uri",
        "steam://rungameid/958260",
        "--launcher-process",
        "DOAX_VV_Launcher.exe",
        "--game-window",
        "^DOAX VenusVacation$",
        "--timeout",
        "300s",
    ]
    interface["agent"]["child_exec"] = "./agent/MaDOAXVV.Agent.exe"
    interface["agent"]["child_args"] = ["agent", "--root", "."]
    # assets/interface.json 源码调试态使用当前目录下的 icon.png / logo.png；
    # 发布包中 interface.json 位于根目录，视觉资产统一归入 assets/ 目录。
    interface["icon"] = "assets/icon.png"
    interface["logo"] = "assets/logo.png"

    with open(install_path / "interface.json", "w", encoding="utf-8") as f:
        jsonc.dump(interface, f, ensure_ascii=False, indent=4)


def install_chores():
    shutil.copy2(
        working_dir / "README.md",
        install_path,
    )
    shutil.copy2(
        working_dir / "LICENSE",
        install_path,
    )

    # 仅将应用图标与 Logo 拷贝至 install/assets 目录供界面与 README 使用
    install_assets_dir = install_path / "assets"
    install_assets_dir.mkdir(parents=True, exist_ok=True)
    for icon_name in ("icon.ico", "icon.png", "logo.png"):
        src = working_dir / "assets" / icon_name
        if src.exists():
            shutil.copy2(src, install_assets_dir / icon_name)


def install_agent():
    source_path = resolve_project_path(
        build_config["agent"]["source"],
        "agent.source",
    )
    output_path = resolve_install_path(
        expand_agent_output(build_config["agent"]["output"]),
        "agent.output",
    )
    output_path.parent.mkdir(parents=True, exist_ok=True)

    go_os = {"win": "windows", "linux": "linux", "macos": "darwin"}[os_name]
    go_arch = {"x86_64": "amd64", "aarch64": "arm64"}[arch]
    environment = os.environ.copy()
    environment.update({
        "CGO_ENABLED": "0",
        "GOOS": go_os,
        "GOARCH": go_arch,
    })
    subprocess.run(
        [
            "go",
            "build",
            "-trimpath",
            "-buildvcs=false",
            "-ldflags=-s -w",
            "-o",
            str(output_path),
            str(source_path),
        ],
        cwd=working_dir,
        env=environment,
        check=True,
    )

    if os_name == "win":
        set_windows_exe_icon(output_path, working_dir / "assets" / "icon.ico")


if __name__ == "__main__":
    install_mxu()
    install_deps()
    install_resource()
    install_chores()
    install_agent()

    print(f"Install to {install_path} successfully.")
