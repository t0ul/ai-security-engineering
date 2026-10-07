package webapp

const dashboardHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Agent Console</title>
<style>
  :root{color-scheme:light dark;--fg:#141414;--bg:#fafafa;--card:#fff;--mut:#667;--line:#e3e3e8;--ok:#0a7d33;--bad:#b00020;--accent:#2f6feb;--warn:#b26a00}
  @media (prefers-color-scheme:dark){:root{--fg:#e8e8ea;--bg:#131316;--card:#1b1b1f;--mut:#9aa;--line:#2a2a30;--ok:#4ade80;--bad:#ff6b6b;--accent:#6ea8fe;--warn:#f0b357}}
  *{box-sizing:border-box} body{font:15px/1.55 system-ui,-apple-system,Segoe UI,sans-serif;margin:0;color:var(--fg);background:var(--bg)}
  header{padding:18px 24px;border-bottom:1px solid var(--line);display:flex;gap:18px;align-items:baseline;flex-wrap:wrap}
  h1{font-size:18px;margin:0} nav{display:flex;gap:4px;flex-wrap:wrap} nav button{font:inherit;padding:6px 12px;border:0;background:none;color:var(--mut);cursor:pointer;border-radius:8px}
  nav button.active{background:var(--card);color:var(--fg);box-shadow:0 1px 0 var(--line)}
  main{padding:24px;max-width:900px;margin:0 auto} .mut{color:var(--mut)} h3{margin:18px 0 6px;font-size:14px;text-transform:uppercase;letter-spacing:.04em;color:var(--mut)}
  .card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:16px 18px;margin:12px 0}
  button.go{font:inherit;padding:8px 14px;border:0;border-radius:9px;background:var(--accent);color:#fff;cursor:pointer}
  button.ghost{font:inherit;padding:6px 11px;border:1px solid var(--line);border-radius:9px;background:none;color:var(--fg);cursor:pointer}
  table{border-collapse:collapse;width:100%} td,th{text-align:left;padding:7px 10px;border-bottom:1px solid var(--line);font-size:14px}
  .pill{font-size:12px;padding:2px 8px;border-radius:999px} .pass{background:color-mix(in srgb,var(--ok) 18%,transparent);color:var(--ok)}
  .fail{background:color-mix(in srgb,var(--bad) 18%,transparent);color:var(--bad)}
  .kind{font-size:11px;padding:1px 7px;border-radius:999px;background:color-mix(in srgb,var(--accent) 15%,transparent);color:var(--accent)}
  .trace{font-family:ui-monospace,Menlo,monospace;cursor:pointer;color:var(--accent)} pre{white-space:pre-wrap;font-size:13px;margin:0}
  .ev{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:11px 0;border-bottom:1px solid var(--line)}
  .ev:last-child{border-bottom:0} .ev b{font-size:15px} .bell{margin-right:4px} a.go{text-decoration:none;display:inline-block}
  .strip{display:flex;gap:10px;overflow-x:auto;padding:4px 0} .chip{flex:0 0 auto;border:1px solid var(--line);border-radius:10px;padding:8px 12px;font-size:13px;background:var(--card)}
  .chip b{display:block} .dz{border:2px dashed var(--line);border-radius:12px;padding:16px;margin:10px 0;transition:border-color .15s} .dz.over{border-color:var(--accent);background:color-mix(in srgb,var(--accent) 7%,transparent)}
  .link{color:var(--accent);cursor:pointer} textarea{font:inherit;background:var(--card);color:var(--fg);border:1px solid var(--line);border-radius:8px;padding:8px;width:100%}
  .feed div{padding:7px 0;border-bottom:1px solid var(--line);font-size:14px} .feed time{color:var(--mut);font-size:12px;margin-right:8px}
  .warn{color:var(--warn)} .hide{display:none} code{font-family:ui-monospace,Menlo,monospace}
  .viewtoggle{display:inline-flex;gap:6px;margin:4px 0 10px}
  .mhead{display:flex;justify-content:space-between;align-items:center;margin-bottom:8px}
  .grid{display:grid;grid-template-columns:repeat(7,1fr);gap:4px}
  .dow{font-size:11px;color:var(--mut);text-align:center;padding:2px}
  .day{min-height:66px;border:1px solid var(--line);border-radius:8px;padding:4px;font-size:12px;cursor:pointer;background:var(--card);overflow:hidden}
  .day.other{opacity:.4} .day.today{border-color:var(--accent);box-shadow:inset 0 0 0 1px var(--accent)}
  .daynum{font-weight:600;font-size:11px;color:var(--mut)}
  .it{display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;border-radius:4px;padding:0 4px;margin-top:2px;font-size:11px}
  .k-event{background:color-mix(in srgb,var(--accent) 20%,transparent)} .k-task{background:color-mix(in srgb,var(--warn) 24%,transparent)}
  .k-heads_up{background:color-mix(in srgb,var(--ok) 20%,transparent)} .k-action{background:color-mix(in srgb,var(--accent) 34%,transparent)}
</style></head><body>
<header><h1>🗓️ Agent Console</h1>
  <nav>
    <button data-tab="calendar" class="active">Calendar</button>
    <button data-tab="tasks">Tasks</button>
    <button data-tab="review">Review</button>
    <button data-tab="activity">Activity</button>
    <button data-tab="ask">Ask school</button>
    <button data-tab="security">Security</button>
    <button data-tab="prompts">Prompts</button>
    <button data-tab="incidents">Incidents</button>
  </nav>
</header>
<main>
  <section id="calendar">
    <div class="card">
      <strong>Upcoming</strong> <span class="mut">— next things across meetings, tasks &amp; heads-ups</span>
      <div class="strip" id="deadlines"></div>
    </div>
    <div class="card">
      <strong>Calendar</strong> <span class="mut">— meetings the agent extracted; accept downloads the .ics (with a reminder)</span>
      <p class="mut" id="inbox"></p>
      <div class="dz" id="dropzone">
        Drag a <b>.txt</b> email here, or <label class="link">choose a file<input id="file" type="file" accept=".txt" hidden></label>, or paste below.
        <div style="margin-top:8px"><textarea id="paste" rows="3" placeholder="paste email text…"></textarea></div>
        <div style="margin-top:8px"><button class="go" onclick="dropText()">Process email</button> <span id="dropmsg" class="mut"></span></div>
      </div>
      <div class="viewtoggle">
        <button class="ghost" id="vList" onclick="setView('list')">List</button>
        <button class="ghost" id="vMonth" onclick="setView('month')">Month</button>
      </div>
      <div id="events"></div>
      <div id="monthView" class="hide"></div>
    </div>
  </section>

  <section id="tasks" class="hide">
    <div class="card">
      <strong>Tasks &amp; reminders</strong> <span class="mut">— the non-meeting things an email asks of you</span>
      <h3>To do</h3><div id="taskList"></div>
      <h3>Heads-up</h3><div id="headsList"></div>
      <h3>Actions (links)</h3><div id="actionList"></div>
    </div>
    <div class="card">
      <strong>By email</strong> <span class="mut">— digest &amp; contacts the agent pulled</span>
      <div id="summaries"></div>
    </div>
  </section>

  <section id="review" class="hide">
    <div class="card">
      <strong>Needs review</strong> <span class="mut">— low-confidence or flagged; not added unattended</span>
      <div id="reviewList"></div>
    </div>
  </section>

  <section id="activity" class="hide">
    <div class="card">
      <strong>Agent activity</strong> <span class="mut">— what the agent did, newest first (from the tamper-evident log)</span>
      <div class="feed" id="feed"></div>
    </div>
  </section>

  <section id="ask" class="hide">
    <div class="card">
      <strong>Ask school</strong> <span class="mut">— search your past emails &amp; the handbook (what you've dropped)</span>
      <div style="margin-top:8px"><input id="askq" placeholder="e.g. nurse, phone policy, book fair" style="width:70%"><button class="go" onclick="doAsk()">Search</button></div>
      <div id="askOut" style="margin-top:10px"></div>
    </div>
  </section>

  <section id="security" class="hide">
    <div class="card">
      <strong>Kill switch</strong> <span class="mut">— halt the agent (M18)</span>
      <div style="margin-top:8px">level: <span id="klevel" class="pill">—</span>
        <button class="ghost" onclick="setKill(0)">Resume</button>
        <button class="ghost" onclick="setKill(1)">Block actions</button>
        <button class="ghost" onclick="setKill(2)">Pause</button>
        <button class="ghost" onclick="setKill(3)">Full halt</button>
      </div>
    </div>
    <div class="card">
      <strong>MCP servers</strong> <span class="mut">— tool gateway: manifest pins &amp; rug-pull defense (M7)</span>
      <div id="mcp"><span class="mut">loading…</span></div>
    </div>
    <div class="card">
      <strong>Security scorecard</strong>
      <p class="mut">Run the full red-team suite against the live controls. Every attack should drop to 0%.</p>
      <button class="go" onclick="runScorecard()">Run scorecard</button>
      <span id="verdict"></span>
      <table id="score" class="hide"><thead><tr><th>technique</th><th>control</th><th>before</th><th>after</th><th></th></tr></thead><tbody></tbody></table>
    </div>
  </section>

  <section id="prompts" class="hide">
    <div class="card">
      <strong>Prompts</strong> <span class="mut">— governed system prompts (M14): versioned, hashed, rollback to default. Resolved by the LLM planner/coder/extractor when enabled.</span>
      <div id="promptList"><span class="mut">loading…</span></div>
    </div>
  </section>

  <section id="incidents" class="hide">
    <div class="card">
      <strong>Incidents</strong> <span class="mut">— click a trace to replay it on tamper-evident evidence</span>
      <table id="traces"><tbody></tbody></table>
    </div>
    <div class="card hide" id="replayCard"><strong>Replay</strong><pre id="replay"></pre></div>
  </section>
</main>
<script>
// Capability token (C4b): when the server runs as an authZ'd API it injects the
// operator Grant here, and this wrapper attaches it to every same-origin API/ICS
// request. Empty (household mode) = no header, unchanged behavior.
const __CAP__="__CAP_TOKEN__";
(function(){const f=window.fetch.bind(window);window.fetch=(u,o)=>{o=o||{};const s=typeof u==='string'?u:(u&&u.url)||'';if(__CAP__&&(s.indexOf('/api/')===0||s.indexOf('/ics/')===0)){o.headers=Object.assign({},o.headers||{},{'Authorization':'Bearer '+__CAP__});}return f(u,o);};})();
const $=s=>document.querySelector(s);
const TABS=['calendar','tasks','review','activity','ask','security','prompts','incidents'];
const esc=s=>{const d=document.createElement('div');d.textContent=s||'';return d.innerHTML;};
const when=e=>e.all_day?((e.due||e.start)+' · all day'):((e.start||e.due)+(e.end?(' – '+e.end.slice(11)):''));
document.querySelectorAll('nav button').forEach(b=>b.onclick=()=>{
  document.querySelectorAll('nav button').forEach(x=>x.classList.toggle('active',x===b));
  TABS.forEach(t=>$('#'+t).classList.toggle('hide',t!==b.dataset.tab));
  const t=b.dataset.tab;
  if(t==='calendar'){loadEvents();loadDeadlines();}
  if(t==='tasks') loadTasks();
  if(t==='review') loadReview();
  if(t==='activity') loadActivity();
  if(t==='incidents') loadIncidents();
  if(t==='security'){loadKill();loadMCP();}
  if(t==='prompts') loadPrompts();
});
async function doAsk(){
  const q=$('#askq').value.trim();const c=$('#askOut');if(!q){c.innerHTML='';return;}
  c.innerHTML='<span class="mut">searching…</span>';
  const d=await getJSON('/api/ask?q='+encodeURIComponent(q));const hits=d.hits||[];
  if(!hits.length){c.innerHTML='<p class="mut">no matches — drop more emails to build the corpus</p>';return;}
  c.innerHTML=hits.map(h=>'<div class="ev"><div><b>'+esc(h.source)+'</b>'+(h.untrusted?' <span class="kind warn">untrusted</span>':'')+'<br><span class="mut">'+esc(h.snippet)+'</span></div></div>').join('');
}
document.addEventListener('keydown',e=>{if(e.key==='Enter'&&document.activeElement&&document.activeElement.id==='askq')doAsk();});
async function loadKill(){const d=await getJSON('/api/safety');const p=$('#klevel');if(p){p.textContent=d.level;p.className='pill '+(d.level==='none'?'pass':'fail');}}
async function setKill(l){await postJSON('/api/killswitch',{level:l});loadKill();loadMCP();}
async function loadMCP(){
  const c=$('#mcp');if(!c)return;const d=await getJSON('/api/mcp');const s=d.servers||[];
  if(!s.length){c.innerHTML='<p class="mut">no MCP servers registered</p>';return;}
  c.innerHTML=s.map(m=>{
    const ok=m.status==='pinned';
    const act=(m.status==='rug-pull'||m.status==='unapproved')?(' <button class="ghost" onclick="approveMCP(\''+esc(m.name)+'\')">Approve</button>'):'';
    const drift=(m.current&&m.current!==m.pinned)?(' → now '+esc(m.current)):'';
    return '<div class="ev"><div><b>'+esc(m.name)+'</b> <span class="pill '+(ok?'pass':'fail')+'">'+esc(m.status)+'</span>'+act+
      '<br><span class="mut">tools: '+esc((m.tools||[]).join(', ')||'—')+' · allowed: '+esc((m.allowed||[]).join(', ')||'any')+'</span>'+
      '<br><span class="mut">pin '+esc(m.pinned||'—')+drift+'</span></div></div>';
  }).join('');
}
async function approveMCP(name){await postJSON('/api/mcp/approve',{server:name});loadMCP();}
async function loadPrompts(){
  const c=$('#promptList');if(!c)return;const d=await getJSON('/api/prompts');const ps=d.prompts||[];
  if(!ps.length){c.innerHTML='<p class="mut">no prompts registered</p>';return;}
  c.innerHTML=ps.map(p=>{
    const governed=p.version>0;
    return '<div class="ev"><div><b>'+esc(p.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+p.version+'</span> <span class="mut">'+esc(p.hash)+'</span>'+
      '<br><textarea id="pt_'+esc(p.name)+'" rows="5" style="width:100%;margin-top:6px">'+esc(p.text)+'</textarea>'+
      '<div style="margin-top:6px"><button class="go" onclick="activatePrompt(\''+esc(p.name)+'\')">Activate new version</button>'+
      (governed?(' <button class="ghost" onclick="resetPrompt(\''+esc(p.name)+'\')">Reset to default</button>'):'')+'</div></div></div>';
  }).join('');
}
async function activatePrompt(name){
  const ta=$('#pt_'+name);if(!ta)return;
  await postJSON('/api/prompts/activate',{name:name,text:ta.value});loadPrompts();
}
async function resetPrompt(name){await postJSON('/api/prompts/reset',{name:name});loadPrompts();}

async function getJSON(u){try{return await (await fetch(u)).json();}catch(e){return {};}}

async function loadDeadlines(){
  const d=await getJSON('/api/items');
  const c=$('#deadlines');c.innerHTML='';
  const dated=(d.items||[]).filter(i=>(i.due||i.start));
  if(!dated.length){c.innerHTML='<span class="mut">nothing scheduled yet</span>';return;}
  dated.slice(0,8).forEach(i=>{const el=document.createElement('div');el.className='chip';
    el.innerHTML='<b>'+esc((i.due||i.start).slice(0,10))+'</b>'+esc(i.title.slice(0,40));c.appendChild(el);});
}
async function loadEvents(){
  const d=await getJSON('/api/events');
  if(d.inbox) $('#inbox').innerHTML='📥 Drop <b>.txt</b> emails here: <code>'+esc(d.inbox)+'</code>';
  const c=$('#events'); c.innerHTML='';
  const evs=(d.events||[]).filter(e=>(e.kind||'event')==='event');
  if(!evs.length){c.innerHTML='<p class="mut">no meetings yet — drop a .txt email above</p>';return;}
  evs.forEach(e=>{const div=document.createElement('div');div.className='ev';
    div.innerHTML='<div><b>'+esc(e.title)+'</b>'+(e.signed?' <span class="kind" title="signed by this agent, unaltered">✓ signed</span>':'')+'<br><span class="mut">'+esc(when(e))+(e.location?(' · '+esc(e.location)):'')+'</span></div>'+
      '<div>'+(e.has_reminder?'<span class="bell">🔔</span>':'')+'<a class="go" href="/ics/'+encodeURIComponent(e.file)+'" download>Accept .ics</a></div>';
    c.appendChild(div);});
}
async function postJSON(url,body){return fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});}
async function accept(file,index,btn){
  const d=await (await postJSON('/api/accept',{file,index})).json(); // phase 1: evidence + nonce
  if(!d.confirm_required){btn.textContent='error';return;}
  if(!confirm(d.summary+'\n'+(d.evidence||[]).join('\n')+'\n\n(approval '+d.nonce.slice(0,6)+') — Confirm?')) return;
  btn.disabled=true;btn.textContent='…';
  const r=await postJSON('/api/accept',{file,index,nonce:d.nonce,confirm:true}); // phase 2
  if(r.ok){btn.textContent='added ✓';loadDeadlines();}else{btn.textContent='refused';btn.disabled=false;}
}
function itemRow(i){
  const div=document.createElement('div');div.className='ev';
  const right=(i.due||i.start)?'<a class="go" href="/ics/'+encodeURIComponent((i.file||'').replace(/\.summary\.json$/,'')+'.items.ics')+'" download>Accept .ics</a>':'';
  const btn='<button class="ghost" onclick="accept(\''+esc(i.file)+'\','+i.index+',this)">Add this</button>';
  div.innerHTML='<div><span class="kind">'+esc(i.kind||'item')+'</span> <b>'+esc(i.title)+'</b><br><span class="mut">'+esc((i.due||i.start)?when(i):'no date')+'</span></div><div>'+btn+'</div>';
  return div;
}
async function loadTasks(){
  const d=await getJSON('/api/items');const items=d.items||[];
  const fill=(id,kind)=>{const c=$('#'+id);c.innerHTML='';const xs=items.filter(i=>(i.kind||'event')===kind);
    if(!xs.length){c.innerHTML='<p class="mut">none</p>';return;}xs.forEach(i=>c.appendChild(itemRow(i)));};
  fill('taskList','task');fill('headsList','heads_up');
  // actions: show the URL as text; fetching is gated (A6), never auto-opened.
  const ac=$('#actionList');ac.innerHTML='';const acts=items.filter(i=>(i.kind||'')==='action');
  if(!acts.length){ac.innerHTML='<p class="mut">none</p>';}
  acts.forEach(i=>{const div=document.createElement('div');div.className='ev';
    div.innerHTML='<div style="width:100%"><b>'+esc(i.title)+'</b><br><span class="mut">'+esc(i.url||'')+'</span>'+
      '<br><span class="warn" style="font-size:12px">fetched in the sandbox, only if the host is allow-listed</span>'+
      '<div class="actionOut mut" style="margin-top:6px"></div></div>'+
      '<div><button class="ghost" onclick="doAction(\''+esc(i.file)+'\','+i.index+',this)">Check &amp; fetch</button></div>';
    ac.appendChild(div);});
  loadSummaries(items);
}
async function doAction(file,index,btn){
  const out=btn.closest('.ev').querySelector('.actionOut');
  const c=await (await postJSON('/api/action',{file,index})).json(); // phase 1
  if(!c.confirm_required){out.textContent='error';return;}
  if(!confirm(c.summary+'\n'+(c.evidence||[]).join('\n')+'\n\n(approval '+c.nonce.slice(0,6)+') — Fetch this link in the sandbox?')) return;
  btn.disabled=true;btn.textContent='…';
  const d=await (await postJSON('/api/action',{file,index,nonce:c.nonce,confirm:true})).json(); // phase 2
  btn.disabled=false;btn.textContent='Check & fetch';
  if(d.refused){out.innerHTML='<span class="warn">refused: '+esc(d.refused)+'</span>';}
  else if(d.ok){out.innerHTML='<pre>'+esc((d.result||'').slice(0,600))+'</pre>';}
  else{out.textContent='error';}
}
async function loadSummaries(items){
  const files=[...new Set(items.map(i=>i.file).filter(Boolean))];
  const c=$('#summaries');c.innerHTML='';
  if(!files.length){c.innerHTML='<p class="mut">nothing processed yet</p>';return;}
  for(const f of files){const s=await getJSON('/api/summary?file='+encodeURIComponent(f));
    const div=document.createElement('div');div.className='ev';
    const digest=(s.tools&&s.tools.digest)||'';const contacts=(s.tools&&s.tools.contacts)||'';
    div.innerHTML='<div style="width:100%"><b>'+esc(s.source||f)+'</b>'+
      (digest?'<pre class="mut" style="margin-top:6px">'+esc(digest)+'</pre>':'')+
      (contacts?'<div class="mut" style="margin-top:6px">✉️ '+esc(contacts.replace(/\n/g,', '))+'</div>':'')+'</div>';
    c.appendChild(div);}
}
async function loadReview(){
  const d=await getJSON('/api/review');const c=$('#reviewList');c.innerHTML='';
  const xs=d.items||[];if(!xs.length){c.innerHTML='<p class="mut">nothing to review 🎉</p>';return;}
  xs.forEach(i=>{const div=document.createElement('div');div.className='ev';
    const why=(i.warnings&&i.warnings.join(', '))||('confidence '+Math.round((i.confidence||0)*100)+'%');
    div.innerHTML='<div><b>'+esc(i.title)+'</b><br><span class="warn" style="font-size:13px">'+esc(why)+'</span></div>';
    c.appendChild(div);});
}
async function loadActivity(){
  const d=await getJSON('/api/activity');const c=$('#feed');c.innerHTML='';
  const xs=d.activity||[];if(!xs.length){c.innerHTML='<p class="mut">no activity logged yet</p>';return;}
  xs.forEach(a=>{const div=document.createElement('div');
    div.innerHTML='<time>'+esc((a.ts||'').slice(11,19))+'</time>'+esc(a.text);c.appendChild(div);});
}

// --- month view (A7) ---
let curMonth=null, monthItems={};
const todayStr=()=>new Date().toISOString().slice(0,10);
const pad=n=>String(n).padStart(2,'0');
const dateStr=(y,m,d)=>{const dt=new Date(y,m,d);return dt.getFullYear()+'-'+pad(dt.getMonth()+1)+'-'+pad(dt.getDate());};
function setView(v){
  $('#vList').classList.toggle('active',v==='list');$('#vMonth').classList.toggle('active',v==='month');
  $('#events').classList.toggle('hide',v!=='list');$('#monthView').classList.toggle('hide',v!=='month');
  if(v==='month') loadMonth();
}
async function loadMonth(){
  const d=await getJSON('/api/items');monthItems={};
  (d.items||[]).forEach(i=>{const k=(i.due||i.start||'').slice(0,10);if(k)(monthItems[k]=monthItems[k]||[]).push(i);});
  if(!curMonth){const ds=Object.keys(monthItems).sort();const pick=ds.find(x=>x>=todayStr())||ds[0]||todayStr();
    const [y,m]=pick.split('-');curMonth=new Date(+y,+m-1,1);}
  renderMonth();
}
function dayCell(y,m,d,other){
  const ds=dateStr(y,m,d);const items=monthItems[ds]||[];
  let inner='<div class="daynum">'+d+'</div>';
  items.slice(0,3).forEach(i=>inner+='<span class="it k-'+(i.kind||'event')+'">'+esc(i.title)+'</span>');
  if(items.length>3)inner+='<span class="mut" style="font-size:11px">+'+(items.length-3)+' more</span>';
  return '<div class="day'+(other?' other':'')+(ds===todayStr()?' today':'')+'" onclick="showDay(\''+ds+'\')">'+inner+'</div>';
}
function renderMonth(){
  const y=curMonth.getFullYear(),m=curMonth.getMonth();
  const name=curMonth.toLocaleString('en',{month:'long',year:'numeric'});
  let h='<div class="mhead"><button class="ghost" onclick="shiftMonth(-1)">‹</button><b>'+name+'</b><button class="ghost" onclick="shiftMonth(1)">›</button></div><div class="grid">';
  ['Mon','Tue','Wed','Thu','Fri','Sat','Sun'].forEach(x=>h+='<div class="dow">'+x+'</div>');
  const startDow=(new Date(y,m,1).getDay()+6)%7, days=new Date(y,m+1,0).getDate(), prev=new Date(y,m,0).getDate();
  for(let i=0;i<startDow;i++) h+=dayCell(y,m-1,prev-startDow+1+i,true);
  for(let d=1;d<=days;d++) h+=dayCell(y,m,d,false);
  const trail=(7-(startDow+days)%7)%7;
  for(let i=1;i<=trail;i++) h+=dayCell(y,m+1,i,true);
  h+='</div><div id="dayDetail"></div>';
  $('#monthView').innerHTML=h;
}
function shiftMonth(n){curMonth=new Date(curMonth.getFullYear(),curMonth.getMonth()+n,1);renderMonth();}
function showDay(ds){const items=monthItems[ds]||[];const c=$('#dayDetail');
  if(!items.length){c.innerHTML='<p class="mut" style="margin-top:12px">'+esc(ds)+' — nothing</p>';return;}
  c.innerHTML='<h3>'+esc(ds)+'</h3>'+items.map(i=>'<div class="ev"><div><span class="kind">'+esc(i.kind||'event')+'</span> <b>'+esc(i.title)+'</b>'+(i.location?(' <span class="mut">· '+esc(i.location)+'</span>'):'')+'</div></div>').join('');
}

const dz=$('#dropzone');
dz.addEventListener('dragover',e=>{e.preventDefault();dz.classList.add('over');});
dz.addEventListener('dragleave',()=>dz.classList.remove('over'));
dz.addEventListener('drop',e=>{e.preventDefault();dz.classList.remove('over');if(e.dataTransfer.files[0])upload(e.dataTransfer.files[0]);});
$('#file').addEventListener('change',e=>{if(e.target.files[0])upload(e.target.files[0]);});
async function upload(f){const fd=new FormData();fd.append('email',f);await postDrop(fd,f.name);}
async function dropText(){const t=$('#paste').value;if(!t.trim())return;const fd=new FormData();fd.append('text',t);await postDrop(fd,'pasted email');$('#paste').value='';}
async function postDrop(fd,label){$('#dropmsg').textContent='processing '+label+'…';
  const r=await fetch('/api/drop',{method:'POST',body:fd});
  if(!r.ok){$('#dropmsg').textContent='error: '+(await r.text());return;}
  $('#dropmsg').textContent='queued — results appear shortly';
  setTimeout(()=>{loadEvents();loadDeadlines();},2500);setTimeout(()=>{loadEvents();loadDeadlines();},5000);}
loadEvents();loadDeadlines();

async function runScorecard(){
  $('#verdict').textContent=' running…';
  const d=await getJSON('/api/scorecard');
  const tb=$('#score tbody'); tb.innerHTML='';
  (d.results||[]).forEach(r=>{const tr=tb.insertRow();
    tr.insertCell().textContent=r.name; tr.insertCell().textContent=r.technique;
    tr.insertCell().textContent=r.before.toFixed(0)+'%'; tr.insertCell().textContent=r.after.toFixed(0)+'%';
    const c=tr.insertCell(); c.innerHTML='<span class="pill '+(r.pass?'pass':'fail')+'">'+(r.pass?'ok':'FAIL')+'</span>';});
  $('#score').classList.remove('hide');
  $('#verdict').innerHTML=d.all_pass?' <span class="pill pass">all defenses hold</span>':' <span class="pill fail">regression!</span>';
}
async function loadIncidents(){
  const d=await getJSON('/api/incidents');
  const tb=$('#traces tbody'); tb.innerHTML='';
  (d.traces||[]).forEach(t=>{const tr=tb.insertRow();
    const c=tr.insertCell(); const a=document.createElement('span'); a.className='trace'; a.textContent=t.id.slice(0,12); a.onclick=()=>replay(t.id); c.appendChild(a);
    tr.insertCell().textContent=t.n+' events';});
  if(!(d.traces||[]).length) tb.innerHTML='<tr><td class="mut">no incidents recorded yet</td></tr>';
}
async function replay(id){
  const d=await getJSON('/api/incident?trace='+id);
  $('#replayCard').classList.remove('hide');
  $('#replay').textContent=(d.timeline||[]).map(e=>e.ts+'  '+e.service+'  '+e.span+'/'+e.event).join('\n');
}
</script></body></html>`
