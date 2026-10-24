// tsscan prints the scan facts for one TypeScript/JavaScript project as JSON.
//
// Usage: node tsscan.js <project-root>
//
// One module per source file; index.* is its directory. Imports are resolved
// with TypeScript's own resolver against the nearest tsconfig, so path aliases
// and ESM-style "./x.js" specifiers land on the right file. Nothing needs to
// be installed in the project: a bare import is third-party by its name alone.
//
// TypeScript arrives as `ts`, defined by the wrapper umlv pipes in ahead of
// this script (see typescript.go). Nothing is looked up in node_modules.
'use strict';
const fs = require('fs');
const path = require('path');
const { builtinModules } = require('module');

const SOURCE = /\.(ts|tsx|mts|cts|js|jsx|mjs|cjs)$/;
const SKIP_DIRS = new Set([
  'node_modules', 'dist', 'build', 'out', 'coverage', 'vendor', 'tmp', 'temp',
  '__tests__', '__mocks__', '__fixtures__', 'test', 'tests', 'e2e', 'fixtures',
  'mutants', 'reports',
]);
const SKIP_FILE = /(\.d\.[cm]?ts$)|(\.(test|spec|stories|config)\.[^.]+$)|(^\..+rc\.[cm]?js$)/;

function sourceFiles(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (!entry.name.startsWith('.') && !SKIP_DIRS.has(entry.name)) sourceFiles(full, out);
    } else if (SOURCE.test(entry.name) && !SKIP_FILE.test(entry.name)) {
      out.push(full);
    }
  }
  return out.sort();
}

// Tree path of a file: no extension, no "src" level, index is its directory.
function treePath(rel) {
  const parts = rel.replace(SOURCE, '').split('/').filter((p) => p !== 'src');
  if (parts[parts.length - 1] === 'index') parts.pop(); // the root index is the project itself
  return parts.join('/');
}

