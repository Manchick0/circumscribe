#!/usr/bin/env node
import fs = require("node:fs");
import { dirname, isAbsolute, join, normalize, relative } from "node:path";
import { readDefinition, readExpression } from "./parser.js";
import { readConfig, type Config } from "./config.js";
import { bootstrap, STANDARD, type Scope } from "./scope.js";
import { display, pointer } from "./position.js";
import { Reader } from "./reader.js";
import type { Macro } from "./expression.js";
import type { Diagnostic } from "./diagnostic.js";
import { fileURLToPath } from "node:url";

const CIRCUMSCRIBED_NAME: RegExp = /^<(.+)>\.(.+)$/;
const DOT_CIRCUMSCRIBE: string = join(process.cwd(), ".circumscribe");
const USAGE: string = `usage: circumscribe [--help | --version] [<command>] [<args>]

Recursively substitute any expressions in all circumscribed files in the current working
directory with their replacements, as defined in '.circumscribe'.
A file is considered circumscribed if its name, that being the part before the last dot,
is surrounded by angle brackets (<example>.txt).

circumscribe            Perform the substitution as described above.

circumscribe init       Create a default .circumscribe file in the current
                        working directory.

circumscribe evaluate   Evaluate the given expression and print the substitution
                        to stdout. The expression is evaluated in the context of
                        the .circumscribe file.

circumscribe --version  Display the current circumscribe version.
                      
circumscribe --help     Print this message.`;

const DEFAULT = `def greet(name): |Hello, <trim'(name)>!|

def greeting: greet(|World|)

#
# trim'
# ---
# An implementation of the standard trim() macro
# in terms of a single match-expression.
#
def trim'(source): match /^\\s*(.+?)?\\s*$/ as content in source: content else ||
`;

function main(args: string[]): number {
    const config = readConfig();
    if (config[0]) {
        const [flags, options] = splitArgs(args.slice(1));
        if (flags.includes("help")) {
            console.log(USAGE);
            return 0;
        }
        if (flags.includes("version")) {
            const ver = version();
            if (ver) {
                console.log(ver);
                return 0;
            }
            console.error(`circumscribe: Couldn't determine the version of circumscribe`);
            return 1;
        }
        if (options.length > 0) {
            const option = options[0];
            if (option === "evaluate") {
                if (options.length > 1) {
                    const scope = loadCircumscribe();
                    if (scope[0]) {
                        const reader = new Reader("--evaluate", options[1]!);
                        const expression = readExpression(reader, 2);
                        if (expression[0]) {
                            const result = expression[0].evaluate(scope[0]);
                            if (result[0] !== undefined) {
                                console.log(result[0]);
                                return 0;
                            }
                            const diagnostic = result[1];
                            if (diagnostic.type === "source") {
                                console.error(display(diagnostic.excerpt));
                                console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                                return 1;
                            }
                            console.error(`${diagnostic.message}`);
                            return 1;
                        }
                        const diagnostic = expression[1];
                        if (diagnostic.type === "source") {
                            console.error(display(diagnostic.excerpt));
                            console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                            return 1;
                        }
                        console.error(`${diagnostic.message}`);
                        return 1;
                    }
                    const diagnostic = scope[1];
                    if (diagnostic.type === "source") {
                        console.error(display(diagnostic.excerpt));
                        console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                        return 1;
                    }
                    console.error(`${diagnostic.message}`);
                    return 1;
                }
                return 0;
            }
            if (option === "init") {
                if (fs.existsSync(DOT_CIRCUMSCRIBE) && !flags.includes("force")) {
                    console.error("circumscribe: .circumscribe already exists in the current working directory. Use '--force' to override it");
                    return 1;
                }
                fs.writeFileSync(DOT_CIRCUMSCRIBE, DEFAULT);
                return 0;
            }
            console.error(`circumscribe: Unknown option '${options[0]}'. See circumscribe --help`);
            return 1;
        }
        if (fs.existsSync(DOT_CIRCUMSCRIBE)) {
            const scope = loadCircumscribe();
            if (scope[0]) {
                const diagnostic = subsituteDirectory(normalize(join(process.cwd(), config[0].structure.root)), config[0], blacklist(config[0]), scope[0]);
                if (diagnostic) {
                    if (diagnostic.type === "source") {
                        console.error(display(diagnostic.excerpt));
                        console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                        return 1;
                    }
                    console.error(`${diagnostic.message}`);
                    return 1;
                }
                return 0;
            }
            const diagnostic = scope[1];
            if (diagnostic.type === "source") {
                console.error(display(diagnostic.excerpt));
                console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                return 1;
            }
            console.error(`${diagnostic.message}`);
            return 1;
        }
        console.error("circumscribe: Couldn't find the .circumscribe file in the current working directory. Perhaps circumscribe init?");
        return 1;
    }
    console.error(`circumscribe: Couldn't load 'circumscribe.json': ${config[1]}`);
    return 1;
}

