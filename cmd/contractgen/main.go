// contractgen 从官方文档的 SSR 数据中提取契约；它仅是构建工具，不参与服务运行。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 4 && os.Args[1] == "generate-qx" {
		return generateQX(os.Args[2], os.Args[3])
	}
	if len(os.Args) == 4 && os.Args[1] == "generate-mihoyo" {
		return generateMihoyo(os.Args[2], os.Args[3])
	}
	if len(os.Args) == 3 && os.Args[1] == "supplement-mihoyo" {
		return supplementMihoyo(os.Args[2])
	}
	if len(os.Args) == 4 && os.Args[1] == "fetch-qx" {
		return fetch(os.Args[2], os.Args[3])
	}
	if len(os.Args) != 3 {
		return fmt.Errorf("用法: contractgen input.html output.json")
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`streamController\.enqueue\(("(?:\\.|[^"\\])*")\)`)
	matches := re.FindAllSubmatch(b, -1)
	if len(matches) == 0 {
		return fmt.Errorf("页面未包含可解析的 SSR 数据")
	}
	output, err := decode(matches)
	if err != nil {
		return err
	}
	return os.WriteFile(os.Args[2], append(output, '\n'), 0644)
}
func decode(matches [][][]byte) ([]byte, error) {
	var text string
	for _, m := range matches {
		var s string
		if err := json.Unmarshal(m[1], &s); err != nil {
			return nil, err
		}
		text += s
	}
	var flat []any
	if err := json.Unmarshal([]byte(text), &flat); err != nil {
		return nil, err
	}
	memo := make(map[int]any)
	var resolve func(int) any
	resolve = func(i int) any {
		if i < 0 || i >= len(flat) {
			return nil
		}
		if x, ok := memo[i]; ok {
			return x
		}
		switch v := flat[i].(type) {
		case map[string]any:
			out := make(map[string]any)
			memo[i] = out
			for key, value := range v {
				j, e := strconv.Atoi(key[1:])
				if e != nil {
					continue
				}
				name, _ := resolve(j).(string)
				if n, ok := value.(float64); ok {
					out[name] = resolve(int(n))
				}
			}
			return out
		case []any:
			out := make([]any, 0, len(v))
			memo[i] = out
			for _, x := range v {
				if n, ok := x.(float64); ok {
					out = append(out, resolve(int(n)))
				} else {
					out = append(out, x)
				}
			}
			memo[i] = out
			return out
		default:
			memo[i] = v
			return v
		}
	}
	return json.MarshalIndent(resolve(0), "", "  ")
}

type page struct {
	URL      string         `json:"url"`
	Group    string         `json:"group"`
	Name     string         `json:"name"`
	Resource map[string]any `json:"resource"`
}

func object(x any) map[string]any { m, _ := x.(map[string]any); return m }
func fetch(seed, output string) error {
	b, err := os.ReadFile(seed)
	if err != nil {
		return err
	}
	var root map[string]any
	if err = json.Unmarshal(b, &root); err != nil {
		return err
	}
	state := object(object(object(root["loaderData"])["root"])["docsDataState"])
	var pages []page
	var walk func(any, string)
	walk = func(x any, group string) {
		switch v := x.(type) {
		case []any:
			for _, a := range v {
				walk(a, group)
			}
		case map[string]any:
			name, _ := v["name"].(string)
			if strings.HasPrefix(name, "方案") {
				group = name
			}
			to, _ := v["to"].(string)
			typ, _ := v["type"].(string)
			if group != "" && (typ == "apiDetail" || typ == "doc") {
				pages = append(pages, page{URL: "https://qxpro1.apifox.cn" + to, Group: group, Name: name})
			}
			walk(v["children"], group)
		}
	}
	walk(object(state["sidebarTree"])["sidebarTreeList"], "")
	// 完整树中的双方案目录各自包含全部事件和API，不依赖旧版49项清单。
	if previous, e := os.ReadFile(output); e == nil {
		var old struct {
			Pages []page `json:"pages"`
		}
		if json.Unmarshal(previous, &old) == nil {
			known := map[string]map[string]any{}
			for _, p := range old.Pages {
				known[p.URL] = p.Resource
			}
			for i := range pages {
				pages[i].Resource = known[pages[i].URL]
			}
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	jobs := make(chan int)
	errorsOut := make(chan error, len(pages))
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if pages[i].Resource != nil {
					continue
				}
				func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					req, e := http.NewRequestWithContext(ctx, "GET", pages[i].URL, nil)
					if e != nil {
						errorsOut <- e
						return
					}
					resp, e := client.Do(req)
					if e != nil {
						errorsOut <- e
						return
					}
					defer resp.Body.Close()
					if resp.StatusCode != 200 {
						errorsOut <- fmt.Errorf("文档状态码 %d", resp.StatusCode)
						return
					}
					raw, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
					if e != nil {
						errorsOut <- e
						return
					}
					matches := regexp.MustCompile(`streamController\.enqueue\(("(?:\\.|[^"\\])*")\)`).FindAllSubmatch(raw, -1)
					decoded, e := decode(matches)
					if e != nil {
						errorsOut <- e
						return
					}
					var data map[string]any
					if e = json.Unmarshal(decoded, &data); e != nil {
						errorsOut <- e
						return
					}
					s := object(object(object(data["loaderData"])["root"])["docsDataState"])
					pages[i].Resource = object(s["resourceData"])
				}()
			}
		}()
	}
	for i := range pages {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(errorsOut)
	for e := range errorsOut {
		if e != nil {
			return e
		}
	}
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Group != pages[j].Group {
			return pages[i].Group < pages[j].Group
		}
		return pages[i].URL < pages[j].URL
	})
	result := map[string]any{"source": "https://qxpro1.apifox.cn", "fetched_at": time.Now().UTC().Format(time.RFC3339), "pages": pages}
	b, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(output, append(b, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("已提取 %d 个官方接口/事件页面\n", len(pages))
	return nil
}
