#!/usr/bin/env python3
"""Leak-safe artifact redaction scanners for podlaz E2E jobs."""

from __future__ import annotations

import base64
import os
import re
import sys
import urllib.parse
from collections.abc import Iterable

UUID_RE = re.compile(
    r"(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b"
)
SECRET_QUERY_KEYS = {
    "id",
    "uuid",
    "password",
    "passwd",
    "pass",
    "token",
    "access_token",
    "auth",
    "authorization",
    "secret",
}
COMMON_DERIVED_VALUES = {
    "tcp",
    "udp",
    "tls",
    "none",
    "auto",
    "http",
    "https",
    "socks",
    "chrome",
    "firefox",
    "reality",
    "grpc",
    "ws",
    "httpupgrade",
}
NOISY_PATH_PARTS = {
    "api",
    "client",
    "clients",
    "config",
    "configs",
    "download",
    "feed",
    "feeds",
    "import",
    "link",
    "links",
    "node",
    "nodes",
    "profile",
    "profiles",
    "proxy",
    "proxies",
    "sub",
    "subs",
    "subscription",
    "subscriptions",
    "user",
    "users",
    "vless",
    "vmess",
    "vpn",
    "xray",
}


class SensitiveNeedles:
    def __init__(self) -> None:
        self.values: list[bytes] = []
        self._seen: set[bytes] = set()

    def add(self, value: str | bytes, *, derived: bool = False) -> None:
        if isinstance(value, bytes):
            raw = value.strip()
            text = raw.decode("utf-8", "ignore")
        else:
            text = str(value).strip()
            raw = text.encode("utf-8")
        if not raw:
            return
        if derived and not useful_derived_value(text):
            return
        if raw in self._seen:
            return
        self._seen.add(raw)
        self.values.append(raw)


def useful_derived_value(text: str) -> bool:
    lowered = text.lower()
    if lowered in COMMON_DERIVED_VALUES:
        return False
    if len(text) < 8 and not UUID_RE.fullmatch(text):
        return False
    return True


def useful_path_part(part: str) -> bool:
    if not useful_derived_value(part):
        return False
    lowered = part.lower()
    if lowered in NOISY_PATH_PARTS:
        return False
    if lowered in {".", ".."}:
        return False
    if re.fullmatch(r"[a-z]+", part):
        return False
    if re.fullmatch(r"\d+", part):
        return False
    return True


def maybe_base64_decode(token: str) -> list[str]:
    token = urllib.parse.unquote(token).strip()
    if len(token) < 8 or not re.fullmatch(r"[A-Za-z0-9+/=_-]+", token):
        return []
    normalized = token.replace("-", "+").replace("_", "/")
    normalized += "=" * ((4 - len(normalized) % 4) % 4)
    try:
        decoded = base64.b64decode(normalized, validate=False)
    except Exception:
        return []
    try:
        return [decoded.decode("utf-8")]
    except UnicodeDecodeError:
        return []


def add_decoded_sensitive_parts(needles: SensitiveNeedles, text: str) -> None:
    needles.add(text, derived=True)
    for match in UUID_RE.finditer(text):
        needles.add(match.group(0), derived=True)
    if ":" in text:
        needles.add(text.rsplit(":", 1)[1], derived=True)


def add_path_parts(needles: SensitiveNeedles, path: str) -> None:
    for raw_part in path.split("/"):
        part = urllib.parse.unquote(raw_part).strip()
        if not useful_path_part(part):
            continue
        add_decoded_sensitive_parts(needles, part)
        for decoded in maybe_base64_decode(part):
            add_decoded_sensitive_parts(needles, decoded)


