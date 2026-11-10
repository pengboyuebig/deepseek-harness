#!/usr/bin/env python3
"""Deterministic GLKJ brand transform for the dsh/glh harness tree.

Derived from the 12 brand commits on glkj/glkj_main (0.1.0-rc.5) so the same
brand layer can be replayed on top of any upstream master revision.

Policy (matches the 0.1 brand branch):
  * rename the user-facing command `dsh` -> `glh`, env prefix `GLH_` -> `GLH_`,
    harness home `.dsh` -> `.glh`, and the DeepSeek API key/base defaults;
  * rebrand product copy and endpoints to GLKJ;
  * keep everything the runtime reads verbatim: the `@deepseek-ai/dsh-*` package
    namespace, `"dsh": {...}` manifest keys, `dsh.<field>` keys, `dshHome`
    identifiers, and model/provider ids such as `deepseek-chat`.

Usage:
  python3 glkj-brand.py [--dry-run] [root]
"""
import os
import re
import sys

SKIP_DIRS = {".git", "node_modules", "dist", "lib", "coverage", ".turbo", ".cache"}
SKIP_EXT = {
    ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz",
    ".tgz", ".woff", ".woff2", ".ttf", ".wasm", ".bin", ".so", ".dylib", ".exe",
    ".lock", ".snap",
}
DOC_EXT = {".md", ".txt"}

# Files whose `dsh` tokens are package-manifest fields or build-layout names the
# 0.1 brand branch deliberately kept (renaming them breaks the loader).
KEEP_DSH_PATHS = (
    "packages/util/plugin-market-index/src/",
    "apps/desktop/src/main.ts",
    "apps/desktop/tests/desktop-build-paths.spec.ts",
    "packages/boot/plugin-manager/src/versions.ts",
)

# Command-name literals the 0.1 brand branch renamed; every other quoted `dsh`
# is a manifest key or a runtime directory name and must stay.
COMMAND_LITERALS = (
    (re.compile(r"loadLayeredEnv\(['\"`]dsh['\"`]\)"), "loadLayeredEnv('glh')"),
    (re.compile(r"\.name\(['\"`]dsh['\"`]\)"), ".name('glh')"),
)

