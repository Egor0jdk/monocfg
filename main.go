// Copyright (C) 2026 Egor0jdk
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PMCommands struct {
	Query   string
	Install string
	Orphan  string
	Check   string
}

var pmRegistry = map[string]PMCommands{
	"pacman": {
		Query:   "pacman -Qeq",
		Install: "pacman -S --noconfirm",
		Orphan:  "pacman -D --asdeps",
		Check:   "pacman -Si",
	},
	"xbps": {
		Query:   "xbps-query -m",
		Install: "xbps-install -Sy",
		Orphan:  "xbps-pkgdb -m auto",
		Check:   "xbps-query -R",
	},
	"apt": {
		Query:   "apt-mark showmanual",
		Install: "apt-get install -y",
		Orphan:  "apt-mark auto",
		Check:   "apt-cache show",
	},
	"apk": {
		Query:   "cat",
		Install: "apk add",
		Orphan:  "",
		Check:   "apk search -e",
	},
}

type Config struct {
	Managers map[string][]string
	Imports  []string
	Excludes []string
	Files    map[string]string
}

var isBedrock = false

func checkRoot() {
	if os.Getuid() != 0 {
		log.Fatal("Error: root privileges required! Please run with sudo.")
	}
}

func detectBedrock() {
	if _, err := os.Stat("/bedrock/strata"); err == nil {
		isBedrock = true
	}
}

func detectManagerInStrata(strata string) string {
	base := "/bedrock/strata/" + strata
	if _, err := os.Stat(base + "/usr/bin/pacman"); err == nil { return "pacman" }
	if _, err := os.Stat(base + "/bin/pacman"); err == nil { return "pacman" }
	if _, err := os.Stat(base + "/usr/bin/xbps-install"); err == nil { return "xbps" }
	if _, err := os.Stat(base + "/bin/xbps-install"); err == nil { return "xbps" }
	if _, err := os.Stat(base + "/sbin/apk"); err == nil { return "apk" }
	if _, err := os.Stat(base + "/usr/bin/apk"); err == nil { return "apk" }
	if _, err := os.Stat(base + "/usr/bin/apt"); err == nil { return "apt" }
	return ""
}

func askConfirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [Y/n]: ", prompt)
		input, err := reader.ReadString('\n')
		if err != nil { return false }
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "" { return true }
		if input == "n" { return false }
		fmt.Println("Please enter Y or N.")
	}
}

