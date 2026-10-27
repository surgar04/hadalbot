package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

func text(x any) string     { s, _ := x.(string); return s }
func list(x any) []any      { a, _ := x.([]any); return a }
func quote(x string) string { b, _ := json.Marshal(x); return string(b) }
func keys(m map[string]any) []string {
	a := make([]string, 0, len(m))
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}
func comment(s string) string {
	s = strings.Join(strings.Fields(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, " ")), " ")
	if s == "" {
		return "上游文档字段；具体语义以对应接口说明为准。"
	}
	return s
}

var initials = map[string]string{"id": "ID", "ids": "IDs", "wxid": "WXID", "pid": "PID", "url": "URL", "uri": "URI", "api": "API", "http": "HTTP", "xml": "XML", "qr": "QR", "ip": "IP", "uid": "UID", "ds": "DS", "rsa": "RSA", "fp": "FP", "zzz": "ZZZ", "bbs": "BBS", "pv": "PV", "cn": "CN", "os": "OS"}
var descriptions = map[string]string{"role_id": "游戏角色UID，必须与当前接口的server区服组合使用；不能替换成HoYoLAB账号ID。", "server": "游戏区服标识，与角色UID严格对应，例如os_asia；不在SDK中推测或跨服改写。", "uid": "当前接口要求的账号或角色UID，含义随接口族变化，不得混用登录账号ID和游戏UID。", "game_biz": "上游游戏业务标识，例如hk4e_global、hkrpg_global、nap_global。", "lang": "上游语言标识，按接口选择zh-cn、zh-tw或其他明确支持的值。", "authkey": "抽卡授权临时密钥，属于凭证；只进入本次请求，不能写入日志或公开URL。", "authkey_ver": "抽卡授权版本，使用授权链接中的原值。", "sign_type": "抽卡授权签名类型，使用与authkey一致的原值。", "end_id": "分页游标，保持字符串以免超长记录ID丢失精度。", "max_id": "上游分页游标，必须使用上一次响应对应字段，不能当作页码。", "page": "分页页码，边界与起点以所属接口约定为准。", "size": "每页数量，调用方应设置有界大小，避免一次拉取全部历史。", "password": "RSA加密后的密码材料，不是明文；不可记录或持久化。", "account": "所属接口要求的账号材料；登录接口中须先RSA加密。", "token_type": "请求的令牌类型：App登录通常为2，Web登录通常为6，不可混用。", "stoken": "当前账号的长期SToken，只能由所属Provider使用，不记录日志。", "action_ticket": "当前账号的安全验证票据，不能跨账号、跨登录会话复用。", "act_id": "活动标识，签到时必须与游戏及活动周期匹配。", "cdkey": "待兑换的兑换码；兑换具有副作用，不应自动重试。", "device_id": "当前账号会话设备ID，保持稳定并与设备指纹匹配。", "device_fp": "上游签发的设备指纹，不得以固定全局假指纹替代。", "ext_fields": "调用方实际设备环境的序列化特征，不在SDK内伪造浏览器环境。", "region": "角色区服/区域标识，与当前游戏UID对应。"}

