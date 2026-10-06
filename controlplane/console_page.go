package controlplane

// consoleHTML is the operator console page: a thin client over the /api routes.
// All authority lives server-side in goverlord, so this is only a view +
// action surface wired to live state (never a mockup).
const consoleHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Governed Control Plane</title>
<style>
  :root { color-scheme: light dark; --fg:#111; --bg:#fff; --mut:#666; --line:#ddd; --bad:#b00020; --ok:#0a7d33; }
  @media (prefers-color-scheme: dark){ :root{ --fg:#e6e6e6; --bg:#141414; --mut:#9aa; --line:#333; --bad:#ff6b6b; --ok:#4ade80; } }
  body{ font:14px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace; margin:0; padding:24px; color:var(--fg); background:var(--bg); max-width:860px; }
  h1{ font-size:18px; margin:0 0 4px; } .mut{ color:var(--mut); }
  section{ border:1px solid var(--line); border-radius:8px; padding:14px 16px; margin:16px 0; }
  label{ display:inline-block; min-width:70px; } input{ font:inherit; padding:4px 6px; }
  button{ font:inherit; padding:5px 10px; margin:2px; cursor:pointer; }
  table{ border-collapse:collapse; width:100%; } td,th{ text-align:left; padding:3px 8px; border-bottom:1px solid var(--line); }
  .kill{ color:var(--bad); font-weight:bold; } .live{ color:var(--ok); }
  pre{ margin:0; white-space:pre-wrap; } #msg{ min-height:20px; }
</style>
</head>
<body>
<h1>Governed Control Plane <span id="status"></span></h1>
<p class="mut">Operator actions flow through goverlord: RBAC, four-eyes approval, versioned rollback, kill switch.</p>

<section>
  <label>Operator</label><input id="op" value="alice" size="10">
  <span class="mut">alice=operator · bob=approver · carol=sre</span>
</section>

<section>
  <strong>State</strong> — version <span id="ver">?</span>
  <pre id="config"></pre>
  <div id="pending"></div>
</section>

<section>
  <strong>Propose config change</strong><br>
  <label>key</label><input id="ckey" value="planner_model" size="16">
  <label>value</label><input id="cval" value="planner-v2" size="16">
  <input id="note" placeholder="note" size="20">
  <button onclick="propose()">Propose</button>
</section>

<section>
  <strong>Controls</strong><br>
  <label>rollback→</label><input id="to" value="0" size="4"><button onclick="rollback()">Rollback</button>
  <button onclick="kill(true)">Engage kill switch</button>
  <button onclick="kill(false)">Disengage</button>
</section>

<section>
  <strong>History</strong>
  <table id="history"><thead><tr><th>v</th><th>by</th><th>note</th></tr></thead><tbody></tbody></table>
</section>

<section>
  <strong>Persisted decisions</strong> <span class="mut">(survive restart)</span>
  <table id="decisions"><thead><tr><th>proposal</th><th>proposer</th><th>approver</th><th>note</th></tr></thead><tbody></tbody></table>
</section>

<section id="msg" class="mut"></section>

<script>
const op = () => document.getElementById('op').value.trim();
const msg = (t, bad) => { const m=document.getElementById('msg'); m.textContent=t; m.className = bad?'kill':'live'; };
async function api(path, body){
  const r = await fetch(path, body?{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:undefined);
  const data = await r.json().catch(()=>({}));
  if(!r.ok){ throw new Error(data.error || ('HTTP '+r.status)); }
  return data;
}
async function refresh(){
  const s = await api('/api/state');
  document.getElementById('ver').textContent = s.version;
  document.getElementById('config').textContent = JSON.stringify(s.config, null, 2);
  document.getElementById('status').innerHTML = s.killed ? '<span class="kill">[KILLED]</span>' : '<span class="live">[live]</span>';
  const pend = (s.pending||[]).map(p =>
    'proposal '+p.ID.slice(0,8)+' by '+p.By+' — '+JSON.stringify(p.Change.Set)+
    ' <button onclick="approve(\''+p.ID+'\')">Approve</button>'+
    ' <button onclick="reject(\''+p.ID+'\')">Reject</button>').join('<br>');
  document.getElementById('pending').innerHTML = pend ? '<em>pending:</em><br>'+pend : '<span class="mut">no pending proposals</span>';
  const tb = document.querySelector('#history tbody'); tb.innerHTML='';
  (s.history||[]).forEach(v => { const tr=tb.insertRow(); tr.insertCell().textContent=v.N; tr.insertCell().textContent=v.By; tr.insertCell().textContent=v.Note; });
  try {
    const h = await api('/api/history');
    const dt = document.querySelector('#decisions tbody'); dt.innerHTML='';
    (h.approvals||[]).forEach(a => { const tr=dt.insertRow();
      tr.insertCell().textContent=(a.ID||'').slice(0,8); tr.insertCell().textContent=a.Proposer;
      tr.insertCell().textContent=a.Approver; tr.insertCell().textContent=a.Note; });
  } catch(e) {}
}
async function guard(fn){ try{ await fn(); msg('ok'); }catch(e){ msg(e.message, true); } await refresh(); }
const propose = () => guard(async()=>{ const set={}; set[document.getElementById('ckey').value]=document.getElementById('cval').value;
  const d=await api('/api/propose',{op:op(),note:document.getElementById('note').value,set}); msg(d.applied?'applied':'pending approval ('+d.proposal_id.slice(0,8)+')'); });
const approve = (id) => guard(()=>api('/api/approve',{op:op(),id}));
const reject = (id) => guard(()=>api('/api/reject',{op:op(),id}));
const rollback = () => guard(()=>api('/api/rollback',{op:op(),to:parseInt(document.getElementById('to').value,10)}));
const kill = (engage) => guard(()=>api('/api/killswitch',{op:op(),engage}));
refresh();
</script>
</body>
</html>`
