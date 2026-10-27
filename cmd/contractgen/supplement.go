package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// supplementMihoyo 补充旧OpenAPI未收录、但旧Go迁移参考源码实际调用的端点。
// 每项保留源文件依据，不将浏览器埋点、静态图片或旧项目本地服务误当成米哈游业务API。
func supplementMihoyo(directory string) error {
	schema := func(fields ...string) map[string]any {
		p := map[string]any{}
		for _, f := range fields {
			p[f] = map[string]any{"type": "string", "description": "源实现同名字段；账号、票据及验证码仅用于本次请求，不写入日志。"}
		}
		return map[string]any{"type": "object", "properties": p, "required": fields}
	}
	param := func(name string, required bool) any {
		return map[string]any{"name": name, "in": "query", "required": required, "description": "上游同名参数，保留原始字符串格式。", "schema": map[string]any{"type": "string"}}
	}
	write := func(name, origin, source string, paths map[string]any) error {
		document := map[string]any{"openapi": "3.0.3", "info": map[string]any{"title": "源码补充：" + name, "version": "1.0.0", "description": "来自旧项目已存在的调用逻辑；本轮仅验证封装契约，不宣称真实账号验收。依据：" + source}, "servers": []any{map[string]any{"url": origin}}, "paths": paths}
		b, e := json.MarshalIndent(document, "", "  ")
		if e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(directory, name+".openapi.json"), append(b, '\n'), 0644)
	}
	operation := func(id, summary string, body map[string]any, params ...any) map[string]any {
		o := map[string]any{"operationId": id, "summary": summary, "parameters": params, "x-candidate-unverified": false, "responses": map[string]any{"200": map[string]any{"description": "上游JSON响应，retcode为0表示成功"}}}
		if body != nil {
			o["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": body}}}
		}
		return o
	}
	verifier := map[string]any{}
	verifier["/account/ma-verifier/api/getActionTicketInfo"] = map[string]any{"post": operation("getActionTicketInfo", "查询账号安全验证票据要求", schema("action_type", "action_ticket"))}
	verifier["/account/ma-verifier/api/createEmailCaptchaByActionTicket"] = map[string]any{"post": operation("createEmailCaptchaByActionTicket", "显式发送邮箱验证码，不自动重试", schema("action_type", "action_ticket"))}
	verifySchema := schema("action_type", "action_ticket")
	p := object(verifySchema["properties"])
	for _, k := range []string{"password", "email_captcha"} {
		p[k] = map[string]any{"type": "string", "description": "安全验证内容，必须与当前票据所属账号一致，不记录日志。"}
	}
	p["verify_method"] = map[string]any{"type": "integer", "description": "上游安全验证方法标识，不枚举或绕过验证组合。"}
	verifier["/account/ma-verifier/api/verifyActionTicketPartly"] = map[string]any{"post": operation("verifyActionTicketPartly", "提交明确选择的安全验证材料", verifySchema)}
	if err := write("account-verifier", "https://sg-public-api.hoyoverse.com", "password-login.ts", verifier); err != nil {
		return err
	}
	if err := write("passport-extra", "https://passport-api-eu.hoyolab.com", "hoyolab-client.ts", map[string]any{"/account/ma-passport/token/verifyCookieToken": map[string]any{"post": operation("verifyCookieToken", "验证已有CookieToken会话", schema())}}); err != nil {
		return err
	}
	rewards := []struct {
		name, origin, path string
		post               bool
	}{
		{"genshin-sign", "https://sg-hk4e-api.hoyolab.com", "/event/sol/sign", true}, {"star-rail-sign", "https://sg-act-public-api.hoyolab.com", "/event/luna/hkrpg/os/sign", true}, {"zzz-sign", "https://sg-act-public-api.hoyolab.com", "/event/luna/zzz/os/sign", true},
		{"genshin-redeem", "https://sg-hk4e-api.hoyolab.com", "/common/apicdkey/api/webExchangeCdkey", false}, {"star-rail-redeem", "https://sg-hkrpg-api.hoyolab.com", "/common/apicdkey/api/webExchangeCdkey", false}, {"zzz-redeem", "https://public-operation-nap.hoyoverse.com", "/common/apicdkey/api/webExchangeCdkeyRisk", true},
	}
	for _, r := range rewards {
		var body map[string]any
		var parameters []any
		method := "get"
		if r.post {
			method = "post"
			if strings.HasSuffix(r.name, "sign") {
				body = schema("act_id")
				properties := object(body["properties"])
				for _, k := range []string{"lang", "uid", "region"} {
					properties[k] = map[string]any{"type": "string"}
				}
			} else {
				body = schema("game_biz", "uid", "region", "cdkey", "lang", "platform", "device_uuid")
				object(body["properties"])["t"] = map[string]any{"type": "integer", "description": "客户端毫秒时间戳"}
			}
		} else {
			for _, k := range []string{"uid", "region", "cdkey", "game_biz", "lang", "sLangKey"} {
				parameters = append(parameters, param(k, true))
			}
		}
		id := "redeemCode"
		if strings.HasSuffix(r.name, "sign") {
			id = "signIn"
		}
		o := operation(id, "显式执行"+r.name+"，即使GET也不自动重试", body, parameters...)
		o["x-idempotent"] = false
		if err := write(r.name, r.origin, "hoyolab-rewards.ts", map[string]any{r.path: map[string]any{method: o}}); err != nil {
			return err
		}
	}
	gacha := map[string]any{}
	o := operation("genAuthKey", "为明确选择的角色生成祈愿授权密钥", schema("auth_appid", "game_biz", "game_uid", "region"))
	o["x-signature"] = "legacy-ds2"
	gacha["/binding/api/genAuthKey"] = map[string]any{"post": o}
	if err := write("gacha-auth", "https://api-os-takumi.hoyoverse.com", "genshin-commands.ts", gacha); err != nil {
		return err
	}
	for _, scope := range []struct{ name, origin string }{{"star-rail-gacha-global", "https://public-operation-hkrpg-sg.hoyoverse.com"}, {"star-rail-gacha-cn", "https://public-operation-hkrpg.mihoyo.com"}} {
		paths := map[string]any{}
		for _, op := range []string{"getGachaLog", "getLdGachaLog"} {
			paths["/common/gacha_record/api/"+op] = map[string]any{"get": operation(op, "按授权链接分页读取星铁跃迁记录", nil, param("authkey", true), param("authkey_ver", true), param("sign_type", true), param("game_biz", true), param("gacha_type", true), param("lang", true), param("end_id", false), param("size", false), param("page", false))}
		}
		if err := write(scope.name, scope.origin, "genshin-commands.ts", paths); err != nil {
			return err
		}
	}
	if err := write("star-rail-official-gacha", "https://sg-act-public-api.hoyolab.com", "genshin-commands.ts", map[string]any{
		"/common/badge/v1/login/info":            map[string]any{"get": operation("getOfficialGachaSession", "读取官方跃迁会话并保留Set-Cookie", nil, param("game_biz", true), param("lang", true), param("ts", true))},
		"/event/rpg_gacha_record/five_star_list": map[string]any{"get": operation("getOfficialFiveStarList", "读取官方星铁五星跃迁分页", nil, param("game_biz", true), param("badge_region", true), param("badge_uid", true), param("region", true), param("uid", true), param("gacha_type", true), param("max_id", false), param("version_id", false))},
	}); err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(directory, "starrail-app.openapi.json"))
	if err != nil {
		return err
	}
	var web map[string]any
	if err = json.Unmarshal(b, &web); err != nil {
		return err
	}
	paths := map[string]any{}
	for path, item := range object(web["paths"]) {
		paths[strings.Replace(path, "/event/game_record/app/", "/event/game_record/", 1)] = item
	}
	if err = write("star-rail-web", "https://sg-act-public-api.hoyolab.com", "official-game-record.ts", paths); err != nil {
		return err
	}
	enka := map[string]any{}
	for _, v := range []struct{ id, path string }{{"getGenshinShowcase", "/api/uid/{uid}"}, {"getStarRailShowcase", "/api/hsr/uid/{uid}"}, {"getZzzShowcase", "/api/zzz/uid/{uid}"}} {
		p := param("uid", true)
		object(p)["in"] = "path"
		o := operation(v.id, "读取Enka公开展柜，不发送HoYoLAB凭证", nil, p)
		o["x-raw-response"] = true
		enka[v.path] = map[string]any{"get": o}
	}
	return write("enka", "https://enka.network", "enka-showcase.ts", enka)
}
