package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/urfave/cli/v3"
)

type devConfig struct {
	Command      []string
	Watch        []string
	Exclude      []string
	TemplateDirs []string
	Debounce     time.Duration
	BuildOutput  string
}

type changeKind int

const (
	changeNone changeKind = iota
	changeRestart
	changeTemplate
)

func devFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "cmd", Value: "serve", Usage: "Application command, for example 'serve' or 'queue:work default'"},
		&cli.StringSliceFlag{Name: "watch", Value: []string{"."}, Usage: "Path to watch; can be repeated"},
		&cli.StringSliceFlag{Name: "exclude", Value: []string{".git", "data", "tmp", "node_modules", "vendor", "docs/dist", "docs/.astro"}, Usage: "Path fragment to exclude; can be repeated"},
		&cli.StringSliceFlag{Name: "template", Value: []string{"views", "templates", "web/templates"}, Usage: "Template/view directory that does not require process restart"},
		&cli.DurationFlag{Name: "debounce", Value: 300 * time.Millisecond, Usage: "Debounce duration for rebuilds"},
		&cli.StringFlag{Name: "bin", Value: filepath.Join(".runa", "dev-app"), Usage: "Temporary binary path"},
	}
}

func dev(ctx context.Context, cmd *cli.Command) error {
	cfg := devConfig{
		Command:      shellFields(cmd.String("cmd")),
		Watch:        nonEmpty(cmd.StringSlice("watch"), "."),
		Exclude:      cmd.StringSlice("exclude"),
		TemplateDirs: cmd.StringSlice("template"),
		Debounce:     cmd.Duration("debounce"),
		BuildOutput:  cmd.String("bin"),
	}
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"serve"}
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = 300 * time.Millisecond
	}
	return runDev(ctx, cfg)
}

func runDev(ctx context.Context, cfg devConfig) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := os.MkdirAll(filepath.Dir(cfg.BuildOutput), 0o755); err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := addWatchRoots(watcher, cfg); err != nil {
		return err
	}

	runner := &devRunner{cfg: cfg}
	defer runner.stop(5 * time.Second)
	if err := runner.rebuildAndRestart(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[runa dev] initial build failed: %v\n", err)
	}

	changes := make(chan changeKind, 16)
	go collectDevEvents(ctx, watcher, cfg, changes)

	var pending changeKind
	var timer *time.Timer
	for {
		var timerC <-chan time.Time
		if timer != nil {
			timerC = timer.C
		}
		select {
		case <-ctx.Done():
			return nil
		case kind := <-changes:
			pending = mergeChange(pending, kind)
			if timer == nil {
				timer = time.NewTimer(cfg.Debounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(cfg.Debounce)
			}
		case <-timerC:
			kind := pending
			pending = changeNone
			timer = nil
			switch kind {
			case changeRestart:
				if err := runner.rebuildAndRestart(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "[runa dev] build failed; keeping previous process: %v\n", err)
				}
			case changeTemplate:
				fmt.Fprintln(os.Stdout, "[runa dev] template changed; process not restarted")
			}
		}
	}
}

type devRunner struct {
	cfg     devConfig
	process *exec.Cmd
	done    chan struct{}
	mu      sync.Mutex
}

func (runner *devRunner) rebuildAndRestart(ctx context.Context) error {
	fmt.Fprintln(os.Stdout, "[runa dev] building...")
	build := exec.CommandContext(ctx, "go", "build", "-o", runner.cfg.BuildOutput, ".")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return err
	}
	runner.stop(5 * time.Second)
	args := append([]string(nil), runner.cfg.Command...)
	command := exec.CommandContext(ctx, runner.cfg.BuildOutput, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if err := command.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	runner.mu.Lock()
	runner.process = command
	runner.done = done
	runner.mu.Unlock()
	go func() {
		err := command.Wait()
		runner.mu.Lock()
		if runner.process == command {
			runner.process = nil
			runner.done = nil
		}
		runner.mu.Unlock()
		close(done)
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "[runa dev] process exited: %v\n", err)
		}
	}()
	fmt.Fprintf(os.Stdout, "[runa dev] started %s %s\n", runner.cfg.BuildOutput, strings.Join(args, " "))
	return nil
}

