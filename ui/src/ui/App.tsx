import React, { useEffect, useMemo, useState } from 'react'

type BlockStatus = 'pending' | 'cached' | 'running' | 'success' | 'failed' | 'skipped'

type StatusItem = {
  id: string
  status: BlockStatus
  digest?: string
  hash?: string
  timestamp?: string
  durationMs?: number
  error?: string
}

type Project = {
  name: string
  version: string
  blocks: { id: string; from?: string; from_block?: string }[]
}

// Helper function to get status color and styling
const getStatusDisplay = (status: BlockStatus) => {
  switch (status) {
    case 'pending':
      return { 
        color: 'text-zinc-500 dark:text-zinc-400', 
        bgColor: 'bg-zinc-50 dark:bg-zinc-800/50',
        borderColor: 'border-zinc-200 dark:border-zinc-700',
        label: 'Pending'
      }
    case 'cached':
      return { 
        color: 'text-blue-600 dark:text-blue-400', 
        bgColor: 'bg-blue-50 dark:bg-blue-900/20',
        borderColor: 'border-blue-200 dark:border-blue-800',
        label: 'Cached'
      }
    case 'running':
      return { 
        color: 'text-amber-600 dark:text-amber-400', 
        bgColor: 'bg-amber-50 dark:bg-amber-900/20',
        borderColor: 'border-amber-200 dark:border-amber-800',
        label: 'Running'
      }
    case 'success':
      return { 
        color: 'text-green-600 dark:text-green-400', 
        bgColor: 'bg-green-50 dark:bg-green-900/20',
        borderColor: 'border-green-200 dark:border-green-800',
        label: 'Success'
      }
    case 'failed':
      return { 
        color: 'text-red-600 dark:text-red-400', 
        bgColor: 'bg-red-50 dark:bg-red-900/20',
        borderColor: 'border-red-200 dark:border-red-800',
        label: 'Failed'
      }
    case 'skipped':
      return { 
        color: 'text-zinc-400 dark:text-zinc-500', 
        bgColor: 'bg-zinc-50 dark:bg-zinc-800/30',
        borderColor: 'border-zinc-200 dark:border-zinc-700',
        label: 'Skipped'
      }
    default:
      return { 
        color: 'text-zinc-500 dark:text-zinc-400', 
        bgColor: 'bg-zinc-50 dark:bg-zinc-800/50',
        borderColor: 'border-zinc-200 dark:border-zinc-700',
        label: 'Unknown'
      }
  }
}

