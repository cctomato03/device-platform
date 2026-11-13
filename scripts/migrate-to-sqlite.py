#!/usr/bin/env python3
"""把 MySQL 方言的迁移脚本转换为 SQLite 方言。

用法：python scripts/migrate-to-sqlite.py migrations
原地改写 migrations/*.up.sql（先 git commit，便于对比）。

转换规则：
  - BIGINT/INT/TINYINT/SMALLINT/... -> INTEGER
  - VARCHAR(n)/CHAR(n)/JSON/ENUM -> TEXT
  - COMMENT '...' 删除
  - ON UPDATE CURRENT_TIMESTAMP 删除（由 GORM 的 autoUpdateTime 负责）
  - DEFAULT CURRENT_TIMESTAMP -> DEFAULT (datetime('now','localtime'))
  - 表尾 ) ENGINE=... COMMENT='...' -> )
  - 表内 KEY/UNIQUE KEY 索引 -> 独立 CREATE INDEX / CREATE UNIQUE INDEX
  - INSERT IGNORE -> INSERT OR IGNORE
  - SET NAMES / SET FOREIGN_KEY_CHECKS 删除
"""
import re
import sys
from pathlib import Path

TYPE_MAP = [
    (re.compile(r"\bBIGINT\s+UNSIGNED\b", re.I), "INTEGER"),
    (re.compile(r"\bINT\s+UNSIGNED\b", re.I), "INTEGER"),
    (re.compile(r"\bTINYINT\s+UNSIGNED\b", re.I), "INTEGER"),
    (re.compile(r"\bSMALLINT\s+UNSIGNED\b", re.I), "INTEGER"),
    (re.compile(r"\bBIGINT\b", re.I), "INTEGER"),
    (re.compile(r"\bTINYINT\s*\(\s*\d+\s*\)", re.I), "INTEGER"),
    (re.compile(r"\bSMALLINT\s*\(\s*\d+\s*\)", re.I), "INTEGER"),
    (re.compile(r"\bINT\s*\(\s*\d+\s*\)", re.I), "INTEGER"),
    (re.compile(r"\bTINYINT\b", re.I), "INTEGER"),
    (re.compile(r"\bSMALLINT\b", re.I), "INTEGER"),
    (re.compile(r"\bMEDIUMINT\b", re.I), "INTEGER"),
    (re.compile(r"\bINT\b", re.I), "INTEGER"),
    (re.compile(r"\bVARCHAR\s*\(\s*\d+\s*\)", re.I), "TEXT"),
    (re.compile(r"\bNVARCHAR\s*\(\s*\d+\s*\)", re.I), "TEXT"),
    (re.compile(r"\bCHAR\s*\(\s*\d+\s*\)", re.I), "TEXT"),
    (re.compile(r"\bLONGTEXT\b", re.I), "TEXT"),
    (re.compile(r"\bMEDIUMTEXT\b", re.I), "TEXT"),
    (re.compile(r"\bJSON\b", re.I), "TEXT"),
    (re.compile(r"\bDOUBLE\b", re.I), "REAL"),
    (re.compile(r"\bFLOAT\b", re.I), "REAL"),
    (re.compile(r"\bDATETIME\b", re.I), "DATETIME"),
    (re.compile(r"\bTIMESTAMP\b", re.I), "DATETIME"),
]

COMMENT_RE = re.compile(r"\s+COMMENT\s+'(?:[^'\\]|\\.|'')*'", re.I)
ONUPDATE_RE = re.compile(r"\s+ON\s+UPDATE\s+CURRENT_TIMESTAMP", re.I)
INDEX_LINE_RE = re.compile(
    r"^\s*(UNIQUE\s+)?KEY\s+`?(\w+)`?\s*\((.*?)\)\s*$", re.I | re.S
)
CREATE_TABLE_RE = re.compile(
    r"CREATE\s+TABLE\s+(IF\s+NOT\s+EXISTS\s+)?`?(\w+)`?\s*\((.*?)\n\)\s*(ENGINE[^;]*)?;",
    re.I | re.S,
)


def strip_comment(text: str) -> str:
    return COMMENT_RE.sub("", text)


def convert_types(text: str) -> str:
    for pat, rep in TYPE_MAP:
        text = pat.sub(rep, text)
    return text


