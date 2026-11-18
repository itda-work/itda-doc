#!/usr/bin/env python3
"""Bash 로 저장소 파일을 쓰는 명령을 막는 PreToolUse 훅.

저장소 파일은 Write/Edit 도구로만 고친다. 이 훅은 그 규칙을 어기는 흔한 꼴을
막는다(.claude/settings.json 의 PreToolUse Bash 훅이 부른다).

- sed -i / perl -i 로 저장소 파일을 고침
- 셸 리다이렉트(> >> >| &>)·tee 로 저장소 파일을 씀(heredoc 포함)
- cp / mv 의 목적지가 저장소 안
- python 인라인 코드(-c, heredoc)가 저장소 경로에 씀

저장소 밖(/tmp, /private/tmp, /dev, 스크래치패드 등)과 git 이 무시하는 경로
(build/ 등)는 허용한다. 판정할 수 없는 경로($VAR 가 남은 경로)는 허용한다.
해석에 실패하면 막지 않는다(훅 오류로 일을 멈추지 않는다).
"""

import glob
import json
import os
import re
import shlex
import subprocess
import sys

SEPARATORS = {";", "&&", "||", "|", "&", "(", ")", "|&", ";;"}
WRAPPERS = {"sudo", "env", "command", "builtin", "time", "nohup", "exec", "xargs"}
REDIRECTS = {">", ">>", ">|", "&>", "&>>"}
PY_WRITE = re.compile(
    r"open\([^)]*['\"][^'\"]*[wax+][^'\"]*['\"]"
    r"|\.write_text\(|\.write_bytes\("
    r"|shutil\.(copy|copy2|copyfile|move)\(|os\.(rename|replace)\("
    r"|inplace\s*=\s*True"
)
PY_LITERAL = re.compile(r"['\"]([^'\"\n\s]+)['\"]")
HEREDOC = re.compile(r"<<(-?)\s*(['\"]?)([A-Za-z_][A-Za-z0-9_]*)\2")


def git(cwd, *args):
    try:
        r = subprocess.run(
            ["git", "-C", cwd, *args], capture_output=True, text=True, timeout=5
        )
    except Exception:
        return None
    return r


def repo_roots(cwd):
    roots = []
    r = git(cwd, "rev-parse", "--show-toplevel", "--git-common-dir")
    if r is None or r.returncode != 0:
        return roots
    lines = r.stdout.splitlines()
    if lines:
        roots.append(os.path.realpath(lines[0]))
    if len(lines) > 1:
        common = lines[1]
        if not os.path.isabs(common):
            common = os.path.join(cwd, common)
        main = os.path.dirname(os.path.realpath(common))
        if main not in roots:
            roots.append(main)
    return roots


def resolve(path, cwd):
    """경로를 절대 경로로. 판정할 수 없으면 None."""
    if not path or path == "-":
        return None
    p = os.path.expanduser(os.path.expandvars(path))
    if "$" in p or "`" in p:
        return None
    if not os.path.isabs(p):
        if cwd is None:
            return None
        p = os.path.join(cwd, p)
    # 없는 파일도 다룰 수 있게 부모 디렉터리만 실경로로 푼다
    head, tail = os.path.split(os.path.normpath(p))
    return os.path.join(os.path.realpath(head), tail)


def in_repo(path, cwd, roots):
    """저장소 안이고 git 이 무시하지 않는 경로면 True."""
    abs_path = resolve(path, cwd)
    if abs_path is None:
        return False
    for root in roots:
        if abs_path == root or abs_path.startswith(root + os.sep):
            if os.sep + ".git" + os.sep in abs_path[len(root):] + os.sep:
                return False  # .git 내부는 git 이 쓴다
            r = git(root, "check-ignore", "-q", "--no-index", abs_path)
            if r is not None and r.returncode == 0:
                return False
            return True
    return False


def expand(token, cwd):
    if any(c in token for c in "*?[") and (cwd is not None or os.path.isabs(token)):
        base = token if os.path.isabs(token) else os.path.join(cwd, token)
        hits = glob.glob(os.path.expanduser(base))
        return hits or [token]
    return [token]


def strip_heredocs(text):
    """heredoc 본문을 빼고 명령 줄만 남긴다. 본문은 따로 돌려준다."""
    lines = text.split("\n")
    out, bodies = [], []
    i = 0
    while i < len(lines):
        line = lines[i]
        out.append(line)
        i += 1
        for m in HEREDOC.finditer(line):
            if line[max(0, m.start() - 1):m.start()] == "<":
                continue  # <<< here-string
            dash, delim = m.group(1), m.group(3)
            body = []
            while i < len(lines):
                cur = lines[i].lstrip("\t") if dash else lines[i]
                i += 1
                if cur == delim:
                    break
                body.append(lines[i - 1])
            bodies.append("\n".join(body))
    return "\n".join(out), bodies