func runSystemCmd(managerOrStrata, fullCmd string) (string, error) {
	var cmdArgs []string
	if isBedrock {
		cmdArgs = []string{"strat", managerOrStrata}
		cmdArgs = append(cmdArgs, strings.Fields(fullCmd)...)
	} else {
		cmdArgs = strings.Fields(fullCmd)
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	if strings.Contains(fullCmd, "-Q") || strings.Contains(fullCmd, "-m") || strings.Contains(fullCmd, "-Si") || strings.Contains(fullCmd, "-R") || strings.Contains(fullCmd, "show") || strings.Contains(fullCmd, "search") {
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return "", err
}

func parseMasterConf(path string) (*Config, error) {
	cfg := &Config{Managers: make(map[string][]string), Files: make(map[string]string)}
	file, err := os.Open(path)
	if os.IsNotExist(err) { return cfg, nil } else if err != nil { return nil, err }
	defer file.Close()

	reader := bufio.NewReader(file)
	inPackageBlock, inFileBlock := false, false
	var currentManager, currentFilePath string
	var packageLines, fileLines []string

	for {
		line, err := reader.ReadString('\n')
		isEOF := err == io.EOF
		trimmed := strings.TrimSpace(line)

		if inFileBlock {
			if strings.HasPrefix(trimmed, "// @end") {
				inFileBlock = false
				cfg.Files[currentFilePath] = strings.Join(fileLines, "")
				fileLines = nil
			} else { fileLines = append(fileLines, line) }
		} else if inPackageBlock {
			if trimmed == "]" {
				inPackageBlock = false
				cfg.Managers[currentManager] = strings.Fields(strings.Join(packageLines, " "))
				packageLines = nil
			} else { packageLines = append(packageLines, line) }
		} else {
			if strings.HasPrefix(trimmed, "packages ") && strings.Contains(trimmed, "[") {
				inPackageBlock = true
				currentManager = strings.Trim(strings.Split(strings.TrimPrefix(trimmed, "packages "), "[")[0], " \"")
			} else if strings.HasPrefix(trimmed, "import ") {
				cfg.Imports = append(cfg.Imports, strings.Trim(strings.TrimPrefix(trimmed, "import "), "\""))
			} else if strings.HasPrefix(trimmed, "exclude ") {
				cfg.Excludes = append(cfg.Excludes, strings.Trim(strings.TrimPrefix(trimmed, "exclude "), "\""))
			} else if strings.HasPrefix(trimmed, "// @begin ") {
				inFileBlock = true
				currentFilePath = strings.Trim(strings.TrimPrefix(trimmed, "// @begin "), "\"")
			}
		}
		if isEOF { break }
	}
	return cfg, nil
}

func gatherFiles(cfg *Config) []string {
	fileMap := make(map[string]bool)
	for _, impPath := range cfg.Imports {
		// Check whether the path contains a mask (an asterisk)
		if strings.Contains(impPath, "*") {
			matches, err := filepath.Glob(impPath)
			if err == nil {
				for _, match := range matches {
					info, err := os.Stat(match)
					if err == nil && !info.IsDir() {
						fileMap[match] = true
					}
				}
			}
			continue
		}

		// If there is no asterisk, we follow the old, standard folder/file scheme
		info, err := os.Stat(impPath)
		if err != nil { continue }
		if info.IsDir() {
			_ = filepath.WalkDir(impPath, func(path string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() { fileMap[path] = true }
				return nil
			})
		} else { fileMap[impPath] = true }
	}

	var finalFiles []string
	for path := range fileMap {
		excluded := false
		for _, exclPattern := range cfg.Excludes {
			matched, _ := filepath.Match(exclPattern, path)
			if matched || strings.HasPrefix(path, exclPattern) {
				excluded = true
				break
			}
		}
		if !excluded { finalFiles = append(finalFiles, path) }
	}
	return finalFiles
}

func applyFiles(cfg *Config) {
	for path, content := range cfg.Files {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { continue }
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil { continue }
		if err := f.Truncate(0); err != nil { f.Close(); continue }
		_, _ = f.Seek(0, 0)
		_, _ = f.WriteString(content)
		f.Close()
	}
	fmt.Println("[monocfg] Text configuration files successfully updated.")
}

func syncPackages(cfg *Config, isPullMode, needConfirm bool) map[string][]string {
	pulledState := make(map[string][]string)
	targets := make(map[string]string)

	if len(cfg.Managers) > 0 {
		for targetName := range cfg.Managers {
			if isBedrock {
				targets[targetName] = detectManagerInStrata(targetName)
			} else {
				targets[targetName] = targetName
			}
		}
	} else {
		if isBedrock {
			files, err := os.ReadDir("/bedrock/strata")
			if err == nil {
				for _, f := range files {
					name := f.Name()
					// Important: f.Type().IsDir() returns true ONLY for actual physical directories,
					// automatically filtering out any symlinks (links) and files!
					if name != "bedrock" && f.Type().IsDir() {
						pmType := detectManagerInStrata(name)
						if pmType != "" { 
							targets[name] = pmType 
						}
					}
				}
			}
		} else {
			if _, err := exec.LookPath("pacman"); err == nil { targets["pacman"] = "pacman" }
			if _, err := exec.LookPath("xbps-install"); err == nil { targets["xbps"] = "xbps" }
			if _, err := exec.LookPath("apt-get"); err == nil { targets["apt"] = "apt" }
			if _, err := exec.LookPath("apk"); err == nil { targets["apk"] = "apk" }
		}
	}

	for target, pmType := range targets {
		cmds, exists := pmRegistry[pmType]
		if !exists || pmType == "" { continue }

		if isPullMode {
			var pkgs []string
			if pmType == "apk" {
				worldPath := "/etc/apk/world"
				if isBedrock { worldPath = "/bedrock/strata/" + target + "/etc/apk/world" }
				data, err := os.ReadFile(worldPath)
				if err == nil { pkgs = strings.Fields(string(data)) }
			} else {
				out, err := runSystemCmd(target, cmds.Query)
				if err == nil {
					rawPkgs := strings.Fields(out)
					for _, p := range rawPkgs {
						if pmType == "xbps" {
							lastDash := strings.LastIndex(p, "-")
							if lastDash != -1 { p = p[:lastDash] }
						}
						pkgs = append(pkgs, p)
					}
				}
			}
			pulledState[target] = pkgs
		} else {
			declared := cfg.Managers[target]
			var current []string

			if pmType == "apk" {
				worldPath := "/etc/apk/world"
				if isBedrock { worldPath = "/bedrock/strata/" + target + "/etc/apk/world" }
				data, err := os.ReadFile(worldPath)
				if err == nil { current = strings.Fields(string(data)) }
			} else {
				out, _ := runSystemCmd(target, cmds.Query)
				rawCurrent := strings.Fields(out)
				for _, p := range rawCurrent {
					if pmType == "xbps" {
						lastDash := strings.LastIndex(p, "-")
						if lastDash != -1 { p = p[:lastDash] }
					}
					current = append(current, p)
				}
			}

			currentMap := make(map[string]bool)
			for _, p := range current { currentMap[p] = true }
			declaredMap := make(map[string]bool)
			for _, p := range declared { declaredMap[p] = true }

			var toInstall []string
			var toOrphan []string

			for _, pkg := range declared {
				if !currentMap[pkg] {
					_, err := runSystemCmd(target, cmds.Check+" "+pkg)
					if err != nil {
						fmt.Printf("[Warning] Package '%s' is not available in '%s' repositories!\n", pkg, target)
						if !askConfirm("Continue applying without this package?") { log.Fatal("Operation aborted by user.") }
						continue
					}
					toInstall = append(toInstall, pkg)
				}
			}

			for _, pkg := range current {
				if pkg == "monocfg" { continue }
				if !declaredMap[pkg] { toOrphan = append(toOrphan, pkg) }
			}

			if needConfirm {
				if len(toInstall) > 0 || len(toOrphan) > 0 {
					fmt.Printf("\n[monocfg] Planned package changes for target '%s':\n", target)
					if len(toInstall) > 0 { fmt.Println("  -> Will install:", strings.Join(toInstall, ", ")) }
					if len(toOrphan) > 0 { fmt.Println("  -> Will mark as dependency (orphan):", strings.Join(toOrphan, ", ")) }
					if !askConfirm("Apply these package changes?") { continue }
				}
			}

			if len(toInstall) > 0 {
				fmt.Printf("[monocfg] Installing packages into %s...\n", target)
				for _, pkg := range toInstall { _, _ = runSystemCmd(target, cmds.Install+" "+pkg) }
			}

			if len(toOrphan) > 0 {
				fmt.Printf("[monocfg] Marking packages as orphans in %s...\n", target)
				if pmType == "apk" {
					worldPath := "/etc/apk/world"
					if isBedrock { worldPath = "/bedrock/strata/" + target + "/etc/apk/world" }
					var newWorld []string
					for _, p := range current { if declaredMap[p] { newWorld = append(newWorld, p) } }
					_ = os.WriteFile(worldPath, []byte(strings.Join(newWorld, "\n")+"\n"), 0644)
				} else {
					for _, pkg := range toOrphan { _, _ = runSystemCmd(target, cmds.Orphan+" "+pkg) }
				}
			}
		}
	}
	return pulledState
}

func runEditor(path string) {
	editor := os.Getenv("EDITOR")
	if editor == "" { editor = "nano" }
	cmd := exec.Command(editor, path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
}

func main() {
	var pull, apply, edit, confirm, gen bool
	pullLong := flag.Bool("pull", false, "")
	applyLong := flag.Bool("apply", false, "")
	editLong := flag.Bool("edit", false, "")
	confirmLong := flag.Bool("confirm", false, "")
	genLong := flag.Bool("gen", false, "")

	pullShort := flag.Bool("p", false, "")
	applyShort := flag.Bool("a", false, "")
	editShort := flag.Bool("e", false, "")
	confirmShort := flag.Bool("c", false, "")
	genShort := flag.Bool("g", false, "")

	flag.Usage = func() {
		fmt.Println("Usage: monocfg [ACTION] [FLAGS]")
		fmt.Println("\nActions:\n  -p, --pull       Gather system state into configuration file\n  -a, --apply      Apply configuration changes to the system\n  -e, --edit       Open configuration file in editor and apply on exit\n  -g, --gen        Generate base template file with inline help documentation")
		fmt.Println("\nFlags:\n  -c, --confirm    Show list of package changes before running package managers")
		fmt.Println("\nThis software is distributed under the terms of the GNU GPL v3")
	}
	flag.Parse()

	pull = *pullLong || *pullShort
	apply = *applyLong || *applyShort
	edit = *editLong || *editShort
	confirm = *confirmLong || *confirmShort
	gen = *genLong || *genShort

	if !pull && !apply && !edit && !gen { flag.Usage(); return }

	detectBedrock()
	userHome := os.Getenv("HOME")
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser != "" && sudoUser != "root" {
		realHome := "/home/" + sudoUser
		if _, err := os.Stat(realHome); err == nil { userHome = realHome }
	}

	userConfigPath := filepath.Join(userHome, ".master.conf")
	systemConfigPath := "/root/.master.conf"
	masterPath := userConfigPath

	if os.Getuid() == 0 && sudoUser == "" { masterPath = systemConfigPath } else {
		if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
			if _, err := os.Stat(systemConfigPath); err == nil { masterPath = systemConfigPath }
		}
	}

	if gen {
		// Precise path calculation for template generation:
		// If the real process ID is 0, we are root (regardless of whether via su or sudo -i), so we write to /root/
		targetTemplate := userConfigPath
		if os.Getuid() == 0 {
			targetTemplate = systemConfigPath
		}

		if _, err := os.Stat(targetTemplate); err == nil {
			log.Fatalf("Error: File %s already exists! Generation aborted.", targetTemplate)
		}

		if err := os.MkdirAll(filepath.Dir(targetTemplate), 0755); err != nil {
			log.Fatalf("Error creating directory for template: %v", err)
		}

		f, err := os.Create(targetTemplate)
		if err != nil {
			log.Fatalf("Error creating template file: %v", err)
		}
		defer f.Close()

		// Determine which username to insert into the template comment
		currentEffectiveUser := sudoUser
		if currentEffectiveUser == "" {
			currentEffectiveUser = os.Getenv("USER")
		}
		if currentEffectiveUser == "root" || os.Getuid() == 0 {
			// If we are running as root, we check—just in case—whether there is an actual user behind this
			if sudoUser != "" {
				currentEffectiveUser = sudoUser
			} else {
				currentEffectiveUser = "user" // default placeholder
			}
		}

		templateText := `# .master.conf — monocfg syntax guide
#
# packages "manager_or_strata" [ pkg1 pkg2 ] (no commas)
# import "/path/to/file_or_dir"
# exclude "/path/to/ignore_pattern"

# packages "arch" [
#     neovim
#     git
# ]

# import "/home/` + currentEffectiveUser + `/.bashrc"
`
		_, _ = f.WriteString(templateText)
		fmt.Printf("[monocfg] Base template successfully generated at: %s\n", targetTemplate)
		return
	}

	if apply || edit { checkRoot() }
	fmt.Printf("[monocfg] Using configuration target file: %s\n", masterPath)

	cfg, err := parseMasterConf(masterPath)
	if err != nil { log.Fatalf("Error: %v", err) }

	if edit {
		runEditor(masterPath)
		cfg, err = parseMasterConf(masterPath)
		if err != nil { log.Fatalf("Error: %v", err) }
		apply = true
	}

	if pull {
		pulledPackages := syncPackages(cfg, true, false)
		files := gatherFiles(cfg)
		_ = os.MkdirAll(filepath.Dir(masterPath), 0755)
		f, err := os.Create(masterPath)
		if err != nil { log.Fatalf("Error: %v", err) }
		defer f.Close()

		fmt.Fprintln(f, "# .master.conf — Generated by monocfg (GPL v3)\n")
		for target, pkgs := range pulledPackages {
			fmt.Fprintf(f, "packages \"%s\" [\n", target)
			for _, pkg := range pkgs { fmt.Fprintf(f, "    %s\n", pkg) }
			fmt.Fprintln(f, "]\n")
		}
		for _, imp := range cfg.Imports { fmt.Fprintf(f, "import \"%s\"\n", imp) }
		for _, excl := range cfg.Excludes { fmt.Fprintf(f, "exclude \"%s\"\n", excl) }
		fmt.Fprintln(f)

		for _, path := range files {
			content, err := os.ReadFile(path)
			if err != nil { continue }
			fmt.Fprintf(f, "// @begin \"%s\"\n", path)
			f.Write(content)
			
			// HACK: If the file inside does not end with a line break,
			// we force-add a \n before the closing marker so as not to break the syntax!
			if len(content) > 0 && content[len(content)-1] != '\n' {
				fmt.Fprintln(f)
			}
			
			fmt.Fprintf(f, "// @end \"%s\"\n\n", path)
		}
		fmt.Printf("[monocfg] System state snapshot successfully saved to %s\n", masterPath)
		return
	}

	if apply {
		syncPackages(cfg, false, confirm)
		applyFiles(cfg)
	}
}