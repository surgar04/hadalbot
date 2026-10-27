package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/mod/semver"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// Normalize在注册前校验并深拷贝清单；调用方后续修改不会改变运行中的授权和预算。
func Normalize(m Manifest) (Manifest, error) {
	b, e := json.Marshal(m)
	if e != nil {
		return m, errors.New("插件清单不能编码")
	}
	if len(b) > 256<<10 {
		return m, errors.New("插件清单过大")
	}
	var cloned Manifest
	_ = json.Unmarshal(b, &cloned)
	m = cloned
	if m.API == 0 {
		m.API = APIVersion
	}
	if m.Runtime == "" {
		m.Runtime = "go"
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if !idPattern.MatchString(m.ID) || !fullVersion(m.Version) || m.API != APIVersion || (m.Runtime != "go" && m.Runtime != "typescript") {
		return m, errors.New("插件ID、版本、运行时或协议版本无效")
	}
	l := &m.Limits
	if l.Concurrency == 0 {
		l.Concurrency = 1
	}
	if l.HookTimeoutMS == 0 {
		l.HookTimeoutMS = 5000
	}
	if l.LifecycleTimeoutMS == 0 {
		l.LifecycleTimeoutMS = 10000
	}
	if l.MaxTasks == 0 {
		l.MaxTasks = 8
	}
	if l.MaxCleanups == 0 {
		l.MaxCleanups = 64
	}
	if l.Concurrency < 1 || l.Concurrency > 64 || l.Queue < 0 || l.Queue > 256 || l.HookTimeoutMS < 1 || l.HookTimeoutMS > 300000 || l.LifecycleTimeoutMS < 1 || l.LifecycleTimeoutMS > 300000 || l.MaxTasks < 1 || l.MaxTasks > 64 || l.MaxCleanups < 1 || l.MaxCleanups > 256 {
		return m, errors.New("插件资源预算无效")
	}
	if len(m.Dependencies) > 128 || len(m.Capabilities) > 256 {
		return m, errors.New("插件依赖或能力声明过多")
	}
	for id, rule := range m.Dependencies {
		if !idPattern.MatchString(id) || id == m.ID || !validRule(rule) {
			return m, fmt.Errorf("插件依赖声明无效: %s", m.ID)
		}
	}
	for _, c := range m.Capabilities {
		if c == "" || len(c) > 128 || strings.ContainsAny(c, "*\r\n\x00") {
			return m, errors.New("插件能力必须是明确的方法名")
		}
	}
	return m, nil
}
func validRule(r string) bool {
	if r == "*" {
		return true
	}
	for _, p := range []string{">=", "^", "~"} {
		if strings.HasPrefix(r, p) {
			return fullVersion(strings.TrimPrefix(r, p))
		}
	}
	return fullVersion(r)
}
func fullVersion(v string) bool {
	return strings.Count(strings.SplitN(strings.SplitN(v, "-", 2)[0], "+", 2)[0], ".") == 2 && semver.IsValid("v"+v)
}
func matches(v, r string) bool {
	if r == "*" {
		return true
	}
	current := "v" + v
	for _, p := range []string{">=", "^", "~"} {
		if strings.HasPrefix(r, p) {
			min := "v" + strings.TrimPrefix(r, p)
			if semver.Compare(current, min) < 0 {
				return false
			}
			if p == ">=" {
				return true
			}
			if p == "~" {
				return semver.MajorMinor(current) == semver.MajorMinor(min)
			}
			if semver.Major(min) != "v0" {
				return semver.Major(current) == semver.Major(min)
			}
			if semver.MajorMinor(min) != "v0.0" {
				return semver.MajorMinor(current) == semver.MajorMinor(min)
			}
			return semver.Compare(current, min) == 0
		}
	}
	return semver.Compare(current, "v"+r) == 0
}