# Applied to every text file, in order.
RULES = [
    # --- env vars -----------------------------------------------------------
    (r"GLKJ_API_KEY_EXTERNAL", "GLKJ_API_KEY_EXTERNAL"),
    (r"GLKJ_API_KEY", "GLKJ_API_KEY"),
    (r"GLKJ_BASE_URL", "GLKJ_BASE_URL"),
    (r"GLKJ_SEARCH_BASE_URL", "GLKJ_SEARCH_BASE_URL"),
    (r"GLKJ_PROVIDER_ID", "GLKJ_PROVIDER_ID"),
    (r"GLKJ_DEFAULT_([A-Z_]+)", r"GLKJ_DEFAULT_\1"),
    (r"DEEPSEEK_([A-Z0-9_]+)", r"GLKJ_\1"),
    (r"\bDSH_([A-Z0-9_]*)", r"GLH_\1"),
    (r"\bDSH-owned\b", "GLH-owned"),
    # --- harness home, command name and string tokens -----------------------
    (r"~/\.dsh\b", "~/.glh"),
    (r"\.glh/", ".glh/"),
    (r"'\.dsh'", "'.glh'"),
    (r'"\.dsh"', '".glh"'),
    (r"pnpm glh\b", "pnpm glh"),
    (r'"dsh": "node --import tsx/esm apps/cli/src/bin\.ts"',
     '"glh": "node --import tsx/esm apps/cli/src/bin.ts"'),
    (r'"dsh": "lib/bin\.js"', '"glh": "lib/bin.js"'),
    (r"npx @deepseek-ai/dsh\b", "npx @deepseek-ai/dsh"),
    (r"\bdsh web", "glh web"),
    (r"\bdsh --profile", "glh --profile"),
    (r"\bdsh --dump-config", "glh --dump-config"),
    (r"\bdsh-issue-policy\b", "glkj-issue-policy"),
    (r"\bdsh-pre-push-checks\b", "glh-pre-push-checks"),
    (r"refs/glh/", "refs/glh/"),
    # A bare quoted command token: 'dsh', "dsh", `dsh`.
    # Help/error labels that start a quoted sentence: 'glh: boot a profile'.
    (r"(?<=['\"`])dsh(?=: )", "glh"),
    # Labels after an escaped newline or at the start of a line: 
glh: warning.
    (r"\\ndsh(?=: )", "\\nglh"),
    (r"(?m)^dsh(?=: )", "glh"),
    (r'"short_name": "GLKJ"', '"short_name": "GLKJ"'),
    # --- endpoints and repo URLs -------------------------------------------
    # package.json `repository.url` stays on the upstream repo (as the 0.1 brand
    # branch did) so npm metadata keeps pointing at the source project.
    (r"git\+https://github\.com/deepseek-ai/deepseek-harness\.git",
     "\x00KEEP_REPO_URL\x00"),
    (r"https://api\.deepseek\.com", "https://api.glkj.example.com"),
    (r"https://www\.deepseek\.com", "https://www.glkj.example.com"),
    (r"https://deepseek\.com", "https://glkj.example.com"),
    (r"https://deepseek\.com/harness", "https://glkj.example.com/harness"),
    (r"https://github\.com/deepseek-ai/deepseek-harness\.git",
     "https://git.glkj.example.com/glkj/harness.git"),
    (r"https://github\.com/deepseek-ai/deepseek-harness",
     "https://git.glkj.example.com/glkj/harness"),
    # --- brand copy and identifiers ----------------------------------------
    ("GLKJ Harness", "GLKJ Harness"),
    (r"\bDeepSeekOnboardingDialogProps\b", "GlkjOnboardingDialogProps"),
    (r"\bDeepSeekOnboardingDialog\b", "GlkjOnboardingDialog"),
    (r"\bdeepSeekCopy\b", "glkjCopy"),
    (r"\bFishLogo\b", "BrandMark"),
    (r"powered_by-glh-4D6BFE\?style=flat-square&logo=deepseek&logoColor=white",
     "powered_by-glh-4D6BFE?style=flat-square"),
    (r"powered_by-glh\b", "powered_by-glh"),
    # --- china mirror for the python sdk lock --------------------------------
    (r"https://pypi\.org/simple", "https://pypi.tuna.tsinghua.edu.cn/simple"),
]

# Extra prose rule for documentation only: a bare `dsh` word is the command.
DOC_DSH = re.compile(
    r"(?<![\w@/.-])dsh(?![\w:/.-])")

COMPILED = [(re.compile(p), r) for p, r in RULES]


def transform(text: str, path: str = "") -> str:
    for rx, repl in COMPILED:
        text = rx.sub(repl, text)
    text = text.replace("\x00KEEP_REPO_URL\x00",
                        "git+https://github.com/deepseek-ai/deepseek-harness.git")
    ext = os.path.splitext(path)[1].lower()
    keep = any(p in path.replace(os.sep, "/") for p in KEEP_DSH_PATHS)
    if not keep:
        for rx, repl in COMMAND_LITERALS:
            text = rx.sub(repl, text)
        if ext in DOC_EXT:
            text = DOC_DSH.sub("glh", text)
    return text


def iter_files(root):
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for name in filenames:
            path = os.path.join(dirpath, name)
            if os.path.splitext(name)[1].lower() in SKIP_EXT:
                continue
            yield path


def main(argv):
    dry = "--dry-run" in argv
    args = [a for a in argv[1:] if not a.startswith("--")]
    root = args[0] if args else "."
    changed = 0
    for path in iter_files(root):
        try:
            with open(path, "r", encoding="utf-8") as fh:
                old = fh.read()
        except (UnicodeDecodeError, OSError):
            continue
        new = transform(old, path)
        if new != old:
            changed += 1
            print(("would update " if dry else "update ") + os.path.relpath(path, root))
            if not dry:
                with open(path, "w", encoding="utf-8") as fh:
                    fh.write(new)
    print(f"{'would change' if dry else 'changed'} {changed} files")


if __name__ == "__main__":
    main(sys.argv)
