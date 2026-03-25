#!/usr/bin/env python3
"""Create an isolated asql profiles.yaml for VHS/e2e runs."""

from pathlib import Path
import sys


def main() -> int:
    if len(sys.argv) != 4:
        print(f"Usage: {sys.argv[0]} <config_home> <prod_dsn> <staging_dsn>", file=sys.stderr)
        return 1
    config_home = Path(sys.argv[1])
    prod_dsn = sys.argv[2]
    staging_dsn = sys.argv[3]

    profile_dir = config_home / "asql"
    profile_dir.mkdir(parents=True, exist_ok=True)
    profile_path = profile_dir / "profiles.yaml"
    profile_path.write_text(
        f"- name: prod\n  dsn: {prod_dsn}\n- name: staging\n  dsn: {staging_dsn}\n",
        encoding="utf-8",
    )
    profile_path.chmod(0o600)
    return 0


if __name__ == "__main__":
    sys.exit(main())
