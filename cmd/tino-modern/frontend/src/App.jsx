import React, { useEffect, useMemo, useState } from 'react'
import {
  Activity, Archive, ArrowDown, ArrowUp, Bot, Check, ChevronRight, CircleHelp,
  CalendarRange, Download, Eye, FileText, Receipt, Share2, WalletCards,
  Database, FileLock2, FileSpreadsheet, FolderOpen, GitBranch, LoaderCircle,
  LockKeyhole, MessageCircle, MoreHorizontal, Plus, RefreshCw, Search, Send,
  Settings2, ShieldCheck, Sparkles, Trash2, Upload, Users, Workflow, X
} from 'lucide-react'
import {
  ChooseCSV, ChooseWorkspace, Connect, ExportAudit, GetCapabilities, GetFlow,
  GetMessages, GetStatus, ListChats, ResetSession, RunBatch, SaveCapabilities,
  SaveFlow, SendMessage, StartFlow, TestFlow
  , ExportOperationalPDF, FinanceDashboard, GetReceiptPreview, ListAdvances,
  ListReceipts, ListStatementWeeks, ShareOperationalPDF
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { normalizeCapabilities } from './capabilities'

const nav = [
  ['conversations', 'Conversas', MessageCircle],
  ['contacts', 'Base e notificações', Users],
  ['operations', 'Operação financeira', WalletCards],
  ['flow', 'Flow Builder', GitBranch],
  ['capabilities', 'Central de recursos', Settings2],
  ['activity', 'Atividade', Activity],
]

const emptyStatus = { authenticated: false, sessionSaved: false, text: 'Carregando...', profile: 'default', version: '' }

export default function App() {
  const [page, setPage] = useState('conversations')
  const [status, setStatus] = useState(emptyStatus)
  const [qr, setQR] = useState('')
  const [busy, setBusy] = useState('')
  const [toast, setToast] = useState(null)
  const [activities, setActivities] = useState([])

  const notify = (message, tone = 'success') => {
    setToast({ message, tone }); setTimeout(() => setToast(null), 4200)
  }
  const run = async (name, action, success) => {
    setBusy(name)
    try { const value = await action(); if (success) notify(success); return value }
    catch (e) { notify(String(e), 'error'); throw e }
    finally { setBusy('') }
  }
  const refreshStatus = async () => setStatus(await GetStatus())

  useEffect(() => {
    refreshStatus()
    const offStatus = EventsOn('session:status', setStatus)
    const offQR = EventsOn('pairing:qr', setQR)
    const offActivity = EventsOn('activity', item => setActivities(old => [item, ...old].slice(0, 150)))
    return () => { offStatus(); offQR(); offActivity() }
  }, [])

  const connect = () => run('connect', Connect).catch(() => {})
  const reset = async () => {
    if (!window.confirm('Remover a sessão local e gerar um novo QR Code? O histórico será preservado.')) return
    await run('reset', ResetSession, 'Sessão local removida')
    setQR(''); connect()
  }

  const active = nav.find(x => x[0] === page)
  return <div className="shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark"><MessageCircle/><ShieldCheck/></div><div><strong>TINO</strong><span>Comunicação inteligente</span></div></div>
      <nav>{nav.map(([id, label, Icon]) => <button key={id} className={page === id ? 'active' : ''} onClick={() => setPage(id)}><Icon/><span>{label}</span>{page === id && <ChevronRight className="nav-chevron"/>}</button>)}</nav>
      <div className="sidebar-foot"><div className="privacy"><FileLock2/><div><b>Dados protegidos</b><span>Armazenados neste computador</span></div></div><span className="version">Tino {status.version}</span></div>
    </aside>
    <main>
      <header className="topbar"><div><span className="eyebrow">CENTRAL DE COMUNICAÇÃO</span><h1>{active?.[1]}</h1></div><div className="top-actions"><button className="icon-button" title="Ajuda"><CircleHelp/></button><StatusPill status={status}/></div></header>
      <div className="content">
        {page === 'conversations' && <Conversations status={status} qr={qr} busy={busy} connect={connect} reset={reset} refreshStatus={refreshStatus} notify={notify}/>} 
        {page === 'contacts' && <Contacts busy={busy} run={run} notify={notify}/>} 
        {page === 'operations' && <OperationsPage notify={notify}/>} 
        {page === 'flow' && <FlowPage busy={busy} run={run} notify={notify}/>} 
        {page === 'capabilities' && <Capabilities notify={notify}/>} 
        {page === 'activity' && <ActivityPage items={activities}/>} 
      </div>
    </main>
    {toast && <div className={`toast ${toast.tone}`}><span>{toast.tone === 'error' ? <X/> : <Check/>}</span>{toast.message}</div>}
  </div>
}

function StatusPill({ status }) {
  return <div className={`status-pill ${status.authenticated ? 'online' : status.sessionSaved ? 'warning' : ''}`}><span className="dot"/><div><b>{status.text}</b><small>Perfil {status.profile}</small></div></div>
}

function Conversations({ status, qr, busy, connect, reset, refreshStatus, notify }) {
  if (!status.authenticated) return <div className="pairing-grid">
    <section className="hero-card"><div className="hero-icon"><MessageCircle/></div><span className="section-kicker">PRIMEIRO ACESSO</span><h2>Conecte seu WhatsApp</h2><p>Vincule sua conta com segurança. Depois da autenticação, suas conversas aparecem neste mesmo espaço.</p>
      <div className="steps">{['Clique em conectar', 'Abra o WhatsApp no celular', 'Acesse Dispositivos conectados', 'Escaneie o código ao lado'].map((x,i)=><div className="step" key={x}><span>{i+1}</span><p>{x}</p></div>)}</div>
      <button className="primary large" disabled={!!busy} onClick={connect}>{busy === 'connect' ? <LoaderCircle className="spin"/> : <Sparkles/>}Conectar e exibir QR Code</button>
      <div className="button-row"><button className="secondary" onClick={reset}><RefreshCw/>Gerar novo QR</button><button className="ghost" onClick={refreshStatus}>Atualizar estado</button></div>
    </section>
    <section className="qr-card"><div className="card-head"><div><span className="section-kicker">PAREAMENTO SEGURO</span><h3>QR Code de conexão</h3></div><span className="secure-badge"><LockKeyhole/>Local</span></div>
      <div className={`qr-stage ${qr ? 'ready' : ''}`}>{qr ? <img src={qr} alt="QR Code de pareamento"/> : <div className="qr-empty"><div className="scan-frame"><MessageCircle/></div><b>Seu código aparecerá aqui</b><span>Ele será renovado automaticamente quando necessário.</span></div>}</div>
      <div className="info-strip"><ShieldCheck/><span>O Tino não solicita sua senha e mantém a sessão somente neste computador.</span></div>
    </section>
  </div>
  return <ChatWorkspace notify={notify}/>
}

function ChatWorkspace({ notify }) {
  const [search, setSearch] = useState(''), [chats, setChats] = useState([]), [selected, setSelected] = useState(null), [messages, setMessages] = useState([]), [text, setText] = useState(''), [sending, setSending] = useState(false)
  const load = async () => setChats(await ListChats(search))
  useEffect(() => { load(); const off=EventsOn('chats:changed', load); return off }, [search])
  const open = async c => { setSelected(c); setMessages(await GetMessages(c.JID)) }
  const send = async () => { if(!selected || !text.trim()) return; setSending(true); try { await SendMessage(selected.JID,text); setText(''); setMessages(await GetMessages(selected.JID)) } catch(e){ notify(String(e),'error') } finally { setSending(false) } }
  return <div className="chat-shell"><section className="chat-list"><div className="search"><Search/><input value={search} onChange={e=>setSearch(e.target.value)} placeholder="Buscar conversa..."/></div><div className="conversation-list">{chats.map(c=><button key={c.JID} className={selected?.JID===c.JID?'selected':''} onClick={()=>open(c)}><span className="avatar">{c.Name.slice(0,1).toUpperCase()}</span><span className="conversation-copy"><b>{c.Name}</b><small>{c.LastMessage}</small></span>{c.Unread>0&&<span className="unread">{c.Unread}</span>}</button>)}{!chats.length&&<Empty icon={MessageCircle} title="Nenhuma conversa ainda" text="As conversas sincronizadas aparecerão aqui."/>}</div></section>
    <section className="messages">{selected ? <><div className="message-head"><span className="avatar">{selected.Name.slice(0,1)}</span><div><b>{selected.Name}</b><small>Histórico armazenado localmente</small></div><MoreHorizontal/></div><div className="message-scroll">{messages.map(m=><div key={m.ID} className={`bubble ${m.FromMe?'mine':''}`}><p>{m.Text}</p><time>{new Date(m.Timestamp).toLocaleString('pt-BR',{hour:'2-digit',minute:'2-digit'})}</time></div>)}</div><div className="composer"><textarea value={text} onChange={e=>setText(e.target.value)} placeholder="Escreva uma mensagem..."/><button className="primary" onClick={send} disabled={sending}><Send/></button></div></> : <Empty icon={MessageCircle} title="Selecione uma conversa" text="Abra um atendimento para visualizar o histórico e responder."/>}</section></div>
}

function Contacts({ busy, run, notify }) {
  const [csv, setCSV] = useState(null), [message,setMessage]=useState(''), [consent,setConsent]=useState(false)
  const choose=async()=>{try{setCSV(await ChooseCSV())}catch(e){notify(String(e),'error')}}
  const send=()=>run('batch',()=>RunBatch({Path:csv?.Path||'',Message:message,ConfirmConsent:consent}),'Processamento concluído').catch(()=>{})
  const exportData=async()=>{try{const dir=await run('audit',ExportAudit);notify(`Exportação salva em ${dir}`)}catch{}}
  return <div className="stack"><div className="page-intro"><div><span className="section-kicker">BASE DE CLIENTES</span><h2>Dados organizados, comunicação responsável</h2><p>Importe listas consentidas, revise o alcance e mantenha uma fotografia auditável da base.</p></div></div>
    <div className="two-cards"><section className="panel"><div className="panel-icon teal"><Upload/></div><h3>Notificações consentidas</h3><p>Selecione seu CSV. O Tino reconhece listas simples com a coluna telefone.</p><button className="dropzone" onClick={choose}><FileSpreadsheet/><span>{csv?<><b>{csv.FileName}</b><small>{csv.Total} contatos encontrados</small></>:<><b>Selecionar arquivo CSV</b><small>Clique para escolher no computador</small></>}</span></button>
      {csv&&<div className="stats-row"><div><b>{csv.Total}</b><span>Contatos</span></div><div><b>{csv.HasConsent?csv.Consented:'—'}</b><span>Consentidos no arquivo</span></div><div><b>{csv.PhoneField}</b><span>Coluna detectada</span></div></div>}
      <label className="field"><span>Mensagem da campanha</span><textarea value={message} onChange={e=>setMessage(e.target.value)} placeholder="Olá! Escreva aqui sua comunicação..."/></label><label className="check"><input type="checkbox" checked={consent} onChange={e=>setConsent(e.target.checked)}/><span>Confirmo que estes contatos autorizaram esta comunicação.</span></label><button className="primary wide" disabled={!csv||busy==='batch'} onClick={send}><Send/>Revisar e iniciar</button></section>
      <section className="panel audit-panel"><div className="panel-icon navy"><Database/></div><h3>Auditoria da base</h3><p>Gere arquivos JSON e CSV com os contatos sincronizados e participantes dos grupos.</p><div className="audit-visual"><Archive/><div><b>Exportação local</b><span>Nenhum dado é enviado para serviços externos.</span></div></div><button className="secondary wide" disabled={busy==='audit'} onClick={exportData}><Archive/>Exportar contatos e grupos</button></section></div></div>
}

function OperationsPage({ notify }) {
  const today=()=>new Date().toLocaleDateString('sv-SE'), month=()=>today().slice(0,7)
  const [tab,setTab]=useState('dashboard'),[from,setFrom]=useState(today()),[to,setTo]=useState(today()),[employee,setEmployee]=useState(''),[target,setTarget]=useState('')
  const [finance,setFinance]=useState(null),[receipts,setReceipts]=useState([]),[advances,setAdvances]=useState([]),[weeks,setWeeks]=useState([]),[preview,setPreview]=useState(null),[loading,setLoading]=useState(false)
  const money=cents=>(Number(cents||0)/100).toLocaleString('pt-BR',{style:'currency',currency:'BRL'})
  const guard=async action=>{setLoading(true);try{return await action()}catch(e){notify(String(e),'error')}finally{setLoading(false)}}
  const loadDashboard=()=>guard(async()=>setFinance(await FinanceDashboard(from,to)))
  const loadReceipts=()=>guard(async()=>setReceipts(await ListReceipts(from,to)||[]))
  const loadAdvances=()=>guard(async()=>setAdvances(await ListAdvances(from.slice(0,7),employee)||[]))
  const loadWeeks=()=>guard(async()=>setWeeks(await ListStatementWeeks()||[]))
  useEffect(()=>{loadDashboard()},[])
  const preset=kind=>{const end=new Date(),start=new Date(end);if(kind==='week')start.setDate(end.getDate()-6);if(kind==='month')start.setDate(1);const f=start.toLocaleDateString('sv-SE'),t=end.toLocaleDateString('sv-SE');setFrom(f);setTo(t)}
  const exportPDF=kind=>guard(async()=>{const path=await ExportOperationalPDF(kind,kind==='ADVANCES'?from.slice(0,7):from,to,employee);if(path)notify(`PDF salvo em ${path}`)})
  const share=kind=>{if(!target)return notify('Informe o telefone com DDI ou JID do grupo','error');guard(async()=>{await ShareOperationalPDF({Kind:kind,Target:target,From:kind==='ADVANCES'?from.slice(0,7):from,To:to,Employee:employee});notify('PDF compartilhado pelo WhatsApp')})}
  const openReceipt=id=>guard(async()=>setPreview(await GetReceiptPreview(id)))
  return <div className="stack operations"><div className="page-intro"><div><span className="section-kicker">GESTÃO OPERACIONAL</span><h2>Financeiro, documentos e auditoria</h2><p>Consulte os dados registrados pelo WhatsApp, gere PDFs e compartilhe relatórios sem sair do Tino.</p></div></div>
    <div className="segmented"><button className={tab==='dashboard'?'active':''} onClick={()=>setTab('dashboard')}>Dashboard</button><button className={tab==='pdfs'?'active':''} onClick={()=>setTab('pdfs')}>PDFs</button><button className={tab==='receipts'?'active':''} onClick={()=>{setTab('receipts');loadReceipts()}}>Comprovantes</button><button className={tab==='statements'?'active':''} onClick={()=>{setTab('statements');loadWeeks()}}>Extratos</button><button className={tab==='advances'?'active':''} onClick={()=>{setTab('advances');loadAdvances()}}>Vales</button></div>
    {(tab==='dashboard'||tab==='receipts'||tab==='pdfs')&&<section className="panel filter-bar"><label>Data inicial<input type="date" value={from} onChange={e=>setFrom(e.target.value)}/></label><label>Data final<input type="date" value={to} onChange={e=>setTo(e.target.value)}/></label><div className="preset-buttons"><button className="secondary" onClick={()=>{setFrom(today());setTo(today())}}>Hoje</button><button className="secondary" onClick={()=>preset('week')}>7 dias</button><button className="secondary" onClick={()=>preset('month')}>Mês</button></div>{tab==='dashboard'&&<button className="primary" onClick={loadDashboard}><RefreshCw/>Atualizar</button>}{tab==='receipts'&&<button className="primary" onClick={loadReceipts}><Search/>Consultar</button>}</section>}
    {loading&&<div className="loading-line"><LoaderCircle className="spin"/>Atualizando dados...</div>}
    {tab==='dashboard'&&finance&&<><div className="metric-grid"><Metric title="Saldo consolidado" value={money(finance.BalanceCents)} tone="teal"/><Metric title="Entradas" value={money(finance.EntryCents)}/><Metric title="Saídas" value={money(finance.ExitCents)} tone="danger"/><Metric title="Saldo de abertura" value={money(finance.OpeningCents)}/></div><div className="two-cards"><section className="panel"><div className="section-title"><div><h3>Recebimentos por meio</h3><span>{finance.Label}</span></div></div><div className="channel-bars">{[['Dinheiro',finance.CashCents],['PIX',finance.PixCents],['Cartão',finance.CardCents]].map(([name,value])=><div key={name}><span>{name}</span><b>{money(value)}</b><i style={{width:`${finance.EntryCents?Math.max(4,value/finance.EntryCents*100):0}%`}}/></div>)}</div></section><section className="panel operational-summary"><h3>Volume operacional</h3><div><b>{finance.Days}</b><span>dias com caixa</span></div><div><b>{finance.Movements}</b><span>movimentos</span></div><div><b>{finance.Receipts}</b><span>comprovantes</span></div></section></div></>}
    {tab==='pdfs'&&<><section className="panel share-panel"><div><span className="section-kicker">DESTINO WHATSAPP</span><h3>Compartilhar documento</h3><p>Use telefone com DDI ou JID de grupo.</p></div><input value={target} onChange={e=>setTarget(e.target.value)} placeholder="5573999999999 ou 123@g.us"/></section><div className="document-grid"><DocumentCard title="Situação dos quartos" text="Resumo visual atualizado dos 42 quartos." onDownload={()=>exportPDF('ROOMS')} onShare={()=>share('ROOMS')}/><DocumentCard title="Caixa por período" text={`${from} a ${to}, com movimentos e totais.`} onDownload={()=>exportPDF('CASH')} onShare={()=>share('CASH')}/><DocumentCard title="Vales do mês" text="Relatório mensal completo ou filtrado por funcionário." onDownload={()=>exportPDF('ADVANCES')} onShare={()=>share('ADVANCES')}/></div></>}
    {tab==='receipts'&&<DataTable headers={['ID','Data','Método','Descrição','Valor','Arquivo']} empty="Nenhum comprovante no período.">{receipts.map(x=><tr key={x.ID}><td>#{x.ID}</td><td>{x.Date}</td><td><span className="tag ready">{x.Method}</span></td><td>{x.Description}</td><td><b>{money(x.Cents)}</b></td><td><button className="secondary compact" disabled={!x.HasFile} onClick={()=>openReceipt(x.ID)}><Eye/>Visualizar</button></td></tr>)}</DataTable>}
    {tab==='statements'&&<DataTable headers={['Semana','Status','Arquivos','Gerado em','Responsável']} empty="Nenhuma semana de extratos registrada.">{weeks.map(x=><tr key={x.ID}><td><b>{x.Label}</b><small>{(x.Files||[]).map(f=>f.Name).join(' • ')}</small></td><td><span className="tag ready">{x.Status}</span></td><td>{x.FileCount}</td><td>{x.LastGeneratedAt||'—'}</td><td>{x.CreatedBy}</td></tr>)}</DataTable>}
    {tab==='advances'&&<><section className="panel filter-bar"><label>Mês<input type="month" value={from.slice(0,7)} onChange={e=>setFrom(e.target.value+'-01')}/></label><label>Funcionário<input value={employee} onChange={e=>setEmployee(e.target.value)} placeholder="Todos"/></label><button className="primary" onClick={loadAdvances}><Search/>Consultar</button><button className="secondary" onClick={()=>exportPDF('ADVANCES')}><Download/>Gerar PDF</button></section><DataTable headers={['Data','Funcionário','Observação','Valor','Caixa']} empty="Nenhum vale encontrado.">{advances.map(x=><tr key={x.ID}><td>{x.Date}</td><td><b>{x.Employee}</b></td><td>{x.Note||'—'}</td><td>{money(x.Cents)}</td><td>{x.DeductCash?'Descontado':'Não descontado'}</td></tr>)}</DataTable></>}
    {preview&&<div className="modal-backdrop" onClick={()=>setPreview(null)}><section className="receipt-modal" onClick={e=>e.stopPropagation()}><button className="modal-close" onClick={()=>setPreview(null)}><X/></button><span className="section-kicker">COMPROVANTE #{preview.receipt.ID}</span><h3>{preview.receipt.Description}</h3><p>{preview.receipt.Date} • {preview.receipt.Method} • {money(preview.receipt.Cents)}</p>{preview.dataURL?(preview.receipt.MediaType?.startsWith('image/')?<img src={preview.dataURL}/>:<iframe src={preview.dataURL} title="Comprovante PDF"/>):<Empty icon={Receipt} title="Original indisponível" text="Este lançamento não possui arquivo armazenado."/>}</section></div>}
  </div>
}

function Metric({title,value,tone=''}){return <section className={`metric-card ${tone}`}><span>{title}</span><b>{value}</b></section>}
function DocumentCard({title,text,onDownload,onShare}){return <section className="panel document-card"><div className="panel-icon"><FileText/></div><h3>{title}</h3><p>{text}</p><div className="button-row"><button className="secondary" onClick={onDownload}><Download/>Salvar PDF</button><button className="primary" onClick={onShare}><Share2/>WhatsApp</button></div></section>}
function DataTable({headers,empty,children}){const rows=React.Children.count(children);return <section className="panel table-panel">{rows?<div className="data-table-wrap"><table className="data-table"><thead><tr>{headers.map(x=><th key={x}>{x}</th>)}</tr></thead><tbody>{children}</tbody></table></div>:<Empty icon={Database} title={empty} text="Ajuste os filtros ou registre dados pelo WhatsApp."/>}</section>}

function FlowPage({ busy, run, notify }) {
  const [def,setDef]=useState({Rules:[],DefaultReply:''}),[test,setTest]=useState(''),[result,setResult]=useState('')
  useEffect(()=>{GetFlow().then(x=>setDef({Rules:x.Rules||[],DefaultReply:x.DefaultReply||''}))},[])
  const update=(i,k,v)=>setDef(d=>({...d,Rules:d.Rules.map((r,n)=>n===i?{...r,[k]:v}:r)}))
  const add=()=>setDef(d=>({...d,Rules:[...d.Rules,{Name:'Nova regra',Contains:'',Reply:'',CaseSensitive:false}]}))
  const remove=i=>setDef(d=>({...d,Rules:d.Rules.filter((_,n)=>n!==i)}))
  const move=(i,delta)=>setDef(d=>{const a=[...d.Rules],j=i+delta;if(j<0||j>=a.length)return d;[a[i],a[j]]=[a[j],a[i]];return {...d,Rules:a}})
  const simulate=async()=>setResult(await TestFlow(test,def))
  const save=()=>run('flow-save',()=>SaveFlow(def),'Fluxo salvo').catch(()=>{})
  const start=()=>run('flow-start',()=>StartFlow(def),'Atendimento automático ativado').catch(()=>{})
  return <div className="stack"><div className="page-intro"><div><span className="section-kicker">AUTOMAÇÃO VISUAL</span><h2>Desenhe o atendimento como uma conversa</h2><p>A primeira regra correspondente responde ao cliente. Reordene os caminhos conforme a prioridade.</p></div><div className="button-row"><button className="secondary" onClick={save}>Salvar alterações</button><button className="primary" onClick={start}><Bot/>Ativar atendimento</button></div></div>
    <div className="flow-layout"><section className="rules"><div className="section-title"><div><h3>Caminhos de resposta</h3><span>{def.Rules.length} regras configuradas</span></div><button className="secondary compact" onClick={add}><Plus/>Nova regra</button></div>{def.Rules.map((r,i)=><article className="rule-card" key={i}><div className="rule-order">{String(i+1).padStart(2,'0')}</div><div className="rule-fields"><input value={r.Name} onChange={e=>update(i,'Name',e.target.value)} aria-label="Nome da regra"/><label>Se a mensagem contém<input value={r.Contains} onChange={e=>update(i,'Contains',e.target.value)} placeholder="ex.: orçamento"/></label><label>Responder com<textarea value={r.Reply} onChange={e=>update(i,'Reply',e.target.value)} placeholder="Digite a resposta..."/></label></div><div className="rule-actions"><button onClick={()=>move(i,-1)}><ArrowUp/></button><button onClick={()=>move(i,1)}><ArrowDown/></button><button className="danger" onClick={()=>remove(i)}><Trash2/></button></div></article>)}</section>
      <aside className="flow-side"><section className="panel"><h3>Resposta padrão</h3><p>Usada quando nenhuma regra corresponde.</p><textarea value={def.DefaultReply} onChange={e=>setDef({...def,DefaultReply:e.target.value})}/></section><section className="panel simulator"><span className="section-kicker">SIMULADOR</span><h3>Teste antes de ativar</h3><textarea value={test} onChange={e=>setTest(e.target.value)} placeholder="Digite como se fosse o cliente..."/><button className="secondary wide" onClick={simulate}>Simular resposta</button>{result&&<div className="test-result"><Bot/><p>{result}</p></div>}</section></aside></div></div>
}

function Capabilities({ notify }) {
  const [cfg,setCfg]=useState(null),[tab,setTab]=useState('modules'),[error,setError]=useState(''),[operator,setOperator]=useState('')
  useEffect(()=>{GetCapabilities().then(value=>setCfg(normalizeCapabilities(value))).catch(err=>setError(String(err)))},[])
  if(error)return <section className="panel error-state"><div className="panel-icon"><X/></div><h3>Não foi possível carregar os recursos</h3><p>{error}</p><button className="secondary" onClick={()=>{setError('');GetCapabilities().then(value=>setCfg(normalizeCapabilities(value))).catch(err=>setError(String(err)))}}><RefreshCw/>Tentar novamente</button></section>
  if(!cfg)return <div className="loading"><LoaderCircle className="spin"/></div>
  const save=async next=>{setCfg(next);try{await SaveCapabilities(next);notify('Configuração salva')}catch(e){notify(String(e),'error')}}
  const toggle=id=>{const mod=cfg.modules.find(m=>m.id===id);if(!mod?.available)return;save({...cfg,modules:cfg.modules.map(m=>m.id===id?{...m,enabled:!m.enabled}:m)})}
  const workspace=async()=>{try{const p=await ChooseWorkspace();if(p)setCfg({...cfg,workspaceRoot:p})}catch(e){notify(String(e),'error')}}
  const addOperator=()=>{const phone=operator.replace(/\D/g,'');if(!phone)return notify('Informe o telefone com DDI','error');if(cfg.operators.includes(phone))return notify('Este operador já está cadastrado','error');save({...cfg,operators:[...cfg.operators,phone]});setOperator('')}
  const removeOperator=phone=>save({...cfg,operators:cfg.operators.filter(x=>x!==phone)})
  return <div className="stack"><div className="page-intro"><div><span className="section-kicker">GOVERNANÇA</span><h2>Central de recursos e permissões</h2><p>Controle o que está ativo, quais perfis podem usar cada função e quais arquivos ficam dentro do escopo autorizado.</p></div></div>
    <div className="segmented"><button className={tab==='modules'?'active':''} onClick={()=>setTab('modules')}>Recursos</button><button className={tab==='operators'?'active':''} onClick={()=>setTab('operators')}>Operadores do WhatsApp</button><button className={tab==='roles'?'active':''} onClick={()=>setTab('roles')}>Perfis e privilégios</button><button className={tab==='files'?'active':''} onClick={()=>setTab('files')}>Arquivos</button></div>
    {tab==='modules'&&<div className="module-groups">{['Tino','Plataforma','Assistente Paraíso'].map(cat=><section key={cat}><div className="section-title"><div><h3>{cat}</h3><span>{cat==='Assistente Paraíso'?'Mapeados para integração progressiva':'Disponíveis nesta versão'}</span></div></div><div className="module-grid">{cfg.modules.filter(m=>m.category===cat).map(m=><article className={`module-card ${!m.available?'planned':''}`} key={m.id}><div className="module-top"><div className="panel-icon small"><Workflow/></div><span className={`tag ${m.available?'ready':'planned'}`}>{m.available?'Disponível':'Planejado'}</span></div><h4>{m.name}</h4><p>{m.description}</p><button className={`switch ${m.enabled?'on':''}`} disabled={!m.available} onClick={()=>toggle(m.id)} aria-label={`Ativar ${m.name}`}><span/></button></article>)}</div></section>)}</div>}
    {tab==='roles'&&<div className="role-grid">{cfg.roles.map(role=><section className="panel" key={role.id}><div className="role-title"><span className="avatar"><ShieldCheck/></span><div><h3>{role.name}</h3><p>{role.modules.length} recursos permitidos</p></div></div><div className="permission-list">{cfg.modules.filter(m=>m.available).map(m=><label key={m.id}><input type="checkbox" checked={role.modules.includes(m.id)} onChange={e=>{const roles=cfg.roles.map(r=>r.id!==role.id?r:{...r,modules:e.target.checked?[...r.modules,m.id]:r.modules.filter(x=>x!==m.id)});save({...cfg,roles})}}/><span>{m.name}</span></label>)}</div></section>)}</div>}
    {tab==='operators'&&<section className="panel file-access"><div className="panel-icon teal"><Users/></div><h3>Operadores autorizados</h3><p>Somente estes números, além da conversa da própria conta, podem executar comandos internos de quartos, caixa, relatórios, comprovantes, vales e backup.</p><div className="button-row"><input value={operator} onChange={e=>setOperator(e.target.value)} onKeyDown={e=>e.key==='Enter'&&addOperator()} placeholder="Ex.: 5573999999999"/><button className="primary" onClick={addOperator}><Plus/>Adicionar</button></div><div className="permission-list">{cfg.operators.length?cfg.operators.map(phone=><div className="role-title" key={phone}><span className="avatar"><ShieldCheck/></span><div><h3>+{phone}</h3><p>Operador de comandos internos</p></div><button className="danger" onClick={()=>removeOperator(phone)}><Trash2/></button></div>):<Empty icon={Users} title="Nenhum operador cadastrado" text="Cadastre ao menos um telefone com DDI para administrar pelo WhatsApp."/>}</div></section>}
    {tab==='files'&&<section className="panel file-access"><div className="panel-icon teal"><FolderOpen/></div><h3>Pasta autorizada</h3><p>O módulo de arquivos será limitado a esta raiz. Caminhos fora dela não entram no escopo do assistente.</p><div className="path-box"><FolderOpen/><span>{cfg.workspaceRoot||'Nenhuma pasta autorizada'}</span></div><button className="primary" onClick={workspace}>Escolher pasta</button><div className="info-strip"><ShieldCheck/><span>Esta seleção não concede acesso automático: cada operação futura ainda deverá validar o perfil e registrar auditoria.</span></div></section>}
  </div>
}

function ActivityPage({ items }) { return <div className="stack"><div className="page-intro"><div><span className="section-kicker">OBSERVABILIDADE</span><h2>Atividade do Tino</h2><p>Conexões, sincronizações, exportações e automações em uma linha do tempo legível.</p></div></div><section className="panel timeline">{items.length?items.map((x,i)=><div className={`timeline-item ${x.level}`} key={i}><span className="timeline-dot"/><div><b>{x.title}</b><p>{x.message}</p><time>{new Date(x.at).toLocaleString('pt-BR')}</time></div></div>):<Empty icon={Activity} title="Nenhuma atividade nesta sessão" text="Os eventos operacionais aparecerão aqui."/>}</section></div> }

function Empty({ icon:Icon,title,text }) { return <div className="empty"><Icon/><b>{title}</b><span>{text}</span></div> }
