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
<script>tailwind.config={darkMode:'class',theme:{extend:{fontFamily:{sans:['system-ui','-apple-system','Segoe UI','Roboto','sans-serif']}}}}</script>
<script>(function(){try{var t=localStorage.getItem('hello.theme')||'system';var d=t==='dark'||(t==='system'&&matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);}catch(e){}})();</script>
<style>:root{color-scheme:light dark}summary{cursor:pointer}summary:focus-visible,button:focus-visible,input:focus-visible,select:focus-visible,a:focus-visible{outline:2px solid #78716c;outline-offset:2px}</style>
</head>
<body class="h-full bg-stone-100 text-stone-900 antialiased dark:bg-stone-950 dark:text-stone-100">
<div class="mx-auto max-w-5xl px-4 sm:px-6 py-8">
<header class="flex flex-wrap items-end justify-between gap-4">
<div><p class="text-xs uppercase tracking-widest text-stone-500">Wake-on-LAN</p>
<h1 class="text-2xl font-semibold tracking-tight">hello</h1></div>
<div class="flex flex-wrap items-center gap-2 text-sm">
<label class="text-xs text-stone-500" for="theme">Theme</label>
<select id="theme" aria-label="Theme" class="rounded-lg border border-stone-300 bg-transparent px-2 py-1.5 dark:border-stone-700"><option value="system">System</option><option value="light">Light</option><option value="dark">Dark</option></select>
<label class="text-xs text-stone-500" for="refresh">Auto-refresh</label>
<select id="refresh" aria-label="Auto-refresh interval" class="rounded-lg border border-stone-300 bg-transparent px-2 py-1.5 dark:border-stone-700"><option value="0">Off</option><option value="10">10s</option><option value="30">30s</option><option value="60">60s</option></select>
<nav class="flex items-center gap-2"><a class="rounded-lg border border-stone-300 px-3 py-1.5 hover:bg-stone-200/60 dark:border-stone-700 dark:hover:bg-stone-800" href="/docs">API docs</a><a class="rounded-lg border border-stone-300 px-3 py-1.5 hover:bg-stone-200/60 dark:border-stone-700 dark:hover:bg-stone-800" href="/openapi.yaml">OpenAPI</a></nav>
</div>
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
<details id="sec-devices" class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900" open>
<summary class="text-sm font-semibold" aria-label="Devices section">Devices</summary>
<div class="mt-3 flex flex-col sm:flex-row gap-2 sm:items-center"><input id="search" type="search" placeholder="Filter by name, MAC, IP, tag… (/ to focus)" aria-label="Filter devices" class="w-full rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm outline-none focus:border-stone-500 dark:border-stone-700"><div id="filters" class="flex gap-1 text-sm" role="group" aria-label="Filter by status"><button data-f="all" class="rounded-lg px-3 py-1.5 bg-stone-900 text-white dark:bg-stone-100 dark:text-stone-900">All</button><button data-f="up" class="rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700">Up</button><button data-f="down" class="rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700">Down</button><button data-f="unknown" class="rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700">Unknown</button></div></div>
<div id="tagRow" class="mt-2 hidden flex-wrap gap-1 text-xs" aria-label="Active tag filter"></div>
<div class="mt-2 flex flex-wrap items-center gap-2 text-sm"><label class="text-xs text-stone-500" for="sort">Sort</label><select id="sort" class="rounded-lg border border-stone-300 bg-transparent px-2 py-1.5 dark:border-stone-700"><option value="name">Name</option><option value="status">Status</option><option value="last_seen">Last seen</option><option value="wake_count">Wake count</option></select><button id="sortDir" title="Toggle direction" aria-label="Toggle sort direction" class="rounded-lg border border-stone-300 px-2 py-1.5 dark:border-stone-700">↑</button><span class="flex-1"></span><button id="wakeDown" class="rounded-lg border border-stone-300 px-3 py-1.5 hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Wake all down</button><button id="expBtn" class="rounded-lg border border-stone-300 px-3 py-1.5 hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Export</button></div>
<div class="mt-2 flex flex-col sm:flex-row gap-2 text-sm"><input id="impFile" type="file" accept="application/json,.json" aria-label="Import devices file" class="text-sm"><select id="impMode" aria-label="Import mode" class="rounded-lg border border-stone-300 bg-transparent px-2 py-1.5 dark:border-stone-700"><option value="abort">abort</option><option value="skip">skip</option><option value="replace">replace</option></select><button id="impBtn" class="rounded-lg border border-stone-300 px-3 py-1.5 hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Import</button></div>
<div id="impMsg" role="status" class="mt-2 text-sm text-stone-500"></div>
<div id="msg" role="status" class="min-h-6 mt-3 text-sm text-stone-500"></div>
<ul id="rows" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-6 text-sm text-stone-500">Enter your token, then Connect.</li></ul>
</details>
<details id="sec-history" class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900" open>
<summary class="text-sm font-semibold" aria-label="Wake history section">Wake history</summary>
<p class="mt-1 text-xs text-stone-500">Latest wake events, newest first.</p>
<p id="histStats" class="mt-1 text-xs text-stone-500" aria-live="polite"></p>
<div class="mt-2 grid grid-cols-2 sm:grid-cols-5 gap-2 text-sm">
<input id="hDev" placeholder="device id" aria-label="Filter history by device" class="rounded-xl border border-stone-300 bg-transparent px-3 py-1.5 dark:border-stone-700">
<select id="hTrig" aria-label="Filter history by trigger" class="rounded-xl border border-stone-300 bg-transparent px-3 py-1.5 dark:border-stone-700"><option value="">trigger: all</option><option value="manual">manual</option><option value="schedule">schedule</option></select>
<select id="hOk" aria-label="Filter history by result" class="rounded-xl border border-stone-300 bg-transparent px-3 py-1.5 dark:border-stone-700"><option value="">result: all</option><option value="true">sent</option><option value="false">failed</option></select>
<input id="hSince" type="date" aria-label="History since date" class="rounded-xl border border-stone-300 bg-transparent px-3 py-1.5 dark:border-stone-700">
<button id="hClear" class="rounded-xl border border-stone-300 px-3 py-1.5 hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Clear filters</button>
</div>
<ul id="hist" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-4 text-sm text-stone-500">Connect to see history.</li></ul>
</details>
<details id="sec-sched" class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900" open>
<summary class="text-sm font-semibold" aria-label="Schedules section">Schedules</summary>
<p class="mt-1 text-xs text-stone-500">Cron uses 5 fields (ex: 0 7 * * 1-5). New schedules start enabled.</p>
<div class="mt-2 flex gap-2 text-sm"><button id="pauseAll" class="rounded-xl border border-stone-300 px-3 py-1.5 hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Pause all</button><button id="resumeAll" class="rounded-xl border border-stone-300 px-3 py-1.5 hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Resume all</button></div>
<ul id="scheds" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-4 text-sm text-stone-500">Connect to see schedules.</li></ul>
<form id="schedCreate" class="mt-2 grid grid-cols-1 sm:grid-cols-4 gap-2"><input id="sId" required placeholder="id (ex: nas-morning)" aria-label="Schedule id" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><select id="sDev" required aria-label="Schedule device" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"></select><input id="sCron" required placeholder="cron 0 7 * * *" aria-label="Schedule cron" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700"><button class="rounded-xl border border-stone-300 px-4 py-2 text-sm font-medium hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Add schedule</button></form>
</details>
<details id="sec-discover" class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<summary class="text-sm font-semibold" aria-label="Discover section">Discover</summary>
<p class="mt-1 text-xs text-stone-500">Scan the LAN, then adopt what you recognise. One scan every 30s.</p>
<div class="mt-2 flex flex-col sm:flex-row gap-2"><input id="cidr" placeholder="CIDR (optional, ex: 10.1.20.0/24)" aria-label="Scan CIDR" class="w-full rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700"><button id="scan" class="rounded-xl bg-stone-900 px-4 py-2 text-sm font-medium text-white hover:bg-stone-700 disabled:opacity-50 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white">Scan the LAN</button><button id="adoptSel" disabled class="rounded-xl border border-stone-300 px-4 py-2 text-sm font-medium hover:bg-stone-100 disabled:opacity-50 dark:border-stone-700 dark:hover:bg-stone-800">Adopt selected</button></div>
<div class="mt-2 flex flex-col sm:flex-row gap-2 text-sm"><input id="adoptTags" placeholder="adopt tags, comma separated (optional)" aria-label="Adopt tags" class="w-full rounded-xl border border-stone-300 bg-transparent px-3 py-2 dark:border-stone-700"><label class="flex items-center gap-2 text-sm"><input id="adoptDry" type="checkbox" class="h-4 w-4"> Dry run</label></div>
<div id="scanMsg" role="status" class="min-h-6 mt-2 text-sm text-stone-500"></div>
<ul id="found" class="mt-2 divide-y divide-stone-200 dark:divide-stone-800"><li class="py-4 text-sm text-stone-500">No scan yet.</li></ul>
</details>
<details id="sec-add" class="mt-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-900">
<summary class="text-sm font-semibold" aria-label="Add device section">Add a device</summary>
<p class="mt-1 text-xs text-stone-500">ID and MAC are required.</p>
<form id="create" class="mt-2 grid grid-cols-1 sm:grid-cols-4 gap-2"><input id="cId" required placeholder="id (ex: nas)" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><input id="cName" required placeholder="name" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><input id="cMac" required placeholder="MAC AA:BB:CC:DD:EE:FF" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700"><input id="cIp" placeholder="IP (optional)" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700"><input id="cTags" placeholder="tags, comma separated (optional)" class="sm:col-span-2 rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><input id="cNotes" placeholder="note (optional)" class="sm:col-span-2 rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700"><button class="sm:col-span-4 rounded-xl border border-stone-300 px-4 py-2 text-sm font-medium hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800">Add device</button></form>
</details>
<dialog id="editDlg" class="rounded-2xl border border-stone-200 bg-white p-4 text-stone-900 backdrop:bg-black/40 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-100">
<form id="editForm" method="dialog" class="grid grid-cols-1 gap-2 sm:min-w-96"><h2 class="text-sm font-semibold">Edit device</h2>
<input id="eName" required placeholder="name" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700">
<input id="eMac" required placeholder="MAC AA:BB:CC:DD:EE:FF" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700">
<input id="eIp" placeholder="IP (optional)" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm font-mono dark:border-stone-700">
<input id="eTags" placeholder="tags, comma separated" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700">
<input id="eNotes" placeholder="note" class="rounded-xl border border-stone-300 bg-transparent px-3 py-2 text-sm dark:border-stone-700">
<label class="flex items-center gap-2 text-sm"><input id="ePing" type="checkbox" class="h-4 w-4"> Ping enabled</label>
<div class="flex justify-end gap-2"><button value="cancel" formnovalidate class="rounded-xl border border-stone-300 px-4 py-2 text-sm dark:border-stone-700">Cancel</button><button id="eSave" value="default" class="rounded-xl bg-stone-900 px-4 py-2 text-sm font-medium text-white dark:bg-stone-100 dark:text-stone-900">Save</button></div></form>
</dialog>
<footer class="mt-6 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-stone-500"><span>hello — Wake-on-LAN service.</span><span id="healthDot" aria-label="Server health">●</span><span id="footVer"></span><span id="footUp"></span><span id="footRef"></span><span class="flex-1"></span><span>Press / to search, r to refresh.</span></footer>
</div>
<script>
const $=id=>document.getElementById(id);
const say=t=>{$('msg').textContent=t||''};
let devices=[],filter='all',ws=null,editing=null,history=[],schedules=[],sortKey='name',sortDir=1,tagFilter='',refreshSecs=0,refreshTimer=null,lastRefresh=null;
const ls={get(k,d){try{const v=localStorage.getItem(k);return v===null?d:v;}catch(e){return d;}},set(k,v){try{localStorage.setItem(k,v);}catch(e){}},del(k){try{localStorage.removeItem(k);}catch(e){}}};
const token=()=>$('token').value.trim();
async function api(path,opts={}){
  const t=token();if(!t)throw new Error('Enter API token first');
  const res=await fetch(path,{...opts,headers:{'Content-Type':'application/json','Authorization':'Bearer '+t,...(opts.headers||{})}});
  if(res.status===204)return null;
  const data=await res.json().catch(()=>({}));
  if(!res.ok)throw new Error(data.detail||data.title||('HTTP '+res.status));
  return data;
}
function applyTheme(v){const d=v==='dark'||(v==='system'&&matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);}
function pill(st){const base='inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ';if(st==='up')return base+'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200';if(st==='down')return base+'bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-200';return base+'bg-stone-200/70 text-stone-600 dark:bg-stone-800 dark:text-stone-300';}
function dot(st){if(st==='up')return 'h-1.5 w-1.5 rounded-full bg-emerald-500';if(st==='down')return 'h-1.5 w-1.5 rounded-full bg-rose-500';return 'h-1.5 w-1.5 rounded-full bg-stone-400';}
function setFooterLoading(){$('footRef').textContent='Refreshing…';}
function setFooterDone(){$('footRef').textContent=lastRefresh?('Refreshed '+lastRefresh.toLocaleTimeString()):'';}
async function load(silent){
  if(!silent){say('');setFooterLoading();}
  try{
    const data=await api('/devices?per_page=500');devices=data.devices||data||[];
    const up=devices.filter(d=>(d.status||'unknown')==='up').length,down=devices.filter(d=>(d.status||'unknown')==='down').length;
    $('statUp').textContent=up;$('statDown').textContent=down;$('statUnknown').textContent=devices.length-up-down;
    render();await Promise.all([loadHistory(),loadSchedules()]);loadFoot();
    lastRefresh=new Date();setFooterDone();
  }catch(e){if(!silent){$('rows').innerHTML='';const li=document.createElement('li');li.className='py-6 text-sm text-rose-600';li.textContent='Error — '+String(e.message||e);$('rows').appendChild(li);}$('footRef').textContent='';}
}
function valOf(d,k){if(k==='wake_count')return Number(d.wake_count||0);if(k==='last_seen')return Number(d.last_seen||0);if(k==='status')return String(d.status||'unknown');return String(d.name||d.id||'').toLowerCase();}
function emptyState(ul,text,btn,fn){ul.innerHTML='';const li=document.createElement('li');li.className='py-6 text-sm text-stone-500 flex items-center gap-3';const s=document.createElement('span');s.textContent=text;li.appendChild(s);if(btn){const b=document.createElement('button');b.className='rounded-lg border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';b.textContent=btn;b.onclick=fn;li.appendChild(b);}ul.appendChild(li);}
function renderTagRow(){const row=$('tagRow');row.innerHTML='';if(!tagFilter){row.classList.add('hidden');row.classList.remove('flex');return;}row.classList.remove('hidden');row.classList.add('flex');const s=document.createElement('span');s.textContent='Tag:';row.appendChild(s);const b=document.createElement('button');b.className='rounded-full border border-stone-300 px-2 py-0.5 dark:border-stone-700';b.textContent='#'+tagFilter+' ✕';b.setAttribute('aria-label','Clear tag filter');b.onclick=()=>{tagFilter='';render();};row.appendChild(b);}
function render(){
  const q=($('search').value||'').toLowerCase(),ul=$('rows');ul.innerHTML='';renderTagRow();
  const list=devices.filter(d=>{const st=(d.status||'unknown');if(filter!=='all'&&st!==filter)return false;if(tagFilter&&!(d.tags||[]).includes(tagFilter))return false;return ((d.name||'')+' '+(d.id||'')+' '+(d.mac||d.MAC||'')+' '+(d.ip||'')+' '+((d.tags||[]).join(' '))+' '+(d.notes||'')).toLowerCase().includes(q);});
  list.sort((a,b)=>{const x=valOf(a,sortKey),y=valOf(b,sortKey);return (x<y?-1:x>y?1:String(a.id||'').localeCompare(String(b.id||'')))*sortDir;});
  if(!list.length){if(devices.length&&!q&&filter==='all'&&!tagFilter)emptyState(ul,'No devices yet.','Add device',()=>{$('sec-add').open=true;$('cId').focus();});else emptyState(ul,'Nothing here — adjust filter or add a device.','Clear filters',()=>{filter='all';tagFilter='';$('search').value='';syncFilterBtns();render();});return;}
  for(const d of list){
    const st=(d.status||'unknown'),id=d.id||'';
    const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-3';
    const main=document.createElement('div');main.className='min-w-0 flex-1';
    const nm=document.createElement('p');nm.className='truncate text-sm font-medium cursor-pointer hover:underline';nm.textContent=d.name||id;nm.title='Show details';
    const mm=document.createElement('p');mm.className='truncate font-mono text-xs text-stone-500';mm.textContent=(d.mac||d.MAC||'')+(d.ip?' · '+d.ip:'');
    const tg=document.createElement('p');tg.className='truncate text-xs text-stone-500';
    const det=document.createElement('div');det.className='mt-1 hidden text-xs text-stone-500';
    for(const t of (d.tags||[])){const chip=document.createElement('button');chip.className='mr-1 rounded-full border border-stone-300 px-2 py-0.5 hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';chip.textContent='#'+t;chip.setAttribute('aria-label','Filter by tag '+t);chip.onclick=e=>{e.stopPropagation();tagFilter=t;render();};tg.appendChild(chip);}
    const hlink=document.createElement('button');hlink.className='ml-1 underline hover:no-underline';hlink.textContent='history';hlink.setAttribute('aria-label','Show wake history for '+id);hlink.onclick=e=>{e.stopPropagation();$('sec-history').open=true;$('hDev').value=id;saveHistFilters();loadHistory();$('sec-history').scrollIntoView();};
    tg.appendChild(hlink);
    const dline=document.createElement('span');det.appendChild(dline);
    dline.textContent=id+' · ping '+(d.ping_enabled?'on':'off')+' · '+(d.wake_count||0)+' wake(s) · '+(d.last_wake_at?fmtTime(d.last_wake_at):'never');
    if(d.notes){det.appendChild(document.createElement('br'));const ns=document.createElement('span');ns.textContent='note: '+d.notes;det.appendChild(ns);}
    nm.onclick=()=>det.classList.toggle('hidden');
    const last=document.createElement('span');last.className='text-xs text-stone-500';last.textContent=d.last_seen||d.lastSeen||'';
    const badge=document.createElement('span');badge.className=pill(st);const bdot=document.createElement('span');bdot.className=dot(st);badge.appendChild(bdot);const bt=document.createElement('span');bt.textContent=st;badge.appendChild(bt);
    main.append(nm,mm,tg,det);li.append(main,badge,last);
    const w=document.createElement('button');w.className='rounded-xl bg-stone-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-stone-700 disabled:opacity-50 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white';w.textContent='Wake';
    w.onclick=async()=>{w.disabled=true;try{const r=await api('/devices/'+encodeURIComponent(id)+'/wake?retries=2&interval=2',{method:'POST'});const n=(r&&r.attempts)||1;say('Wake '+(d.name||id)+': '+n+' attempt'+(n===1?'':'s')+', '+((r&&r.up)?'host is up':'host not answering'));setTimeout(()=>load(true),1500);}catch(e){say(String(e.message||e));}finally{w.disabled=false;}};
    const e=document.createElement('button');e.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';e.textContent='Edit';
    e.onclick=()=>{editing=d;$('eName').value=d.name||'';$('eMac').value=d.mac||d.MAC||'';$('eIp').value=d.ip||'';$('ePing').checked=!!d.ping_enabled;$('eTags').value=(d.tags||[]).join(', ');$('eNotes').value=d.notes||'';$('editDlg').showModal();};
    const del=document.createElement('button');del.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';del.textContent='Delete';
    del.onclick=async()=>{if(!confirm('Delete '+(d.name||id)+'?'))return;try{await api('/devices/'+encodeURIComponent(id),{method:'DELETE'});say('Deleted '+(d.name||id));load(true);}catch(e){say(String(e.message||e));}};
    li.append(w,e,del);ul.appendChild(li);
  }
}
function syncFilterBtns(){document.querySelectorAll('#filters button').forEach(x=>x.className='rounded-lg px-3 py-1.5 border border-stone-300 dark:border-stone-700');const b=document.querySelector('#filters button[data-f="'+filter+'"]');if(b)b.className='rounded-lg px-3 py-1.5 bg-stone-900 text-white dark:bg-stone-100 dark:text-stone-900';}
function nameOf(id){const d=devices.find(x=>(x.id||'')===id);return d?(d.name||d.id||id):id;}
function fmtTime(at){try{return new Date(Number(at)*1000).toLocaleString();}catch{return '';}}
function histQuery(){const p=new URLSearchParams();const dev=$('hDev').value.trim();if(dev)p.set('device_id',dev);if($('hTrig').value)p.set('trigger',$('hTrig').value);if($('hOk').value)p.set('success',$('hOk').value);if($('hSince').value){try{p.set('since',new Date($('hSince').value+'T00:00:00Z').toISOString());}catch(e){}}const q=p.toString();return '/history?limit=20'+(q?'&'+q:'');}
function saveHistFilters(){ls.set('hello.hist',JSON.stringify({dev:$('hDev').value,trig:$('hTrig').value,ok:$('hOk').value,since:$('hSince').value}));}
function restoreHistFilters(){try{const f=JSON.parse(ls.get('hello.hist','null')||'null');if(!f)return;$('hDev').value=f.dev||'';$('hTrig').value=f.trig||'';$('hOk').value=f.ok||'';$('hSince').value=f.since||'';}catch(e){}}
async function loadHistory(){
  const ul=$('hist');if(!ul)return;
  try{
    const data=await api(histQuery());history=data.history||[];ul.innerHTML='';
    try{const st=await api('/history/stats');$('histStats').textContent='total '+st.total+' · ok '+st.ok+' · failed '+st.error+' · rate '+Math.round((st.success_rate||0)*100)+'%';}catch(e){$('histStats').textContent='';}
    if(!history.length){emptyState(ul,'No wake events match these filters.','Clear filters',()=>{$('hDev').value='';$('hTrig').value='';$('hOk').value='';$('hSince').value='';saveHistFilters();loadHistory();});return;}
    for(const h of history){
      const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-2';
      const box=document.createElement('div');box.className='min-w-0 flex-1';
      const t1=document.createElement('p');t1.className='truncate text-sm font-medium';t1.textContent=nameOf(h.device_id||'');
      const t2=document.createElement('p');t2.className='truncate text-xs text-stone-500';t2.textContent=(h.trigger||'manual')+' · '+fmtTime(h.at)+(h.device_id?' · '+h.device_id:'');
      box.append(t1,t2);
      const badge=document.createElement('span');badge.className=h.success?'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200':'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-200';badge.textContent=h.success?'sent':'failed';
      li.append(box,badge);ul.appendChild(li);
    }
  }catch(e){ul.innerHTML='';const li=document.createElement('li');li.className='py-4 text-sm text-stone-500';li.textContent='History unavailable.';ul.appendChild(li);}
}
function schedLine(s){const parts=[];if(s.next_run)parts.push('next '+fmtTime(s.next_run));if(s.last_run_at)parts.push('last '+fmtTime(s.last_run_at));if(s.last_result)parts.push(s.last_result);if(s.once)parts.push('once '+(s.once_at?fmtTime(s.once_at):''));return parts.join(' · ');}
async function loadSchedules(){
  const ul=$('scheds');if(!ul)return;
  try{
    const data=await api('/schedules');schedules=data.schedules||[];
    const sel=$('sDev');if(sel){const cur=sel.value;sel.innerHTML='';for(const d of devices){const o=document.createElement('option');o.value=d.id||'';o.textContent=(d.name||d.id||'')+' ('+(d.id||'')+')';sel.appendChild(o);}if(cur)sel.value=cur;}
    ul.innerHTML='';
    if(!schedules.length){emptyState(ul,'No schedules yet.','Add schedule',()=>{$('sId').focus();});return;}
    for(const s of schedules){
      const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-2';
      const box=document.createElement('div');box.className='min-w-0 flex-1';
      const t1=document.createElement('p');t1.className='truncate text-sm font-medium';t1.textContent=nameOf(s.device_id||'');
      const t2=document.createElement('p');t2.className='truncate font-mono text-xs text-stone-500';t2.textContent=(s.id||'')+' · '+(s.cron||'');
      const t3=document.createElement('p');t3.className='truncate text-xs text-stone-500';t3.textContent=schedLine(s);
      box.append(t1,t2,t3);
      li.appendChild(box);
      if(s.last_result){const b=document.createElement('span');b.className=s.last_result==='success'?'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200':'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-200';b.textContent=s.last_result;li.appendChild(b);}
      const t=document.createElement('button');t.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';t.textContent=s.enabled?'On':'Off';
      t.onclick=async()=>{t.disabled=true;try{await api('/schedules/'+encodeURIComponent(s.id),{method:'PUT',body:JSON.stringify({...s,enabled:!s.enabled})});loadSchedules();}catch(e){say(String(e.message||e));}finally{t.disabled=false;}};
      const dup=document.createElement('button');dup.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';dup.textContent='Duplicate';
      dup.onclick=async()=>{dup.disabled=true;try{await api('/schedules/'+encodeURIComponent(s.id)+'/duplicate',{method:'POST'});say('Schedule duplicated.');loadSchedules();}catch(e){say(String(e.message||e));}finally{dup.disabled=false;}};
      const fire=document.createElement('button');fire.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';fire.textContent='Fire';
      fire.onclick=async()=>{fire.disabled=true;try{await api('/schedules/'+encodeURIComponent(s.id)+'/fire',{method:'POST'});say('Schedule fired.');loadSchedules();loadHistory();}catch(e){say(String(e.message||e));}finally{fire.disabled=false;}};
      const del=document.createElement('button');del.className='rounded-xl border border-stone-300 px-3 py-1.5 text-sm hover:bg-stone-100 dark:border-stone-700 dark:hover:bg-stone-800';del.textContent='Delete';
      del.onclick=async()=>{if(!confirm('Delete schedule '+s.id+'?'))return;try{await api('/schedules/'+encodeURIComponent(s.id),{method:'DELETE'});loadSchedules();}catch(e){say(String(e.message||e));}};
      li.append(t,dup,fire,del);ul.appendChild(li);
    }
  }catch(e){ul.innerHTML='';const li=document.createElement('li');li.className='py-4 text-sm text-stone-500';li.textContent='Schedules unavailable.';ul.appendChild(li);}
}
async function loadFoot(){
  try{const h=await api('/health');$('healthDot').textContent='●';$('healthDot').className=h.status==='ok'?'text-emerald-500':'text-rose-500';$('footVer').textContent=h.version||'';const u=Number(h.uptime_seconds||0);$('footUp').textContent=u?('up '+Math.floor(u/3600)+'h '+Math.floor(u%3600/60)+'m'):'';}
  catch(e){$('healthDot').className='text-rose-500';try{const v=await api('/version');$('footVer').textContent=v.version||'';}catch(e2){}}
}
let found=[];
function scanMsg(t){$('scanMsg').textContent=t||'';}
function renderFound(){
  const ul=$('found');ul.innerHTML='';
  if(!found.length){emptyState(ul,'No hosts found.','Scan LAN',()=>scan());return;}
  let selectable=0;
  for(const h of found){
    const li=document.createElement('li');li.className='flex flex-wrap items-center gap-3 py-2';
    const canAdopt=h.mac&&!h.known;
    if(canAdopt)selectable++;
    const box=document.createElement('div');box.className='min-w-0 flex-1';
    const t1=document.createElement('p');t1.className='truncate font-mono text-sm';t1.textContent=h.mac||'(no MAC)';
    const t2=document.createElement('p');t2.className='truncate text-xs text-stone-500';t2.textContent=[h.ip,h.hostname].filter(Boolean).join(' · ');
    box.append(t1,t2);
    const tag=document.createElement('span');tag.className='text-xs text-stone-500';tag.textContent=h.known?'known':'new';
    li.append(box,tag);
    if(canAdopt){const cb=document.createElement('input');cb.type='checkbox';cb.className='h-4 w-4';cb.dataset.mac=h.mac;cb.dataset.ip=h.ip||'';cb.dataset.hostname=h.hostname||'';cb.setAttribute('aria-label','Adopt '+(h.mac||h.ip||'host'));cb.onchange=()=>{$('adoptSel').disabled=!document.querySelectorAll('#found input:checked').length;};li.prepend(cb);}
    ul.appendChild(li);
  }
  $('adoptSel').disabled=!selectable;
}
async function scan(){
  const b=$('scan');b.disabled=true;scanMsg('Scanning the LAN… (up to 90s)');
  try{
    const cidr=$('cidr').value.trim();const path=cidr?('/discover?cidr='+encodeURIComponent(cidr)):'/discover';
    const data=await api(path,{method:'POST'});found=data.hosts||[];renderFound();
    scanMsg(found.length?found.length+' host(s) found'+(data.ignored?(' ('+data.ignored+' ignored)'):'' )+'.':'No hosts found.');
  }catch(e){scanMsg(String(e.message||e));}finally{b.disabled=false;}
}
async function adoptSelected(){
  const sel=[...document.querySelectorAll('#found input:checked')].map(cb=>({mac:cb.dataset.mac,ip:cb.dataset.ip||undefined,hostname:cb.dataset.hostname||undefined}));
  if(!sel.length)return;
  const b=$('adoptSel');b.disabled=true;
  try{
    const q=[];const tags=$('adoptTags').value.split(',').map(s=>s.trim()).filter(Boolean);if(tags.length)q.push('tags='+encodeURIComponent(tags.join(',')));if($('adoptDry').checked)q.push('dry_run=1');
    const data=await api('/discover/adopt'+(q.length?'?'+q.join('&'):''),{method:'POST',body:JSON.stringify({hosts:sel})});say($('adoptDry').checked?('Dry run: '+(data.devices||[]).length+' would be adopted.'):('Adopted '+(data.devices||[]).length+' device(s).'));load(true);scanMsg('Adopted. Rescan to refresh.');}
  catch(e){scanMsg(String(e.message||e));}finally{b.disabled=false;}
}
$('scan').onclick=scan;
$('adoptSel').onclick=adoptSelected;
function live(){
  try{if(ws)ws.close()}catch{}
  const t=token();if(!t)return;
  ws=new WebSocket((location.protocol==='https:'?'wss://':'ws://')+location.host+'/ws?token='+encodeURIComponent(t));
  ws.onmessage=()=>{if(!document.hidden)load(true);};ws.onclose=()=>{setTimeout(()=>{if(token())live();},5000);};
}
function armRefresh(){if(refreshTimer){clearInterval(refreshTimer);refreshTimer=null;}if(refreshSecs>0){refreshTimer=setInterval(()=>{if(!document.hidden&&token())load(true);},refreshSecs*1000);}}
$('connect').onclick=()=>{say('Connecting…');load().then(()=>{say('Connected. Live updates on.');live();});};
$('search').oninput=render;
document.querySelectorAll('#filters button').forEach(b=>b.onclick=()=>{filter=b.dataset.f;syncFilterBtns();render();});
$('create').onsubmit=async e=>{e.preventDefault();const tags=$('cTags').value.split(',').map(s=>s.trim()).filter(Boolean);try{await api('/devices',{method:'POST',body:JSON.stringify({id:$('cId').value.trim(),name:$('cName').value.trim(),mac:$('cMac').value.trim(),ip:$('cIp').value.trim()||undefined,tags:tags.length?tags:undefined,notes:$('cNotes').value.trim()||undefined})});say('Device added.');e.target.reset();load(true);}catch(err){say(String(err.message||err));}};
$('editDlg').addEventListener('close',async()=>{if($('editDlg').returnValue!=='default'||!editing)return;const d=editing;const tags=$('eTags').value.split(',').map(s=>s.trim()).filter(Boolean);const body={};const nm=$('eName').value.trim();if(nm!==(d.name||''))body.name=nm;const mc=$('eMac').value.trim();if(mc!==(d.mac||d.MAC||''))body.mac=mc;const ip=$('eIp').value.trim();if(ip!==(d.ip||''))body.ip=ip;if($('ePing').checked!==!!d.ping_enabled)body.ping_enabled=$('ePing').checked;const curTags=(d.tags||[]).join(',');if(tags.join(',')!==curTags)body.tags=tags;if($('eNotes').value.trim()!==(d.notes||''))body.notes=$('eNotes').value.trim();if(!Object.keys(body).length){editing=null;return;}try{await api('/devices/'+encodeURIComponent(d.id),{method:'PATCH',body:JSON.stringify(body)});say('Device saved.');editing=null;load(true);}catch(err){say(String(err.message||err));}});
$('sort').onchange=()=>{sortKey=$('sort').value;render();};
$('sortDir').onclick=()=>{sortDir=-sortDir;$('sortDir').textContent=sortDir===1?'↑':'↓';render();};
$('wakeDown').onclick=async()=>{const down=devices.filter(d=>(d.status||'unknown')==='down');if(!down.length){say('Nothing down.');return;}if(!confirm('Wake '+down.length+' down device(s)?'))return;try{const r=await api('/devices/wake-batch',{method:'POST',body:JSON.stringify({ids:down.map(d=>d.id)})});const fails=(r.failed||[]).map(f=>f.id+': '+(f.error||'failed')).join('; ');say('Woke '+(r.woken||[]).length+'/'+(r.matched||[]).length+(fails?'. Failed: '+fails:'.'));setTimeout(()=>load(true),1500);}catch(e){say(String(e.message||e));}};
$('expBtn').onclick=async()=>{try{const t=token();if(!t)throw new Error('Enter API token first');const res=await fetch('/devices/export',{headers:{'Authorization':'Bearer '+t}});if(!res.ok)throw new Error('HTTP '+res.status);const blob=await res.blob();const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='devices-export.json';a.click();setTimeout(()=>URL.revokeObjectURL(a.href),5000);say('Exported.');}catch(e){say(String(e.message||e));}};
$('impBtn').onclick=async()=>{const f=$('impFile').files[0];if(!f){$('impMsg').textContent='Pick the export JSON first.';return;}try{const raw=JSON.parse(await f.text());const arr=Array.isArray(raw)?raw:(raw.devices||null);if(!arr){$('impMsg').textContent='File must be an array of devices or {devices:[...]}.';return;}const r=await api('/devices/import',{method:'POST',body:JSON.stringify({devices:arr,mode:$('impMode').value})});$('impMsg').textContent='Imported '+(r.created||[]).length+', skipped '+(r.skipped||[]).length+'.';say('Import done.');load(true);}catch(e){$('impMsg').textContent=String(e.message||e);}};
$('schedCreate').onsubmit=async e=>{e.preventDefault();try{await api('/schedules',{method:'POST',body:JSON.stringify({id:$('sId').value.trim(),device_id:$('sDev').value,cron:$('sCron').value.trim()})});say('Schedule added.');e.target.reset();loadSchedules();}catch(err){say(String(err.message||err));}};
$('pauseAll').onclick=async()=>{try{const r=await api('/schedules/pause-all',{method:'POST'});say('Paused '+(r.changed||0)+'.');loadSchedules();}catch(e){say(String(e.message||e));}};
$('resumeAll').onclick=async()=>{try{const r=await api('/schedules/resume-all',{method:'POST'});say('Resumed '+(r.changed||0)+'.');loadSchedules();}catch(e){say(String(e.message||e));}};
['hDev','hTrig','hOk','hSince'].forEach(id=>{$(id).addEventListener('change',()=>{saveHistFilters();loadHistory();});});
$('hDev').addEventListener('input',()=>{saveHistFilters();loadHistory();});
$('hClear').onclick=()=>{$('hDev').value='';$('hTrig').value='';$('hOk').value='';$('hSince').value='';saveHistFilters();loadHistory();};
$('theme').value=ls.get('hello.theme','system');applyTheme($('theme').value);
$('theme').onchange=()=>{ls.set('hello.theme',$('theme').value);applyTheme($('theme').value);};
$('refresh').value=ls.get('hello.refresh','0');refreshSecs=Number($('refresh').value)||0;
$('refresh').onchange=()=>{refreshSecs=Number($('refresh').value)||0;ls.set('hello.refresh',$('refresh').value);armRefresh();};
armRefresh();
restoreHistFilters();
document.querySelectorAll('details[id]').forEach(d=>{const k='hello.sec.'+d.id;const v=ls.get(k,'');if(v==='open')d.open=true;else if(v==='closed')d.open=false;d.addEventListener('toggle',()=>ls.set(k,d.open?'open':'closed'));});
document.addEventListener('keydown',e=>{if(e.key==='/'&&!/INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName)&&!$('editDlg').open){e.preventDefault();$('sec-devices').open=true;$('search').focus();}else if((e.key==='r'||e.key==='R')&&!/INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName)&&!$('editDlg').open&&!e.metaKey&&!e.ctrlKey&&!e.altKey){if(token())load(true);}});
document.addEventListener('visibilitychange',()=>{if(!document.hidden&&refreshSecs>0&&token())load(true);});
</script>
</body>
</html>`
