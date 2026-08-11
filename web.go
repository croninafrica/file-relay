package main

import (
	"html/template"
	"net/http"
)

const pageStyle = `
:root{color-scheme:dark;--bg:#0b1020;--card:#131a2d;--line:#29324b;--text:#edf2ff;--muted:#9aa7c2;--accent:#67e8f9;--danger:#fb7185}*{box-sizing:border-box}body{margin:0;background:radial-gradient(circle at top,#16213d,var(--bg) 45%);color:var(--text);font:15px/1.55 system-ui,-apple-system,"Segoe UI",sans-serif;min-height:100vh}.wrap{width:min(960px,calc(100% - 32px));margin:48px auto}.card{background:rgba(19,26,45,.96);border:1px solid var(--line);border-radius:18px;padding:26px;box-shadow:0 18px 60px rgba(0,0,0,.28)}h1,h2{margin:0 0 18px}h1{font-size:28px}h2{font-size:19px}.muted{color:var(--muted)}label{display:block;margin:14px 0 6px;font-weight:650}input,select,button{font:inherit}input[type=password],input[type=number],input[type=file],input[type=text]{width:100%;padding:11px 12px;border-radius:10px;border:1px solid var(--line);background:#0d1426;color:var(--text)}button,.button{display:inline-block;border:0;border-radius:10px;padding:11px 16px;background:var(--accent);color:#071019;font-weight:800;cursor:pointer;text-decoration:none}button.danger{background:transparent;color:var(--danger);border:1px solid #5f2b3a;padding:7px 10px}.error{background:#3b1824;border:1px solid #723044;color:#fecdd3;padding:11px;border-radius:10px;margin:12px 0}.grid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.actions{display:flex;gap:10px;align-items:center;justify-content:space-between;margin-top:20px}.file{display:grid;grid-template-columns:minmax(180px,1fr) auto;gap:14px;border-top:1px solid var(--line);padding:16px 0}.file:first-child{border-top:0}.file a{color:var(--accent);word-break:break-all}.tag{display:inline-block;border:1px solid var(--line);border-radius:999px;padding:2px 8px;color:var(--muted);font-size:12px;margin-right:5px}.meta{color:var(--muted);font-size:13px}.spacer{height:22px}@media(max-width:700px){.grid{grid-template-columns:1fr}.file{grid-template-columns:1fr}.wrap{margin:22px auto}.card{padding:20px}}`

var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>文件中转 · 管理登录</title><style>` + pageStyle + `</style></head><body><main class="wrap" style="max-width:480px"><section class="card"><h1>文件中转</h1><p class="muted">管理入口仅用于上传和管理分享。</p>{{if .Error}}<div class="error">{{.Error}}</div>{{end}}<form method="post" action="{{.BasePath}}/admin/login"><label for="password">管理员密码</label><input id="password" type="password" name="password" autocomplete="current-password" required autofocus><div class="actions"><span></span><button type="submit">登录</button></div></form></section></main></body></html>`))

type loginView struct{ BasePath, Error string }

var adminTemplate = template.Must(template.New("admin").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>文件中转 · 管理</title><style>` + pageStyle + `</style></head><body><main class="wrap"><section class="card"><div class="actions" style="margin-top:0"><div><h1>文件中转</h1><p class="muted">上传文件并生成私密分享链接。单文件上限 {{.MaxUpload}}。</p></div><form method="post" action="{{.BasePath}}/admin/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="danger" type="submit">退出</button></form></div><form method="post" action="{{.BasePath}}/admin/upload" enctype="multipart/form-data"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="file">选择文件</label><input id="file" type="file" name="file" required><div class="grid"><div><label for="password">下载密码（可选，至少 12 位）</label><input id="password" type="password" name="password" minlength="12" autocomplete="new-password"></div><div><label for="expires">有效时间（小时）</label><input id="expires" type="number" name="expires_hours" value="24" min="1" max="720" required></div></div><div class="actions"><span class="muted">每个 IP 最多授权下载 3 次</span><button type="submit">上传并创建链接</button></div></form></section><div class="spacer"></div><section class="card"><h2>当前分享</h2>{{if not .Shares}}<p class="muted">还没有文件。</p>{{end}}{{range .Shares}}<article class="file"><div><strong>{{.Name}}</strong><div><a href="{{.URL}}" target="_blank" rel="noreferrer">{{.URL}}</a></div><div class="meta">{{.Size}} · 创建 {{.Created}} · 到期 {{.Expires}} · 已授权 {{.Downloads}} 次</div><div>{{if .Protected}}<span class="tag">有密码</span>{{else}}<span class="tag">无密码</span>{{end}}{{if .Expired}}<span class="tag">已过期</span>{{end}}</div></div><form method="post" action="{{$.BasePath}}/admin/delete"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="danger" type="submit">删除</button></form></article>{{end}}</section></main></body></html>`))

type adminShareView struct {
	ID, Name, Size, Created, Expires, URL string
	Protected, Expired                    bool
	Downloads                             int
}
type adminView struct {
	BasePath, CSRF, MaxUpload string
	Shares                    []adminShareView
}

var shareTemplate = template.Must(template.New("share").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>下载 {{.Name}}</title><style>` + pageStyle + `</style></head><body><main class="wrap" style="max-width:620px"><section class="card"><h1>{{.Name}}</h1><p class="muted">文件大小 {{.Size}} · 有效期至 {{.Expires}}</p><form method="post" action="{{.BasePath}}/s/{{.ID}}/authorize">{{if .Protected}}<label for="password">下载密码</label><input id="password" type="password" name="password" autocomplete="current-password" required autofocus>{{else}}<p>此文件无需密码。</p>{{end}}<div class="actions"><span class="muted">链接和下载权限均有时效限制</span><button type="submit">开始下载</button></div></form></section></main></body></html>`))

type shareView struct {
	BasePath, ID, Name, Size, Expires string
	Protected                         bool
}

var messageTemplate = template.Must(template.New("message").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>` + pageStyle + `</style></head><body><main class="wrap" style="max-width:560px"><section class="card"><h1>{{.Title}}</h1><p class="muted">{{.Message}}</p></section></main></body></html>`))

type messageView struct{ Title, Message string }

func render(w http.ResponseWriter, tmpl *template.Template, data any) {
	renderStatus(w, tmpl, data, http.StatusOK)
}

func renderStatus(w http.ResponseWriter, tmpl *template.Template, data any, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = tmpl.Execute(w, data)
}
