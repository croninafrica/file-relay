package main

import (
	"html/template"
	"net/http"
	"strings"
)

const pageStyle = `
:root{color-scheme:light;--bg:#f5f7fa;--surface:#fff;--surface-soft:#f8fafc;--text:#172033;--muted:#68758a;--line:#e2e7ee;--accent:#2563eb;--accent-hover:#1d4ed8;--accent-soft:#eff6ff;--success:#087a55;--success-bg:#ecfdf5;--danger:#c33b4a;--danger-bg:#fff1f2;--shadow:0 12px 35px rgba(23,32,51,.07)}*{box-sizing:border-box}[hidden]{display:none!important}html{background:var(--bg)}body{margin:0;color:var(--text);font:15px/1.55 Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif;min-height:100vh}.shell{width:min(980px,calc(100% - 32px));margin:0 auto;padding:40px 0 64px}.compact{width:min(460px,calc(100% - 32px));padding-top:10vh}.topbar{display:flex;align-items:center;justify-content:space-between;gap:24px;margin-bottom:24px}.brand{display:flex;align-items:center;gap:13px}.brand-mark{display:grid;place-items:center;width:42px;height:42px;border-radius:12px;background:var(--text);color:#fff;font-size:20px;font-weight:800}.brand h1{font-size:24px;line-height:1.2;margin:0}.brand p{margin:4px 0 0;color:var(--muted)}.panel{background:var(--surface);border:1px solid var(--line);border-radius:16px;box-shadow:var(--shadow);padding:24px}.panel+.panel{margin-top:20px}.section-head{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;margin-bottom:20px}.section-head h2{font-size:18px;margin:0}.section-head p{color:var(--muted);margin:4px 0 0}.count{min-width:28px;height:28px;padding:3px 9px;border-radius:999px;background:var(--surface-soft);color:var(--muted);text-align:center;font-weight:700}.field{margin-top:16px}.field:first-child{margin-top:0}label{display:block;margin-bottom:7px;font-weight:650}.hint{color:var(--muted);font-size:13px;font-weight:400}.input,input[type=password],input[type=number]{width:100%;height:44px;padding:10px 12px;border:1px solid #d5dce6;border-radius:10px;background:#fff;color:var(--text);font:inherit;outline:none;transition:border-color .15s,box-shadow .15s}.input:focus,input:focus{border-color:#7ba4f8;box-shadow:0 0 0 3px rgba(37,99,235,.12)}.file-box{display:flex;align-items:center;gap:14px;padding:18px;border:1px dashed #b8c5d8;border-radius:12px;background:var(--surface-soft);transition:border-color .15s,background .15s}.file-box:hover{border-color:#7ba4f8;background:var(--accent-soft)}.file-box input{min-width:0;max-width:100%;font:inherit}.file-meta{margin-top:8px;color:var(--muted);font-size:13px}.settings-grid{display:grid;grid-template-columns:2fr 1fr;gap:16px}.form-footer{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-top:22px;padding-top:18px;border-top:1px solid var(--line)}.form-note{color:var(--muted);font-size:13px}.button,button{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:40px;padding:9px 15px;border:0;border-radius:9px;background:var(--accent);color:#fff;font:inherit;font-weight:750;text-decoration:none;cursor:pointer;transition:background .15s,opacity .15s}.button:hover,button:hover{background:var(--accent-hover)}button:disabled{cursor:not-allowed;opacity:.58}.button-secondary{background:#fff;color:var(--text);border:1px solid var(--line)}.button-secondary:hover{background:var(--surface-soft)}.button-danger{min-height:34px;padding:6px 10px;background:#fff;color:var(--danger);border:1px solid #f0c8ce}.button-danger:hover{background:var(--danger-bg)}.notice,.error{border-radius:11px;padding:12px 14px;margin-bottom:18px}.notice{color:var(--success);background:var(--success-bg);border:1px solid #b6ead8}.error{color:var(--danger);background:var(--danger-bg);border:1px solid #f3c2ca}.progress-wrap{margin-top:18px;padding:15px;border:1px solid #cfe0ff;border-radius:12px;background:var(--accent-soft)}.progress-top{display:flex;align-items:center;justify-content:space-between;margin-bottom:9px;font-weight:700}.progress-wrap progress{display:block;width:100%;height:9px;border:0;border-radius:999px;overflow:hidden;background:#dbe7fb}.progress-wrap progress::-webkit-progress-bar{background:#dbe7fb;border-radius:999px}.progress-wrap progress::-webkit-progress-value{background:var(--accent);border-radius:999px}.progress-wrap progress::-moz-progress-bar{background:var(--accent);border-radius:999px}.progress-status{color:var(--muted);font-size:13px;margin-top:8px}.empty{padding:30px 12px;text-align:center;color:var(--muted);border:1px dashed var(--line);border-radius:12px}.share-list{border-top:1px solid var(--line)}.share-row{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:18px;padding:18px 0;border-bottom:1px solid var(--line)}.share-row:last-child{padding-bottom:0;border-bottom:0}.share-title{display:flex;align-items:center;gap:8px;min-width:0}.share-title strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.badges{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}.badge{display:inline-flex;align-items:center;height:24px;padding:2px 8px;border-radius:999px;background:var(--surface-soft);border:1px solid var(--line);color:var(--muted);font-size:12px}.share-link{display:block;width:fit-content;max-width:100%;margin-top:7px;color:var(--accent);text-decoration:none;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.share-link:hover{text-decoration:underline}.meta{margin-top:7px;color:var(--muted);font-size:13px}.share-actions{display:flex;align-items:flex-start}.download-card{text-align:center}.download-icon{display:grid;place-items:center;width:58px;height:58px;margin:0 auto 17px;border-radius:16px;background:var(--accent-soft);color:var(--accent);font-size:25px;font-weight:800}.download-card h1{font-size:22px;margin:0;overflow-wrap:anywhere}.download-card .meta{margin:8px 0 22px}.download-card form{text-align:left}.download-card .button{width:100%;margin-top:18px}.login-card h1{font-size:22px;margin:0 0 4px}.login-card>p{margin:0 0 22px;color:var(--muted)}.login-card .button{width:100%;margin-top:18px}@media(max-width:680px){.shell{width:min(100% - 22px,980px);padding:22px 0 40px}.compact{padding-top:7vh}.topbar{align-items:flex-start}.brand p{font-size:13px}.panel{padding:18px;border-radius:14px}.settings-grid{grid-template-columns:1fr}.form-footer{align-items:stretch;flex-direction:column}.form-footer .button{width:100%}.share-row{grid-template-columns:1fr}.share-actions button{width:100%}.share-actions form{width:100%}.section-head{margin-bottom:16px}}`