export function App() {
  const [project, setProject] = useState<Project | null>(null)
  const [status, setStatus] = useState<StatusItem[]>([])
  const [error, setError] = useState<string | null>(null)
  const [theme, setTheme] = useState<'light'|'dark'>("dark")
  const [showAddModal, setShowAddModal] = useState(false)
  const [showYamlModal, setShowYamlModal] = useState(false)
  const [yamlText, setYamlText] = useState('')
  const [showDockerfileModal, setShowDockerfileModal] = useState(false)
  const [dockerfileText, setDockerfileText] = useState('')
  const [dockerfileMeta, setDockerfileMeta] = useState<{ title?: string; tag?: string; digest?: string } | null>(null)
  const [unsavedIds, setUnsavedIds] = useState<string[]>([])
  const [showUnsavedModal, setShowUnsavedModal] = useState(false)
  const pendingProceedResolve = React.useRef<((ok: boolean)=>void) | null>(null)
  const [newBlockId, setNewBlockId] = useState('')
  const [newFrom, setNewFrom] = useState<string | undefined>(undefined)
  const [newFromBlock, setNewFromBlock] = useState<string | undefined>(undefined)
  const saveRegistry = React.useRef(new Map<string, ()=>Promise<void>>())
  const runRegistry = React.useRef(new Map<string, ()=>Promise<void>>())
  const [runningById, setRunningById] = useState<Record<string, boolean>>({})
  const [isSaving, setIsSaving] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [blockToDelete, setBlockToDelete] = useState<string | null>(null)
  const [dropdownOpen, setDropdownOpen] = useState<Record<string, boolean>>({})
  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark')
      document.documentElement.classList.remove('light')
    } else {
      document.documentElement.classList.add('light')
      document.documentElement.classList.remove('dark')
    }
  }, [theme])

  useEffect(() => {
    fetch('/api/project').then(r => r.json()).then(setProject)
    const load = () => fetch('/api/status').then(r => r.json()).then(setStatus)
    load()
    const t = setInterval(load, 2000)
    return () => clearInterval(t)
  }, [])

  // Save all blocks (used by button and Cmd/Ctrl+S)
  const saveAll = async () => {
    setIsSaving(true)
    try {
      const entries = Array.from(saveRegistry.current.values())
      await Promise.all(entries.map(fn => fn()))
      // refresh project to reflect saved cmds
      const p = await (await fetch('/api/project')).json()
      setProject(p)
      setUnsavedIds([])
    } finally {
      setIsSaving(false)
    }
  }

  // Bind Cmd/Ctrl+S to save notebook
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && (e.key === 's' || e.key === 'S')) {
        e.preventDefault()
        saveAll()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const onDirtyChange = (id: string, dirty: boolean) => {
    setUnsavedIds(prev => {
      const has = prev.includes(id)
      if (dirty && !has) return [...prev, id]
      if (!dirty && has) return prev.filter(x => x !== id)
      return prev
    })
  }

  const ensureSaved = React.useCallback(async (): Promise<boolean> => {
    if (unsavedIds.length === 0) return true
    setShowUnsavedModal(true)
    const decision = await new Promise<boolean>((resolve) => {
      pendingProceedResolve.current = resolve
    })
    if (!decision) return false
    await saveAll()
    return true
  }, [unsavedIds])

  const projName = (project as any)?.name ?? (project as any)?.Name ?? ''
  const projVersion = (project as any)?.version ?? (project as any)?.Version ?? ''
  const rawBlocks: any[] = ((project as any)?.blocks ?? (project as any)?.Blocks ?? [])
  const blocks = rawBlocks.map(b => ({
    ...b,
    _id: (b as any).id ?? (b as any).ID,
  }))

  const exportDockerfile = async (blockId: string) => {
    const ok = await ensureSaved()
    if (!ok) return
    const resp = await fetch('/api/export/dockerfile', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ endBlockId: blockId }) })
    if (!resp.ok) return
    const txt = await resp.text()
    setDockerfileText(txt)
    setDockerfileMeta({ title: `Dockerfile for ${blockId}` })
    setShowDockerfileModal(true)
  }

  const deleteBlock = async (blockId: string) => {
    const resp = await fetch(`/api/block?id=${encodeURIComponent(blockId)}`, { method: 'DELETE' })
    if (resp.ok) {
      const p = await (await fetch('/api/project')).json()
      setProject(p)
      setShowDeleteModal(false)
      setBlockToDelete(null)
    }
  }

  return (
    <div className="min-h-screen p-4 font-sans bg-zinc-50 text-zinc-900 dark:bg-zinc-900 dark:text-zinc-100">
      <header className="flex items-center gap-3 rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/70 dark:bg-zinc-900/60 backdrop-blur px-4 py-2 shadow-sm">
        <h1 className="text-xl font-semibold tracking-tight">Dockstep</h1>
        <div className="text-sm text-zinc-600 dark:text-zinc-400">{project ? `${projName} · v${projVersion}` : ''}</div>
        <div className="flex-1" />
        {/** theme toggle moved to sidebar **/}
        <button 
          className="ml-2 inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40 disabled:opacity-50 disabled:cursor-not-allowed" 
          onClick={saveAll}
          disabled={isSaving}
        >
          {isSaving ? (
            <>
              <svg className="animate-spin w-4 h-4" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
              </svg>
              Saving...
            </>
          ) : (
            <>Save Notebook <span className="text-xs opacity-60">⌘S</span></>
          )}
        </button>
        <button className="ml-2 inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={async()=>{
          const txt = await fetch('/api/config').then(r=>r.text())
          setYamlText(txt)
          setShowYamlModal(true)
        }}>Edit YAML</button>
      </header>
      {error && <div className="text-red-600 mt-2">{error}</div>}
      <main className="grid grid-cols-[280px_1fr] gap-4 mt-4">
        <aside className="border-r border-zinc-200/70 dark:border-zinc-800/80 pr-3">
          <h3 className="mt-0 text-sm font-medium text-zinc-600 dark:text-zinc-400">Blocks</h3>
          <ul className="list-none p-0 m-0">
            {blocks.map(b => {
              const s = status.find(x => x.id === (b as any)._id)
              const isRunning = runningById[(b as any)._id]
              const currentStatus = s?.status ?? 'pending'
              const statusDisplay = getStatusDisplay(currentStatus)
              return (
                <li key={(b as any)._id} className="flex items-center justify-between py-1.5">
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    <button className="text-sm text-left text-zinc-800 dark:text-zinc-200 hover:underline decoration-zinc-300/70 truncate" onClick={()=>{}}>{(b as any)._id}</button>
                    <div className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium border ${statusDisplay.color} ${statusDisplay.bgColor} ${statusDisplay.borderColor}`}>
                      <span className="hidden sm:inline">{statusDisplay.label}</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      title="Run"
                      className={`w-7 h-7 grid place-items-center rounded-md text-white text-[11px] shadow-sm hover:shadow-md transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 active:translate-y-px ${isRunning ? 'bg-blue-400/90 cursor-wait' : 'bg-blue-600 hover:bg-blue-500 dark:bg-blue-500/60 dark:hover:bg-blue-400/90'}`}
                      onClick={async()=>{
                        const id = (b as any)._id
                        const run = runRegistry.current.get(id)
                        if (run) await run()
                      }}
                      disabled={!!isRunning}
                    >
                      {isRunning ? (
                        <svg className="animate-spin w-4 h-4" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
                        </svg>
                      ) : (
                        <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                          <path d="M8 5v14l11-7z"/>
                        </svg>
                      )}
                    </button>
                  </div>
                </li>
              )
            })}
          </ul>
          <div className="mt-3">
            <button className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={()=>{
              setNewBlockId('')
              setNewFrom(undefined)
              setNewFromBlock(undefined)
              setShowAddModal(true)
            }}>+ Add Block</button>
          </div>
          <div className="fixed left-2 bottom-2 z-50">
            <button
              aria-label="Toggle theme"
              className="inline-flex items-center justify-center w-9 h-9 rounded-full text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40"
              onClick={()=>setTheme(t=>t==='dark'?'light':'dark')}
            >
              {theme==='dark' ? (
                // sun icon (switch to light)
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z" />
                </svg>
              ) : (
                // moon icon (switch to dark)
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z" />
                </svg>
              )}
            </button>
          </div>
        </aside>
        <section>
          <div className="flex flex-col gap-3">
            {blocks.map(b => (
              <BlockCell
                key={(b as any)._id}
                block={b}
                blocksSuggest={blocks.map(x => (x as any)._id)}
                onRegisterSave={(id, fn)=>{ saveRegistry.current.set(id, fn) }}
                onRegisterRun={(id, fn)=>{ runRegistry.current.set(id, fn) }}
                onDirtyChange={onDirtyChange}
                ensureSaved={ensureSaved}
                onExportDockerfile={exportDockerfile}
                runningGlobal={!!runningById[(b as any)._id]}
                setRunningGlobal={(id, isRunning)=> setRunningById(prev => ({ ...prev, [id]: isRunning }))}
                onShowDockerfile={(payload)=>{ setDockerfileText(payload.dockerfile); setDockerfileMeta({ title: payload.tag || payload.digest || 'Dockerfile', tag: payload.tag, digest: payload.digest }); setShowDockerfileModal(true) }}
                isSaving={isSaving}
                dropdownOpen={dropdownOpen}
                setDropdownOpen={setDropdownOpen}
                onDeleteBlock={(id) => { setBlockToDelete(id); setShowDeleteModal(true) }}
                status={status}
              />
            ))}
          </div>
        </section>
      </main>
      {showAddModal && (
        <div className="fixed inset-0 z-50">
          <div className="absolute inset-0 bg-black/30" onClick={()=>setShowAddModal(false)} />
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <div className="w-full max-w-md rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl">
              <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80">
                <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Add Block</div>
              </div>
              <div className="p-4 space-y-3">
                <div>
                  <label className="block text-xs text-zinc-600 dark:text-zinc-400 mb-1">Name</label>
                  <input value={newBlockId} onChange={e=>setNewBlockId(e.target.value)} placeholder="e.g. build" className="w-full rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500/40" />
                </div>
                <div>
                  <label className="block text-xs text-zinc-600 dark:text-zinc-400 mb-1">Base</label>
                  <BasePicker id="__new__" from={newFrom} fromBlock={newFromBlock} onChange={(f, fb)=>{ setNewFrom(f); setNewFromBlock(fb) }} blocksSuggest={blocks.map(b => (b as any)._id)} />
                </div>
              </div>
              <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
                <button className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={()=>setShowAddModal(false)}>Cancel</button>
                <button
                  className="px-3.5 py-1.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-500 dark:bg-blue-500/60 dark:hover:bg-blue-400/90 text-white shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 active:translate-y-px"
                  disabled={!newBlockId.trim()}
                  onClick={async()=>{
                    const body: any = { id: newBlockId.trim() }
                    if (newFrom) body.from = newFrom
                    if (newFromBlock) body.from_block = newFromBlock
                    const resp = await fetch('/api/block',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
                    if (resp.ok) {
                      const p = await (await fetch('/api/project')).json()
                      setProject(p)
                      setShowAddModal(false)
                    }
                  }}
                >Create</button>
              </div>
            </div>
          </div>
        </div>
      )}
      {showYamlModal && (
        <div className="fixed inset-0 z-50">
          <div className="absolute inset-0 bg-black/30" onClick={()=>setShowYamlModal(false)} />
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <div className="w-full max-w-3xl h-[70vh] overflow-auto rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl flex flex-col">
              <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80 flex items-center justify-between">
                <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Edit dockstep.yaml</div>
                <div className="text-xs text-zinc-500">Saving will reload the UI</div>
              </div>
              <div className="p-3 flex-1">
                <textarea spellCheck={false} className="w-full h-full border border-zinc-200 dark:border-zinc-800 rounded-md bg-white dark:bg-zinc-900 p-2 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500/40" value={yamlText} onChange={e=>setYamlText(e.target.value)} />
              </div>
              <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
                <button className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={()=>setShowYamlModal(false)}>Cancel</button>
                <button className="px-3.5 py-1.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-500 dark:bg-blue-500/60 dark:hover:bg-blue-400/90 text-white shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 active:translate-y-px" onClick={async()=>{
                  const resp = await fetch('/api/config',{method:'PUT',body:yamlText,headers:{'Content-Type':'text/yaml'}})
                  if (resp.ok || resp.status===204) {
                    location.reload()
                  }
                }}>Save</button>
              </div>
            </div>
          </div>
        </div>
      )}
      {showUnsavedModal && (
        <div className="fixed inset-0 z-50">
          <div className="absolute inset-0 bg-black/30" onClick={()=>{
            setShowUnsavedModal(false)
            if (pendingProceedResolve.current) pendingProceedResolve.current(false)
            pendingProceedResolve.current = null
          }} />
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <div className="w-full max-w-md rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl">
              <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80">
                <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Unsaved changes</div>
              </div>
              <div className="p-4 text-sm text-zinc-700 dark:text-zinc-200">
                Some cells have unsaved changes. Save now before running?
              </div>
              <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
                <button className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={()=>{
                  setShowUnsavedModal(false)
                  if (pendingProceedResolve.current) pendingProceedResolve.current(false)
                  pendingProceedResolve.current = null
                }}>Cancel</button>
                <button className="px-3.5 py-1.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-500 dark:bg-blue-500/60 dark:hover:bg-blue-400/90 text-white shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 active:translate-y-px" onClick={()=>{
                  setShowUnsavedModal(false)
                  if (pendingProceedResolve.current) pendingProceedResolve.current(true)
                  pendingProceedResolve.current = null
                }}>Save and Run</button>
              </div>
            </div>
          </div>
        </div>
      )}
      {showDockerfileModal && (
        <div className="fixed inset-0 z-50">
          <div className="absolute inset-0 bg-black/30" onClick={()=>setShowDockerfileModal(false)} />
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <div className="w-full max-w-3xl h-[70vh] overflow-auto rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl flex flex-col">
              <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80 flex items-center justify-between">
                <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">{dockerfileMeta?.title || 'Dockerfile'}</div>
              </div>
              <div className="p-3 flex-1 flex flex-col gap-3">
                {dockerfileMeta?.tag || dockerfileMeta?.digest ? (
                  <div className="rounded-lg border border-zinc-200/70 dark:border-zinc-800/80 bg-white/70 dark:bg-zinc-900/60">
                    <div className="px-3 py-2 border-b border-zinc-200/70 dark:border-zinc-800/80 text-xs font-medium text-zinc-700 dark:text-zinc-200">Commands</div>
                    <div className="p-3 space-y-2">
                      <div className="group relative">
                        <code className="block px-3 py-2 rounded-md bg-zinc-100 dark:bg-zinc-800 overflow-x-auto text-xs">docker run --rm -it {dockerfileMeta.tag || dockerfileMeta.digest} /bin/sh</code>
                        <button className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity text-xs px-2 py-1 rounded-md bg-zinc-900/80 text-white dark:bg-white/10 dark:text-zinc-100" onClick={()=>navigator.clipboard.writeText(`docker run --rm -it ${(dockerfileMeta?.tag||dockerfileMeta?.digest)||''} /bin/sh`)}>Copy</button>
                      </div>
                      <div className="group relative">
                        <code className="block px-3 py-2 rounded-md bg-zinc-100 dark:bg-zinc-800 overflow-x-auto text-xs">docker tag {dockerfileMeta.digest || dockerfileMeta.tag} myrepo/{dockerfileMeta.tag || (dockerfileMeta.digest || '').replace(/:/g,'_')}</code>
                        <button className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity text-xs px-2 py-1 rounded-md bg-zinc-900/80 text-white dark:bg-white/10 dark:text-zinc-100" onClick={()=>navigator.clipboard.writeText(`docker tag ${(dockerfileMeta?.digest||dockerfileMeta?.tag)||''} myrepo/${dockerfileMeta?.tag || ((dockerfileMeta?.digest||'').replace(/:/g,'_'))}`)}>Copy</button>
                      </div>
                      <div className="group relative">
                        <code className="block px-3 py-2 rounded-md bg-zinc-100 dark:bg-zinc-800 overflow-x-auto text-xs">docker push myrepo/{dockerfileMeta.tag || (dockerfileMeta.digest || '').replace(/:/g,'_')}</code>
                        <button className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity text-xs px-2 py-1 rounded-md bg-zinc-900/80 text-white dark:bg-white/10 dark:text-zinc-100" onClick={()=>navigator.clipboard.writeText(`docker push myrepo/${dockerfileMeta?.tag || ((dockerfileMeta?.digest||'').replace(/:/g,'_'))}`)}>Copy</button>
                      </div>
                    </div>
                  </div>
                ) : null}
                <div className="rounded-lg border border-zinc-200/70 dark:border-zinc-800/80 bg-white/70 dark:bg-zinc-900/60 flex-1 min-h-[200px] flex flex-col">
                  <div className="px-3 py-2 border-b border-zinc-200/70 dark:border-zinc-800/80 text-xs font-medium text-zinc-700 dark:text-zinc-200 flex items-center justify-between">
                    <span>Dockerfile</span>
                    <button className="text-xs px-2 py-1 rounded-md bg-zinc-900/80 text-white dark:bg-white/10 dark:text-zinc-100" onClick={()=>navigator.clipboard.writeText(dockerfileText)}>Copy</button>
                  </div>
                  <div className="p-3 flex-1">
                    <textarea spellCheck={false} readOnly className="w-full h-full border border-zinc-200 dark:border-zinc-800 rounded-md bg-white dark:bg-zinc-900 p-2 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500/40" value={dockerfileText} />
                  </div>
                </div>
              </div>
              <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
                <button className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={()=>setShowDockerfileModal(false)}>Close</button>
              </div>
            </div>
          </div>
        </div>
      )}
      {showDeleteModal && (
        <div className="fixed inset-0 z-50">
          <div className="absolute inset-0 bg-black/30" onClick={()=>{
            setShowDeleteModal(false)
            setBlockToDelete(null)
          }} />
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <div className="w-full max-w-md rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl">
              <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80">
                <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Delete Block</div>
              </div>
              <div className="p-4 text-sm text-zinc-700 dark:text-zinc-200">
                Are you sure you want to delete block "{blockToDelete}"? This action cannot be undone.
              </div>
              <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
                <button className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={()=>{
                  setShowDeleteModal(false)
                  setBlockToDelete(null)
                }}>Cancel</button>
                <button className="px-3.5 py-1.5 text-sm font-medium rounded-lg bg-red-600 hover:bg-red-500 dark:bg-red-500/90 dark:hover:bg-red-400/90 text-white shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500/50 active:translate-y-px" onClick={()=>{
                  if (blockToDelete) deleteBlock(blockToDelete)
                }}>Delete</button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function BlockCell({
  block,
  blocksSuggest,
  onRegisterSave,
  onRegisterRun,
  onDirtyChange,
  ensureSaved,
  onExportDockerfile,
  runningGlobal,
  setRunningGlobal,
  onShowDockerfile,
  isSaving,
  dropdownOpen,
  setDropdownOpen,
  onDeleteBlock,
  status,
}: {
  block: any;
  blocksSuggest: string[];
  onRegisterSave: (id: string, fn: ()=>Promise<void>)=>void;
  onRegisterRun: (id: string, fn: ()=>Promise<void>)=>void;
  onDirtyChange: (id: string, dirty: boolean)=>void;
  ensureSaved: ()=>Promise<boolean>;
  onExportDockerfile: (id: string)=>Promise<void>;
  runningGlobal: boolean;
  setRunningGlobal: (id: string, running: boolean)=>void;
  onShowDockerfile: (payload: { dockerfile: string; tag?: string; digest?: string })=>void;
  isSaving: boolean;
  dropdownOpen: Record<string, boolean>;
  setDropdownOpen: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
  onDeleteBlock: (id: string)=>void;
  status: StatusItem[];
}) {
  const id: string = block._id
  const [tab, setTab] = useState<'logs' | 'diff' | 'images' | 'error'>('logs')
  const [logs, setLogs] = useState('')
  const [diff, setDiff] = useState<{ kind: string; path: string }[]>([])
  const [images, setImages] = useState<{ tag: string; digest: string; timestamp: string; dockerfile?: string }[]>([])
  const cmd: string | undefined = (block as any).cmd ?? (block as any).Cmd
  const workdir: string | undefined = (block as any).workdir ?? (block as any).Workdir
  const [from, setFrom] = useState<string | undefined>((block as any).from ?? (block as any).From)
  const [fromBlock, setFromBlock] = useState<string | undefined>((block as any).from_block ?? (block as any).FromBlock)
  const [running, setRunning] = useState(false)
  const currentValueRef = React.useRef(cmd || '')
  
  // Get current block status and error
  const blockStatus = status.find(s => s.id === id)
  const hasError = blockStatus?.status === 'failed' && blockStatus?.error
  const currentStatus = blockStatus?.status ?? 'pending'
  const statusDisplay = getStatusDisplay(currentStatus)
  
  // Auto-switch to error tab when there's an error
  useEffect(() => {
    if (hasError && tab !== 'error') {
      setTab('error')
    }
  }, [hasError, tab])

  useEffect(() => {
    fetch(`/api/logs?id=${encodeURIComponent(id)}`).then(r => (r.ok ? r.text() : Promise.resolve(''))).then(setLogs)
    fetch(`/api/diff?id=${encodeURIComponent(id)}`).then(r => (r.ok ? r.json() : Promise.resolve([]))).then((arr:any)=>{
      setDiff(Array.isArray(arr) ? arr : [])
    })
  }, [id])
  useEffect(()=>{
    if (tab==='images') {
      fetch(`/api/history?id=${encodeURIComponent(id)}`).then(r=>r.ok?r.json():Promise.resolve([])).then((arr:any[])=>{
        // normalize timestamps to ISO strings
        setImages(arr.map(x=>({ tag: x.tag, digest: x.digest, timestamp: x.timestamp || '', dockerfile: x.dockerfile })))
      })
    }
  }, [tab, id])

  const save = async (nextCmd: string) => {
    // Only persist cmd here; do not touch base fields to avoid clearing them unintentionally
    await fetch('/api/block', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id, cmd: nextCmd })
    })
  }

  // keep a getter for latest editor content; register a save handler so global Save Notebook can persist this block
  const getCurrentValueRef = React.useRef<()=>string>(() => currentValueRef.current)
  useEffect(()=>{ getCurrentValueRef.current = () => currentValueRef.current }, [])
  useEffect(()=>{
    onRegisterSave(id, async ()=>{ await save(getCurrentValueRef.current()) })
  }, [id])
  useEffect(()=>{
    onRegisterRun(id, async ()=>{ await run() })
  }, [id])
  const [dirty, setDirty] = useState(false)
  const run = async (nextCmd?: string) => {
    if (nextCmd !== undefined) await save(nextCmd)
    const ok = await ensureSaved()
    if (!ok) return
    setRunning(true)
    setRunningGlobal(id, true)
    // clear previous logs immediately so only new run output is shown
    setLogs('')
    // start SSE follow to update logs live
    const es = new EventSource(`/api/logs?id=${encodeURIComponent(id)}&follow=true`)
    es.onmessage = (e) => {
      setLogs(prev => prev ? prev + "\n" + e.data : e.data)
    }
    es.onerror = () => {
      // ignore; stream may end when container stops
    }
    try {
      // kick off synchronous run and wait for completion
      const resp = await fetch('/api/run', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id }) })
      es.close()
      // final fetch of logs to ensure we have everything
      const l = await fetch(`/api/logs?id=${encodeURIComponent(id)}`).then(r=>r.ok?r.text():'')
      setLogs(l)
    } finally {
      setRunning(false)
      setRunningGlobal(id, false)
    }
  }

  return (
    <div className="border border-zinc-200/70 dark:border-zinc-800/80 bg-white/70 dark:bg-zinc-700/60 rounded-xl overflow-hidden shadow-sm focus-within:ring-2 focus-within:ring-blue-500/30 hover:ring-2 hover:ring-blue-400/20 transition-all">
      <div className="flex items-center gap-2 px-4 py-3 bg-white/60 dark:bg-zinc-900/60 border-b border-zinc-200/70 dark:border-zinc-800/80">
        <strong className="mr-2 tracking-tight">{id}</strong>
        <div className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium border ${statusDisplay.color} ${statusDisplay.bgColor} ${statusDisplay.borderColor}`}>
          <span className="hidden sm:inline">{statusDisplay.label}</span>
        </div>
        <span className="text-xs text-zinc-600 dark:text-zinc-400">Base:</span>
        <BaseReadonly from={from} fromBlock={fromBlock} />
        <div className="flex-1" />
        <div className="relative">
          <button 
            onClick={() => setDropdownOpen((prev: Record<string, boolean>) => ({ ...prev, [id]: !prev[id] }))}
            className="p-2 rounded-lg text-zinc-600 dark:text-zinc-400 hover:bg-zinc-900/5 dark:hover:bg-white/5 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40"
          >
            <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z"/>
            </svg>
          </button>
          {dropdownOpen[id] && (
            <div className="absolute right-0 top-full mt-1 w-48 rounded-lg border border-zinc-200/70 dark:border-zinc-800/80 bg-white/90 dark:bg-zinc-900/90 backdrop-blur shadow-lg z-10">
              <button 
                onClick={() => {
                  onExportDockerfile(id)
                  setDropdownOpen((prev: Record<string, boolean>) => ({ ...prev, [id]: false }))
                }}
                className="w-full px-3 py-2 text-left text-sm text-zinc-700 dark:text-zinc-200 hover:bg-zinc-900/5 dark:hover:bg-white/5 transition-colors first:rounded-t-lg"
              >
                Export to Dockerfile
              </button>
              <button 
                onClick={() => {
                  onDeleteBlock(id)
                  setDropdownOpen((prev: Record<string, boolean>) => ({ ...prev, [id]: false }))
                }}
                className="w-full px-3 py-2 text-left text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors last:rounded-b-lg"
              >
                Delete Block
              </button>
            </div>
          )}
        </div>
        <button onClick={() => run()} className={`px-3.5 py-1.5 rounded-lg text-sm font-medium text-white shadow-sm hover:shadow-md transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 active:translate-y-px ${(running||runningGlobal)?'bg-blue-400/90 cursor-wait':'bg-blue-600 hover:bg-blue-500 dark:bg-blue-500/60 dark:hover:bg-blue-400/90'}`}>
          {(running||runningGlobal) ? (
            <>
              <svg className="animate-spin w-4 h-4" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
              </svg>
            </>
          ) : (
            <div className="flex flex-row items-center">
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                <path d="M8 5v14l11-7z"/>
              </svg>
              <div className="ml-1 text-xs opacity-80">⌘↵</div>
            </div>
          )}
        </button>
      </div>
      <EditorCell initial={cmd || ''} onRun={run} workdir={workdir} isSaving={isSaving} blockId={id} onDirtyChange={onDirtyChange} onRegister={(get)=>{
        // keep latest editor content for save via getter
        getCurrentValueRef.current = get
        currentValueRef.current = get()
      }} />
      <div className="px-3 pt-2">
        <div className="flex items-center gap-2 border-b border-zinc-200/70 dark:border-zinc-800/80">
          <button
            className={`px-3 py-2 text-sm rounded-t-lg border-b-2 transition-colors ${tab==='logs' ? 'border-blue-600 text-zinc-900 dark:text-white' : 'border-transparent text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white'}`}
            onClick={()=>setTab('logs')}
          >
            Logs
          </button>
          <button
            className={`px-3 py-2 text-sm rounded-t-lg border-b-2 transition-colors ${tab==='diff' ? 'border-blue-600 text-zinc-900 dark:text-white' : 'border-transparent text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white'}`}
            onClick={()=>setTab('diff')}
          >
            Diff
          </button>
          <button
            className={`px-3 py-2 text-sm rounded-t-lg border-b-2 transition-colors ${tab==='images' ? 'border-blue-600 text-zinc-900 dark:text-white' : 'border-transparent text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white'}`}
            onClick={()=>setTab('images')}
          >
            Images
          </button>
          {hasError && (
            <button
              className={`px-3 py-2 text-sm rounded-t-lg border-b-2 transition-colors ${tab==='error' ? 'border-red-600 text-red-600 dark:text-red-400' : 'border-transparent text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300'}`}
              onClick={()=>setTab('error')}
            >
              Error
            </button>
          )}
        </div>
        <div className="py-2">
          {tab==='logs' && (<pre className="whitespace-pre-wrap m-0">{logs || 'No logs yet.'}</pre>)}
          {tab==='diff' && (
            <div>
              {diff.length===0 ? (<div className="text-sm text-zinc-600 dark:text-zinc-400">No changes</div>) : (
                <ul className="m-0">
                  {diff.map((d,i)=>(<li key={i}><code>{d.kind}</code> {d.path}</li>))}
                </ul>
              )}
            </div>
          )}
          {tab==='images' && (
            <div>
              {images.length===0 ? (
                <div className="text-sm text-zinc-600 dark:text-zinc-400">No images yet</div>
              ) : (
                <ul className="m-0">
                  {images.map((im,i)=>(
                    <li key={i} className="text-sm">
                      <button
                        className="w-full text-left rounded-md px-2 py-1 hover:bg-zinc-900/5 dark:hover:bg-white/10 transition-colors"
                        title="View Dockerfile for this image"
                        onClick={async()=>{
                          if (im.dockerfile) {
                            onShowDockerfile({ dockerfile: im.dockerfile, tag: im.tag, digest: im.digest })
                          } else if (im.digest) {
                            const txt = await fetch(`/api/dockerfile?digest=${encodeURIComponent(im.digest)}`).then(r=> r.ok ? r.text() : '')
                            if (txt) {
                              onShowDockerfile({ dockerfile: txt, tag: im.tag, digest: im.digest })
                            } else {
                              onExportDockerfile(id)
                            }
                          } else {
                            onExportDockerfile(id)
                          }
                        }}
                      >
                        <code className="mr-2">{im.tag}</code>
                        <code className="mr-2">{im.digest}</code>
                        <span className="text-zinc-500">{im.timestamp}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
          {tab==='error' && (
            <div>
              <pre className="whitespace-pre-wrap m-0 text-red-600 dark:text-red-400 font-mono text-sm">{blockStatus?.error || 'No error details available'}</pre>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function EditorCell({ initial, onRun, workdir, onRegister, isSaving, blockId, onDirtyChange }: { initial: string; onRun: (nextCmd: string)=>Promise<void>; workdir?: string; onRegister?: (get: ()=>string)=>void; isSaving?: boolean; blockId?: string; onDirtyChange?: (id: string, dirty: boolean)=>void }) {
  const [value, setValue] = useState(initial)
  useEffect(()=>{ setValue(initial) }, [initial])
  useEffect(()=>{ onRegister && onRegister(()=>value) }, [value, onRegister])
  
  // Track dirty state
  useEffect(() => {
    if (blockId && onDirtyChange) {
      const isDirty = value !== initial
      onDirtyChange(blockId, isDirty)
    }
  }, [value, initial, blockId, onDirtyChange])
  const onKey = async (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault()
      await onRun(value)
    }
  }
  return (
    <div className="p-3">
      {workdir && <div className="text-xs text-zinc-600 dark:text-zinc-400 mb-1">WORKDIR {workdir}</div>}
      <textarea 
        spellCheck={false} 
        disabled={isSaving}
        className="w-full h-32 border border-zinc-200 dark:border-zinc-800 rounded-md bg-white dark:bg-zinc-900 p-2 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500/40 disabled:opacity-50 disabled:cursor-not-allowed"
        value={value} 
        onChange={e=>setValue(e.target.value)} 
        onKeyDown={onKey} 
      />
    </div>
  )
}

function BasePicker({ id, from, fromBlock, onChange, blocksSuggest }: { id: string; from?: string; fromBlock?: string; onChange: (from?: string, fromBlock?: string)=>void; blocksSuggest: string[] }) {
  const [mode, setMode] = useState<'image'|'block'>(from ? 'image' : 'block')
  const [image, setImage] = useState(from||'')
  const [block, setBlock] = useState(fromBlock||'')
  useEffect(()=>{ setMode(from ? 'image':'block'); setImage(from||''); setBlock(fromBlock||'') }, [from, fromBlock])
  return (
    <div className="flex items-center gap-2 text-xs">
      <div className="inline-flex items-center rounded-md border border-zinc-200 dark:border-zinc-800 overflow-hidden bg-zinc-50 dark:bg-zinc-900/50">
        <button
          className={`px-2.5 py-1.5 text-xs transition-all ${mode==='image' ? 'bg-white dark:bg-zinc-900 text-blue-700 dark:text-blue-200 shadow-[inset_0_-2px_0_rgba(59,130,246,0.25)]' : 'bg-transparent text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100/70 dark:hover:bg-zinc-800/60'}`}
          onClick={()=>{ setMode('image'); onChange(image, undefined) }}
          type="button"
        >Image</button>
        <button
          className={`px-2.5 py-1.5 text-xs transition-all ${mode==='block' ? 'bg-white dark:bg-zinc-900 text-blue-700 dark:text-blue-200 shadow-[inset_0_-2px_0_rgba(59,130,246,0.25)]' : 'bg-transparent text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100/70 dark:hover:bg-zinc-800/60'}`}
          onClick={()=>{ setMode('block'); onChange(undefined, block) }}
          type="button"
        >from_block</button>
      </div>
      {mode==='image' ? (
        <input className="border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1 w-56 bg-white dark:bg-zinc-900 text-zinc-800 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-500 focus:outline-none focus:ring-2 focus:ring-blue-500/40"
          placeholder="e.g. alpine:latest" value={image} onChange={e=>{ setImage(e.target.value); onChange(e.target.value, undefined) }} />
      ) : (
        <select className="border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1 w-48 bg-white dark:bg-zinc-900 text-zinc-800 dark:text-zinc-200 focus:outline-none focus:ring-2 focus:ring-blue-500/40"
          value={block} onChange={e=>{ setBlock(e.target.value); onChange(undefined, e.target.value) }}>
          <option value="" disabled>{blocksSuggest.length ? 'Pick a block…' : 'No blocks'}</option>
          {blocksSuggest.map(b => (<option key={b} value={b}>{b}</option>))}
        </select>
      )}
    </div>
  )
}

function BaseReadonly({ from, fromBlock }: { from?: string; fromBlock?: string }) {
  const isImage = !!from
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className={`inline-flex items-center gap-1 rounded-md border px-2.5 py-1.5 ${isImage ? 'border-emerald-300/70 dark:border-emerald-700/70 bg-emerald-50 dark:bg-emerald-900/20 text-emerald-800 dark:text-emerald-200' : 'border-blue-300/70 dark:border-blue-700/70 bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-200'}`}>
        {isImage ? (
          <>
            <span className="text-[10px] uppercase tracking-wide">Image</span>
            <code className="text-xs">{from}</code>
          </>
        ) : (
          <>
            <span className="text-[10px] uppercase tracking-wide">from_block</span>
            <code className="text-xs">{fromBlock || '—'}</code>
          </>
        )}
      </span>
    </div>
  )
}


