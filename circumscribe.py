#!usr/bin/env python
import sys
import os
import subprocess
import pathlib

#
# The default path of the circumscribe
# installation.
# ---
# I personally enjoy having third party
# tools installed in /opt, hence the default.
# You may, however, set a different path
# with CIRCUMSCRIBE_ROOT.
#
DEFAULT = "/opt/circumscribe"
REPOSITORY = "https://github.com/Manchick0/circumscribe.git"
ROOT = "CIRCUMSCRIBE_ROOT"

def main() -> None:
    if sys.platform == "linux" or sys.platform == "darwin":
        directory = pathlib.Path(os.getenv(ROOT, DEFAULT))
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

if __name__ == "__main__":
    main()
