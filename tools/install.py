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


if __name__ == "__main__":
    install_mxu()
    install_deps()
    install_resource()
    install_chores()
    install_agent()

    print(f"Install to {install_path} successfully.")