function main() {
  const root = fs.realpathSync(path.resolve(process.argv[2] || '.'));
  const rel = (file) => path.relative(root, file).split(path.sep).join('/');

  const files = sourceFiles(root);
  const known = new Set(files);

  // Compiler options of the nearest tsconfig, cached per directory.
  const optionCache = new Map();
  const fallback = {
    allowJs: true,
    moduleResolution: ts.ModuleResolutionKind.Bundler || ts.ModuleResolutionKind.NodeJs,
  };
  function optionsFor(dir) {
    if (optionCache.has(dir)) return optionCache.get(dir);
    let options = fallback;
    const config = ts.findConfigFile(dir, ts.sys.fileExists, 'tsconfig.json');
    if (config && config.startsWith(root)) {
      const read = ts.readConfigFile(config, ts.sys.readFile);
      if (!read.error) {
        const parsed = ts.parseJsonConfigFileContent(read.config, ts.sys, path.dirname(config));
        options = { ...parsed.options, allowJs: true };
        if (options.moduleResolution === undefined) options.moduleResolution = fallback.moduleResolution;
      }
    }
    optionCache.set(dir, options);
    return options;
  }

  // Workspace packages by name, so "@scope/pkg" finds its source entry.
  const workspaces = new Map();
  (function findPackages(dir, depth) {
    if (depth > 4) return;
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isDirectory() || entry.name.startsWith('.') || entry.name === 'node_modules') continue;
      const sub = path.join(dir, entry.name);
      const manifest = path.join(sub, 'package.json');
      if (fs.existsSync(manifest)) {
        try {
          const name = JSON.parse(fs.readFileSync(manifest, 'utf8')).name;
          if (name) workspaces.set(name, sub);
        } catch (_) { /* not a readable manifest */ }
      }
      findPackages(sub, depth + 1);
    }
  })(root, 0);

  function firstKnown(bases) {
    for (const base of bases) {
      for (const ext of ['', '.ts', '.tsx', '.mts', '.cts', '.js', '.jsx', '.mjs', '.cjs']) {
        if (known.has(base + ext)) return base + ext;
      }
    }
    return null;
  }

  function workspaceEntry(spec) {
    for (const [name, dir] of workspaces) {
      if (spec === name) {
        return firstKnown([path.join(dir, 'src', 'index'), path.join(dir, 'index'),
          path.join(dir, 'src', 'main'), path.join(dir, 'lib', 'index')]);
      }
      if (spec.startsWith(name + '/')) {
        const tail = spec.slice(name.length + 1);
        return firstKnown([path.join(dir, 'src', tail), path.join(dir, tail),
          path.join(dir, 'src', tail, 'index'), path.join(dir, tail, 'index')]);
      }
    }
    return null;
  }

  function resolveImport(spec, from) {
    const hit = ts.resolveModuleName(spec, from, optionsFor(path.dirname(from)), ts.sys).resolvedModule;
    if (hit && !hit.isExternalLibraryImport) {
      const file = fs.existsSync(hit.resolvedFileName) ? fs.realpathSync(hit.resolvedFileName) : hit.resolvedFileName;
      if (known.has(file)) return file;
    }
    if (spec.startsWith('.')) {
      const base = path.resolve(path.dirname(from), spec);
      const bare = base.replace(/\.[cm]?jsx?$/, '');
      return firstKnown([base, bare, path.join(base, 'index'), path.join(bare, 'index')]);
    }
    return workspaceEntry(spec);
  }

  function packageName(spec) {
    const parts = spec.split('/');
    return spec.startsWith('@') ? parts.slice(0, 2).join('/') : parts[0];
  }

  const builtins = new Set(builtinModules);

  function importsOf(file, text) {
    const out = [];
    const seen = new Set();
    for (const { fileName: spec } of ts.preProcessFile(text, true, true).importedFiles) {
      const target = resolveImport(spec, file);
      if (target) {
        if (target !== file && !seen.has(target)) {
          seen.add(target);
          out.push({ to: rel(target), project: true });
        }
      } else if (!spec.startsWith('.') && !spec.startsWith('/') && !/^[@~#]\//.test(spec)) {
        const bare = spec.replace(/^node:/, '');
        const std = spec.startsWith('node:') || builtins.has(bare);
        const name = std ? 'node:' + bare.split('/')[0] : packageName(spec);
        if (!seen.has(name)) {
          seen.add(name);
          out.push(std ? { to: name, project: false, std: true, module: name }
            : { to: name, project: false, module: name });
        }
      }
    }
    return out;
  }

  // 1 for the function, plus each branch, loop, case, catch, and short circuit.
  function complexity(fn) {
    const K = ts.SyntaxKind;
    let n = 1;
    (function visit(node) {
      switch (node.kind) {
        case K.IfStatement: case K.ConditionalExpression: case K.ForStatement:
        case K.ForInStatement: case K.ForOfStatement: case K.WhileStatement:
        case K.DoStatement: case K.CatchClause: case K.CaseClause:
          n++;
          break;
        case K.BinaryExpression: {
          const op = node.operatorToken.kind;
          if (op === K.AmpersandAmpersandToken || op === K.BarBarToken || op === K.QuestionQuestionToken) n++;
          break;
        }
        default:
      }
      ts.forEachChild(node, visit);
    })(fn);
    return n;
  }

  function functionsOf(file, text) {
    const K = ts.SyntaxKind;
    const sf = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
    const out = [];
    const isFn = (n) => n && (n.kind === K.ArrowFunction || n.kind === K.FunctionExpression);
    const nameOf = (n) => (n && n.name && n.name.getText(sf)) || null;
    const flags = (n) => ts.getCombinedModifierFlags(n);

    // Top-level names exported by a statement of their own: `export { a, b as c }`,
    // `export default a`, or CommonJS `module.exports = { a }` and `exports.b = b`.
    const exportedNames = new Set();
    const addName = (n) => { if (n && n.kind === K.Identifier) exportedNames.add(n.text); };
    const isExports = (n) => /^(module\.exports|(module\.)?exports\.[\w$]+)$/.test(n.getText(sf));
    for (const st of sf.statements) {
      if (st.kind === K.ExportDeclaration && !st.moduleSpecifier && st.exportClause && st.exportClause.kind === K.NamedExports) {
        for (const el of st.exportClause.elements) exportedNames.add((el.propertyName || el.name).text);
      } else if (st.kind === K.ExportAssignment) {
        addName(st.expression);
      } else if (st.kind === K.ExpressionStatement && st.expression.kind === K.BinaryExpression &&
          st.expression.operatorToken.kind === K.EqualsToken && isExports(st.expression.left)) {
        const value = st.expression.right;
        addName(value);
        for (const p of value.kind === K.ObjectLiteralExpression ? value.properties : []) {
          addName(p.kind === K.ShorthandPropertyAssignment ? p.name : p.initializer);
        }
      }
    }
    // Whether a declaration is exported: by its own `export`, or, at the top
    // level, by name in one of the statements above.
    const top = (n) => n.parent === sf || (n.parent.parent.kind === K.VariableStatement && n.parent.parent.parent === sf);
    const isExported = (n) => (flags(n) & ts.ModifierFlags.Export) !== 0 ||
      (top(n) && n.name !== undefined && exportedNames.has(n.name.getText(sf)));
    // A private, protected or #private class member stays inside the class.
    const hidden = (member) => (flags(member) & (ts.ModifierFlags.Private | ts.ModifierFlags.Protected)) !== 0 ||
      (member.name !== undefined && member.name.kind === K.PrivateIdentifier);

    function record(name, fn, exported) {
      if (!fn.body) return; // an overload or an abstract method
      const start = sf.getLineAndCharacterOfPosition(fn.getStart(sf));
      const end = sf.getLineAndCharacterOfPosition(fn.getEnd());
      // Coverage counts from the body. `const f = () => ...` is itself a
      // statement that runs at import, on the same line as a one-line body.
      const body = sf.getLineAndCharacterOfPosition(fn.body.getStart(sf));
      out.push({
        name, file: rel(file), start: start.line + 1, end: end.line + 1,
        cover_start: body.line + 1, cover_col: body.character, cc: complexity(fn), exported,
      });
    }

    // The function a class or object-literal member defines, or null.
    function memberFn(member) {
      if (member.kind === K.MethodDeclaration || member.kind === K.GetAccessor || member.kind === K.SetAccessor) return member;
      if ((member.kind === K.PropertyDeclaration || member.kind === K.PropertyAssignment) && isFn(member.initializer)) {
        return member.initializer;
      }
      return null;
    }

    // An object literal seen through `satisfies`, `as`, and parentheses.
    function objectOf(n) {
      while (n && (n.kind === K.SatisfiesExpression || n.kind === K.AsExpression || n.kind === K.ParenthesizedExpression)) {
        n = n.expression;
      }
      return n && n.kind === K.ObjectLiteralExpression ? n : null;
    }

    // Members of an exported class or object are exported unless hidden.
    function recordMembers(owner, members, exported) {
      for (const member of members) {
        const open = exported && !hidden(member);
        if (member.kind === K.Constructor) record([...owner, 'constructor'].join('.'), member, open);
        else if (memberFn(member)) record([...owner, nameOf(member)].join('.'), memberFn(member), open);
      }
    }

    (function visit(node, owner) {
      if (node.kind === K.FunctionDeclaration) {
        record([...owner, nameOf(node) || 'default'].join('.'), node, isExported(node));
        return; // nested functions belong to their parent
      }
      if (node.kind === K.ClassDeclaration || node.kind === K.ClassExpression) {
        const exported = node.kind === K.ClassDeclaration && isExported(node);
        recordMembers([...owner, nameOf(node) || 'default'], node.members, exported);
        return;
      }
      if (node.kind === K.VariableDeclaration && node.name.kind === K.Identifier) {
        if (isFn(node.initializer)) {
          record([...owner, node.name.text].join('.'), node.initializer, isExported(node));
          return;
        }
        if (objectOf(node.initializer)) {
          recordMembers([...owner, node.name.text], objectOf(node.initializer).properties, isExported(node));
          return;
        }
      }
      if (node.kind === K.ExportAssignment) {
        if (isFn(node.expression)) {
          record('default', node.expression, true);
          return;
        }
        if (objectOf(node.expression)) {
          recordMembers(['default'], objectOf(node.expression).properties, true);
          return;
        }
      }
      ts.forEachChild(node, (child) => visit(child, owner));
    })(sf, []);
    return out;
  }

  const taken = new Set();
  const modules = files.map((file) => {
    const text = fs.readFileSync(file, 'utf8');
    let tree = treePath(rel(file));
    if (taken.has(tree)) tree = rel(file).replace(SOURCE, ''); // foo.ts beside foo/index.ts
    taken.add(tree);
    return {
      ns: rel(file), path: tree, name: path.basename(tree) || path.basename(root),
      file: rel(file), imports: importsOf(file, text), functions: functionsOf(file, text),
    };
  });

  // A single shared top level (usually "packages" or "apps") adds nothing.
  const tops = new Set(modules.map((m) => m.path.split('/')[0]));
  let prefix = '';
  if (tops.size === 1 && modules.every((m) => m.path.includes('/'))) {
    prefix = [...tops][0];
    for (const m of modules) m.path = m.path.slice(prefix.length + 1);
  }

  process.stdout.write(JSON.stringify({ lang: 'typescript', prefix, modules, notes: [] }));
}

main();
