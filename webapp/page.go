package webapp

const dashboardHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Agent Console</title>
<style>
  :root{color-scheme:light dark;--fg:#141414;--bg:#fafafa;--card:#fff;--mut:#667;--line:#e3e3e8;--ok:#0a7d33;--bad:#b00020;--accent:#2f6feb}
  @media (prefers-color-scheme:dark){:root{--fg:#e8e8ea;--bg:#131316;--card:#1b1b1f;--mut:#9aa;--line:#2a2a30;--ok:#4ade80;--bad:#ff6b6b;--accent:#6ea8fe}}
  *{box-sizing:border-box} body{font:15px/1.55 system-ui,-apple-system,Segoe UI,sans-serif;margin:0;color:var(--fg);background:var(--bg)}
  header{padding:18px 24px;border-bottom:1px solid var(--line);display:flex;gap:18px;align-items:baseline}
  h1{font-size:18px;margin:0} nav{display:flex;gap:4px} nav button{font:inherit;padding:6px 12px;border:0;background:none;color:var(--mut);cursor:pointer;border-radius:8px}
  nav button.active{background:var(--card);color:var(--fg);box-shadow:0 1px 0 var(--line)}
  main{padding:24px;max-width:900px;margin:0 auto} .mut{color:var(--mut)}
  .card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:16px 18px;margin:12px 0}
  button.go{font:inherit;padding:8px 14px;border:0;border-radius:9px;background:var(--accent);color:#fff;cursor:pointer}
  table{border-collapse:collapse;width:100%} td,th{text-align:left;padding:7px 10px;border-bottom:1px solid var(--line);font-size:14px}
  .pill{font-size:12px;padding:2px 8px;border-radius:999px} .pass{background:color-mix(in srgb,var(--ok) 18%,transparent);color:var(--ok)}
  .fail{background:color-mix(in srgb,var(--bad) 18%,transparent);color:var(--bad)}
  .trace{font-family:ui-monospace,Menlo,monospace;cursor:pointer;color:var(--accent)} pre{white-space:pre-wrap;font-size:13px;margin:0}
  .ev{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:11px 0;border-bottom:1px solid var(--line)}
  .ev:last-child{border-bottom:0} .ev b{font-size:15px} .bell{margin-right:4px} a.go{text-decoration:none;display:inline-block}
  .dz{border:2px dashed var(--line);border-radius:12px;padding:16px;margin:10px 0;transition:border-color .15s} .dz.over{border-color:var(--accent);background:color-mix(in srgb,var(--accent) 7%,transparent)}
  .link{color:var(--accent);cursor:pointer} textarea{font:inherit;background:var(--card);color:var(--fg);border:1px solid var(--line);border-radius:8px;padding:8px;width:100%}
  .hide{display:none}
</style></head><body>
<header><h1>🗓️ Agent Console</h1>
  <nav>
    <button data-tab="calendar" class="active">Calendar</button>
    <button data-tab="security">Security</button>
    <button data-tab="incidents">Incidents</button>
  </nav>
</header>
<main>
  <section id="calendar">
    <div class="card">
      <strong>Calendar</strong> <span class="mut">— events the agent extracted; accept downloads the signed .ics (with a reminder)</span>
      <p class="mut" id="inbox"></p>
      <div class="dz" id="dropzone">
        Drag a <b>.txt</b> email here, or <label class="link">choose a file<input id="file" type="file" accept=".txt" hidden></label>, or paste below.
        <div style="margin-top:8px"><textarea id="paste" rows="3" placeholder="paste email text…"></textarea></div>
        <div style="margin-top:8px"><button class="go" onclick="dropText()">Process email</button> <span id="dropmsg" class="mut"></span></div>
      </div>
      <div id="events"></div>
    </div>
  </section>
  <section id="security" class="hide">
    <div class="card">
      <strong>Security scorecard</strong>
      <p class="mut">Run the full red-team suite against the live controls. Every attack should drop to 0%.</p>
      <button class="go" onclick="runScorecard()">Run scorecard</button>
      <span id="verdict"></span>
      <table id="score" class="hide"><thead><tr><th>technique</th><th>control</th><th>before</th><th>after</th><th></th></tr></thead><tbody></tbody></table>
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
const $=s=>document.querySelector(s);
const TABS=['calendar','security','incidents'];
const esc=s=>{const d=document.createElement('div');d.textContent=s||'';return d.innerHTML;};
document.querySelectorAll('nav button').forEach(b=>b.onclick=()=>{
  document.querySelectorAll('nav button').forEach(x=>x.classList.toggle('active',x===b));
  TABS.forEach(t=>$('#'+t).classList.toggle('hide',t!==b.dataset.tab));
  if(b.dataset.tab==='incidents') loadIncidents();
  if(b.dataset.tab==='calendar') loadEvents();
});
async function loadEvents(){
  const d=await (await fetch('/api/events')).json();
  if(d.inbox) $('#inbox').innerHTML='📥 Drop <b>.txt</b> emails here: <code>'+esc(d.inbox)+'</code>';
  const c=$('#events'); c.innerHTML='';
  if(!(d.events||[]).length){c.innerHTML='<p class="mut">no events yet — drop a .txt email in the inbox folder above</p>';return;}
  d.events.forEach(e=>{const div=document.createElement('div');div.className='ev';
    const when=e.all_day?(e.start+' · all day'):(e.start+(e.end?(' – '+e.end.slice(11)):''));
    div.innerHTML='<div><b>'+esc(e.title)+'</b><br><span class="mut">'+esc(when)+(e.location?(' · '+esc(e.location)):'')+'</span></div>'+
      '<div>'+(e.has_reminder?'<span class="bell">🔔</span>':'')+'<a class="go" href="/ics/'+encodeURIComponent(e.file)+'" download>Accept .ics</a></div>';
    c.appendChild(div);});
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
  $('#dropmsg').textContent='queued — events appear shortly';
  setTimeout(loadEvents,2500);setTimeout(loadEvents,5000);}
loadEvents();
async function runScorecard(){
  $('#verdict').textContent=' running…';
  const d=await (await fetch('/api/scorecard')).json();
  const tb=$('#score tbody'); tb.innerHTML='';
  (d.results||[]).forEach(r=>{const tr=tb.insertRow();
    tr.insertCell().textContent=r.name; tr.insertCell().textContent=r.technique;
    tr.insertCell().textContent=r.before.toFixed(0)+'%'; tr.insertCell().textContent=r.after.toFixed(0)+'%';
    const c=tr.insertCell(); c.innerHTML='<span class="pill '+(r.pass?'pass':'fail')+'">'+(r.pass?'ok':'FAIL')+'</span>';});
  $('#score').classList.remove('hide');
  $('#verdict').innerHTML=d.all_pass?' <span class="pill pass">all defenses hold</span>':' <span class="pill fail">regression!</span>';
}
async function loadIncidents(){
  const d=await (await fetch('/api/incidents')).json();
  const tb=$('#traces tbody'); tb.innerHTML='';
  (d.traces||[]).forEach(t=>{const tr=tb.insertRow();
    const c=tr.insertCell(); const a=document.createElement('span'); a.className='trace'; a.textContent=t.id.slice(0,12); a.onclick=()=>replay(t.id); c.appendChild(a);
    tr.insertCell().textContent=t.n+' events';});
  if(!(d.traces||[]).length) tb.innerHTML='<tr><td class="mut">no incidents recorded yet</td></tr>';
}
async function replay(id){
  const d=await (await fetch('/api/incident?trace='+id)).json();
  $('#replayCard').classList.remove('hide');
  $('#replay').textContent=(d.timeline||[]).map(e=>e.ts+'  '+e.service+'  '+e.span+'/'+e.event).join('\n');
}
</script></body></html>`
