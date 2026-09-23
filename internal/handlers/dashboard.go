package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterDashboard mounts the device dashboard.
// Page is public like /docs; API calls use a user-entered Bearer via Authorization header.
// WS live updates reuse the same token via ?token= compat only (documented, user input only).
func RegisterDashboard(r *gin.Engine) {
	r.GET("/dashboard", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, dashboardHTML)
	})
	r.GET("/manifest.json", func(c *gin.Context) {
		c.Header("Content-Type", "application/manifest+json")
		c.String(http.StatusOK, dashboardManifest)
	})
}

// dashboardManifest makes the dashboard installable as a minimal PWA.
const dashboardManifest = `{"name":"hello","short_name":"hello",` +
	`"description":"Wake-on-LAN service","start_url":"/dashboard","scope":"/",` +
	`"display":"standalone","background_color":"#fafaf9","theme_color":"#1c1917"}`

const dashboardHTML = `<!doctype html>
<html lang="en" class="h-full">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#1c1917">
<link rel="manifest" href="/manifest.json">
<title>hello</title>
<script src="https://cdn.tailwindcss.com"></script>
<script>tailwind.config={theme:{extend:{fontFamily:{sans:['system-ui','-apple-system','Segoe UI','Roboto','sans-serif']}}}}</script>
<style>:root{color-scheme:light dark}</style>
</head>
<body class="h-full bg-stone-100 text-stone-900 antialiased dark:bg-stone-950 dark:text-stone-100">
<div class="mx-auto max-w-5xl px-4 sm:px-6 py-8">
<header class="flex flex-wrap items-end justify-between gap-4">
<div><p class="text-xs uppercase tracking-widest text-stone-500">Wake-on-LAN</p>
<h1 class="text-2xl font-semibold tracking-tight">hello</h1></div>
<nav class="flex items-center gap-2 text-sm"><a class="rounded-lg border border-stone-300 px-3 py-1.5 hover:bg-stone-200/60 dark:border-stone-700 dark:hover:bg-stone-800" href="/docs">API docs</a><a class="rounded-lg border border-stone-300 px-3 py-1.5 hover:bg-stone-200/60 dark:border-stone-700 dark:hover:bg-stone-800" href="/openapi.yaml">OpenAPI</a></nav>
</header>
<section class="mt-6 grid grid-cols-3 gap-3" aria-label="Overview">
<div class="rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900"><p class="text-xs uppercase tracking-wider text-stone-500">Up</p><p id="statUp" class="mt-1 text-2xl font-semibold">–</p></div>
<div class="rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900"><p class="text-xs uppercase tracking-wider text-stone-500">Down</p><p id="statDown" class="mt-1 text-2xl font-semibold">–</p></div>
<div class="rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900"><p class="text-xs uppercase tracking-wider text-stone-500">Unknown</p><p id="statUnknown" class="mt-1 text-2xl font-semibold">–</p></div>
</section>
<section class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<label for="token" class="block text-xs font-medium uppercase tracking-wider text-stone-500">API token — memory only, never in URL</label>
<div class="mt-2 flex flex-col sm:flex-row gap-2"><input id="token" type="password" autocomplete="off" placeholder="Bearer token" class="w-full rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm outline-none focus:border-stone-500 dark:border-stone-700"><button id="connect" class="rounded-xl bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-700 disabled:opacity-50 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white">Connect</button></div>
</section>
<section class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<div class="flex flex-col sm:flex-row gap-2 sm:items-center"><input id="search" type="search" placeholder="Filter by name, MAC, IP…" class="w-full rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm outline-none focus:border-stone-500 dark:border-stone-700"><div id="filters" class="flex gap-1 text-sm" role="group" aria-label="Filter by status"><button data-f="all" class="rounded-lg px-3 py-1.5 bg-stone-900 text-white dark:bg-stone-100 dark:text-stone-900">All</button><button data-f="up" class="rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700">Up</button><button data-f="down" class="rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700">Down</button><button data-f="unknown" class="rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700">Unknown</button></div></div>
<div id="msg" role="status" class="min-h-6 mt-3 text-sm text-stone-500"></div>
<ul id="rows" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-6 text-sm text-stone-500">Enter your token, then Connect.</li></ul>
</section>
<section class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<h2 class="text-sm font-semibold">Wake history</h2>
<p class="mt-1 text-xs text-stone-500">Latest wake events, newest first.</p>
<ul id="hist" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-4 text-sm text-stone-500">Connect to see history.</li></ul>
</section>
<section class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<h2 class="text-sm font-semibold">Schedules</h2>
<p class="mt-1 text-xs text-stone-500">Cron uses 5 fields (ex: 0 7 * * 1-5). New schedules start enabled.</p>
<ul id="scheds" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-4 text-sm text-stone-500">Connect to see schedules.</li></ul>
<form id="schedCreate" class="mt-2 grid grid-cols-1 sm:grid-cols-4 gap-2"><input id="sId" required placeholder="id (ex: nas-morning)" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><select id="sDev" required class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"></select><input id="sCron" required placeholder="cron 0 7 * * *" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700"><button class="rounded-xl border border-stone-300 px-4 py-2 text-sm font-medium hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Add schedule</button></form>
</section>
<section class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<h2 class="text-sm font-semibold">Discover</h2>
<p class="mt-1 text-xs text-stone-500">Scan the LAN, then adopt what you recognise. One scan every 30s.</p>
<div class="mt-2 flex flex-col sm:flex-row gap-2"><button id="scan" class="rounded-xl bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-700 disabled:opacity-50 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white">Scan the LAN</button><button id="adoptSel" disabled class="rounded-xl border border-stone-300 px-4 py-2 text-sm font-medium hover:bg-stone-100 disabled:opacity-50 dark:border-stone-700 dark:hover:bg-stone-800">Adopt selected</button></div>
<div id="scanMsg" role="status" class="min-h-6 mt-2 text-sm text-stone-500"></div>
<ul id="found" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-4 text-sm text-stone-500">No scan yet.</li></ul>
</section>
<section class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<h2 class="text-sm font-semibold">Add a device</h2>
<p class="mt-1 text-xs text-stone-500">ID and MAC are required.</p>
<form id="create" class="mt-2 grid grid-cols-1 sm:grid-cols-4 gap-2"><input id="cId" required placeholder="id (ex: nas)" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><input id="cName" required placeholder="name" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><input id="cMac" required placeholder="MAC AA:BB:CC:DD:EE:FF" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700"><input id="cIp" placeholder="IP (optional)" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700"><button class="sm:col-span-4 rounded-xl border border-stone-300 px-4 py-2 text-sm font-medium hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Add device</button></form>
</section>
<dialog id="editDlg" class="rounded-2xl border border-stone-200 bg-white p-4 text-stone-900 backdrop:bg-black/40 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-100">
<form id="editForm" method="dialog" class="grid grid-cols-1 gap-2 sm:min-w-96"><h2 class="text-sm font-semibold">Edit device</h2>
<input id="eName" required placeholder="name" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700">
<input id="eMac" required placeholder="MAC AA:BB:CC:DD:EE:FF" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700">
<input id="eIp" placeholder="IP (optional)" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700">
<label class="flex items-center gap-2 text-sm"><input id="ePing" type="checkbox" class="h-4 w-4"> Ping enabled</label>
<div class="flex justify-end gap-2"><button value="cancel" formnovalidate class="rounded-xl border border-stone-300 px-4 py-2 text-sm dark:border-stone-700">Cancel</button><button id="eSave" value="default" class="rounded-xl bg-stone-900 px-4 py-2 text-sm font-medium text-white dark:bg-stone-100 dark:text-stone-900">Save</button></div></form>
</dialog>
<footer class="mt-6 text-xs text-stone-500">hello — Wake-on-LAN service.</footer>
</div>
<script>
const $=id=>document.getElementById(id);
const say=t=>{$('msg').textContent=t||''};
let devices=[],filter='all',ws=null,editing=null,history=[],schedules=[];
const token=()=>$('token').value.trim();
async function api(path,opts={}){
  const t=token();if(!t)throw new Error('Enter API token first');
  const res=await fetch(path,{...opts,headers:{'Content-Type':'application/json','Authorization':'Bearer '+t,...(opts.headers||{})}});
  if(res.status===204)return null;
  const data=await res.json().catch(()=>({}));
  if(!res.ok)throw new Error(data.detail||data.title||('HTTP '+res.status));
  return data;
}
function pill(st){const base='inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ';if(st==='up')return base+'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200';if(st==='down')return base+'bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-200';return base+'bg-stone-200/70 text-stone-600 dark:bg-stone-800 dark:text-stone-300';}
function dot(st){if(st==='up')return 'h-1.5 w-1.5 rounded-full bg-emerald-500';if(st==='down')return 'h-1.5 w-1.5 rounded-full bg-rose-500';return 'h-1.5 w-1.5 rounded-full bg-stone-400';}
async function load(silent){
  if(!silent)say('');
  try{
    const data=await api('/devices');devices=data.devices||data||[];
    const up=devices.filter(d=>(d.status||'unknown')==='up').length,down=devices.filter(d=>(d.status||'unknown')==='down').length;
    $('statUp').textContent=up;$('statDown').textContent=down;$('statUnknown').textContent=devices.length-up-down;
    render();loadHistory();loadSchedules();
  }catch(e){if(!silent){$('rows').innerHTML='<li class="py-6 text-sm text-rose-600">Error — '+String(e.message||e)+'</li>';}}
}
function render(){
  const q=($('search').value||'').toLowerCase(),ul=$('rows');ul.innerHTML='';
  const list=devices.filter(d=>{const st=(d.status||'unknown');if(filter!=='all'&&st!==filter)return false;return ((d.name||'')+' '+(d.id||'')+' '+(d.mac||d.MAC||'')+' '+(d.ip||'')).toLowerCase().includes(q);});
  if(!list.length){ul.innerHTML='<li class="py-6 text-sm text-stone-500">Nothing here — adjust filter or add a device.</li>';return;}
  for(const d of list){
    const st=(d.status||'unknown'),id=d.id||'';
    const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-3';
    li.innerHTML='<div class="min-w-0 flex-1"><p class="truncate text-sm font-medium"></p><p class="truncate font-mono text-xs text-stone-500"></p></div><span></span><span class="text-xs text-stone-500"></span>';
    li.children[0].children[0].textContent=d.name||id;li.children[0].children[1].textContent=(d.mac||d.MAC||'')+(d.ip?' · '+d.ip:'');
    li.children[1].className=pill(st);li.children[1].innerHTML='<span class="'+dot(st)+'"></span>'+st;
    li.children[2].textContent=d.last_seen||d.lastSeen||'';
    const w=document.createElement('button');w.className='rounded-xl bg-stone-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-stone-700 disabled:opacity-50 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white';w.textContent='Wake';
    w.onclick=async()=>{w.disabled=true;try{await api('/devices/'+encodeURIComponent(id)+'/wake',{method:'POST'});say('Magic packet sent to '+(d.name||id));setTimeout(load,1500);}catch(e){say(String(e.message||e));}finally{w.disabled=false;}};
    const e=document.createElement('button');e.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';e.textContent='Edit';
    e.onclick=()=>{editing=d;$('eName').value=d.name||'';$('eMac').value=d.mac||d.MAC||'';$('eIp').value=d.ip||'';$('ePing').checked=!!d.ping_enabled;$('editDlg').showModal();};
    const del=document.createElement('button');del.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';del.textContent='Delete';
    del.onclick=async()=>{if(!confirm('Delete '+(d.name||id)+'?'))return;try{await api('/devices/'+encodeURIComponent(id),{method:'DELETE'});say('Deleted '+(d.name||id));load(true);}catch(e){say(String(e.message||e));}};
    li.append(w,e,del);ul.appendChild(li);
  }
}
function nameOf(id){const d=devices.find(x=>(x.id||'')===id);return d?(d.name||d.id||id):id;}
function fmtTime(at){try{return new Date(Number(at)*1000).toLocaleString();}catch{return '';}}
async function loadHistory(){
  const ul=$('hist');if(!ul)return;
  try{
    const data=await api('/history?limit=20');history=data.history||[];ul.innerHTML='';
    if(!history.length){ul.innerHTML='<li class="py-4 text-sm text-stone-500">No wake events yet.</li>';return;}
    for(const h of history){
      const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-2';
      li.innerHTML='<div class="min-w-0 flex-1"><p class="truncate text-sm font-medium"></p><p class="truncate text-xs text-stone-500"></p></div><span></span>';
      li.children[0].children[0].textContent=nameOf(h.device_id||'');
      li.children[0].children[1].textContent=(h.trigger||'manual')+' · '+fmtTime(h.at);
      li.children[1].className=h.success?'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200':'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-200';
      li.children[1].textContent=h.success?'sent':'failed';
      ul.appendChild(li);
    }
  }catch(e){ul.innerHTML='<li class="py-4 text-sm text-stone-500">History unavailable.</li>';}
}
async function loadSchedules(){
  const ul=$('scheds');if(!ul)return;
  try{
    const data=await api('/schedules');schedules=data.schedules||[];
    const sel=$('sDev');if(sel){const cur=sel.value;sel.innerHTML='';for(const d of devices){const o=document.createElement('option');o.value=d.id||'';o.textContent=(d.name||d.id||'')+' ('+(d.id||'')+')';sel.appendChild(o);}if(cur)sel.value=cur;}
    ul.innerHTML='';
    if(!schedules.length){ul.innerHTML='<li class="py-4 text-sm text-stone-500">No schedules yet.</li>';return;}
    for(const s of schedules){
      const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-2';
      li.innerHTML='<div class="min-w-0 flex-1"><p class="truncate text-sm font-medium"></p><p class="truncate font-mono text-xs text-stone-500"></p></div>';
      li.children[0].children[0].textContent=nameOf(s.device_id||'');
      li.children[0].children[1].textContent=s.cron||'';
      const t=document.createElement('button');t.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';t.textContent=s.enabled?'On':'Off';
      t.onclick=async()=>{t.disabled=true;try{await api('/schedules/'+encodeURIComponent(s.id),{method:'PUT',body:JSON.stringify({id:s.id,device_id:s.device_id,cron:s.cron,enabled:!s.enabled})});loadSchedules();}catch(e){say(String(e.message||e));}finally{t.disabled=false;}};
      const del=document.createElement('button');del.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';del.textContent='Delete';
      del.onclick=async()=>{if(!confirm('Delete schedule '+s.id+'?'))return;try{await api('/schedules/'+encodeURIComponent(s.id),{method:'DELETE'});loadSchedules();}catch(e){say(String(e.message||e));}};
      li.append(t,del);ul.appendChild(li);
    }
  }catch(e){ul.innerHTML='<li class="py-4 text-sm text-stone-500">Schedules unavailable.</li>';}
}
let found=[];
function scanMsg(t){$('scanMsg').textContent=t||'';}
function renderFound(){
  const ul=$('found');ul.innerHTML='';
  if(!found.length){ul.innerHTML='<li class="py-4 text-sm text-stone-500">No hosts found.</li>';$('adoptSel').disabled=true;return;}
  let selectable=0;
  for(const h of found){
    const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-2';
    const canAdopt=h.mac&&!h.known;
    if(canAdopt)selectable++;
    li.innerHTML='<div class="min-w-0 flex-1"><p class="truncate font-mono text-sm"></p><p class="truncate text-xs text-stone-500"></p></div><span class="text-xs"></span>';
    li.children[0].children[0].textContent=h.mac||'(no MAC)';li.children[0].children[1].textContent=[h.ip,h.hostname].filter(Boolean).join(' · ');
    li.children[1].textContent=h.known?'known':'new';
    if(canAdopt){const cb=document.createElement('input');cb.type='checkbox';cb.className='h-4 w-4';cb.dataset.mac=h.mac;cb.dataset.ip=h.ip||'';cb.dataset.hostname=h.hostname||'';cb.onchange=()=>{$('adoptSel').disabled=!document.querySelectorAll('#found input:checked').length;};li.prepend(cb);}else{li.children[1].className+=' text-stone-500';}
    ul.appendChild(li);
  }
  $('adoptSel').disabled=!selectable;
}
async function scan(){
  const b=$('scan');b.disabled=true;scanMsg('Scanning the LAN… (up to 90s)');
  try{
    const data=await api('/discover',{method:'POST'});found=data.hosts||[];renderFound();
    scanMsg(found.length?found.length+' host(s) found.':'No hosts found.');
  }catch(e){scanMsg(String(e.message||e));}finally{b.disabled=false;}
}
async function adoptSelected(){
  const sel=[...document.querySelectorAll('#found input:checked')].map(cb=>({mac:cb.dataset.mac,ip:cb.dataset.ip||undefined,hostname:cb.dataset.hostname||undefined}));
  if(!sel.length)return;
  const b=$('adoptSel');b.disabled=true;
  try{const data=await api('/discover/adopt',{method:'POST',body:JSON.stringify({hosts:sel})});say('Adopted '+(data.devices||[]).length+' device(s).');load(true);scanMsg('Adopted. Rescan to refresh.');}
  catch(e){scanMsg(String(e.message||e));}finally{b.disabled=false;}
}
$('scan').onclick=scan;
$('adoptSel').onclick=adoptSelected;
function live(){
  try{if(ws)ws.close()}catch{}
  const t=token();if(!t)return;
  ws=new WebSocket((location.protocol==='https:'?'wss://':'ws://')+location.host+'/ws?token='+encodeURIComponent(t));
  ws.onmessage=()=>load(true);ws.onclose=()=>{setTimeout(()=>{if(token())live();},5000);};
}
$('connect').onclick=()=>{say('Connecting…');load().then(()=>{say('Connected. Live updates on.');live();});};
$('search').oninput=render;
document.querySelectorAll('#filters button').forEach(b=>b.onclick=()=>{filter=b.dataset.f;document.querySelectorAll('#filters button').forEach(x=>x.className='rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700');b.className='rounded-lg px-3 py-1.5 bg-stone-900 text-white dark:bg-stone-100 dark:text-stone-900';render();});
$('create').onsubmit=async e=>{e.preventDefault();try{await api('/devices',{method:'POST',body:JSON.stringify({id:$('cId').value.trim(),name:$('cName').value.trim(),mac:$('cMac').value.trim(),ip:$('cIp').value.trim()||undefined})});say('Device added.');e.target.reset();load(true);}catch(err){say(String(err.message||err));}};
$('editDlg').addEventListener('close',async()=>{if($('editDlg').returnValue!=='default'||!editing)return;const d=editing;try{await api('/devices/'+encodeURIComponent(d.id),{method:'PUT',body:JSON.stringify({id:d.id,name:$('eName').value.trim(),mac:$('eMac').value.trim(),ip:$('eIp').value.trim()||'',ping_enabled:$('ePing').checked,status:d.status||'unknown'})});say('Device saved.');editing=null;load(true);}catch(err){say(String(err.message||err));}});
$('schedCreate').onsubmit=async e=>{e.preventDefault();try{await api('/schedules',{method:'POST',body:JSON.stringify({id:$('sId').value.trim(),device_id:$('sDev').value,cron:$('sCron').value.trim()})});say('Schedule added.');e.target.reset();loadSchedules();}catch(err){say(String(err.message||err));}};
</script>
</body>
</html>`