const adminJS = `(function(){
"use strict";
var form=document.getElementById("upload-form");
if(!form){return;}
var fileInput=document.getElementById("file");
var fileMeta=document.getElementById("file-meta");
var progressWrap=document.getElementById("upload-progress");
var progressBar=document.getElementById("upload-bar");
var progressPercent=document.getElementById("upload-percent");
var progressStatus=document.getElementById("upload-status");
var errorBox=document.getElementById("upload-error");
var submitButton=document.getElementById("upload-button");
var uploading=false;
var maxRetries=3;
function formatBytes(value){
  if(value<1024){return value+" B";}
  var units=["KiB","MiB","GiB","TiB"],size=value,index=-1;
  do{size/=1024;index++;}while(size>=1024&&index<units.length-1);
  return size.toFixed(size>=100?0:size>=10?1:2)+" "+units[index];
}
function parseJSON(request){
  try{return JSON.parse(request.responseText);}catch(ignore){return {};}
}
function setProgress(loaded,total,started,message){
  var percent=Math.min(100,Math.round(loaded/total*100));
  var elapsed=Math.max((Date.now()-started)/1000,0.1);
  progressBar.value=percent;
  progressPercent.textContent=percent+"%";
  progressStatus.textContent=message||formatBytes(loaded)+" / "+formatBytes(total)+" · "+formatBytes(loaded/elapsed)+"/s";
}
function showError(message){
  errorBox.textContent=message;
  errorBox.hidden=false;
  progressStatus.textContent="上传未完成，可以直接重试";
  submitButton.disabled=false;
  submitButton.textContent="重试上传";
  uploading=false;
}
function postForm(url,data,done){
  var request=new XMLHttpRequest();
  request.open("POST",url,true);
  request.setRequestHeader("Content-Type","application/x-www-form-urlencoded;charset=UTF-8");
  request.addEventListener("load",function(){done(null,request);});
  request.addEventListener("error",function(){done(new Error("network"),request);});
  request.send(data.toString());
}
fileInput.addEventListener("change",function(){
  var file=fileInput.files&&fileInput.files[0];
  fileMeta.textContent=file?file.name+" · "+formatBytes(file.size)+" · 分片上传，断线自动重试":"尚未选择文件";
});
form.addEventListener("submit",function(event){
  var file=fileInput.files&&fileInput.files[0];
  if(!file||uploading){return;}
  event.preventDefault();
  uploading=true;
  errorBox.hidden=true;
  progressWrap.hidden=false;
  progressBar.value=0;
  progressPercent.textContent="0%";
  progressStatus.textContent="正在建立安全上传…";
  submitButton.disabled=true;
  submitButton.textContent="正在上传";
  var started=Date.now();
  var csrf=form.elements.csrf.value;
  var initData=new URLSearchParams();
  initData.set("csrf",csrf);
  initData.set("name",file.name);
  initData.set("size",String(file.size));
  initData.set("password",form.elements.password.value);
  initData.set("expires_hours",form.elements.expires_hours.value);
  postForm(form.action+"/init",initData,function(initError,initRequest){
    if(initError){showError("网络连接失败，未开始上传，请重试。");return;}
    if(initRequest.status<200||initRequest.status>=300){showError("无法开始上传（HTTP "+initRequest.status+"）。");return;}
    var init=parseJSON(initRequest);
    if(!init.id||!init.chunk_size){showError("服务器返回了无效的上传信息。");return;}
    uploadChunk(init.id,Number(init.offset)||0,Number(init.chunk_size),0);
  });
  function uploadChunk(uploadID,offset,chunkSize,retry){
    if(offset>=file.size){finishUpload(uploadID);return;}
    var end=Math.min(offset+chunkSize,file.size);
    var request=new XMLHttpRequest();
    request.open("POST",form.action+"/chunk/"+encodeURIComponent(uploadID),true);
    request.setRequestHeader("Content-Type","application/octet-stream");
    request.setRequestHeader("X-CSRF-Token",csrf);
    request.setRequestHeader("X-Upload-Offset",String(offset));
    request.upload.addEventListener("progress",function(progress){
      if(progress.lengthComputable){setProgress(offset+progress.loaded,file.size,started);}
    });
    request.addEventListener("load",function(){
      var result=parseJSON(request);
      if(request.status>=200&&request.status<300&&Number(result.offset)>offset){
        uploadChunk(uploadID,Number(result.offset),chunkSize,0);
        return;
      }
      if(request.status===409&&Number(result.offset)>=offset){
        uploadChunk(uploadID,Number(result.offset),chunkSize,0);
        return;
      }
      if((request.status===429||request.status>=500)&&retry<maxRetries){retryChunk(uploadID,offset,chunkSize,retry);return;}
      showError("分片上传失败（HTTP "+request.status+"），请重试。");
    });
    request.addEventListener("error",function(){
      if(retry<maxRetries){retryChunk(uploadID,offset,chunkSize,retry);return;}
      showError("网络多次中断，上传停在 "+progressPercent.textContent+"，请重试。");
    });
    request.send(file.slice(offset,end));
  }
  function retryChunk(uploadID,offset,chunkSize,retry){
    var nextRetry=retry+1;
    progressStatus.textContent="网络中断，正在自动重试（"+nextRetry+"/"+maxRetries+"）…";
    window.setTimeout(function(){uploadChunk(uploadID,offset,chunkSize,nextRetry);},Math.pow(2,retry)*1000);
  }
  function finishUpload(uploadID){
    setProgress(file.size,file.size,started,"文件已上传，正在校验并创建分享链接…");
    var finishData=new URLSearchParams();
    finishData.set("csrf",csrf);
    postForm(form.action+"/finish/"+encodeURIComponent(uploadID),finishData,function(error,request){
      if(error){showError("文件已传完，但创建分享链接失败，请重试。");return;}
      if(request.status>=200&&request.status<300){
        progressStatus.textContent="上传成功，分享链接已创建";
        uploading=false;
        submitButton.textContent="上传完成";
        window.location.replace(parseJSON(request).success_url||form.dataset.successUrl);
        return;
      }
      showError("创建分享链接失败（HTTP "+request.status+"）。");
    });
  }
});
window.addEventListener("beforeunload",function(event){
  if(!uploading){return;}
  event.preventDefault();
  event.returnValue="";
});
})();`

