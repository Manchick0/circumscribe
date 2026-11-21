#!usr/bin/env python
import sys
import subprocess
import pathlib

#
# The default path of the circumscribe
# installation.
# ---
# I personally enjoy having third party
# tools installed in /opt, hence the default.
# You may, however, pass a different path as
# an argument to the script.
#
DEFAULT = "/opt/circumscribe"
REPOSITORY = "https://github.com/Manchick0/circumscribe.git"

def main() -> None:
    if sys.platform == "linux" or sys.platform == "darwin":
        directory = pathlib.Path(root())
        if directory.exists():
            subprocess.run(["git", "pull"], check=True, cwd=directory)
            subprocess.run(["make", "circumscribe"], check=True, cwd=directory)
            return ()
        subprocess.run(["git", "clone", REPOSITORY, directory], check=True)
        subprocess.run(["make", "circumscribe"], check=True, cwd=directory)
        subprocess.run(["ln", "-frs", "circumscribe", "/usr/bin/circumscribe"], check=True, cwd=directory)
        return ()
    sys.exit("'circumscribe.py' does not support Windows.")
    return ()

def root() -> str:
    if len(sys.argv) > 0:
        return sys.argv[1]
    return DEFAULT

if __name__ == "__main__":
    main()