///
/// Load the `.circumscribe` file from the
/// current working directory.
/// ---
/// Attempts to load the macros defined in the `.circumscribe` file
/// in the current working directory, collecting them in a `Scope`.
///
/// If an error occurs, the respective `Diagnostic` is returned instead.
///
function loadCircumscribe(): [Scope, undefined] | [undefined, Diagnostic] {
    const content = fs.readFileSync(DOT_CIRCUMSCRIBE, "utf8");
    const reader = new Reader(relative(process.cwd(), DOT_CIRCUMSCRIBE), content);
    const entries: Record<string, Macro> = {};
    for (;;) {
        const pattern = readDefinition(reader);
        if (pattern[1]) return [undefined, pattern[1]];
        if (pattern[0]) {
            entries[pattern[0].identifier] = pattern[0];
            continue;
        }
        break;
    }
    return [
        bootstrap({
            parent: STANDARD,
            entries: entries
        }),
        undefined
    ];
}

///
/// Recursively substitute all circumscribed files.
/// ---
/// Starting from the provided `path`, all circumscribed files
/// are recursively collected, substituted, and placed by their
/// normalized name to the corresponding directory in `build/`,
/// as specified by the config.
///
/// An entry is skipped if its absolute path appears in the
/// given `blacklist`. If `mirror` is set to true, any
/// non-circumscribed files are copied to the corresponding
/// directory as well.
///
function subsituteDirectory(path: string, config: Config, blacklist: Set<string>, scope: Scope): Diagnostic | undefined {
    const entries = fs.readdirSync(path, { withFileTypes: true });
    const { structure, mirror } = config;
    const root = normalize(join(process.cwd(), structure.root));
    for (const entry of entries) {
        const twin = normalize(join(process.cwd(), structure.build, relative(root, path)));
        const source = normalize(join(path, entry.name));
        if (blacklist.has(source)) continue;
        fs.mkdirSync(twin, { recursive: true });
        if (entry.isFile()) {
            const name = CIRCUMSCRIBED_NAME.exec(entry.name);
            if (name) {
                const diagnostic = substitute(source, normalize(join(twin, `${name[1]}.${name[2]}`)), scope);
                if (diagnostic) return diagnostic;
                continue;
            }
            if (mirror) fs.copyFileSync(source, join(twin, entry.name));
            continue;
        }
        const diagnostic = subsituteDirectory(source, config, blacklist, scope);
        if (diagnostic) return diagnostic;
    }
}

///
/// Substitute the content of the file at the given `path` and place
/// the replacement at the given `destination`.
///
function substitute(path: string, destination: string, scope: Scope): Diagnostic | undefined {
    const content = fs.readFileSync(path, "utf8");
    const reader = new Reader(relative(process.cwd(), path), content);
    const buffer = [];
    while (reader.canRead()) {
        const c = reader.peek();
        if (reader.isAt("<")) {
            const position = reader.position();
            const expression = readExpression(reader.chainRead(), 2);
            if (expression[0]) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(">")) {
                        const snippet = expression[0].evaluate(scope);
                        if (snippet[0] !== undefined) {
                            buffer.push(snippet[0]);
                            continue;
                        }
                        return snippet[1];
                    }
                    return { type: "source", excerpt: reader.excerpt(position), message: "Expected '>'" };
                }
                return { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete substitution" };
            }
            return expression[1];
        }
        buffer.push(c);
        reader.read();
    }
    fs.writeFileSync(destination, buffer.join(""));
    return undefined;
}

///
/// Compute the 'blacklist' from the given config.
/// ---
/// Each of the 'ignore' glob patterns is resolved against
/// the 'root' directory, as defined by the config.
/// The entries are then normalized, accumulated in a set,
/// and returned.
///
function blacklist(config: Config): Set<string> {
    const buffer = new Set<string>();
    const root = normalize(join(process.cwd(), config.structure.root));
    for (const pattern of config.structure.ignore) {
        const entries = fs.globSync(pattern, { cwd: root });
        for (const entry of entries) buffer.add(normalize(join(root, normalize(entry))));
    }
    return buffer;
}

///
/// Determine the version of circumscribe.
/// ---
/// Starting from the directory of the scriot,
/// the file tree is traversed upwards until a
/// `package.json` is found.
///
function version(path: string = dirname(fileURLToPath(import.meta.url))): string | undefined {
    const ver = join(path, "package.json");
    if (fs.existsSync(ver)) {
        const content = fs.readFileSync(ver, "utf8");
        const { name, version } = JSON.parse(content);
        return `${name} ${version}`;
    }
    if (dirname(path) === path) return undefined;
    return version(dirname(path));
}

function splitArgs(args: string[]): [flags: string[], options: string[]] {
    const flags: string[] = [];
    const options: string[] = [];
    for (const arg of args) {
        if (arg.startsWith("--")) {
            flags.push(arg.substring(2));
            continue;
        }
        options.push(arg);
    }
    return [flags, options];
}

process.exit(main(process.argv.slice(1)));