def extract_sensitive_fragments(needles: SensitiveNeedles, line: str) -> None:
    stripped = line.strip()
    if not stripped:
        return

    for match in UUID_RE.finditer(stripped):
        needles.add(match.group(0), derived=True)

    if stripped.lower().startswith("authorization:"):
        token = stripped.split(":", 1)[1].strip()
        needles.add(token, derived=True)
        parts = token.split(None, 1)
        if len(parts) == 2:
            needles.add(parts[1], derived=True)

    try:
        parsed = urllib.parse.urlsplit(stripped)
    except ValueError:
        return
    if not parsed.scheme:
        return

    scheme = parsed.scheme.lower()
    if "@" in parsed.netloc:
        raw_userinfo = parsed.netloc.rsplit("@", 1)[0]
        decoded_userinfo = urllib.parse.unquote(raw_userinfo)
        add_decoded_sensitive_parts(needles, decoded_userinfo)
        for decoded in maybe_base64_decode(raw_userinfo):
            add_decoded_sensitive_parts(needles, decoded)
        for decoded in maybe_base64_decode(decoded_userinfo):
            add_decoded_sensitive_parts(needles, decoded)

    if scheme in {"http", "https"}:
        add_path_parts(needles, parsed.path)

    if scheme in {"vmess", "ss"}:
        opaque = stripped.split("://", 1)[1].split("#", 1)[0].split("?", 1)[0]
        userinfo = opaque.split("@", 1)[0].strip("/")
        for decoded in maybe_base64_decode(userinfo):
            add_decoded_sensitive_parts(needles, decoded)

    for key, vals in urllib.parse.parse_qs(parsed.query, keep_blank_values=False).items():
        if key.lower() not in SECRET_QUERY_KEYS:
            continue
        for value in vals:
            add_decoded_sensitive_parts(needles, urllib.parse.unquote(value))


def collect_sensitive_needles(values: Iterable[str]) -> SensitiveNeedles:
    needles = SensitiveNeedles()
    for value in values:
        if not value:
            continue
        if "\n" not in value:
            needles.add(value)
        for line in value.splitlines():
            if not line:
                continue
            needles.add(line)
            extract_sensitive_fragments(needles, line)
    return needles


def artifact_paths(artifact_dir: str, report: str) -> list[str]:
    report_abs = os.path.abspath(report)
    paths: list[str] = []
    for root, _, files in os.walk(artifact_dir):
        for name in files:
            path = os.path.join(root, name)
            if os.path.abspath(path) == report_abs:
                continue
            paths.append(path)
    return paths


def scan_sensitive_values(artifact_dir: str, report: str, values: list[str]) -> int:
    needles = collect_sensitive_needles(values)
    leaks: set[str] = set()
    if needles.values:
        for path in artifact_paths(artifact_dir, report):
            try:
                with open(path, "rb") as handle:
                    data = handle.read()
            except OSError:
                continue
            if any(needle in data for needle in needles.values):
                leaks.add(path)

    with open(report, "w", encoding="utf-8") as handle:
        if leaks:
            for path in sorted(leaks):
                handle.write(f"{path}\n")
        else:
            handle.write(f"No configured sensitive values were found in {artifact_dir}\n")

    if leaks:
        for path in sorted(leaks):
            print(f"redaction leak file: {path}", file=sys.stderr)
        return 1
    return 0


def scan_file_contents(artifact_dir: str, report: str, sources: list[str]) -> int:
    errors: list[str] = []
    leaks: set[str] = set()
    artifacts = artifact_paths(artifact_dir, report)

    for source in sources:
        source_abs = os.path.abspath(source)
        if not os.path.isfile(source_abs):
            errors.append(f"missing generated-content source: {source}")
            continue
        try:
            with open(source_abs, "rb") as handle:
                needle = handle.read()
        except OSError as exc:
            errors.append(f"unreadable generated-content source: {source}: {exc}")
            continue
        if len(needle) < 64:
            errors.append(f"generated-content source too small to scan safely: {source}")
            continue
        for path in artifacts:
            if os.path.abspath(path) == source_abs:
                continue
            try:
                with open(path, "rb") as handle:
                    data = handle.read()
            except OSError:
                continue
            if needle in data:
                leaks.add(path)

    with open(report, "w", encoding="utf-8") as handle:
        for error in errors:
            handle.write(f"{error}\n")
        for path in sorted(leaks):
            handle.write(f"generated-content leak file: {path}\n")
        if not errors and not leaks:
            handle.write(f"No generated content sources were found in {artifact_dir}\n")

    if errors or leaks:
        for error in errors:
            print(f"redaction scan error: {error}", file=sys.stderr)
        for path in sorted(leaks):
            print(f"generated-content leak file: {path}", file=sys.stderr)
        return 1
    return 0


def main(argv: list[str]) -> int:
    if len(argv) < 4:
        print("usage: redaction_scan.py <sensitive-values|file-contents> <artifact-dir> <report> [values...]", file=sys.stderr)
        return 2
    mode, artifact_dir, report, *items = argv[1:]
    if mode == "sensitive-values":
        return scan_sensitive_values(artifact_dir, report, items)
    if mode == "file-contents":
        return scan_file_contents(artifact_dir, report, items)
    print(f"unknown redaction scan mode: {mode}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
