#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import re
import sys
import json
import datetime
import subprocess
import urllib.request
from pathlib import Path

# 最大文本限制，防止超长 Commit 导致 API 413/400
MAX_COMMITS_LEN = 10000  # 约 10KB
MAX_STAT_LEN = 4000      # 约 4KB
MAX_COMMITS_COUNT = 60

STATIC_TEMPLATE = """
---

## ⚠️ 使用说明与免责声明

* **完全开源免费**：本项目为非盈利的开源辅助工具，遵循 AGPL-3.0 开源协议，**完全免费，严禁任何形式的倒卖、付费购买或闭源商用**。
* **专注日常减负**：本项目纯粹为自动化清理每日日常设计，不包含任何破坏游戏平衡、篡改内存数据或挂机刷分等侵入性功能。
* **风险评估**：本项目为第三方非官方自动化工具，与游戏开发商及运营商（KOEI TECMO GAMES 光荣特库摩）无关。使用自动化辅助可能违反游戏用户协议，请自行评估使用风险。因使用本工具造成的任何账号异常或数据损失，由使用者自行承担。

---

## 🙏 鸣谢

* 本项目自动化核心与图像推理引擎由 [MaaFramework](https://github.com/MaaXYZ/MaaFramework) 强力驱动。
* 桌面端交互界面基于 [MXU](https://github.com/MistEO/MXU) 及其自维护分支 [MXU_tqdw](https://github.com/TianQuanDiWen/MXU_tqdw)。
* OCR 基础模型与文字词典来自 [MaaCommonAssets](https://github.com/MaaXYZ/MaaCommonAssets)。
"""

def get_fallback_content():
    """读取上一阶段 git-cliff 产生的 CHANGES.md 作为安全兜底"""
    changes_path = Path("CHANGES.md")
    if changes_path.is_file():
        try:
            content = changes_path.read_text(encoding="utf-8").strip()
            if content:
                return content
        except Exception as e:
            print(f"[Warning] Failed to read CHANGES.md: {e}", file=sys.stderr)
    return ""

def find_prev_tag(current_tag="", is_prerelease=False):
    """
    寻找当前提交所属历史中的上一个有效基准标签：
    1. 必须是当前 HEAD 的祖先提交（属于当前分支历史），排除来自其他分支的较新标签。
    2. 若为正式版（is_prerelease=False），仅匹配正式版标签（如 v1.2.3），跳过 alpha/beta/rc/ci 等预发布标签。
    3. 若为预发布版（is_prerelease=True），匹配任何以 v 开头的有效版本标签。
    4. 拓扑排序：选取与 HEAD 拓扑距离最近的基准标签。
    """
    formal_pattern = re.compile(r"^v[0-9]+\.[0-9]+\.[0-9]+$")

    try:
        merged_tags = [
            t.strip() for t in subprocess.check_output(
                ["git", "tag", "--merged", "HEAD"],
                stderr=subprocess.DEVNULL,
                text=True,
                encoding="utf-8"
            ).splitlines() if t.strip()
        ]
    except Exception as e:
        print(f"[Warning] Failed to list merged git tags: {e}", file=sys.stderr)
        merged_tags = []

    candidates = []
    for t in merged_tags:
        if t == current_tag or not t.startswith("v"):
            continue
        if not is_prerelease and not formal_pattern.match(t):
            continue
        candidates.append(t)

    if not candidates:
        return ""

    def get_distance(tag):
        try:
            return int(subprocess.check_output(
                ["git", "rev-list", "--count", f"{tag}..HEAD"],
                stderr=subprocess.DEVNULL,
                text=True,
                encoding="utf-8"
            ).strip())
        except Exception:
            return 999999999

    candidates.sort(key=get_distance)
    return candidates[0]

def get_git_info(current_tag="", is_prerelease=False):
    """获取当前版本与上一版本之间的 Git 提交和 diff 统计，并施加严格的体积上限保护"""
    prev_tag = find_prev_tag(current_tag, is_prerelease)

    cmd = ["git", "log", "--pretty=format:%h - %s%n%b", "--no-merges"]
    if prev_tag:
        cmd.extend([f"{prev_tag}..HEAD", f"-n{MAX_COMMITS_COUNT}"])
        print(f"Comparing changes between base tag {prev_tag} and HEAD (max {MAX_COMMITS_COUNT} commits)")
    else:
        cmd.extend([f"-n{MAX_COMMITS_COUNT}"])
        print(f"No previous ancestor tag found, analyzing up to {MAX_COMMITS_COUNT} recent commits")

    try:
        commits = subprocess.check_output(cmd, stderr=subprocess.DEVNULL, text=True, encoding="utf-8").strip()
    except Exception as e:
        print(f"[Warning] Failed to get git log: {e}", file=sys.stderr)
        commits = ""

    # 体积硬上限保护，防止超长 Commit 挤爆请求体
    if len(commits) > MAX_COMMITS_LEN:
        commits = commits[:MAX_COMMITS_LEN] + "\n\n... (提交内容已截断以防超出模型上下文上限)"

    try:
        cmd_stat = ["git", "diff", "--stat"]
        if prev_tag:
            cmd_stat.append(prev_tag)
        else:
            cmd_stat.extend(["HEAD~5", "HEAD"])
        stat = subprocess.check_output(cmd_stat, stderr=subprocess.DEVNULL, text=True, encoding="utf-8").strip()
    except Exception as e:
        print(f"[Warning] Failed to get git diff stat: {e}", file=sys.stderr)
        stat = ""

    if len(stat) > MAX_STAT_LEN:
        stat = stat[:MAX_STAT_LEN] + "\n... (文件改动统计已截断)"

    return prev_tag, commits, stat

