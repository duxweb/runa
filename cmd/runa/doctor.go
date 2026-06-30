package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/urfave/cli/v3"
)

type doctorIssue struct {
	Level   string `json:"level"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

func doctorFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{Name: "json", Usage: "Output JSON"},
	}
}

func doctor(_ context.Context, cmd *cli.Command) error {
	issues := runDoctor(".")
	if cmd.Bool("json") {
		return writeJSON(os.Stdout, map[string]any{"ok": len(issues) == 0, "issues": issues})
	}
	if len(issues) == 0 {
		fmt.Println("OK")
		return nil
	}
	for _, issue := range issues {
		if issue.Path != "" {
			fmt.Printf("%s %s: %s\n", issue.Level, issue.Path, issue.Message)
			continue
		}
		fmt.Printf("%s: %s\n", issue.Level, issue.Message)
	}
	return fmt.Errorf("doctor found %d issue(s)", len(issues))
}

func runDoctor(root string) []doctorIssue {
	var issues []doctorIssue
	issues = append(issues, checkToolingDeps(root)...)
	issues = append(issues, checkGoWork(root)...)
	issues = append(issues, checkNamingPatterns(root)...)
	return issues
}

func checkToolingDeps(root string) []doctorIssue {
	rootMod := filepath.Join(root, "go.mod")
	body, err := os.ReadFile(rootMod)
	if err != nil {
		return []doctorIssue{{Level: "ERROR", Path: rootMod, Message: err.Error()}}
	}
	for _, dep := range []string{"github.com/fsnotify/fsnotify"} {
		if strings.Contains(string(body), dep) {
			return []doctorIssue{{Level: "ERROR", Path: rootMod, Message: dep + " must stay out of the framework module"}}
		}
	}
	return nil
}

func checkGoWork(root string) []doctorIssue {
	mods := map[string]struct{}{}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "go.mod" || strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return nil
		}
		if rel == "." {
			rel = "."
		} else {
			rel = "./" + filepath.ToSlash(rel)
		}
		mods[rel] = struct{}{}
		return nil
	})
	body, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		return []doctorIssue{{Level: "ERROR", Path: "go.work", Message: err.Error()}}
	}
	text := string(body)
	var issues []doctorIssue
	for mod := range mods {
		if !strings.Contains(text, mod) {
			issues = append(issues, doctorIssue{Level: "ERROR", Path: "go.work", Message: "missing use entry for " + mod})
		}
	}
	return issues
}

func checkNamingPatterns(root string) []doctorIssue {
	patterns := []struct {
		re  *regexp.Regexp
		msg string
	}{
		{regexp.MustCompile(`^func\s+Driver\s*\(\s*name\s+string\s*\)`), "driver selection option must be Use(name), not Driver(name)"},
		{regexp.MustCompile(`^func\s+Provider\s*\([^)]*\)\s+\*provider\b`), "Provider must return provider.Provider, not *provider"},
		{regexp.MustCompile(`^func\s+\(.*\)\s+(CacheDriver|QueueDriver|StorageDriver|LockDriver|RateDriver|SessionDriver)\s*\(`), "registry driver registration should be RegisterDriver"},
	}
	var issues []doctorIssue
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"tmp-gencheck-") {
			return filepath.SkipDir
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := strings.TrimSpace(scanner.Text())
			for _, pattern := range patterns {
				if pattern.re.MatchString(line) {
					issues = append(issues, doctorIssue{Level: "ERROR", Path: fmt.Sprintf("%s:%d", path, lineNo), Message: pattern.msg})
				}
			}
		}
		return nil
	})
	return issues
}