def convert_column_line(line: str) -> str:
    line = strip_comment(line)
    line = ONUPDATE_RE.sub("", line)
    line = convert_types(line)
    line = re.sub(r"\bCURRENT_TIMESTAMP\b", "(datetime('now','localtime'))", line, flags=re.I)
    return line


def split_top_level(inner: str):
    """按顶层逗号切分列定义（忽略括号与引号内的逗号）。"""
    parts, buf, depth, quote = [], [], 0, None
    i = 0
    while i < len(inner):
        ch = inner[i]
        if quote:
            buf.append(ch)
            if ch == quote:
                quote = None
            elif ch == "\\" and i + 1 < len(inner):
                i += 1
                buf.append(inner[i])
        elif ch in "'\"`":
            quote = ch
            buf.append(ch)
        elif ch == "(":
            depth += 1
            buf.append(ch)
        elif ch == ")":
            depth -= 1
            buf.append(ch)
        elif ch == "," and depth == 0:
            parts.append("".join(buf).strip())
            buf = []
        else:
            buf.append(ch)
        i += 1
    tail = "".join(buf).strip()
    if tail:
        parts.append(tail)
    return parts


def convert_create_table(match: re.Match) -> str:
    table = match.group(2)
    body = match.group(3)
    cols, indexes = [], []
    for item in split_top_level(body):
        item = item.strip()
        if not item:
            continue
        m = INDEX_LINE_RE.match(item)
        if m and not item.upper().startswith("PRIMARY"):
            unique = "UNIQUE" if m.group(1) else ""
            name = m.group(2)
            cols_expr = m.group(3)
            # 去掉 MySQL 前缀长度：ancestors(191) -> ancestors
            cols_expr = re.sub(r"\(\s*\d+\s*\)", "", cols_expr)
            indexes.append(
                f"CREATE {unique} INDEX IF NOT EXISTS idx_{table}_{name} ON {table} ({cols_expr});"
            )
            continue
        # 多行 KEY 定义被切分时也可能出现 KEY 前缀
        if re.match(r"^\s*(UNIQUE\s+)?KEY\s", item, re.I) and not item.upper().startswith("PRIMARY"):
            continue
        cols.append(convert_column_line(item))

    out = f"CREATE TABLE IF NOT EXISTS {table} (\n  " + ",\n  ".join(cols) + "\n);"
    if indexes:
        out += "\n" + "\n".join(indexes)
    return out


def convert_alter_table(stmt: str) -> str:
    """ALTER TABLE 的 MySQL 扩展语法在 SQLite 需拆成多条语句（这里只处理单条 ADD COLUMN）。"""
    m = re.match(
        r"ALTER\s+TABLE\s+`?(\w+)`?\s+ADD\s+COLUMN\s+(.*)$", stmt.strip(), re.I | re.S
    )
    if not m:
        return stmt
    table, rest = m.group(1), m.group(2)
    rest = re.sub(r"\s+AFTER\s+`?\w+`?", "", rest, flags=re.I)
    rest = strip_comment(rest)
    rest = convert_types(rest)
    return f"ALTER TABLE {table} ADD COLUMN {rest.strip()}"


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else "migrations")
    for path in sorted(root.glob("*.up.sql")):
        text = path.read_text(encoding="utf-8")
        orig = text
        # 删除 MySQL 会话设置
        text = re.sub(r"^\s*SET\s+NAMES[^;]*;\s*$", "", text, flags=re.I | re.M)
        text = re.sub(r"^\s*SET\s+FOREIGN_KEY_CHECKS[^;]*;\s*$", "", text, flags=re.I | re.M)
        # CREATE TABLE 整体转换
        text = CREATE_TABLE_RE.sub(convert_create_table, text)
        # ALTER TABLE ADD COLUMN
        text = re.sub(
            r"ALTER\s+TABLE\s+`?(\w+)`?\s+ADD\s+COLUMN\s+[^;]*;",
            lambda m: convert_alter_table(m.group(0)) + ";",
            text,
            flags=re.I | re.S,
        )
        # 剩余语句：只替换 INSERT IGNORE 与 NOW()（INSERT/UPDATE 的文本值不能被类型替换破坏）
        text = re.sub(r"\bINSERT\s+IGNORE\s+INTO\b", "INSERT OR IGNORE INTO", text, flags=re.I)
        text = re.sub(r"\bNOW\s*\(\s*\)", "datetime('now','localtime')", text, flags=re.I)
        if text != orig:
            path.write_text(text, encoding="utf-8")
            print(f"converted {path.name}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