def call_ai(token, base_url, model, prompt):
    """
    统一调用大模型。支持：
    1. Google Gemini 官方端点（默认）
    2. 任意第三方 OpenAI 兼容端点（当指定 AI_BASE_URL 时，如 DeepSeek, OpenRouter, OneAPI 等）
    """
    token = token.strip()
    base_url = (base_url or "").strip().rstrip("/")

    # 若未指定 base_url 或属于 Google 官方端点，默认走 Gemini 官方接口
    is_gemini = not base_url or "googleapis.com" in base_url

    if is_gemini:
        gemini_model = model or "gemini-3.5-flash-lite"
        if not base_url:
            base_url = "https://generativelanguage.googleapis.com/v1beta/models"

        if not base_url.endswith("/models"):
            target_url = f"{base_url}/models/{gemini_model}:generateContent?key={token}"
        else:
            target_url = f"{base_url}/{gemini_model}:generateContent?key={token}"

        print(f"Calling Google Gemini API (model: {gemini_model})...")
        payload = {
            "contents": [{"parts": [{"text": prompt}]}],
            "generationConfig": {
                "temperature": 0.3,
                "maxOutputTokens": 4096
            }
        }
        req = urllib.request.Request(
            target_url,
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=40) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            candidates = data.get("candidates", [])
            if not candidates:
                raise ValueError(f"Gemini API returned no candidates: {data}")

            candidate = candidates[0]
            finish_reason = candidate.get("finishReason", "STOP")
            if finish_reason == "MAX_TOKENS":
                raise ValueError(f"Gemini output was truncated due to token limit (finishReason={finish_reason})")
            if finish_reason not in ("STOP", "FINISH_REASON_UNSPECIFIED", ""):
                raise ValueError(f"Gemini generation terminated abnormally (finishReason={finish_reason})")

            parts = candidate.get("content", {}).get("parts", [])
            text_parts = [
                part["text"] for part in parts
                if isinstance(part, dict) and "text" in part and not part.get("thought", False)
            ]
            full_text = "".join(text_parts).strip()
            if not full_text:
                raise ValueError("Gemini returned empty text content across all response parts")
            return full_text

    # 任意第三方 OpenAI 兼容协议（当用户指定了自定义 AI_BASE_URL 时）
    target_url = f"{base_url}/chat/completions" if not base_url.endswith("/chat/completions") else base_url
    openai_model = model or "deepseek-chat"

    print(f"Calling OpenAI-compatible API at {target_url} (model: {openai_model})...")
    payload = {
        "model": openai_model,
        "messages": [
            {
                "role": "system",
                "content": (
                    "你是一个资深开源项目发布说明撰写专家。你正在为《DEAD OR ALIVE Xtreme Venus Vacation》（死或生：沙滩排球女神假期）"
                    "的自动化减负辅助工具 MaDOAXVV 编写版本更新说明。"
                    "请仅输出版本导语和主要更新模块列表，保持专业严谨、因果逻辑清晰。"
                )
            },
            {
                "role": "user",
                "content": prompt
            }
        ],
        "temperature": 0.3,
        "max_tokens": 4096
    }
    req = urllib.request.Request(
        target_url,
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
            "User-Agent": "MaDOAXVV-Release-Notes-Generator"
        },
        method="POST"
    )
    with urllib.request.urlopen(req, timeout=40) as resp:
        data = json.loads(resp.read().decode("utf-8"))
        choices = data.get("choices", [])
        if not choices:
            raise ValueError(f"OpenAI-compatible API returned no choices: {data}")

        choice = choices[0]
        finish_reason = choice.get("finish_reason", "stop")
        if finish_reason == "length":
            raise ValueError(f"OpenAI output was truncated due to token limit (finish_reason={finish_reason})")
        if finish_reason not in ("stop", None, ""):
            raise ValueError(f"OpenAI generation terminated abnormally (finish_reason={finish_reason})")

        content = choice.get("message", {}).get("content", "").strip()
        if not content:
            raise ValueError("OpenAI-compatible API returned empty message content")
        return content

