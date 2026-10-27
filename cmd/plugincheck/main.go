// plugincheck只做静态盘点，不import、执行或安装插件，也不访问旧配置和数据库。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type report struct {
	ID          string   `json:"id"`
	Unsupported []string `json:"unsupported_modules"`
	Skipped     int      `json:"skipped_sources"`
	SourceFiles int      `json:"source_files"`
}

var imports = regexp.MustCompile(`(?s)\bimport\s+(type\s+)?(?:[^;]*?\s+from\s+)?['"](@/[^'"]+)['"]`)
var dynamic = regexp.MustCompile(`\bimport\s*\(\s*['"](@/[^'"]+)['"]`)

func main() {
	root := flag.String("root", "plugins", "待审计插件根目录")
	output := flag.String("out", "", "JSON输出路径，省略则stdout")
	strict := flag.Bool("strict", false, "有未适配引用时返回非零")
	flag.Parse()
	reports, err := scan(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "插件静态审计失败")
		os.Exit(1)
	}
	body, _ := json.MarshalIndent(map[string]any{"kind": "static-import-audit", "limitations": "仅扫描非资源目录中的静态字面量import；不证明业务、权限、npm依赖、动态导入、存储和定时任务已兼容。", "plugins": reports}, "", "  ")
	body = append(body, '\n')
	if *output != "" {
		if err = os.WriteFile(*output, body, 0600); err != nil {
			fmt.Fprintln(os.Stderr, "审计文件写入失败")
			os.Exit(1)
		}
	} else {
		_, _ = os.Stdout.Write(body)
	}
	if *strict {
		for _, r := range reports {
			if len(r.Unsupported) > 0 || r.Skipped > 0 {
				os.Exit(2)
			}
		}
	}
}
func scan(root string) ([]report, error) {
	dir, e := os.Open(root)
	if e != nil {
		return nil, e
	}
	defer dir.Close()
	entries, e := dir.ReadDir(1025)
	if e != nil && !errors.Is(e, io.EOF) {
		return nil, e
	}
	if len(entries) > 1024 {
		return nil, errors.New("插件目录过大")
	}
	reports := []report{}
	supported := map[string]bool{"@/bot/plugin-base.js": true, "@/api/types/events.js": true, "@/api/types/api.js": true, "@/api/types/index.js": true, "@/types/message.js": true}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		body, e := os.ReadFile(filepath.Join(path, "plugin.json"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil || len(body) > 256<<10 {
			return nil, errors.New("清单无效")
		}
		var meta struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &meta) != nil {
			return nil, errors.New("清单JSON无效")
		}
		r := report{ID: meta.ID, Unsupported: []string{}}
		refs := map[string]bool{}
		visited := 0
		err := filepath.WalkDir(path, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			visited++
			if visited > 8192 {
				return errors.New("插件扫描条目超限")
			}
			if d.IsDir() {
				switch d.Name() {
				case "node_modules", "resources", "assets", "dist", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(p) != ".ts" && filepath.Ext(p) != ".js" && filepath.Ext(p) != ".mjs" {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if info.Size() > 2<<20 {
				r.Skipped++
				return nil
			}
			source, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			r.SourceFiles++
			for _, match := range imports.FindAllStringSubmatch(string(source), -1) {
				if match[1] == "" {
					refs[match[2]] = true
				}
			}
			for _, match := range dynamic.FindAllStringSubmatch(string(source), -1) {
				refs[match[1]] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		for ref := range refs {
			if !supported[ref] && !strings.HasPrefix(ref, "@/plugins/"+meta.ID+"/") {
				r.Unsupported = append(r.Unsupported, ref)
			}
		}
		sort.Strings(r.Unsupported)
		reports = append(reports, r)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].ID < reports[j].ID })
	return reports, nil
}