func (runner *devRunner) stop(timeout time.Duration) {
	runner.mu.Lock()
	process := runner.process
	done := runner.done
	runner.process = nil
	runner.done = nil
	runner.mu.Unlock()
	if process == nil || process.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = process.Process.Kill()
		return
	}
	_ = process.Process.Signal(syscall.SIGTERM)
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
		_ = process.Process.Kill()
		<-done
	}
}

func collectDevEvents(ctx context.Context, watcher *fsnotify.Watcher, cfg devConfig, changes chan<- changeKind) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "[runa dev] watch error: %v\n", err)
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if info, err := os.Stat(event.Name); err == nil && info.IsDir() && event.Op&fsnotify.Create != 0 {
				_ = addWatchDir(watcher, event.Name, cfg)
			}
			kind := classifyDevChange(event.Name, cfg)
			if kind != changeNone {
				changes <- kind
			}
		}
	}
}

func addWatchRoots(watcher *fsnotify.Watcher, cfg devConfig) error {
	seen := map[string]struct{}{}
	for _, root := range cfg.Watch {
		if root == "" {
			continue
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !entry.IsDir() {
				return nil
			}
			if shouldExclude(path, cfg.Exclude) {
				if path == root {
					return nil
				}
				return filepath.SkipDir
			}
			clean := filepath.Clean(path)
			if _, ok := seen[clean]; ok {
				return nil
			}
			seen[clean] = struct{}{}
			return watcher.Add(clean)
		}); err != nil {
			return err
		}
	}
	return nil
}

func addWatchDir(watcher *fsnotify.Watcher, path string, cfg devConfig) error {
	if shouldExclude(path, cfg.Exclude) {
		return nil
	}
	return watcher.Add(path)
}

func classifyDevChange(path string, cfg devConfig) changeKind {
	if shouldExclude(path, cfg.Exclude) {
		return changeNone
	}
	name := filepath.Base(path)
	if strings.HasPrefix(name, ".") && name != ".env" {
		return changeNone
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".go" || strings.HasPrefix(filepath.ToSlash(path), "config/") || name == "go.mod" || name == "go.sum" {
		return changeRestart
	}
	if isTemplatePath(path, cfg.TemplateDirs) || isTemplateExt(ext) {
		return changeTemplate
	}
	return changeNone
}

func isTemplatePath(path string, dirs []string) bool {
	slash := filepath.ToSlash(filepath.Clean(path))
	for _, dir := range dirs {
		dir = strings.Trim(filepath.ToSlash(filepath.Clean(dir)), "/")
		if dir == "." || dir == "" {
			continue
		}
		if slash == dir || strings.HasPrefix(slash, dir+"/") || strings.Contains(slash, "/"+dir+"/") {
			return true
		}
	}
	return false
}

func isTemplateExt(ext string) bool {
	switch ext {
	case ".html", ".tmpl", ".tpl", ".rhtml":
		return true
	default:
		return false
	}
}

func shouldExclude(path string, excludes []string) bool {
	slash := filepath.ToSlash(filepath.Clean(path))
	for _, exclude := range excludes {
		exclude = strings.Trim(filepath.ToSlash(filepath.Clean(exclude)), "/")
		if exclude == "" || exclude == "." {
			continue
		}
		if slash == exclude || strings.HasPrefix(slash, exclude+"/") || strings.Contains(slash, "/"+exclude+"/") {
			return true
		}
	}
	return false
}

func mergeChange(old changeKind, next changeKind) changeKind {
	if old == changeRestart || next == changeRestart {
		return changeRestart
	}
	if old == changeTemplate || next == changeTemplate {
		return changeTemplate
	}
	return changeNone
}

func shellFields(value string) []string {
	scanner := bufio.NewScanner(strings.NewReader(value))
	scanner.Split(bufio.ScanWords)
	var out []string
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	return out
}

func nonEmpty(values []string, fallback string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 && fallback != "" {
		out = append(out, fallback)
	}
	return out
}
