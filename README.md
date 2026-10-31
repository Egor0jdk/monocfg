# monocfg 📦🚀

> **NixOS-like declarativity for everyone.** Manage your system packages and configuration files using a single `.master.conf` sheet. Totally free, transparent, and built for the community.

Distributed under the terms of the **GNU GPL v3** license.

## 📌 Philosophy (KISS)
Unlike heavy DevOps tools (Ansible, Nix, Chezmoi) that require complex markup languages or break your filesystem with symlinks, `monocfg` acts as a straightforward text pipeline. It parses a single `.master.conf` file, synchronizes package lists, and automatically overwrites file contents via `Truncate(0)` to safely preserve original file permissions and `inode`.

## ✨ Key Features
- **One-click Snapshot:** `--pull` (-p) scans your repos and paths to save a full text snapshot into `.master.conf`.
- **"Comma-free" Packages Syntax:** Declare software line-by-line or space-separated inside `[ ]`. No quotes, no commas.
- **Safe Orphans Management:** Removed packages from the sheet are safely marked as dependencies (orphans), leaving your base system clean without risky raw purges.
- **Bedrock Linux Deep Integration:** Automatically turns into Bedrock mode, inspects strata DNA via physical binaries (`pacman`, `xbps`, `apk`, `apt`), filters out symlinks, and calls operations via native `strat` without using `-r` flag isolation.

## 📁 Configuration Paths & Priority
The utility is designed to keep files highly accessible right at the top level of your home directories (no more digging through hidden nested folders):
1. **User Mode:** Looks for and creates `.master.conf` directly in your home root: `~/.master.conf`
2. **Root / Pure Su Mode:** Looks for and creates `.master.conf` directly in root's home: `/root/.master.conf`
*Note: When running via `sudo`, `monocfg` smartly detects your real `$SUDO_USER` and targets your personal `~/.master.conf` file automatically.*

## 🛠️ Usage

1. **Generate empty base template with syntax help:**
   ```bash
   monocfg --gen     # or -g
   ```
2. **Take system snapshot into the configuration file:**
   ```bash
   monocfg --pull    # or -p
   ```
3. **Apply state changes to the system (with safe interactive prompt):**
   ```bash
   sudo monocfg --apply --confirm  # or -a -c
   ```
4. **On-the-fly interactive config editor:**
   ```bash
   sudo monocfg --edit   # or -e
   ```

## ⚙️ Example .master.conf

```text
packages "cachyos" [
    neovim
    git
    kitty
    fastfetch
]

packages "void" [
    htop
    tmux
    fish
]

import "/home/user/.bashrc"
import "/home/user/.config/kitty"
import "/home/user/.config/fastfetch"
exclude "*/history"
```