func fieldComment(key string, schema map[string]any) string {
	s := strings.TrimSpace(text(schema["title"]) + " " + text(schema["description"]))
	if s == "" {
		s = descriptions[key]
	}
	return comment(s)
}
func nameOf(s string) string {
	s = regexp.MustCompile(`([a-z0-9])([A-Z])`).ReplaceAllString(s, `${1}_${2}`)
	parts := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	var out strings.Builder
	for _, p := range parts {
		if v, ok := initials[strings.ToLower(p)]; ok {
			out.WriteString(v)
		} else {
			r := []rune(p)
			r[0] = unicode.ToUpper(r[0])
			out.WriteString(string(r))
		}
	}
	n := out.String()
	if n == "" {
		return "Value"
	}
	if unicode.IsDigit([]rune(n)[0]) {
		n = "Field" + n
	}
	return n
}
func decodeJSON(s string) any {
	var v any
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	if d.Decode(&v) != nil {
		return nil
	}
	return v
}
func infer(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		p := map[string]any{}
		for k, a := range x {
			p[k] = infer(a)
		}
		return map[string]any{"type": "object", "properties": p}
	case []any:
		s := map[string]any{"type": "array"}
		if len(x) > 0 {
			s["items"] = infer(x[0])
		}
		return s
	case string:
		return map[string]any{"type": "string"}
	case json.Number:
		return map[string]any{"type": "number"}
	case bool:
		return map[string]any{"type": "boolean"}
	}
	return map[string]any{}
}
func resolveSchema(s, defs map[string]any, depth int) map[string]any {
	if depth > 20 {
		return map[string]any{}
	}
	out := map[string]any{}
	if ref := text(s["$ref"]); ref != "" {
		for k, v := range resolveSchema(object(defs[strings.TrimPrefix(ref, "#/definitions/")]), defs, depth+1) {
			out[k] = v
		}
	}
	for k, v := range s {
		out[k] = v
	}
	for _, v := range object(s["x-apifox-refs"]) {
		ref := object(v)
		target := resolveSchema(ref, defs, depth+1)
		p := object(out["properties"])
		if p == nil {
			p = map[string]any{}
		}
		for k, a := range object(target["properties"]) {
			if _, exists := p[k]; !exists {
				p[k] = a
			}
		}
		out["properties"] = p
	}
	return out
}

type typeEmitter struct {
	types   bytes.Buffer
	used    map[string]bool
	defs    map[string]any
	request bool
}

func (e *typeEmitter) typ(schema map[string]any, example any, name string) string {
	schema = resolveSchema(schema, e.defs, 0)
	if text(schema["type"]) == "" {
		schema = infer(example)
	}
	if !e.request {
		if _, number := example.(json.Number); number && text(schema["type"]) == "string" {
			return "Scalar"
		}
		if _, stringValue := example.(string); stringValue && (text(schema["type"]) == "integer" || text(schema["type"]) == "number") {
			return "Scalar"
		}
	}
	switch text(schema["type"]) {
	case "string":
		return "string"
	case "integer":
		return "int64"
	case "number":
		return "json.Number"
	case "boolean":
		return "bool"
	case "array":
		var first any
		if a := list(example); len(a) > 0 {
			first = a[0]
		}
		return "[]" + e.typ(object(schema["items"]), first, name+"Item")
	case "object":
		p := object(schema["properties"])
		if len(p) == 0 {
			if m := object(example); len(m) > 0 {
				p = object(infer(m)["properties"])
			}
		}
		if len(p) == 0 {
			return "map[string]json.RawMessage"
		}
		if e.used[name] {
			return name
		}
		e.used[name] = true
		var declaration bytes.Buffer
		fmt.Fprintf(&declaration, "// %s %s\ntype %s struct {\n", name, comment(text(schema["description"])), name)
		required := map[string]bool{}
		for _, k := range list(schema["required"]) {
			required[text(k)] = true
		}
		for _, k := range keys(p) {
			s := object(p[k])
			t := e.typ(s, object(example)[k], name+nameOf(k))
			suffix := ""
			if e.request && !required[k] {
				t = "*" + t
				suffix = ",omitempty"
			}
			fmt.Fprintf(&declaration, "// %s %s\n%s %s `json:%s`\n", nameOf(k), fieldComment(k, s), nameOf(k), t, quote(k+suffix))
		}
		declaration.WriteString("}\n\n")
		e.types.Write(declaration.Bytes())
		return name
	}
	return "json.RawMessage"
}
func writeGo(file string, b []byte) error {
	formatted, err := format.Source(b)
	if err != nil {
		return fmt.Errorf("生成Go文件 %s: %w", file, err)
	}
	if err = os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	return os.WriteFile(file, formatted, 0644)
}

