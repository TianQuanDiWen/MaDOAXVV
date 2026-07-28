from pathlib import Path

import argparse
import json
import shutil
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
    shutil.copytree(
        working_dir / "agent",
        install_path / "agent",
        dirs_exist_ok=True,
    )


if __name__ == "__main__":
    install_mxu()
    install_deps()
    install_resource()
    install_chores()
    install_agent()

    print(f"Install to {install_path} successfully.")
