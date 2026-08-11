package main

import (
	"html/template"
	"net/http"
	"strings"
)

const pageStyle = `
:root{color-scheme:light;--bg:#f5f7fa;--surface:#fff;--surface-soft:#f8fafc;--text:#172033;--muted:#68758a;--line:#e2e7ee;--accent:#2563eb;--accent-hover:#1d4ed8;--accent-soft:#eff6ff;--success:#087a55;--success-bg:#ecfdf5;--danger:#c33b4a;--danger-bg:#fff1f2;--shadow:0 12px 35px rgba(23,32,51,.07)}*{box-sizing:border-box}[hidden]{display:none!important}html{background:var(--bg)}body{margin:0;color:var(--text);font:15px/1.55 Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif;min-height:100vh}.shell{width:min(980px,calc(100% - 32px));margin:0 auto;padding:40px 0 64px}.compact{width:min(460px,calc(100% - 32px));padding-top:10vh}.topbar{display:flex;align-items:center;justify-content:space-between;gap:24px;margin-bottom:24px}.brand{display:flex;align-items:center;gap:13px}.brand-mark{display:grid;place-items:center;width:42px;height:42px;border-radius:12px;background:var(--text);color:#fff;font-size:20px;font-weight:800}.brand h1{font-size:24px;line-height:1.2;margin:0}.brand p{margin:4px 0 0;color:var(--muted)}.panel{background:var(--surface);border:1px solid var(--line);border-radius:16px;box-shadow:var(--shadow);padding:24px}.panel+.panel{margin-top:20px}.section-head{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;margin-bottom:20px}.section-head h2{font-size:18px;margin:0}.section-head p{color:var(--muted);margin:4px 0 0}.count{min-width:28px;height:28px;padding:3px 9px;border-radius:999px;background:var(--surface-soft);color:var(--muted);text-align:center;font-weight:700}.field{margin-top:16px}.field:first-child{margin-top:0}label{display:block;margin-bottom:7px;font-weight:650}.hint{color:var(--muted);font-size:13px;font-weight:400}.input,input[type=password],input[type=number]{width:100%;height:44px;padding:10px 12px;border:1px solid #d5dce6;border-radius:10px;background:#fff;color:var(--text);font:inherit;outline:none;transition:border-color .15s,box-shadow .15s}.input:focus,input:focus{border-color:#7ba4f8;box-shadow:0 0 0 3px rgba(37,99,235,.12)}.file-box{display:flex;align-items:center;gap:14px;padding:18px;border:1px dashed #b8c5d8;border-radius:12px;background:var(--surface-soft);transition:border-color .15s,background .15s}.file-box:hover{border-color:#7ba4f8;background:var(--accent-soft)}.file-box input{min-width:0;max-width:100%;font:inherit}.file-meta{margin-top:8px;color:var(--muted);font-size:13px}.settings-grid{display:grid;grid-template-columns:2fr 1fr;gap:16px}.form-footer{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-top:22px;padding-top:18px;border-top:1px solid var(--line)}.form-note{color:var(--muted);font-size:13px}.button,button{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:40px;padding:9px 15px;border:0;border-radius:9px;background:var(--accent);color:#fff;font:inherit;font-weight:750;text-decoration:none;cursor:pointer;transition:background .15s,opacity .15s}.button:hover,button:hover{background:var(--accent-hover)}button:disabled{cursor:not-allowed;opacity:.58}.button-secondary{background:#fff;color:var(--text);border:1px solid var(--line)}.button-secondary:hover{background:var(--surface-soft)}.button-danger{min-height:34px;padding:6px 10px;background:#fff;color:var(--danger);border:1px solid #f0c8ce}.button-danger:hover{background:var(--danger-bg)}.notice,.error{border-radius:11px;padding:12px 14px;margin-bottom:18px}.notice{color:var(--success);background:var(--success-bg);border:1px solid #b6ead8}.error{color:var(--danger);background:var(--danger-bg);border:1px solid #f3c2ca}.progress-wrap{margin-top:18px;padding:15px;border:1px solid #cfe0ff;border-radius:12px;background:var(--accent-soft)}.progress-top{display:flex;align-items:center;justify-content:space-between;margin-bottom:9px;font-weight:700}.progress-wrap progress{display:block;width:100%;height:9px;border:0;border-radius:999px;overflow:hidden;background:#dbe7fb}.progress-wrap progress::-webkit-progress-bar{background:#dbe7fb;border-radius:999px}.progress-wrap progress::-webkit-progress-value{background:var(--accent);border-radius:999px}.progress-wrap progress::-moz-progress-bar{background:var(--accent);border-radius:999px}.progress-status{color:var(--muted);font-size:13px;margin-top:8px}.empty{padding:30px 12px;text-align:center;color:var(--muted);border:1px dashed var(--line);border-radius:12px}.share-list{border-top:1px solid var(--line)}.share-row{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:18px;padding:18px 0;border-bottom:1px solid var(--line)}.share-row:last-child{padding-bottom:0;border-bottom:0}.share-title{display:flex;align-items:center;gap:8px;min-width:0}.share-title strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.badges{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}.badge{display:inline-flex;align-items:center;height:24px;padding:2px 8px;border-radius:999px;background:var(--surface-soft);border:1px solid var(--line);color:var(--muted);font-size:12px}.share-link{display:block;width:fit-content;max-width:100%;margin-top:7px;color:var(--accent);text-decoration:none;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.share-link:hover{text-decoration:underline}.meta{margin-top:7px;color:var(--muted);font-size:13px}.share-actions{display:flex;align-items:flex-start}.download-card{text-align:center}.download-icon{display:grid;place-items:center;width:58px;height:58px;margin:0 auto 17px;border-radius:16px;background:var(--accent-soft);color:var(--accent);font-size:25px;font-weight:800}.download-card h1{font-size:22px;margin:0;overflow-wrap:anywhere}.download-card .meta{margin:8px 0 22px}.download-card form{text-align:left}.download-card .button{width:100%;margin-top:18px}.login-card h1{font-size:22px;margin:0 0 4px}.login-card>p{margin:0 0 22px;color:var(--muted)}.login-card .button{width:100%;margin-top:18px}@media(max-width:680px){.shell{width:min(100% - 22px,980px);padding:22px 0 40px}.compact{padding-top:7vh}.topbar{align-items:flex-start}.brand p{font-size:13px}.panel{padding:18px;border-radius:14px}.settings-grid{grid-template-columns:1fr}.form-footer{align-items:stretch;flex-direction:column}.form-footer .button{width:100%}.share-row{grid-template-columns:1fr}.share-actions button{width:100%}.share-actions form{width:100%}.section-head{margin-bottom:16px}}`

