import fs from "node:fs";
import { join } from "node:path";

const PATH: string = join(process.cwd(), "circumscribe.json");
const DEFAULT: Config = {
    structure: {
        root: process.cwd(),
        build: process.cwd(),
        exclude: []
    },
    mirror: false
}

export type Config = {
    readonly structure: {
        readonly root: string,
        readonly build: string;
        readonly exclude: string[];
    }
    readonly mirror: boolean;
}

export function readConfig(): Config {
    const content = fs.readFileSync(PATH, "utf8");
    const json = JSON.parse(content);
    return {
        structure: structure(json),
        mirror: mirror(json)
    }
}

function mirror(config: any): Config["mirror"] {
    if (Object.hasOwn(config, "mirror")) {
        if (typeof config["mirror"] === "boolean")
            return config["mirror"];
        return DEFAULT["mirror"];
    }
    return DEFAULT["mirror"];
}

function structure(config: any): Config["structure"] {
    function root(structure: any): string {
        if (Object.hasOwn(structure, "root")) {
            if (typeof structure["root"] === "string")
                return structure["root"];
            return DEFAULT["structure"]["root"];
        }
        return DEFAULT["structure"]["root"];
    }
    function build(structure: any): string {
        if (Object.hasOwn(structure, "build")) {
            if (typeof structure["build"] === "string")
                return structure["build"];
            return DEFAULT["structure"]["build"];
        }
        return DEFAULT["structure"]["build"];
    }
    if (Object.hasOwn(config, "structure")) {
        const structure = config["structure"];
        return {
            root: root(structure),
            build: build(structure),
            exclude: []
        };
    }
    return DEFAULT["structure"];
}