def tokenize(text):
    text = text.replace("\\\n", " ").replace("\n", " ; ")
    lex = shlex.shlex(text, posix=True, punctuation_chars=True)
    lex.whitespace_split = True
    lex.commenters = ""
    return list(lex)


def segments(tokens):
    seg = []
    for t in tokens:
        if t in SEPARATORS:
            if seg:
                yield seg
            seg = []
        else:
            seg.append(t)
    if seg:
        yield seg


def command_word(seg):
    i = 0
    while i < len(seg):
        t = seg[i]
        if re.match(r"^[A-Za-z_][A-Za-z0-9_]*=", t):
            i += 1
            continue
        if t in WRAPPERS:
            i += 1
            while i < len(seg) and seg[i].startswith("-"):
                i += 1
            continue
        return os.path.basename(t), seg[i + 1:]
    return None, []


def check(command, cwd, roots):
    head, bodies = strip_heredocs(command)
    tokens = tokenize(head)
    found = []

    for seg in segments(tokens):
        # 리다이렉트
        words = []
        k = 0
        while k < len(seg):
            t = seg[k]
            if t in REDIRECTS or t == ">&":
                target = seg[k + 1] if k + 1 < len(seg) else ""
                k += 2
                if t == ">&" and (target.isdigit() or target == "-"):
                    continue
                if in_repo(target, cwd, roots):
                    found.append(f"리다이렉트 {t} {target}")
                continue
            if t in {"<", "<<", "<<-", "<<<", "<&", "<>"}:
                k += 2
                continue
            words.append(t)
            k += 1

        name, args = command_word(words)
        if name is None:
            continue
        if name in {"cd", "pushd"} and args:
            nxt = resolve(args[0], cwd)
            if nxt is None:
                # $VAR 등으로 옮겨 간 곳을 모른다 — 뒤의 상대 경로는 판정하지 않는다
                cwd = None
                continue
            cwd = os.path.realpath(nxt)
            continue
        opts = [a for a in args if a.startswith("-") and a != "-"]
        plain = [a for a in args if not (a.startswith("-") and a != "-")]

        if name in {"sed", "gsed"} and any(
            re.match(r"^-[A-Za-z]*i", a) or a.startswith("--in-place") for a in opts
        ):
            for a in plain:
                for p in expand(a, cwd):
                    if os.path.isfile(resolve(p, cwd) or "") and in_repo(p, cwd, roots):
                        found.append(f"sed -i {a}")
                        break
        elif name == "perl" and any(re.match(r"^-[A-Za-z]*i", a) for a in opts):
            for a in plain:
                for p in expand(a, cwd):
                    if os.path.isfile(resolve(p, cwd) or "") and in_repo(p, cwd, roots):
                        found.append(f"perl -i {a}")
                        break
        elif name == "tee":
            for a in plain:
                if in_repo(a, cwd, roots):
                    found.append(f"tee {a}")
        elif name in {"cp", "mv"}:
            dest = None
            for j, a in enumerate(args):
                if a == "-t" and j + 1 < len(args):
                    dest = args[j + 1]
                elif a.startswith("--target-directory="):
                    dest = a.split("=", 1)[1]
            if dest is None and len(plain) >= 2:
                dest = plain[-1]
            if dest and in_repo(dest, cwd, roots):
                found.append(f"{name} → {dest}")
        elif re.match(r"^python(\d+(\.\d+)?)?$", name):
            code = []
            if "-c" in args:
                j = args.index("-c")
                code.append(" ".join(args[j:]))
            code.extend(bodies)
            text = "\n".join(code)
            if text and PY_WRITE.search(text):
                for lit in PY_LITERAL.findall(text):
                    p = resolve(lit, cwd)
                    if p is None:
                        continue
                    exists = os.path.exists(p)
                    if (exists or ("/" in lit and os.path.isdir(os.path.dirname(p)))) and in_repo(lit, cwd, roots):
                        found.append(f"python 쓰기 {lit}")
                        break
    return found


def main():
    try:
        data = json.load(sys.stdin)
        command = data.get("tool_input", {}).get("command", "") or ""
        cwd = data.get("cwd") or os.getcwd()
        roots = repo_roots(cwd)
        if not command or not roots:
            return 0
        found = check(command, cwd, roots)
    except Exception:
        return 0
    if not found:
        return 0
    reason = (
        "저장소 파일은 Write/Edit 도구로만 고친다. Bash 로 저장소 파일을 쓰는 명령을 "
        "감지했다: " + "; ".join(dict.fromkeys(found)) + ". "
        "Write/Edit 로 다시 하라. 임시 결과는 저장소 밖(/tmp·스크래치패드)이나 "
        "git 이 무시하는 경로(build/ 등)에 쓴다. go.mod 는 go get/go mod tidy 로 고친다."
    )
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