var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>文件中转 · 管理登录</title><style>` + pageStyle + `</style></head><body><main class="shell compact"><section class="panel login-card"><div class="brand-mark">↗</div><div style="height:18px"></div><h1>管理登录</h1><p>登录后上传和管理文件分享。</p>{{if .Error}}<div class="error">{{.Error}}</div>{{end}}<form method="post" action="{{.BasePath}}/admin/login"><label for="password">管理员密码</label><input id="password" type="password" name="password" autocomplete="current-password" required autofocus><button class="button" type="submit">登录</button></form></section></main></body></html>`))

type loginView struct{ BasePath, Error string }

var adminTemplate = template.Must(template.New("admin").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>文件中转 · 管理</title><style>` + pageStyle + `</style><script src="{{.BasePath}}/assets/admin.js" defer></script></head><body><main class="shell"><header class="topbar"><div class="brand"><div class="brand-mark">↗</div><div><h1>文件中转</h1><p>仅你可以上传，收件人只能下载</p></div></div><form method="post" action="{{.BasePath}}/admin/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="button-secondary" type="submit">退出登录</button></form></header>{{if .Notice}}<div class="notice" role="status">{{.Notice}}</div>{{end}}<section class="panel"><header class="section-head"><div><h2>创建分享</h2><p>选择文件，按需设置下载密码和有效期。</p></div></header><form id="upload-form" method="post" action="{{.BasePath}}/admin/upload" data-success-url="{{.BasePath}}/admin?uploaded=1" enctype="multipart/form-data"><input type="hidden" name="csrf" value="{{.CSRF}}"><div class="field"><label for="file">文件 <span class="hint">· 最大 {{.MaxUpload}}</span></label><div class="file-box"><input id="file" type="file" name="file" required></div><div id="file-meta" class="file-meta">尚未选择文件</div></div><div class="settings-grid"><div class="field"><label for="password">下载密码 <span class="hint">· 可选，至少 12 位</span></label><input id="password" type="password" name="password" minlength="12" autocomplete="new-password" placeholder="留空表示无密码"></div><div class="field"><label for="expires">有效时间 <span class="hint">· 小时</span></label><input id="expires" type="number" name="expires_hours" value="24" min="1" max="720" required></div></div><div id="upload-error" class="error" role="alert" hidden></div><div id="upload-progress" class="progress-wrap" aria-live="polite" hidden><div class="progress-top"><span>文件上传</span><span id="upload-percent">0%</span></div><progress id="upload-bar" max="100" value="0"></progress><div id="upload-status" class="progress-status">准备上传…</div></div><footer class="form-footer"><div class="form-note">同一 IP 最多创建 3 次下载授权</div><button id="upload-button" class="button" type="submit">上传并创建链接</button></footer></form></section><section class="panel"><header class="section-head"><div><h2>分享记录</h2><p>链接不可枚举，过期文件会自动清理。</p></div><span class="count">{{len .Shares}}</span></header>{{if not .Shares}}<div class="empty">还没有分享，上传第一个文件吧。</div>{{else}}<div class="share-list">{{range .Shares}}<article class="share-row"><div><div class="share-title"><strong>{{.Name}}</strong></div><a class="share-link" href="{{.URL}}" target="_blank" rel="noreferrer">{{.URL}}</a><div class="badges">{{if .Protected}}<span class="badge">密码保护</span>{{else}}<span class="badge">无需密码</span>{{end}}{{if .Expired}}<span class="badge">已过期</span>{{end}}<span class="badge">{{.Size}}</span></div><div class="meta">创建 {{.Created}} · 到期 {{.Expires}} · 已授权 {{.Downloads}} 次</div></div><div class="share-actions"><form method="post" action="{{$.BasePath}}/admin/delete"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><button class="button-danger" type="submit">删除</button></form></div></article>{{end}}</div>{{end}}</section></main></body></html>`))

