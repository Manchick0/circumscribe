import fs from "node:fs";
import { join } from "node:path";

const PATH: string = join(process.cwd(), "circumscribe.json");
const DEFAULT: Config = {
    structure: {
        root: process.cwd(),
        build: process.cwd(),
        ignore: []
    },
    mirror: false
};

export type Config = {
    readonly structure: {
        readonly root: string;
        readonly build: string;
        readonly ignore: string[];
    };
    readonly mirror: boolean;
};

export function readConfig(): [Config, undefined] | [undefined, string] {
    if (fs.existsSync(PATH)) {
        const content = fs.readFileSync(PATH, "utf8");
        const json = JSON.parse(content);
        const s = structure(json);
        if (s[0] === undefined) return s;
        const m = mirror(json);
        if (m[0] === undefined) return m;
        return [{ structure: s[0], mirror: m[0] }, undefined];
    }
    return [DEFAULT, undefined];
}

function mirror(config: any): [Config["mirror"], undefined] | [undefined, string] {
    if (Object.hasOwn(config, "mirror")) {
        const mirror = config["mirror"];
        if (typeof mirror === "boolean") return [mirror, undefined];
        return [undefined, `The 'mirror' property must be of type 'boolean', got '${typeof mirror}'`];
    }
    return [DEFAULT["mirror"], undefined];
}

function structure(config: any): [Config["structure"], undefined] | [undefined, string] {
    function root(structure: any): [string, undefined] | [undefined, string] {
        if (Object.hasOwn(structure, "root")) {
            const root = structure["root"];
            if (typeof root === "string") return [root, undefined];
            return [undefined, `The 'structure/root' property must be of type 'string', got '${typeof root}'`];
        }
        return [DEFAULT["structure"]["root"], undefined];
    }
    function build(structure: any): [string, undefined] | [undefined, string] {
        if (Object.hasOwn(structure, "build")) {
            const build = structure["build"];
            if (typeof build === "string") return [build, undefined];
            return [undefined, `The 'structure/build' property must be of type 'string', got '${typeof build}'`];
        }
        return [DEFAULT["structure"]["build"], undefined];
    }
    function ignore(structure: any): [string[], undefined] | [undefined, string] {
        if (Object.hasOwn(structure, "ignore")) {
            const ignore = structure["ignore"];
            if (Array.isArray(ignore)) {
                for (let i = 0; i < ignore.length; i++) {
                    const element = ignore[i];
                    if (typeof element === "string") continue;
                    return [undefined, `The 'structure/ignore[${i}]' property must be of type 'string', got '${typeof element}'`];
                }
                return [ignore, undefined];
            }
            return [undefined, `The 'structure/build' property must be of type 'string', got '${typeof ignore}'`];
        }
        return [DEFAULT["structure"]["ignore"], undefined];
    }
    if (Object.hasOwn(config, "structure")) {
        const structure = config["structure"];
        if (typeof structure === "object") {
            const r = root(structure);
            if (r[0] === undefined) return r;
            const b = build(structure);
            if (b[0] === undefined) return b;
            const i = ignore(structure);
            if (i[0] === undefined) return i;
            return [
                {
                    root: r[0],
                    build: b[0],
                    ignore: i[0]
                },
                undefined
            ];
        }
        return [undefined, `The 'structure' property must be of type 'object', got '${typeof structure}'`];
    }
    return [DEFAULT["structure"], undefined];
}