def write_output(content):
    """将最终文案写入 GITHUB_OUTPUT 及本地 CHANGES.md"""
    output_file = os.environ.get("GITHUB_OUTPUT")
    if output_file:
        delimiter = "RELEASE_BODY_EOF"
        with open(output_file, "a", encoding="utf-8") as f:
            f.write(f"content<<{delimiter}\n{content}\n{delimiter}\n")
        print("Successfully written release notes to GITHUB_OUTPUT")

    try:
        with open("CHANGES.md", "w", encoding="utf-8") as f:
            f.write(content)
        print("Updated CHANGES.md locally")
    except Exception as e:
        print(f"[Warning] Failed to write CHANGES.md: {e}", file=sys.stderr)

def main():
    token = os.environ.get("AI_TOKEN", "").strip()
    base_url = os.environ.get("AI_BASE_URL", "").strip()
    model = os.environ.get("AI_MODEL", "").strip()
    tag_name = os.environ.get("TAG_NAME", "v0.1.0").strip()
    current_date = datetime.date.today().strftime("%Y-%m-%d")

    # 判断是否为预发布版本
    is_prerelease_env = os.environ.get("IS_PRERELEASE", "").strip().lower()
    if is_prerelease_env:
        is_prerelease = is_prerelease_env == "true"
    else:
        is_prerelease = bool(re.search(r"-(alpha|beta|rc|dev|ci)", tag_name, re.I))

    fallback_content = get_fallback_content()

    if not token:
        print("[Notice] No AI_TOKEN configured, falling back to git-cliff output.")
        write_output(fallback_content)
        return

    # 变更信息源：优先采用 git-cliff 已按工作流规则精确计算的变更列表，避免重复执行 Git 提交与标签距离计算
    stat_info = ""
    if fallback_content:
        changes_input = fallback_content.strip()
        print("Using git-cliff generated changes as authoritative summary input (skipped redundant git queries).")
        scope_desc = f"本次版本范围（{'预发布版' if is_prerelease else '正式版'}）"
    else:
        # 当缺少 CHANGES.md 时（如本地单独测试），延迟执行 Git 拓扑与提交查询
        prev_tag, commits, stat = get_git_info(tag_name, is_prerelease)
        if not commits:
            print("[Notice] No changes or commits found to summarize, falling back to git-cliff.")
            write_output(fallback_content)
            return
        changes_input = commits.strip()
        scope_desc = f"本次版本范围（{'预发布版' if is_prerelease else '正式版'}，对比基准：{prev_tag or '初始版本'}）"
        if stat:
            stat_info = f"\n--- 文件改动统计 (相对于 {prev_tag or 'HEAD~5'}) ---\n{stat}\n"

    if not changes_input:
        print("[Notice] No changes or commits found to summarize, falling back to git-cliff.")
        write_output(fallback_content)
        return

    # 保护上限
    if len(changes_input) > MAX_COMMITS_LEN:
        changes_input = changes_input[:MAX_COMMITS_LEN] + "\n\n... (变更内容已截断以防超出模型上下文上限)"

    # 采用模板拼装方式：仅由 AI 归纳更新要点，免责声明与鸣谢等固定章节由脚本直接追加，既减少 Token 消耗，也保证固定章节内容的准确规范
    prompt = f"""请根据以下由版本工具根据{scope_desc}生成的变更信息，为 MaDOAXVV 的新版本 {tag_name} 提炼核心更新说明。

【输出要求】：
1. 第一行标题：# MaDOAXVV {tag_name}  ({current_date})
2. 版本导语：紧跟标题后，用 1~2 句流畅的自然语言提炼概括本版本的核心更新重点与优化方向（例如：本版本重构了旁路防遮挡退避机制，优化了自动排位与新比赛作战指示配置...）。
3. ## 主要更新：
   - 按功能模块组织分类（例如「自动打新比赛」、「自动排位」、「日常流程与识别精度优化」、「防作弊与安全点击机制」等）。
   - 每个模块使用加粗标题：`* **新增「模块名称」**：` 或 `* **优化「模块名称」**：` 或 `* **重构「模块名称」**：`。
   - 子项使用二级列表（`  * xxx`），深入说明改动原因、解决的具体痛点及稳定性提升，切忌原样复制单行 commit。
4. 注意：仅需输出上述内容！不要输出“运行环境”、“快速上手”、“免责声明”或“鸣谢”章节（这些将由系统自动追加）。

【本次变更信息】：
--- 变更详情 ---
{changes_input}
"""
    if stat_info:
        prompt += stat_info

    try:
        print("Calling AI to generate core release summary...")
        ai_summary = call_ai(token, base_url, model, prompt)
        if ai_summary and len(ai_summary.strip()) > 30:
            print("AI summary generated successfully, assembling with template...")
            # 静态模板拼接
            full_release_note = ai_summary.strip() + "\n" + STATIC_TEMPLATE.strip() + "\n"
            write_output(full_release_note)
            return
        else:
            print("[Warning] AI returned empty or too short response, using fallback.")
            write_output(fallback_content)
    except Exception as e:
        print(f"[Error] Failed to call AI API ({e}), falling back to git-cliff.", file=sys.stderr)
        write_output(fallback_content)

if __name__ == "__main__":
    main()