const visualRefreshStyle = `
:root{--bg:#f3f6fb;--surface:rgba(255,255,255,.92);--surface-solid:#fff;--surface-soft:#f7f9fc;--text:#111b31;--muted:#64718a;--line:#dfe6f0;--accent:#315eea;--accent-hover:#244ed1;--accent-soft:#eef3ff;--success:#08755a;--success-bg:#eafbf4;--danger:#c93f50;--danger-bg:#fff1f3;--radius-xl:24px;--radius-lg:18px;--radius-md:13px;--shadow:0 18px 55px rgba(30,49,83,.09);--shadow-hover:0 24px 70px rgba(30,49,83,.14)}
html{background:var(--bg)}body{position:relative;isolation:isolate;background:linear-gradient(145deg,#f8faff 0%,#f2f5fa 48%,#eef3fb 100%);letter-spacing:.005em}body:before,body:after{content:"";position:fixed;z-index:-1;border-radius:999px;filter:blur(2px);pointer-events:none}body:before{width:420px;height:420px;left:-180px;top:-180px;background:radial-gradient(circle,rgba(49,94,234,.13),transparent 68%)}body:after{width:360px;height:360px;right:-140px;bottom:-160px;background:radial-gradient(circle,rgba(73,184,162,.1),transparent 68%)}
.shell{width:min(1080px,calc(100% - 40px));padding:44px 0 72px}.compact{width:min(460px,calc(100% - 32px));padding-top:10vh}.topbar{margin-bottom:28px;padding:0 4px}.brand{gap:15px}.brand-mark{width:48px;height:48px;border-radius:16px;background:linear-gradient(145deg,#16233b,#273c62);box-shadow:0 10px 24px rgba(22,35,59,.18);transform:rotate(-2deg)}.brand h1{font-size:26px;letter-spacing:-.035em}.brand p{margin-top:3px}.panel{position:relative;overflow:hidden;padding:28px;border:1px solid rgba(210,220,234,.92);border-radius:var(--radius-xl);background:var(--surface);box-shadow:var(--shadow);animation:card-enter .42s cubic-bezier(.2,.8,.2,1) both;transition:transform .24s ease,box-shadow .24s ease,border-color .24s ease}.panel:before{content:"";position:absolute;inset:0 0 auto;height:1px;background:linear-gradient(90deg,transparent,rgba(255,255,255,.95),transparent)}.panel:hover{transform:translateY(-2px);box-shadow:var(--shadow-hover);border-color:#cfd9e8}.panel+.panel{margin-top:24px;animation-delay:.08s}.section-head{margin-bottom:24px}.section-head h2{font-size:20px;letter-spacing:-.02em}.section-head p{margin-top:5px}.count{display:grid;place-items:center;min-width:32px;height:32px;padding:0 10px;background:var(--accent-soft);color:var(--accent);border:1px solid #dce6ff}
.file-box{min-height:76px;padding:13px 16px;border:1.5px dashed #b8c8e1;border-radius:var(--radius-lg);background:linear-gradient(135deg,#f9fbff,#f5f8fd)}.file-box:focus-within,.file-box:hover{border-color:#7697ef;background:#f3f7ff}.file-box input{width:100%;color:var(--text)}.file-box input::file-selector-button{margin-right:14px;padding:10px 14px;border:1px solid #cfdaea;border-radius:11px;background:#fff;color:var(--text);font:inherit;font-weight:700;cursor:pointer;box-shadow:0 3px 10px rgba(30,49,83,.06);transition:transform .16s ease,border-color .16s ease,background .16s ease}.file-box input::file-selector-button:hover{transform:translateY(-1px);border-color:#9db2d3;background:#f9fbff}.file-meta{margin-top:9px}.settings-grid{gap:18px}.input,input[type=password],input[type=number]{height:48px;border-radius:var(--radius-md);background:rgba(255,255,255,.88)}
.button,button{min-height:44px;padding:10px 17px;border-radius:12px;box-shadow:0 7px 18px rgba(49,94,234,.18);transition:transform .16s ease,box-shadow .16s ease,background .16s ease,opacity .16s ease}.button:hover,button:hover{transform:translateY(-1px);box-shadow:0 10px 24px rgba(49,94,234,.24)}.button:active,button:active{transform:translateY(0)}.button-secondary{box-shadow:0 5px 16px rgba(30,49,83,.07);background:rgba(255,255,255,.8)}.button-danger{box-shadow:none}.form-footer{margin-top:26px;padding-top:22px}.notice{border-radius:15px;box-shadow:0 8px 22px rgba(8,117,90,.08);animation:notice-enter .32s ease both}.progress-wrap{border-radius:var(--radius-lg);padding:17px;background:linear-gradient(135deg,#f1f5ff,#edf4ff)}.progress-wrap progress{height:10px}.empty{padding:42px 16px;border-radius:var(--radius-lg);background:rgba(248,250,253,.58)}
.share-list{border:0}.share-row{margin-top:12px;padding:18px;border:1px solid var(--line);border-radius:var(--radius-lg);background:var(--surface-solid);transition:transform .18s ease,border-color .18s ease,box-shadow .18s ease}.share-row:hover{transform:translateY(-2px);border-color:#cbd7e8;box-shadow:0 12px 28px rgba(30,49,83,.08)}.share-row:last-child{padding-bottom:18px;border-bottom:1px solid var(--line)}.share-title strong{font-size:16px}.badge{height:26px;border-radius:9px;background:#f4f7fb}.login-card{padding:32px}.download-card{padding:34px}
.system-dialog[hidden]{display:none!important}.system-dialog{position:fixed;inset:0;z-index:1000;display:grid;place-items:center;padding:20px;background:rgba(13,23,42,.42);backdrop-filter:blur(8px);opacity:0;transition:opacity .18s ease}.system-dialog.is-open{opacity:1}.dialog-card{width:min(430px,100%);padding:24px;border:1px solid rgba(255,255,255,.82);border-radius:22px;background:#fff;box-shadow:0 28px 90px rgba(10,23,48,.28);transform:translateY(10px) scale(.98);transition:transform .22s cubic-bezier(.2,.8,.2,1)}.system-dialog.is-open .dialog-card{transform:translateY(0) scale(1)}.dialog-icon{display:grid;place-items:center;width:42px;height:42px;margin-bottom:16px;border-radius:13px;background:var(--accent-soft);color:var(--accent);font-size:20px;font-weight:900}.dialog-card h3{margin:0;font-size:20px}.dialog-card p{margin:8px 0 22px;color:var(--muted)}.dialog-actions{display:flex;justify-content:flex-end;gap:10px}.dialog-actions .button-secondary,.dialog-actions .button-danger{min-width:88px}.dialog-actions .button-danger{background:var(--danger);color:#fff;border-color:var(--danger);box-shadow:0 7px 18px rgba(201,63,80,.18)}
@keyframes card-enter{from{opacity:0;transform:translateY(16px) scale(.992)}to{opacity:1;transform:translateY(0) scale(1)}}@keyframes notice-enter{from{opacity:0;transform:translateY(-8px)}to{opacity:1;transform:translateY(0)}}
@media(max-width:680px){.shell{width:min(100% - 20px,1080px);padding:22px 0 44px}.topbar{margin-bottom:20px}.brand-mark{width:44px;height:44px;border-radius:14px}.brand h1{font-size:22px}.panel{padding:19px;border-radius:20px}.panel:hover{transform:none}.file-box{padding:12px}.dialog-card{padding:21px}.dialog-actions{display:grid;grid-template-columns:1fr 1fr}.dialog-actions button{width:100%}}
@media(prefers-reduced-motion:reduce){*,*:before,*:after{scroll-behavior:auto!important;animation-duration:.01ms!important;animation-iteration-count:1!important;transition-duration:.01ms!important}}
`