type adminShareView struct {
	ID, Name, Size, Created, Expires, URL string
	Protected, Expired                    bool
	Downloads                             int
}
type adminView struct {
	BasePath, CSRF, MaxUpload, Notice string
	Shares                            []adminShareView
}

var shareTemplate = template.Must(template.New("share").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>下载 {{.Name}}</title><style>` + pageStyle + `</style></head><body><main class="shell compact"><section class="panel download-card"><div class="download-icon">↓</div><h1>{{.Name}}</h1><div class="meta">{{.Size}} · 有效期至 {{.Expires}}</div><form method="post" action="{{.BasePath}}/s/{{.ID}}/authorize">{{if .Protected}}<label for="password">下载密码</label><input id="password" type="password" name="password" autocomplete="current-password" required autofocus placeholder="输入分享密码">{{else}}<p class="meta">此文件无需密码，点击即可开始下载。</p>{{end}}<button class="button" type="submit">开始下载</button></form></section></main></body></html>`))

type shareView struct {
	BasePath, ID, Name, Size, Expires string
	Protected                         bool
}

var messageTemplate = template.Must(template.New("message").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>` + pageStyle + `</style></head><body><main class="shell compact"><section class="panel download-card"><h1>{{.Title}}</h1><p class="meta">{{.Message}}</p></section></main></body></html>`))

type messageView struct{ Title, Message string }

func render(w http.ResponseWriter, tmpl *template.Template, data any) {
	renderStatus(w, tmpl, data, http.StatusOK)
}

func renderStatus(w http.ResponseWriter, tmpl *template.Template, data any, status int) {
	var output strings.Builder
	if err := tmpl.Execute(&output, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	body := output.String()
	if tmpl.Name() == "admin" {
		body = strings.Replace(body, "/assets/admin.js", "/assets/admin-v3.js", 1)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