func generateQX(input, output string) error {
	b, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var source struct {
		Pages []page `json:"pages"`
	}
	if err = json.Unmarshal(b, &source); err != nil {
		return err
	}
	var generated bytes.Buffer
	generated.WriteString("// Code generated by contractgen; DO NOT EDIT.\n// 依据千寻官方双模式契约生成；业务与边界逻辑位于手写客户端。\npackage qianxun\nimport(\"context\";\"encoding/json\")\n")
	type operation struct {
		Mode, Method, Name, GoMethod, URL, Source string
		RequestSchema                             map[string]any
		ResponseSchema                            map[string]any
		RequestExample, ResponseExample           json.RawMessage
	}
	var ops []operation
	var eventPages []page
	for _, p := range source.Pages {
		data := object(p.Resource["data"])
		method := strings.ToUpper(text(data["method"]))
		if method == "" {
			if strings.Contains(text(data["content"]), "```json") {
				eventPages = append(eventPages, p)
			}
			continue
		}
		mode := "framework"
		prefix := "Framework"
		if strings.Contains(p.Group, "二") {
			mode = "direct"
			prefix = "Direct"
		}
		sample := text(object(data["requestBody"])["data"])
		request := object(decodeJSON(sample))
		op := text(request["type"])
		if method == "GET" {
			op = "downloadFile"
		}
		if op == "" {
			return fmt.Errorf("官方接口缺少type: %s", p.Name)
		}
		schemas := object(object(data["requestBody"])["jsonSchema"])
		rs := object(object(schemas["properties"])["data"])
		response := map[string]any{}
		responseExample := `{"code":200,"msg":"OK","result":null}`
		for _, r := range list(data["responses"]) {
			o := object(r)
			if o["code"] == float64(200) {
				response = object(o["jsonSchema"])
				for _, sample := range list(o["responseExamples"]) {
					if v := text(object(sample)["data"]); v != "" && json.Valid([]byte(v)) {
						responseExample = v
						break
					}
				}
				break
			}
		}
		result := object(object(response["properties"])["result"])
		defs := object(object(p.Resource["extraData"])["dataSchemaDefinitions"])
		reqExample, _ := json.Marshal(request["data"])
		goMethod := nameOf(op)
		if op == "downloadFile" {
			goMethod = "DownloadFileTo" // 文件接口采用Writer流式输出，而非生成JSON RPC方法。
		}
		ops = append(ops, operation{Mode: mode, Method: method, Name: op, GoMethod: goMethod, URL: text(data["path"]), Source: p.URL, RequestSchema: rs, ResponseSchema: response, RequestExample: reqExample, ResponseExample: json.RawMessage(responseExample)})
		if method == "GET" {
			continue
		}
		reqEmitter := typeEmitter{used: map[string]bool{}, defs: defs, request: true}
		reqName := prefix + nameOf(op) + "Request"
		reqType := reqEmitter.typ(rs, request["data"], reqName)
		if reqType != reqName {
			fmt.Fprintf(&reqEmitter.types, "// %s %s的空参数；保持 data 为JSON对象。\ntype %s struct{}\n", reqName, p.Name, reqName)
			reqType = reqName
		}
		resultEmitter := typeEmitter{used: map[string]bool{}, defs: defs}
		resultType := resultEmitter.typ(result, object(decodeJSON(responseExample))["result"], prefix+nameOf(op)+"Result")
		generated.Write(reqEmitter.types.Bytes())
		generated.Write(resultEmitter.types.Bytes())
		fmt.Fprintf(&generated, "// %s %s。\n// 官方契约：%s\n// 请求使用明确的%s模式；写操作不自动重试，原始响应保存在Reply.Raw中。\nfunc(c *%sClient)%s(ctx context.Context,request %s,options ...CallOption)(Reply[%s],error){return call[%s](ctx,c.client,%s,request,options...)}\n", nameOf(op), p.Name, p.URL, mode, prefix, nameOf(op), reqType, resultType, resultType, quote(op))
	}
	if err = writeGo(filepath.Join(output, "methods_generated.go"), generated.Bytes()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(map[string]any{"operations": ops, "events": eventPages}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "contracts.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("已生成 %d 个双模式HTTP契约和 %d 个事件/协议样例\n", len(ops), len(eventPages))
	return nil
}

func generateMihoyo(input, output string) error {
	files, err := filepath.Glob(filepath.Join(input, "*.openapi.json"))
	if err != nil {
		return err
	}
	var generated bytes.Buffer
	generated.WriteString("// Code generated by contractgen; DO NOT EDIT.\n// 依据脱敏上游契约生成，不能代表上游已完成真实账号验收。\npackage mihoyo\nimport(\"context\";\"encoding/json\";\"net/http\";\"net/url\";\"errors\")\n")
	var catalog []map[string]any
	var fields, construct bytes.Buffer
	for _, file := range files {
		b, e := os.ReadFile(file)
		if e != nil {
			return e
		}
		var doc map[string]any
		if e = json.Unmarshal(b, &doc); e != nil {
			return e
		}
		group := strings.TrimSuffix(filepath.Base(file), ".openapi.json")
		goGroup := nameOf(strings.ReplaceAll(group, "starrail", "star-rail"))
		apiType := goGroup + "API"
		fmt.Fprintf(&fields, "// %s %s\n%s %s\n", goGroup, comment(text(object(doc["info"])["description"])), goGroup, apiType)
		fmt.Fprintf(&construct, "client.%s=%s{client:core}\n", goGroup, apiType)
		fmt.Fprintf(&generated, "// %s %s。参数和正文保持上游原始命名。\ntype %s struct{client *clientCore}\n", apiType, comment(text(object(doc["info"])["title"])), apiType)
		servers := list(doc["servers"])
		if len(servers) == 0 {
			return fmt.Errorf("上游规范没有servers: %s", file)
		}
		origin := text(object(servers[0])["url"])
		for _, path := range keys(object(doc["paths"])) {
			pathItem := object(object(doc["paths"])[path])
			for _, method := range keys(pathItem) {
				if method != "get" && method != "post" && method != "put" && method != "patch" && method != "delete" {
					continue
				}
				op := object(pathItem[method])
				id := text(op["operationId"])
				if id == "" {
					return fmt.Errorf("接口缺少operationId: %s", path)
				}
				opID := group + "." + id
				reqName := goGroup + nameOf(id) + "Request"
				var declaration, encoding bytes.Buffer
				fmt.Fprintf(&declaration, "// %s %s的参数；可选值用指针区分省略和显式零值。\ntype %s struct{\n", reqName, comment(text(op["summary"])), reqName)
				encoding.WriteString("params:=Parameters{Query:url.Values{},Headers:http.Header{},Path:map[string]string{}}\n")
				params := append(append([]any{}, list(pathItem["parameters"])...), list(op["parameters"])...)
				seen := map[string]bool{}
				for _, p := range params {
					param := object(p)
					key, location := text(param["name"]), text(param["in"])
					field := nameOf(key)
					if seen[field] {
						field = nameOf(location) + field
					}
					seen[field] = true
					emitter := typeEmitter{used: map[string]bool{}, request: true}
					schema := object(param["schema"])
					typ := emitter.typ(schema, nil, reqName+field)
					required, _ := param["required"].(bool)
					value := "request." + field
					goType := typ
					if !required {
						goType = "*" + goType
						value = "*" + value
						fmt.Fprintf(&encoding, "if request.%s!=nil{\n", field)
					}
					fmt.Fprintf(&declaration, "// %s %s 参数位置：%s；%s。\n%s %s\n", field, fieldComment(key, param), location, map[bool]string{true: "必填", false: "可选"}[required], field, goType)
					if required && typ == "string" {
						fmt.Fprintf(&encoding, "if request.%s==\"\"{return Reply[json.RawMessage]{},errors.New(%s)}\n", field, quote("缺少必填参数: "+key))
					}
					if text(schema["type"]) == "array" {
						fmt.Fprintf(&encoding, "for _,value:=range %s{params.Query.Add(%s,parameterString(value))}\n", value, quote(key))
					} else {
						switch location {
						case "path":
							fmt.Fprintf(&encoding, "params.Path[%s]=parameterString(%s)\n", quote(key), value)
						case "header":
							fmt.Fprintf(&encoding, "params.Headers.Set(%s,parameterString(%s))\n", quote(key), value)
						case "cookie":
							fmt.Fprintf(&encoding, "params.Headers.Add(\"Cookie\",%s+\"=\"+parameterString(%s))\n", quote(key), value)
						default:
							fmt.Fprintf(&encoding, "params.Query.Set(%s,parameterString(%s))\n", quote(key), value)
						}
					}
					if !required {
						encoding.WriteString("}\n")
					}
					generated.Write(emitter.types.Bytes())
				}
				body := object(op["requestBody"])
				content := object(body["content"])
				media := object(content["application/json"])
				schema := object(media["schema"])
				if len(body) > 0 {
					emitter := typeEmitter{used: map[string]bool{}, request: true}
					typ := emitter.typ(schema, nil, reqName+"Body")
					fmt.Fprintf(&declaration, "// Body %s的JSON正文；不是浏览器表单，也不接受预签名后重新序列化。\nBody %s\n", comment(text(op["summary"])), typ)
					encoding.WriteString("params.Body=request.Body\n")
					generated.Write(emitter.types.Bytes())
				}
				declaration.WriteString("}\n")
				generated.Write(declaration.Bytes())
				fmt.Fprintf(&generated, "// %s %s。\n// 文档来源：%s；未验证候选接口仍保持显式标记。\nfunc(a %s)%s(ctx context.Context,request %s,options ...CallOption)(Reply[json.RawMessage],error){\n%sreturn a.client.call(ctx,%s,params,options...)\n}\n", nameOf(id), comment(text(op["summary"])), filepath.ToSlash(file), apiType, nameOf(id), reqName, encoding.String(), quote(opID))
				signature := "none"
				if group == "starrail-bbs" {
					signature = "hsr-ds1"
				}
				if group == "starrail-app" || group == "genshin-bbs-game-record" {
					signature = "app-ds2"
				}
				if s := text(op["x-signature"]); s != "" {
					signature = s
				}
				safe := method == "get"
				if value, ok := op["x-idempotent"].(bool); ok {
					safe = value
				}
				origins := []string{origin}
				for _, v := range list(op["x-allowed-origins"]) {
					origins = append(origins, text(v))
				}
				if group == "genshin-game-record" {
					origins = append(origins, "https://sg-public-api.hoyolab.com")
				}
				catalog = append(catalog, map[string]any{"id": opID, "group": group, "name": id, "go_group": goGroup, "go_method": nameOf(id), "method": strings.ToUpper(method), "origin": origin, "path": path, "summary": op["summary"], "candidate": op["x-candidate-unverified"], "signature": signature, "idempotent": safe, "origins": origins, "raw_response": op["x-raw-response"], "source": filepath.ToSlash(file), "operation": op})
			}
		}
	}
	fmt.Fprintf(&generated, "// Client 是不可变、账号隔离的上游SDK入口，各分组共享同一个有界网关。\ntype Client struct{core *clientCore\n%s}\nfunc clientFrom(core *clientCore)*Client{client:=&Client{core:core};%sreturn client}\n", fields.String(), construct.String())
	if err = writeGo(filepath.Join(output, "methods_generated.go"), generated.Bytes()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "contracts.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("已生成%d个米哈游/关联上游契约\n", len(catalog))
	return nil
}
