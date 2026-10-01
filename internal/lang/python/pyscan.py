"""pyscan prints the scan facts for one Python project as JSON on stdout.

Usage: pyscan.py <project-root> [<subdir-to-scan>]

One module per .py file; a package's __init__.py is the package itself.
Imports are resolved against the project's own modules, so `from a import b`
points at a.b when that is a module and at a otherwise. Standard library only.
"""
import ast
import json
import os
import sys

SKIP_DIRS = {
    ".git", ".hg", ".venv", "venv", "env", ".env", "node_modules", "__pycache__",
    ".tox", ".nox", ".mypy_cache", ".pytest_cache", ".ruff_cache", "build", "dist",
    "site-packages", ".eggs", "tests", "test", "testing", ".uml-viewer", ".metrics",
}


def is_test_file(name):
    return (name.startswith("test_") or name.endswith("_test.py")
            or name == "conftest.py")


def python_files(scan_dir):
    for dirpath, dirnames, filenames in os.walk(scan_dir):
        dirnames[:] = sorted(d for d in dirnames
                             if d not in SKIP_DIRS and not d.endswith(".egg-info"))
        for name in sorted(filenames):
            if name.endswith(".py") and not is_test_file(name):
                yield os.path.join(dirpath, name)


def dotted_name(path):
    """Module name: climb while the directory is a package (has __init__.py)."""
    directory, filename = os.path.split(path)
    parts = [] if filename == "__init__.py" else [filename[:-3]]
    while os.path.isfile(os.path.join(directory, "__init__.py")):
        directory, package = os.path.split(directory)
        parts.insert(0, package)
    return ".".join(parts)


def stdlib_names():
    return getattr(sys, "stdlib_module_names", frozenset())


class Complexity(ast.NodeVisitor):
    """1 for the function, plus each branch, loop, handler, and short circuit."""

    def __init__(self):
        self.n = 1

    def generic_visit(self, node):
        if isinstance(node, (ast.If, ast.For, ast.AsyncFor, ast.While,
                             ast.ExceptHandler, ast.IfExp)):
            self.n += 1
        elif isinstance(node, ast.BoolOp):
            self.n += len(node.values) - 1
        elif isinstance(node, ast.comprehension):
            self.n += 1 + len(node.ifs)
        elif node.__class__.__name__ == "match_case":
            wildcard = (node.pattern.__class__.__name__ == "MatchAs"
                        and node.pattern.pattern is None)
            if not wildcard:
                self.n += 1
        super().generic_visit(node)


def complexity(fn):
    counter = Complexity()
    for child in ast.iter_child_nodes(fn):
        counter.visit(child)
    return counter.n


def cover_start(fn):
    """First line that runs when the function is called, not when it is defined.

    The `def` line executes at import, so counting it would credit an uncalled
    function with coverage. A leading docstring is not a statement to coverage.
    """
    body = fn.body
    if (len(body) > 1 and isinstance(body[0], ast.Expr)
            and isinstance(getattr(body[0], "value", None), ast.Constant)
            and isinstance(body[0].value.value, str)):
        body = body[1:]
    return body[0].lineno


def functions(tree, rel):
    out = []

    def walk(node, owner):
        for child in ast.iter_child_nodes(node):
            if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef)):
                out.append({
                    "name": ".".join(owner + [child.name]),
                    "file": rel,
                    "start": child.lineno,
                    "end": child.end_lineno,
                    "cover_start": cover_start(child),
                    "cc": complexity(child),
                })
            elif isinstance(child, ast.ClassDef):
                walk(child, owner + [child.name])
            elif not isinstance(child, (ast.Lambda,)):
                walk(child, owner)

    walk(tree, [])
    return out


def raw_imports(tree, module, is_package):
    """(absolute dotted base, [names]) for every import in the file."""
    package = module if is_package else module.rpartition(".")[0]
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                yield alias.name, []
        elif isinstance(node, ast.ImportFrom):
            base = node.module or ""
            if node.level:
                parts = package.split(".") if package else []
                parts = parts[:len(parts) - (node.level - 1)]
                base = ".".join(parts + ([base] if base else []))
            yield base, [a.name for a in node.names]


def longest_project_prefix(name, modules):
    parts = name.split(".")
    while parts:
        candidate = ".".join(parts)
        if candidate in modules:
            return candidate
        parts.pop()
    return None


def resolve(base, names, modules):
    """Project modules an import statement reaches."""
    hits = set()
    for n in names:
        full = base + "." + n if base else n
        if full in modules:
            hits.add(full)
    if len(hits) < max(1, len(names)):
        owner = longest_project_prefix(base, modules) if base else None
        if owner:
            hits.add(owner)
    return hits


def main():
    if len(sys.argv) < 2:
        sys.exit("usage: pyscan.py <project-root> [<subdir>]")
    root = os.path.abspath(sys.argv[1])
    scan_dir = os.path.join(root, sys.argv[2]) if len(sys.argv) > 2 else root

    parsed = {}
    for path in python_files(scan_dir):
        try:
            with open(path, encoding="utf-8", errors="replace") as f:
                tree = ast.parse(f.read(), filename=path)
        except (SyntaxError, ValueError):
            continue
        name = dotted_name(path)
        if name and name not in parsed:
            parsed[name] = (path, tree)

    # Scripts outside every package are entry points, not architecture. A
    # project with no packages at all keeps everything.
    notes = []
    packaged = {name for name, (path, _) in parsed.items()
                if "." in name or os.path.basename(path) == "__init__.py"}
    if packaged and len(packaged) < len(parsed):
        loose = len(parsed) - len(packaged)
        notes.append("Left out %d script%s outside any package."
                     % (loose, "" if loose == 1 else "s"))
        parsed = {name: parsed[name] for name in packaged}

    tops = {name.split(".")[0] for name in parsed}
    prefix = next(iter(tops)) if len(tops) == 1 else ""
    std = stdlib_names()

    modules = []
    for name in sorted(parsed):
        path, tree = parsed[name]
        rel = os.path.relpath(path, root).replace(os.sep, "/")
        is_package = os.path.basename(path) == "__init__.py"
        imports, seen = [], set()
        for base, names in raw_imports(tree, name, is_package):
            targets = resolve(base, names, parsed)
            if targets:
                for t in sorted(targets):
                    if t != name and ("p", t) not in seen:
                        seen.add(("p", t))
                        target = os.path.relpath(parsed[t][0], root).replace(os.sep, "/")
                        imports.append({"to": target, "project": True})
            elif base:
                top = base.split(".")[0]
                if ("f", top) not in seen:
                    seen.add(("f", top))
                    imp = {"to": top, "project": False, "module": top}
                    if top in std:
                        imp["std"] = True
                    imports.append(imp)
        tail = name[len(prefix) + 1:] if prefix and name != prefix else (
            "" if name == prefix else name)
        modules.append({
            "ns": rel,
            "path": tail.replace(".", "/"),
            "name": name.split(".")[-1],
            "file": rel,
            "imports": imports,
            "functions": functions(tree, rel),
        })

    json.dump({"lang": "python", "prefix": prefix, "modules": modules,
               "notes": notes}, sys.stdout)


if __name__ == "__main__":
    main()
