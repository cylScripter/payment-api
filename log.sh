#!/bin/bash

exec python3 - "$(dirname "$0")/logs" "$@" <<'PY'
import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path
from urllib.parse import unquote_plus

log_dir = Path(sys.argv.pop(1))
parser = argparse.ArgumentParser(description="按日期和关键词查询日志", prog="log.sh")
parser.add_argument("date", help="日期，例如 20261118")
parser.add_argument("keyword", nargs="?", default="", help="关键词，省略时查看当天全部日志")
args = parser.parse_args()
use_color = sys.stdout.isatty()
level_colors = {
    "TRACE": "90",
    "DEBUG": "90",
    "INFO": "32",
    "WARN": "33",
    "WARNING": "33",
    "ERROR": "31",
    "FATAL": "1;31",
    "PANIC": "1;31",
}


def style(text, code=""):
    text = str(text)
    if not use_color:
        return text
    prefix = f"\033[{code}m" if code else ""
    if args.keyword:
        highlight = f"\033[1;33m{args.keyword}\033[0m{prefix}"
        text = text.replace(args.keyword, highlight)
    return f"{prefix}{text}\033[0m"


try:
    date = datetime.strptime(args.date.replace("-", ""), "%Y%m%d").strftime("%Y-%m-%d")
except ValueError:
    parser.error("日期格式应为 YYYYMMDD，例如 20261118")

files = sorted(log_dir.glob(f"{date}-*.log"))
if not files:
    sys.exit(f"未找到 {log_dir}/{date}-*.log")

matched = False
for path in files:
    container_ip = path.stem[len(date) + 1:]
    with path.open(encoding="utf-8", errors="replace") as stream:
        for line in stream:
            try:
                record = json.loads(line)
            except ValueError:
                record = None

            if isinstance(record, dict) and isinstance(record.get("body"), str):
                try:
                    record["body"] = json.loads(record["body"])
                except ValueError:
                    pass

            formatted = json.dumps(record, ensure_ascii=False) if isinstance(record, dict) else line
            if args.keyword and args.keyword not in line and args.keyword not in formatted:
                continue

            matched = True

            if isinstance(record, dict):
                timestamp = record.pop("time", "")
                level = str(record.pop("level", "info")).upper()
                message = record.pop("msg", "")
                method = record.pop("method", "")
                request_path = record.pop("path", "")
                parts = [
                    f"{style(timestamp, '90')} "
                    f"{style(f'[{level}]', level_colors.get(level, '37'))} "
                    f"{style(f'[{container_ip}]', '1;34')}"
                ]
                if method or request_path:
                    parts.append(f"{style(method, '36')} {style(request_path)}".strip())
                parts.append(style(str(message).replace("\r", r"\r").replace("\n", r"\n")))
                for key, value in record.items():
                    if key == "query" and isinstance(value, str):
                        value = unquote_plus(value)
                    if value == "":
                        continue
                    if isinstance(value, str):
                        value = value.replace("\r", r"\r").replace("\n", r"\n")
                        value_color = "90" if key in ("file", "func") else ""
                        value = style(value, value_color)
                    else:
                        value = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
                        segments = re.split(r'("(?:\\.|[^"\\])*")(?=\s*:)', value)
                        for index, segment in enumerate(segments):
                            segments[index] = style(segment, "1;34" if index % 2 else "")
                        value = "".join(segments)
                    parts.append(f"{style(key, '1;34')}={value}")
                print(style(" | ", "90").join(parts))
            else:
                print(style(line.rstrip("\n")))

if not matched:
    sys.exit("没有匹配的日志")
PY