const systemDialogHTML = `<div id="system-dialog" class="system-dialog" hidden role="dialog" aria-modal="true" aria-labelledby="dialog-title" aria-describedby="dialog-message"><div class="dialog-card"><div class="dialog-icon">!</div><h3 id="dialog-title">请确认</h3><p id="dialog-message"></p><div class="dialog-actions"><button id="dialog-cancel" class="button-secondary" type="button">取消</button><button id="dialog-confirm" class="button-danger" type="button">确认</button></div></div></div>`

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
var dialog=document.getElementById("system-dialog");
var dialogTitle=document.getElementById("dialog-title");
var dialogMessage=document.getElementById("dialog-message");
var dialogCancel=document.getElementById("dialog-cancel");
var dialogConfirm=document.getElementById("dialog-confirm");
var uploading=false;
var maxRetries=3;
var pendingForm=null;
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
function openDialog(targetForm,title,message,confirmText){
  pendingForm=targetForm;
  dialogTitle.textContent=title;
  dialogMessage.textContent=message;
  dialogConfirm.textContent=confirmText||"确认";
  dialog.hidden=false;
  window.requestAnimationFrame(function(){dialog.classList.add("is-open");dialogCancel.focus();});
}
function closeDialog(){
  dialog.classList.remove("is-open");
  pendingForm=null;
  window.setTimeout(function(){dialog.hidden=true;},180);
}
dialogCancel.addEventListener("click",closeDialog);
dialog.addEventListener("click",function(event){if(event.target===dialog){closeDialog();}});
dialogConfirm.addEventListener("click",function(){
  var targetForm=pendingForm;
  closeDialog();
  if(targetForm){targetForm.submit();}
});
document.addEventListener("keydown",function(event){if(event.key==="Escape"&&!dialog.hidden){closeDialog();}});
document.querySelectorAll('form[action$="/admin/delete"]').forEach(function(deleteForm){
  deleteForm.addEventListener("submit",function(event){
    event.preventDefault();
    openDialog(deleteForm,"删除这条分享？",uploading?"当前上传会被中断，分享文件删除后无法恢复。":"分享链接和服务器文件将永久删除，无法恢复。","删除");
  });
});
var logoutForm=document.querySelector('form[action$="/admin/logout"]');
if(logoutForm){logoutForm.addEventListener("submit",function(event){
  if(!uploading){return;}
  event.preventDefault();
  openDialog(logoutForm,"上传仍在进行","退出登录会中断当前上传，确定要离开吗？","退出");
});}
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
	body = strings.Replace(body, "</style>", visualRefreshStyle+"</style>", 1)
	if tmpl.Name() == "admin" {
		if view, ok := data.(adminView); ok {
			externalScript := `<script src="` + view.BasePath + `/assets/admin.js" defer></script>`
			body = strings.Replace(body, externalScript, "", 1)
		}
		body = strings.Replace(body, "</body>", systemDialogHTML+"<script>"+adminJS+"</script></body>", 1)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
