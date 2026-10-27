// 跨平台工程命令只依赖 Go，Windows 无须额外安装 make/bash/Node。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func command(name string, args ...string) error {
	fmt.Println("+", name, strings.Join(args, " "))
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
func goFiles() ([]string, error) {
	var files []string
	for _, dir := range []string{"cmd", "internal", "migrations", "pkg"} {
		err := filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && strings.HasSuffix(p, ".go") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
func run() error {
	task := "check"
	if len(os.Args) > 1 {
		task = os.Args[1]
	}
	switch task {
	case "lint":
		return command("go", "run", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0", "run", "./...")
	case "vuln":
		return command("go", "run", "golang.org/x/vuln/cmd/govulncheck@v1.9.0", "./...")
	case "dev":
		return command("go", "run", "github.com/air-verse/air@v1.67.4", "-c", ".air.toml")
	case "fmt":
		files, err := goFiles()
		if err != nil {
			return err
		}
		return command("gofmt", append([]string{"-w"}, files...)...)
	case "check":
		files, err := goFiles()
		if err != nil {
			return err
		}
		out, err := exec.Command("gofmt", append([]string{"-l"}, files...)...).CombinedOutput()
		if err != nil {
			return err
		}
		if len(out) > 0 {
			return fmt.Errorf("请先格式化:\n%s", out)
		}
		if err = command("go", "mod", "verify"); err != nil {
			return err
		}
		if err = command("go", "vet", "./..."); err != nil {
			return err
		}
		return command("go", "test", "-count=1", "-coverprofile=coverage.out", "./...")
	case "test":
		return command("go", "test", "-count=1", "./...")
	case "build":
		if err := os.MkdirAll("bin", 0755); err != nil {
			return err
		}
		commit := "unknown"
		if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
			commit = strings.TrimSpace(string(out))
		}
		ver := os.Getenv("HADALBOT_BUILD_VERSION")
		if ver == "" {
			ver = "dev"
		}
		flags := fmt.Sprintf("-s -w -X github.com/surgar04/hadalbot/internal/version.Version=%s -X github.com/surgar04/hadalbot/internal/version.Commit=%s -X github.com/surgar04/hadalbot/internal/version.BuiltAt=%s", ver, commit, time.Now().UTC().Format(time.RFC3339))
		ext := ""
		if runtime.GOOS == "windows" {
			ext = ".exe"
		}
		for _, name := range []string{"hadalbot", "migrate"} {
			if err := command("go", "build", "-trimpath", "-ldflags", flags, "-o", filepath.Join("bin", name+ext), "./cmd/"+name); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("未知任务 %q，可用 fmt/check/test/build/lint/vuln/dev", task)
	}
}
