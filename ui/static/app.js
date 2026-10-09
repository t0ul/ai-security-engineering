// Capability token (C4b): when the server runs as an authZ'd API it injects the
// operator Grant here, and this wrapper attaches it to every same-origin API/ICS
// request. Empty (household mode) = no header, unchanged behavior.
(function(){const f=window.fetch.bind(window);window.fetch=(u,o)=>{o=o||{};const s=typeof u==='string'?u:(u&&u.url)||'';if(__CAP__&&(s.indexOf('/api/')===0||s.indexOf('/ics/')===0)){o.headers=Object.assign({},o.headers||{},{'Authorization':'Bearer '+__CAP__});}return f(u,o);};})();
const $=s=>document.querySelector(s);
const TABS=['chat','calendar','week','tasks','review','activity','ask','security','prompts','sampling','models','runtime','skills','retrieval','rag','grammar','policies','budgets','eval','incidents'];
const esc=s=>{const d=document.createElement('div');d.textContent=s||'';return d.innerHTML;};
const when=e=>e.all_day?((e.due||e.start)+' · all day'):((e.start||e.due)+(e.end?(' – '+e.end.slice(11)):''));
document.querySelectorAll('nav button').forEach(b=>b.onclick=()=>{
  document.querySelectorAll('nav button').forEach(x=>x.classList.toggle('active',x===b));
  TABS.forEach(t=>$('#'+t).classList.toggle('hide',t!==b.dataset.tab));
  const t=b.dataset.tab;
  if(t==='chat'){const q=$('#chatq');if(q)q.focus();loadMiniCal();loadChatHistory();}
  if(t==='calendar'){loadEvents();loadDeadlines();}
  if(t==='week'){loadWeek('today');loadProfile();}
  if(t==='tasks') loadTasks();
  if(t==='review') loadReview();
  if(t==='ask') loadDirectory();
  if(t==='activity') loadActivity();
  if(t==='incidents') loadIncidents();
  if(t==='security'){loadKill();loadMCP();loadBundles();}
  if(t==='prompts') loadPrompts();
  if(t==='sampling') loadSampling();
  if(t==='models'){loadModels();loadModelCatalog();}
  if(t==='runtime') loadRuntime();
  if(t==='rag') loadRAG();
  if(t==='skills') loadSkills();
  if(t==='retrieval') loadRetrieval();
  if(t==='grammar') loadGrammar();
  if(t==='budgets') loadBudgets();
  if(t==='policies') loadPolicies();
  if(t==='eval') loadEval();
});
// --- App / Studio surface split: 1 view, 1 job ---
// App = consumer (zero control knobs); Studio = operator (the tuning/governance plane).
const SURFACES={
  app:['chat','calendar','week','tasks','review','ask'],
  studio:['security','prompts','sampling','models','runtime','skills','retrieval','rag','grammar','policies','budgets','eval','incidents','activity']
};
(function(){
  const vis=(SURFACES[SURFACE]||TABS);
  document.querySelectorAll('nav button').forEach(b=>{ if(vis.indexOf(b.dataset.tab)<0) b.style.display='none'; });
  TABS.forEach(t=>{ if(vis.indexOf(t)<0){ const el=$('#'+t); if(el) el.classList.add('hide'); }});
  // hide a group label if no visible button follows it (until the next label)
  const nav=document.querySelector('nav'); let grp=null, seen=false;
  [...nav.children].forEach(el=>{
    if(el.classList&&el.classList.contains('grp')){ if(grp&&!seen) grp.style.display='none'; grp=el; seen=false; }
    else if(el.tagName==='BUTTON' && el.style.display!=='none'){ seen=true; }
  });
  if(grp&&!seen) grp.style.display='none';
  const sw=$('#surfaceSwitch');
  if(sw){ if(SURFACE==='studio'){ sw.textContent='🏠 Back to App'; sw.href='/'; } else { sw.textContent='⚙ Studio'; sw.href='/studio'; } }
  const first=document.querySelector('nav button[data-tab="'+vis[0]+'"]');
  if(first) first.click(); // land on this surface's first tab
})();
async function doAsk(){
  const q=$('#askq').value.trim();const c=$('#askOut');if(!q){c.innerHTML='';return;}
  c.innerHTML='<span class="mut">searching…</span>';
  const d=await getJSON('/api/ask?q='+encodeURIComponent(q));const hits=d.hits||[];
  if(!hits.length){c.innerHTML='<p class="mut">no matches — drop more emails to build the corpus</p>';return;}
  c.innerHTML=hits.map(h=>'<div class="ev"><div><b>'+esc(h.source)+'</b>'+(h.untrusted?' <span class="kind warn">untrusted</span>':'')+'<br><span class="mut">'+esc(h.snippet)+'</span></div></div>').join('');
}
document.addEventListener('keydown',e=>{if(e.key==='Enter'&&document.activeElement&&document.activeElement.id==='askq')doAsk();});
async function loadDirectory(){
  const c=$('#directory');if(!c)return;const d=await getJSON('/api/directory');
  const ppl=d.people||[];const em=d.emails||[];
  if(!ppl.length&&!em.length){c.innerHTML='<p class="mut">no contacts surfaced yet</p>';return;}
  let h='';
  if(ppl.length) h+=ppl.map(p=>'<div class="ev"><div><b>'+esc(p.name)+'</b> <span class="kind">'+esc(p.role)+'</span></div></div>').join('');
  if(em.length) h+='<div class="ev"><div><b>Emails</b><br><span class="mut">'+em.map(esc).join(', ')+'</span></div></div>';
  c.innerHTML=h;
}
function chatFeedbackHTML(turnId,rating){
  if(!turnId)return '';
  const on=x=>rating===x?' on':'';
  return '<div class="fb" data-turn="'+turnId+'">'+
    '<button class="thumb'+on('up')+'" title="good" onclick="rateChat('+turnId+',\'up\',this)">👍</button>'+
    '<button class="thumb'+on('neutral')+'" title="ok" onclick="rateChat('+turnId+',\'neutral\',this)">😐</button>'+
    '<button class="thumb'+on('down')+'" title="bad" onclick="rateChat('+turnId+',\'down\',this)">👎</button></div>';
}
function chatMetaHTML(r){
  const bits=[];
  if(r.model)bits.push('model '+esc(r.model));
  if(r.prompt_tokens)bits.push(r.prompt_tokens+' ctx + '+(r.completion_tokens||0)+' out tok');
  if(r.retrieval)bits.push(r.retrieval.indexOf('unavailable')>=0?'<span style="color:var(--bad)">'+esc(r.retrieval)+'</span>':esc(r.retrieval));
  return bits.length?'<div class="meta mut">'+bits.join(' · ')+'</div>':'';
}
function assistantHTML(r,unsafe){
  const src=(r.sources&&r.sources.length)?'<div class="src">sources: '+r.sources.map(esc).join(', ')+'</div>':'';
  const warn=unsafe?'<div class="src" style="color:var(--bad)">⚠ controls off — retrieved text spliced in raw (injection can hijack)</div>':'';
  return '<div class="msg a">'+esc(r.answer||'(no answer)')+warn+src+chatMetaHTML(r)+chatFeedbackHTML(r.turn_id,r.rating)+'</div>';
}
function updateChatStatus(r){
  const el=$('#chatStatus');if(!el)return;
  if(!r||(!r.model&&!r.prompt_tokens)){el.textContent='';return;}
  let s=r.model?('model: '+r.model):'';
  if(r.context_limit){const pct=Math.round(100*(r.prompt_tokens||0)/r.context_limit);s+=(s?'  ·  ':'')+'context '+(r.prompt_tokens||0)+' / '+r.context_limit+' ('+pct+'%)';}
  else if(r.prompt_tokens){s+=(s?'  ·  ':'')+r.prompt_tokens+' ctx + '+(r.completion_tokens||0)+' out tokens';}
  el.textContent=s;
}
async function loadChatHistory(){
  const log=$('#chatlog');if(!log)return;
  const d=await getJSON('/api/chat/history');const turns=d.turns||[];
  log.innerHTML='';let last=null;
  turns.forEach(t=>{
    if(t.role==='user')log.insertAdjacentHTML('beforeend','<div class="msg u">'+esc(t.content)+'</div>');
    else{log.insertAdjacentHTML('beforeend',assistantHTML({answer:t.content,sources:t.sources,model:t.model,prompt_tokens:t.prompt_tokens,completion_tokens:t.completion_tokens,turn_id:t.id,rating:t.rating},false));last={model:t.model,prompt_tokens:t.prompt_tokens,completion_tokens:t.completion_tokens};}
  });
  updateChatStatus(last);
  log.scrollTop=log.scrollHeight;
}
async function rateChat(turnId,rating,btn){
  const wrap=btn.closest('.fb');
  const r=await postJSON('/api/chat/feedback',{turn_id:turnId,rating:rating});
  if(r.ok&&wrap){wrap.querySelectorAll('.thumb').forEach(b=>b.classList.remove('on'));btn.classList.add('on');}
}
async function clearChat(){
  if(!confirm('Clear the conversation history?'))return;
  const r=await postJSON('/api/chat/clear',{});
  if(r.ok){const log=$('#chatlog');if(log)log.innerHTML='';updateChatStatus(null);}
}
async function sendChat(){
  const inp=$('#chatq');const q=(inp.value||'').trim();if(!q)return;const log=$('#chatlog');
  const unsafe=$('#chatUnsafe')&&$('#chatUnsafe').checked;
  log.insertAdjacentHTML('beforeend','<div class="msg u">'+esc(q)+'</div>');inp.value='';
  log.insertAdjacentHTML('beforeend','<div class="msg a" id="pending"><span class="mut">thinking…</span></div>');log.scrollTop=log.scrollHeight;
  let r;try{r=await (await postJSON('/api/chat',{question:q,unsafe:unsafe})).json();}catch(e){r={answer:'error'};}
  const p=document.getElementById('pending');if(p)p.remove();
  log.insertAdjacentHTML('beforeend',assistantHTML(r,unsafe));
  updateChatStatus(r);
  log.scrollTop=log.scrollHeight;
}
document.addEventListener('keydown',e=>{if(e.key==='Enter'&&document.activeElement&&document.activeElement.id==='chatq')sendChat();});
async function doEnrich(){
  const u=$('#enrichURL').value.trim();const c=$('#enrichOut');if(!u){return;}
  // phase 1: request the HITL challenge
  let r=await (await postJSON('/api/enrich',{url:u})).json();
  if(r.confirm_required){
    if(!confirm('Fetch into the knowledge base?\n'+(r.evidence||[]).join('\n'))){c.textContent='cancelled';return;}
    r=await (await postJSON('/api/enrich',{url:u,nonce:r.nonce,confirm:true})).json();
  }
  if(r.refused){c.innerHTML='<span class="kind warn">refused</span> '+esc(r.refused);return;}
  if(r.ok){c.innerHTML='indexed '+r.bytes+' bytes from <b>'+esc(r.url)+'</b><br><span class="mut">'+esc(r.preview||'')+'</span>';}
}
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
async function loadBundles(){
  const c=$('#bundleList');if(!c)return;const d=await getJSON('/api/bundles');const bs=d.bundles||[];
  if(!bs.length){c.innerHTML='<p class="mut">no snapshots yet</p>';return;}
  c.innerHTML=bs.map(b=>'<div class="ev"><div><b>'+esc(b.label)+'</b> <span class="mut">'+esc(b.at)+'</span> <button class="ghost" onclick="applyBundle(\''+esc(b.label)+'\')">Roll back to this</button></div></div>').join('');
}
async function saveBundle(){
  const l=$('#bundleLabel').value.trim();if(!l)return;
  await postJSON('/api/bundles/save',{label:l});$('#bundleLabel').value='';loadBundles();
}
async function applyBundle(label){
  if(!confirm('Roll the ENTIRE governed plane (prompts + sampling + policies) back to "'+label+'"?'))return;
  await postJSON('/api/bundles/apply',{label:label});loadBundles();
}
async function loadPrompts(){
  const c=$('#promptList');if(!c)return;const d=await getJSON('/api/prompts');const ps=d.prompts||[];
  if(!ps.length){c.innerHTML='<p class="mut">no prompts registered</p>';return;}
  c.innerHTML=ps.map(p=>{
    const governed=p.version>0;
    return '<div class="ev"><div><b>'+esc(p.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+p.version+'</span> <span class="mut">'+esc(p.hash)+'</span>'+
      '<br><textarea id="pt_'+esc(p.name)+'" rows="5" style="width:100%;margin-top:6px">'+esc(p.text)+'</textarea>'+
      '<div style="margin-top:6px"><button class="ghost" onclick="testPrompt(\''+esc(p.name)+'\')">Test (shadow eval)</button> '+
      '<button class="go" onclick="activatePrompt(\''+esc(p.name)+'\')">Activate new version</button>'+
      (governed?(' <button class="ghost" onclick="resetPrompt(\''+esc(p.name)+'\')">Reset to default</button>'):'')+
      '<div id="pttest_'+esc(p.name)+'" class="mut" style="margin-top:4px"></div></div></div></div>';
  }).join('');
}
async function activatePrompt(name){
  const ta=$('#pt_'+name);if(!ta)return;
  await postJSON('/api/prompts/activate',{name:name,text:ta.value});loadPrompts();
}
async function resetPrompt(name){await postJSON('/api/prompts/reset',{name:name});loadPrompts();}
async function loadSampling(){
  const c=$('#samplingList');if(!c)return;const d=await getJSON('/api/sampling');const ss=d.sampling||[];
  if(!ss.length){c.innerHTML='<p class="mut">no models registered</p>';return;}
  c.innerHTML=ss.map(s=>{const cf=s.config||{};const governed=s.version>0;
    return '<div class="ev"><div><b>'+esc(s.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+s.version+'</span> <span class="mut">'+esc(s.hash)+'</span>'+
      '<div style="margin-top:6px">temp <input id="st_'+esc(s.name)+'" type="number" step="0.1" value="'+(cf.temperature||0)+'" style="width:70px"> '+
      'max tokens <input id="sm_'+esc(s.name)+'" type="number" value="'+(cf.max_tokens||0)+'" style="width:90px"> '+
      'seed <input id="ss_'+esc(s.name)+'" type="number" value="'+(cf.seed||0)+'" style="width:90px"></div>'+
      '<div style="margin-top:6px"><button class="go" onclick="activateSampling(\''+esc(s.name)+'\')">Activate</button>'+
      (governed?(' <button class="ghost" onclick="resetSampling(\''+esc(s.name)+'\')">Reset to default</button>'):'')+'</div></div></div>';
  }).join('');
}
async function activateSampling(name){
  const cfg={temperature:parseFloat($('#st_'+name).value)||0,max_tokens:parseInt($('#sm_'+name).value)||0,seed:parseInt($('#ss_'+name).value)||0};
  await postJSON('/api/sampling/activate',{name:name,config:cfg});loadSampling();
}
async function resetSampling(name){await postJSON('/api/sampling/reset',{name:name});loadSampling();}
let __CAT__=[];
// options for a role's model <select>: every catalog name, plus the current binding
// if it is not (any longer) in the catalog, so a stale binding stays visible.
function modelOpts(cur){
  let o=__CAT__.map(e=>'<option'+(e.name===cur?' selected':'')+'>'+esc(e.name)+'</option>').join('');
  if(!__CAT__.some(e=>e.name===cur)) o+='<option selected>'+esc(cur)+'</option>';
  return o;
}
async function loadModels(){
  const c=$('#modelsList');if(!c)return;
  const [d,cd]=await Promise.all([getJSON('/api/models'),getJSON('/api/modelcatalog')]);
  const ms=d.models||[]; __CAT__=cd.catalog||[];
  if(!ms.length){c.innerHTML='<p class="mut">no roles registered</p>';return;}
  c.innerHTML=ms.map(m=>{const governed=m.version>0;
    return '<div class="ev"><div><b>'+esc(m.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+m.version+'</span> <span class="mut">'+esc(m.hash)+'</span>'+
      '<div style="margin-top:6px">model <select id="md_'+esc(m.name)+'">'+modelOpts(m.model)+'</select></div>'+
      '<div style="margin-top:6px"><button class="go" onclick="activateModel(\''+esc(m.name)+'\')">Bind</button>'+
      (governed?(' <button class="ghost" onclick="resetModel(\''+esc(m.name)+'\')">Reset to default</button>'):'')+'</div></div></div>';
  }).join('');
}
async function activateModel(name){await postJSON('/api/models/activate',{name:name,model:$('#md_'+name).value});loadModels();}
async function resetModel(name){await postJSON('/api/models/reset',{name:name});loadModels();}
async function loadModelCatalog(){
  const c=$('#modelCatalog');if(!c)return;const d=await getJSON('/api/modelcatalog');const cat=d.catalog||[];__CAT__=cat;
  if(!cat.length){c.innerHTML='<p class="mut">catalog empty</p>';return;}
  c.innerHTML=cat.map(m=>{
    const pin=m.pinned?'<span class="pill pass">SHA pinned</span>':'<span class="pill">magic-verify only</span>';
    return '<div class="ev"><div><b>'+esc(m.name)+'</b> '+pin+' <span class="mut">:'+m.port+' · ctx '+m.ctx+' · '+esc(m.host)+'</span>'+
      '<div class="mut" style="margin-top:4px">'+esc(m.file)+'</div>'+
      '<div class="mut" style="margin-top:2px;word-break:break-all">'+esc(m.url)+'</div>'+
      '<div style="margin-top:6px"><button class="ghost" onclick="editModel(\''+esc(m.name)+'\')">Edit</button> '+
      '<button class="ghost" onclick="delModel(\''+esc(m.name)+'\')">Delete</button></div></div></div>';
  }).join('');
}
function editModel(name){const m=__CAT__.find(e=>e.name===name);if(!m)return;
  $('#mc_name').value=m.name;$('#mc_file').value=m.file;$('#mc_url').value=m.url;
  $('#mc_sha').value=m.sha256||'';$('#mc_port').value=m.port||'';$('#mc_ctx').value=m.ctx||'';
  $('#mcMsg').textContent='editing '+name;}
async function saveModel(){
  const body={name:$('#mc_name').value.trim(),file:$('#mc_file').value.trim(),url:$('#mc_url').value.trim(),
    sha256:$('#mc_sha').value.trim(),port:parseInt($('#mc_port').value)||0,ctx:parseInt($('#mc_ctx').value)||0};
  if(!body.name||!body.url||!body.file){$('#mcMsg').textContent='name, url and file are required';return;}
  const r=await postJSON('/api/modelcatalog/upsert',body);
  if(r.ok){$('#mcMsg').textContent='saved ✓';['mc_name','mc_file','mc_url','mc_sha','mc_port','mc_ctx'].forEach(id=>$('#'+id).value='');loadModelCatalog();loadModels();}
  else{$('#mcMsg').textContent='error: '+(await r.text());}
}
async function delModel(name){
  if(!confirm('Delete model "'+name+'" from the catalog?'))return;
  const r=await postJSON('/api/modelcatalog/delete',{name:name});
  if(r.ok){loadModelCatalog();loadModels();}
  else{alert(await r.text());}
}
function ragStatsText(s){return s?('corpus: '+s.docs+' docs → '+s.chunks+' chunks · mode '+esc(s.mode)):'';}
async function loadRAG(){
  const c=$('#ragStats');const d=await getJSON('/api/rag');const cfg=d.config||{};
  const chSel=$('#rag_chunker');if(chSel){chSel.innerHTML=(d.chunkers||[]).map(n=>'<option'+(n===cfg.chunker?' selected':'')+'>'+esc(n)+'</option>').join('');}
  const emSel=$('#rag_embedder');if(emSel){emSel.innerHTML=(d.embedders||[]).map(n=>'<option'+(n===cfg.embedder?' selected':'')+'>'+esc(n)+'</option>').join('');}
  if($('#rag_mode'))$('#rag_mode').value=cfg.mode||'fts';
  if($('#rag_size'))$('#rag_size').value=cfg.chunk_size||'';
  if($('#rag_overlap'))$('#rag_overlap').value=cfg.chunk_overlap||'';
  if(c)c.textContent=ragStatsText(d.stats);
}
function ragConfigFromForm(){return{mode:$('#rag_mode').value,chunker:$('#rag_chunker').value,embedder:$('#rag_embedder').value,chunk_size:parseInt($('#rag_size').value)||0,chunk_overlap:parseInt($('#rag_overlap').value)||0};}
async function saveRAG(){
  $('#ragMsg').textContent='saving + reindexing (semantic boots a model, can take a bit)…';
  const r=await postJSON('/api/rag/save',ragConfigFromForm());
  if(r.ok){const j=await r.json();$('#ragMsg').textContent='saved ✓';$('#ragStats').textContent=ragStatsText(j.stats);}
  else{$('#ragMsg').textContent='error: '+(await r.text());}
}
async function reindexRAG(){
  $('#ragMsg').textContent='reindexing…';
  const r=await postJSON('/api/rag/reindex',{});
  if(r.ok){const j=await r.json();$('#ragMsg').textContent='reindexed ✓';$('#ragStats').textContent=ragStatsText(j.stats);}
  else{$('#ragMsg').textContent='error: '+(await r.text());}
}
async function loadRuntime(){
  const c=$('#runtimeOut');if(!c)return;const d=await getJSON('/api/runtime');const gw=d.gateway||{};
  const gwPill=gw.up?'<span class="pill pass">up</span>':'<span class="pill fail">down</span>';
  let html='';
  const id=d.identity||{};
  if(id.ephemeral) html+='<div class="ev"><div><b>agent identity</b> <span class="pill fail">ephemeral</span> <span class="mut">throwaway key — signed .ics will not verify across restarts</span></div></div>';
  html+='<div class="ev"><div><b>gateway</b> '+gwPill+' <span class="mut">'+esc(gw.url||'')+'</span></div></div>';
  html+='<div class="mut" style="margin:10px 0 4px">model servers <span style="opacity:.7">(assets: '+esc(d.assets_dir||'')+')</span></div>';
  (d.models||[]).forEach(m=>{
    const file=m.present?(m.valid_gguf?'<span class="pill pass">file ok</span>':'<span class="pill fail">file invalid</span>'):'<span class="pill fail">file missing</span>';
    const port=m.port_open?'<span class="pill pass">:'+m.port+' serving</span>':'<span class="pill">:'+m.port+' down</span>';
    html+='<div class="ev"><div><b>'+esc(m.name)+'</b> '+file+' '+port+' <span class="mut">'+esc(m.file)+'</span></div></div>';
  });
  if(!d.models||!d.models.length) html+='<p class="mut">no models in catalog</p>';
  c.innerHTML=html;
}
async function loadSkills(){
  const c=$('#skillList');if(!c)return;const d=await getJSON('/api/skills');const cat=d.catalog||[];
  if(!cat.length){c.innerHTML='<p class="mut">no skills in the catalog</p>';}
  else c.innerHTML=cat.map(s=>{
    const trust=s.signer_trusted?'<span class="pill pass">signed · trusted</span>':'<span class="pill fail">untrusted signer</span>';
    const appr=s.approved?'<span class="pill pass">approved</span>':'<span class="pill">unapproved</span>';
    return '<div class="ev"><div><b>'+esc(s.name)+'</b> <span class="mut">v'+s.version+' · '+esc(s.hash)+'</span><div style="margin-top:4px">'+trust+' '+appr+'</div>'+
      '<div style="margin-top:6px">'+
      (s.approved?('<button class="ghost" onclick="revokeSkill(\''+esc(s.name)+'\')">Revoke approval</button>')
                 :('<button class="go" onclick="approveSkill(\''+esc(s.name)+'\')">Approve this version</button>'))+
      '</div></div></div>';
  }).join('');
  const sel=$('#skillLoadName');if(sel){sel.innerHTML=cat.map(s=>'<option value="'+esc(s.name)+'">'+esc(s.name)+'</option>').join('');}
}
async function approveSkill(name){await postJSON('/api/skills/approve',{name:name});loadSkills();}
async function revokeSkill(name){await postJSON('/api/skills/reset',{name:name});loadSkills();}
async function loadSkill(){
  const name=$('#skillLoadName').value;const unsafe=$('#skillUnsafe').checked;
  const r=await postJSON('/api/skills/load',{name:name,unsafe:unsafe});
  const out=$('#skillLoadOut');
  if(r.loaded){out.textContent=(r.unsafe?'⚠ CONTROLS OFF — loaded unchecked:\n\n':'✓ loaded (gate passed):\n\n')+(r.instructions||'');}
  else{out.textContent='⛔ '+(r.reason||'refused');}
}
async function loadRetrieval(){
  const c=$('#retrievalList');if(!c)return;const d=await getJSON('/api/retrieval');const rs=d.retrieval||[];
  if(!rs.length){c.innerHTML='<p class="mut">no corpora registered</p>';return;}
  c.innerHTML=rs.map(s=>{const cf=s.config||{};const governed=s.version>0;
    return '<div class="ev"><div><b>'+esc(s.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+s.version+'</span> <span class="mut">'+esc(s.hash)+'</span>'+
      '<div style="margin-top:6px">top-k <input id="rk_'+esc(s.name)+'" type="number" value="'+(cf.k||0)+'" style="width:80px"></div>'+
      '<div style="margin-top:6px"><button class="go" onclick="activateRetrieval(\''+esc(s.name)+'\')">Activate</button>'+
      (governed?(' <button class="ghost" onclick="resetRetrieval(\''+esc(s.name)+'\')">Reset to default</button>'):'')+'</div></div></div>';
  }).join('');
}
async function activateRetrieval(name){await postJSON('/api/retrieval/activate',{name:name,config:{k:parseInt($('#rk_'+name).value)||0}});loadRetrieval();}
async function resetRetrieval(name){await postJSON('/api/retrieval/reset',{name:name});loadRetrieval();}
async function loadGrammar(){
  const c=$('#grammarList');if(!c)return;const d=await getJSON('/api/grammar');const gs=d.grammars||[];
  if(!gs.length){c.innerHTML='<p class="mut">no grammars registered</p>';return;}
  c.innerHTML=gs.map(g=>{const governed=g.version>0;
    return '<div class="ev"><div><b>'+esc(g.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+g.version+'</span> <span class="mut">'+esc(g.hash)+'</span>'+
      '<div style="margin-top:6px"><textarea id="gt_'+esc(g.name)+'" rows="5" style="width:100%;font-family:monospace;font-size:12px">'+esc(g.text)+'</textarea></div>'+
      '<div style="margin-top:6px"><button class="go" onclick="activateGrammar(\''+esc(g.name)+'\')">Activate</button>'+
      (governed?(' <button class="ghost" onclick="resetGrammar(\''+esc(g.name)+'\')">Reset to default</button>'):'')+'</div></div></div>';
  }).join('');
}
async function activateGrammar(name){await postJSON('/api/grammar/activate',{name:name,text:$('#gt_'+name).value});loadGrammar();}
async function resetGrammar(name){await postJSON('/api/grammar/reset',{name:name});loadGrammar();}
async function loadBudgets(){
  const c=$('#budgetList');if(!c)return;const d=await getJSON('/api/budgets');const bs=d.budgets||[];
  if(!bs.length){c.innerHTML='<p class="mut">no budgets registered</p>';return;}
  c.innerHTML=bs.map(b=>{const cf=b.config||{};const governed=b.version>0;
    const key=cf.key_ref?(' · key <code>'+esc(cf.key_ref)+'</code> <span class="pill '+(b.key_set?'pass':'fail')+'">'+(b.key_set?'set':'unset')+'</span>'):'';
    return '<div class="ev"><div><b>'+esc(b.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+b.version+'</span>'+key+
      '<div style="margin-top:6px">rate/min <input id="br_'+esc(b.name)+'" type="number" value="'+(cf.rate_per_min||0)+'" style="width:80px"> '+
      'max tokens <input id="bt_'+esc(b.name)+'" type="number" value="'+(cf.max_tokens||0)+'" style="width:90px"> '+
      'concurrency <input id="bc_'+esc(b.name)+'" type="number" value="'+(cf.max_concurrency||0)+'" style="width:80px"> '+
      'spend $ <input id="bs_'+esc(b.name)+'" type="number" step="0.01" value="'+(cf.spend_cap_usd||0)+'" style="width:90px"></div>'+
      '<div style="margin-top:6px"><button class="go" onclick="activateBudget(\''+esc(b.name)+'\',\''+esc(cf.key_ref||'')+'\')">Activate</button>'+
      (governed?(' <button class="ghost" onclick="resetBudget(\''+esc(b.name)+'\')">Reset to default</button>'):'')+'</div></div></div>';
  }).join('');
}
async function activateBudget(name,keyRef){
  const cfg={rate_per_min:parseInt($('#br_'+name).value)||0,max_tokens:parseInt($('#bt_'+name).value)||0,max_concurrency:parseInt($('#bc_'+name).value)||0,spend_cap_usd:parseFloat($('#bs_'+name).value)||0,key_ref:keyRef};
  await postJSON('/api/budgets/activate',{name:name,config:cfg});loadBudgets();
}
async function resetBudget(name){await postJSON('/api/budgets/reset',{name:name});loadBudgets();}
async function loadPolicies(){
  const c=$('#policyList');if(!c)return;const d=await getJSON('/api/policies');const ps=d.policies||[];
  if(!ps.length){c.innerHTML='<p class="mut">no policies registered</p>';return;}
  c.innerHTML=ps.map(p=>{
    const governed=p.version>0;
    return '<div class="ev"><div><b>'+esc(p.name)+'</b> <span class="pill '+(governed?'pass':'')+'">v'+p.version+'</span> <span class="mut">'+esc(p.hash)+' · '+(p.items||[]).length+' entries</span>'+
      '<br><textarea id="pol_'+esc(p.name)+'" rows="5" style="width:100%;margin-top:6px" placeholder="one entry per line">'+esc((p.items||[]).join('\n'))+'</textarea>'+
      '<div style="margin-top:6px"><button class="go" onclick="activatePolicy(\''+esc(p.name)+'\')">Activate new version</button>'+
      (governed?(' <button class="ghost" onclick="resetPolicy(\''+esc(p.name)+'\')">Reset to default</button>'):'')+'</div></div></div>';
  }).join('');
}
async function activatePolicy(name){
  const ta=$('#pol_'+name);if(!ta)return;
  const items=ta.value.split('\n').map(s=>s.trim()).filter(Boolean);
  await postJSON('/api/policies/activate',{name:name,items:items});loadPolicies();
}
async function resetPolicy(name){await postJSON('/api/policies/reset',{name:name});loadPolicies();}
async function loadWeek(day){
  const c=$('#timeline');if(!c)return;const d=await getJSON('/api/timeline?day='+day);
  const w=$('#weekDay');if(w)w.innerHTML=esc((d.weekday||'')+' '+(d.day||''))+(d.half_day?' <span class="pill fail">half day</span>':'');
  let html='';
  const anchors=d.anchors||[];
  if(anchors.length) html+='<h3>Daily</h3>'+anchors.map(a=>'<div class="ev"><div><b>'+esc(a.child)+'</b> '+esc(a.label)+' <span class="mut">'+esc(a.time)+'</span></div></div>').join('');
  const items=d.items||[];
  html+='<h3>On this day</h3>';
  if(!items.length) html+='<p class="mut">nothing extracted for this day</p>';
  else html+=items.map(e=>'<div class="ev"><div><b>'+esc(e.title)+'</b>'+(e.kind&&e.kind!=='event'?' <span class="kind">'+esc(e.kind)+'</span>':'')+'<br><span class="mut">'+esc(when(e))+(e.location?(' · '+esc(e.location)):'')+'</span></div></div>').join('');
  c.innerHTML=html;
}
async function loadProfile(){
  const ta=$('#profileJSON');if(!ta)return;const p=await getJSON('/api/profile');
  ta.value=JSON.stringify(p&&p.children?p:{children:[]},null,2);
}
async function saveProfile(){
  const ta=$('#profileJSON');const m=$('#profileMsg');if(!ta)return;
  let body;try{body=JSON.parse(ta.value);}catch(e){if(m)m.textContent='invalid JSON';return;}
  const r=await postJSON('/api/profile/save',body);
  if(m)m.textContent=r.ok?'saved':'save failed';loadWeek('today');
}
async function loadFlywheel(){
  const c=$('#flywheel');if(!c)return;const d=await getJSON('/api/flywheel');
  if(d.accepts===undefined){c.textContent='';return;}
  const total=(d.accepts||0)+(d.rejects||0);
  c.innerHTML='<b>Data flywheel</b> — operator feedback: '+(d.accepts||0)+' accepted · '+(d.rejects||0)+' rejected'+(total?(' · accept rate '+Math.round((d.accept_rate||0)*100)+'%'):'')+' <span class="mut">(real-use ground truth feeding the eval set)</span>';
}
async function loadEval(){
  loadFlywheel();
  const c=$('#evalHistory');if(!c)return;const d=await getJSON('/api/eval');const h=d.history||[];
  if(!h.length){c.innerHTML='<p class="mut">no eval scores yet — click Run eval now (needs the model path up), or run cmd/livecheck</p>';return;}
  c.innerHTML='<table><thead><tr><th>label</th><th>F1</th><th>when</th></tr></thead><tbody>'+
    h.map(e=>'<tr><td>'+esc(e.label)+'</td><td><span class="pill '+(e.f1>=0.87?'pass':'fail')+'">'+e.f1.toFixed(2)+'</span></td><td class="mut">'+esc(e.at||'')+'</td></tr>').join('')+'</tbody></table>';
}
async function runEval(){
  const m=$('#evalMode');if(m)m.textContent='running…';
  const r=await (await fetch('/api/eval/run',{method:'POST'})).json();
  if(m)m.textContent=r.mode||'';loadEval();
}
async function testPrompt(name){
  const ta=$('#pt_'+name);if(!ta)return;const out=$('#pttest_'+name);if(out)out.textContent='testing…';
  const r=await (await fetch('/api/prompts/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:name,text:ta.value})})).json();
  if(!out)return;
  if(r.note){out.innerHTML='<span class="mut">'+esc(r.note)+'</span>';return;}
  out.innerHTML='candidate F1 <b>'+(r.f1||0).toFixed(2)+'</b> vs baseline '+(r.baseline||0).toFixed(2)+
    ' · ADD '+(r.asr_pass?'holds':'BROKEN')+' · gate <span class="pill '+(r.gate_ok?'pass':'fail')+'">'+(r.gate_ok?'PASS':'FAIL')+'</span> <span class="mut">('+esc(r.mode||'')+')</span>';
}

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
async function rejectItem(file,index,btn){btn.disabled=true;const r=await postJSON('/api/reject',{file,index});btn.textContent=r.ok?'rejected ✓':'err';}
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
  const btn='<button class="ghost" onclick="accept(\''+esc(i.file)+'\','+i.index+',this)">Add this</button> '+
    '<button class="ghost" onclick="rejectItem(\''+esc(i.file)+'\','+i.index+',this)">Not real</button>';
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
      (s.doc_type?' <span class="kind'+(s.doc_type==='reference'?' warn':'')+'">'+esc(s.doc_type)+'</span>':'')+
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
// Mini calendar on the Chat tab: a compact month glance sharing monthItems. It is
// the trusted PROJECTION of the extracted calendar (untrusted email → extract +
// sanitize → .ics → here); it renders event titles already sanitized upstream.
let miniMonth=null;
async function loadMiniCal(){
  const c=$('#miniCal');if(!c)return;
  const d=await getJSON('/api/items');monthItems={};
  (d.items||[]).forEach(i=>{const k=(i.due||i.start||'').slice(0,10);if(k)(monthItems[k]=monthItems[k]||[]).push(i);});
  if(!miniMonth)miniMonth=new Date();
  renderMiniCal();
}
function miniCell(y,m,d,other){
  const ds=dateStr(y,m,d);const n=(monthItems[ds]||[]).length;
  const dot=n?'<span class="mdot">'+(n>1?n:'●')+'</span>':'';
  return '<div class="mday'+(other?' other':'')+(ds===todayStr()?' today':'')+(n?' has':'')+'" onclick="jumpDay(\''+ds+'\')">'+d+dot+'</div>';
}
function renderMiniCal(){
  const c=$('#miniCal');if(!c)return;
  const y=miniMonth.getFullYear(),m=miniMonth.getMonth();
  const name=new Date(y,m,1).toLocaleString('en',{month:'short',year:'numeric'});
  let h='<div class="mhead"><button class="ghost" onclick="miniShift(-1)">‹</button><b>'+name+'</b><button class="ghost" onclick="miniShift(1)">›</button></div><div class="grid mini">';
  ['M','T','W','T','F','S','S'].forEach(x=>h+='<div class="dow">'+x+'</div>');
  const sd=(new Date(y,m,1).getDay()+6)%7,dd=new Date(y,m+1,0).getDate(),pv=new Date(y,m,0).getDate();
  for(let i=0;i<sd;i++)h+=miniCell(y,m-1,pv-sd+1+i,true);
  for(let d=1;d<=dd;d++)h+=miniCell(y,m,d,false);
  const tr=(7-(sd+dd)%7)%7;for(let i=1;i<=tr;i++)h+=miniCell(y,m+1,i,true);
  h+='</div>';c.innerHTML=h;
}
function miniShift(n){miniMonth=new Date(miniMonth.getFullYear(),miniMonth.getMonth()+n,1);renderMiniCal();}
function jumpDay(ds){const p=ds.split('-');curMonth=new Date(+p[0],+p[1]-1,1);document.querySelector('nav button[data-tab="calendar"]').click();setView('month');}

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
