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
  const [unsavedIds, setUnsavedIds] = useState<string[]>([])
  const [showUnsavedModal, setShowUnsavedModal] = useState(false)
  const pendingProceedResolve = React.useRef<((ok: boolean)=>void) | null>(null)
  const saveRegistry = React.useRef(new Map<string, ()=>Promise<void>>())
  const runRegistry = React.useRef(new Map<string, ()=>Promise<void>>())
  const [runningById, setRunningById] = useState<Record<string, boolean>>({})
  const [isSaving, setIsSaving] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [blockToDelete, setBlockToDelete] = useState<string | null>(null)
  const [dropdownOpen, setDropdownOpen] = useState<Record<string, boolean>>({})
  const [showCommandsModal, setShowCommandsModal] = useState(false)
  const [commandsModalImageRef, setCommandsModalImageRef] = useState('')
  const [commandsModalBlockId, setCommandsModalBlockId] = useState('')
  const [showImageDeleteModal, setShowImageDeleteModal] = useState(false)
  const [imageToDelete, setImageToDelete] = useState<{ digest: string; tag: string } | null>(null)
  const [showDockerfileModal, setShowDockerfileModal] = useState(false)
  const [dockerfileModalImage, setDockerfileModalImage] = useState<{ digest: string; tag: string; dockerfile?: string } | null>(null)
  const [lastSavedAt, setLastSavedAt] = useState<Date | null>(null)
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
    const load = () => fetch('/api/status').then(r => r.json()).then(data => setStatus(Array.isArray(data) ? data : []))
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
      // Add 0.5 second delay to make save operation visible
      await new Promise(resolve => setTimeout(resolve, 500))
      // refresh project to reflect saved cmds
      const p = await (await fetch('/api/project')).json()
      setProject(p)
      setUnsavedIds([])
      setLastSavedAt(new Date())
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
  const blocks = Array.isArray(rawBlocks) ? rawBlocks.map(b => ({
    ...b,
    _id: (b as any).id ?? (b as any).ID,
  })) : []


  const deleteBlock = async (blockId: string) => {
    const resp = await fetch(`/api/block?id=${encodeURIComponent(blockId)}`, { method: 'DELETE' })
    if (resp.ok) {
      const p = await (await fetch('/api/project')).json()
      setProject(p)
      setShowDeleteModal(false)
      setBlockToDelete(null)
    }
  }

  const forceRebuild = async (blockId: string) => {
    const ok = await ensureSaved()
    if (!ok) return
    setRunningById(prev => ({ ...prev, [blockId]: true }))
    try {
      const resp = await fetch('/api/run', { 
        method: 'POST', 
        headers: { 'Content-Type': 'application/json' }, 
        body: JSON.stringify({ id: blockId, force: true }) 
      })
      if (!resp.ok) {
        console.error(`Force rebuild failed for block ${blockId}: ${resp.status}`)
      }
    } finally {
      setRunningById(prev => ({ ...prev, [blockId]: false }))
    }
  }

  const deleteImage = async (digest: string) => {
    const resp = await fetch(`/api/image?digest=${encodeURIComponent(digest)}`, { method: 'DELETE' })
    if (resp.ok) {
      setShowImageDeleteModal(false)
      setImageToDelete(null)
      // The images will be refreshed when the tab is re-opened
    }
  }

  // Show loading state while project is being fetched
  if (!project) {
    return (
      <div className="min-h-screen p-4 font-sans bg-zinc-50 text-zinc-900 dark:bg-zinc-900 dark:text-zinc-100 flex items-center justify-center">
        <div className="text-center">
          <div className="w-8 h-8 border-2 border-blue-600 border-t-transparent rounded-full animate-spin mx-auto mb-4"></div>
          <div className="text-sm text-zinc-600 dark:text-zinc-400">Loading project...</div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen p-4 font-sans bg-zinc-50 text-zinc-900 dark:bg-zinc-900 dark:text-zinc-100">
      <header className="flex items-center gap-3 rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/70 dark:bg-zinc-900/60 backdrop-blur px-4 py-2 shadow-sm">
        <h1 className="text-xl font-semibold tracking-tight">Dockstep Notebook</h1>
        <div className="text-sm text-zinc-600 dark:text-zinc-400">{`${projName} · v${projVersion}`}</div>
        <div className="flex-1" />
        {/** theme toggle moved to sidebar **/}
        <div className="flex items-center gap-2">
          {lastSavedAt && (
            <div className="text-xs text-zinc-500 dark:text-zinc-400">
              Last saved: {lastSavedAt.toLocaleTimeString()}
            </div>
          )}
          <button 
            className="ml-2 inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40 disabled:opacity-50 disabled:cursor-not-allowed" 
            onClick={saveAll}
            disabled={isSaving || unsavedIds.length === 0}
            title={unsavedIds.length === 0 ? "No changes to save" : "Save all changes"}
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
              <>Save <span className="text-xs opacity-60">⌘S</span></>
            )}
          </button>
        </div>
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
              const statusArray = Array.isArray(status) ? status : []
              const s = statusArray.find(x => x.id === (b as any)._id)
              const isRunning = runningById[(b as any)._id]
              // Show running status if the block is currently running, otherwise use the API status
              const currentStatus = isRunning ? 'running' : (s?.status ?? 'pending')
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
          {blocks.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 px-8 text-center">
              <div className="w-16 h-16 rounded-full bg-zinc-100 dark:bg-zinc-800 flex items-center justify-center mb-4">
                <svg className="w-8 h-8 text-zinc-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                </svg>
              </div>
              <h3 className="text-lg font-medium text-zinc-900 dark:text-zinc-100 mb-2">No blocks yet</h3>
              <p className="text-zinc-600 dark:text-zinc-400 mb-6 max-w-md">
                Get started by creating your first block. You can build from a Docker image like <code className="px-1.5 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 text-sm">alpine:latest</code> or extend from another block.
              </p>
              <button 
                className="inline-flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-500 dark:bg-blue-500/60 dark:hover:bg-blue-400/90 shadow-sm hover:shadow-md transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 active:translate-y-px"
                onClick={()=>{
                  setShowAddModal(true)
                }}
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                </svg>
                Create your first block
              </button>
            </div>
          ) : (
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
                  runningGlobal={!!runningById[(b as any)._id]}
                  setRunningGlobal={(id, isRunning)=> setRunningById(prev => ({ ...prev, [id]: isRunning }))}
                  isSaving={isSaving}
                  dropdownOpen={dropdownOpen}
                  setDropdownOpen={setDropdownOpen}
                  onDeleteBlock={(id) => { setBlockToDelete(id); setShowDeleteModal(true) }}
                  onForceRebuild={forceRebuild}
                  onShowCommandsModal={(imageRef, blockId) => {
                    setCommandsModalImageRef(imageRef)
                    setCommandsModalBlockId(blockId)
                    setShowCommandsModal(true)
                  }}
                  onShowImageDeleteModal={(digest, tag) => {
                    setImageToDelete({ digest, tag })
                    setShowImageDeleteModal(true)
                  }}
                  onShowDockerfileModal={(digest, tag, dockerfile) => {
                    setDockerfileModalImage({ digest, tag, dockerfile })
                    setShowDockerfileModal(true)
                  }}
                  status={status}
                />
              ))}
            </div>
          )}
        </section>
      </main>
      {showAddModal && (
        <AddBlockModal
          isOpen={showAddModal}
          onClose={()=>setShowAddModal(false)}
          onAdd={async (blockId, from, fromBlock) => {
            const body: any = { id: blockId }
            if (from) body.from = from
            if (fromBlock) body.from_block = fromBlock
            const resp = await fetch('/api/block',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
            if (resp.ok) {
              const p = await (await fetch('/api/project')).json()
              setProject(p)
              setShowAddModal(false)
            }
          }}
          blocksSuggest={blocks.map(b => (b as any)._id)}
        />
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
      <CommandsModal 
        isOpen={showCommandsModal}
        onClose={() => setShowCommandsModal(false)}
        imageRef={commandsModalImageRef}
        blockId={commandsModalBlockId}
      />
      <DockerfileModal 
        isOpen={showDockerfileModal}
        onClose={() => setShowDockerfileModal(false)}
        image={dockerfileModalImage}
      />
      {showImageDeleteModal && (
        <div className="fixed inset-0 z-50">
          <div className="absolute inset-0 bg-black/30" onClick={()=>{
            setShowImageDeleteModal(false)
            setImageToDelete(null)
          }} />
          <div className="absolute inset-0 flex items-center justify-center p-4">
            <div className="w-full max-w-md rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl">
              <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80">
                <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Delete Image</div>
              </div>
              <div className="p-4 text-sm text-zinc-700 dark:text-zinc-200">
                Are you sure you want to delete image "{imageToDelete?.tag}"? This action cannot be undone.
              </div>
              <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
                <button className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" onClick={()=>{
                  setShowImageDeleteModal(false)
                  setImageToDelete(null)
                }}>Cancel</button>
                <button className="px-3.5 py-1.5 text-sm font-medium rounded-lg bg-red-600 hover:bg-red-500 dark:bg-red-500/90 dark:hover:bg-red-400/90 text-white shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500/50 active:translate-y-px" onClick={()=>{
                  if (imageToDelete) deleteImage(imageToDelete.digest)
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
  runningGlobal,
  setRunningGlobal,
  isSaving,
  dropdownOpen,
  setDropdownOpen,
  onDeleteBlock,
  onForceRebuild,
  onShowCommandsModal,
  onShowImageDeleteModal,
  onShowDockerfileModal,
  status,
}: {
  block: any;
  blocksSuggest: string[];
  onRegisterSave: (id: string, fn: ()=>Promise<void>)=>void;
  onRegisterRun: (id: string, fn: ()=>Promise<void>)=>void;
  onDirtyChange: (id: string, dirty: boolean)=>void;
  ensureSaved: ()=>Promise<boolean>;
  runningGlobal: boolean;
  setRunningGlobal: (id: string, running: boolean)=>void;
  isSaving: boolean;
  dropdownOpen: Record<string, boolean>;
  setDropdownOpen: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
  onDeleteBlock: (id: string)=>void;
  onForceRebuild: (id: string)=>void;
  onShowCommandsModal: (imageRef: string, blockId: string)=>void;
  onShowImageDeleteModal: (digest: string, tag: string)=>void;
  onShowDockerfileModal: (digest: string, tag: string, dockerfile?: string)=>void;
  status: StatusItem[];
}) {
  const id: string = block._id
  const [tab, setTab] = useState<'logs' | 'diff' | 'images' | 'dockerfile' | 'lineage'>('logs')
  const [logs, setLogs] = useState('')
  const [diff, setDiff] = useState<{ kind: string; path: string }[]>([])
  const [images, setImages] = useState<{ tag: string; digest: string; timestamp: string; dockerfile?: string }[]>([])
  const [dockerfile, setDockerfile] = useState('')
  const [lineage, setLineage] = useState<{ id: string; from: string; from_block: string; from_block_version: string; digest: string; timestamp: string }[]>([])
  const instructions: string[] = (block as any).instructions ?? (block as any).Instructions ?? []
  const cmd: string = instructions.length > 0 ? instructions.join('\n') : ''
  const workdir: string | undefined = (block as any).workdir ?? (block as any).Workdir
  const [from, setFrom] = useState<string | undefined>((block as any).from ?? (block as any).From)
  const [fromBlock, setFromBlock] = useState<string | undefined>((block as any).from_block ?? (block as any).FromBlock)
  const [running, setRunning] = useState(false)
  const currentValueRef = React.useRef(cmd || '')
  
  // Get current block status and error
  const statusArray = Array.isArray(status) ? status : []
  const blockStatus = statusArray.find(s => s.id === id)
  const hasError = blockStatus?.status === 'failed' && blockStatus?.error
  // Show running status if the block is currently running, otherwise use the API status
  const currentStatus = runningGlobal ? 'running' : (blockStatus?.status ?? 'pending')
  const statusDisplay = getStatusDisplay(currentStatus)
  

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

  useEffect(()=>{
    if (tab==='dockerfile') {
      fetch('/api/export/dockerfile', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ endBlockId: id }) })
        .then(r => r.ok ? r.text() : '')
        .then(setDockerfile)
    }
  }, [tab, id])

  useEffect(()=>{
    if (tab==='lineage') {
      fetch(`/api/lineage?id=${encodeURIComponent(id)}`)
        .then(r => r.ok ? r.json() : [])
        .then(setLineage)
    }
  }, [tab, id])

  const save = async (nextCmd: string) => {
    // Convert the command string to instructions array
    const instructions = nextCmd.split('\n').filter(line => line.trim() !== '')
    const body: any = { id, instructions }
    
    // Include from_block_version if it exists
    if ((block as any).from_block_version) {
      body.from_block_version = (block as any).from_block_version
    }
    
    await fetch('/api/block', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
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
      
      // Check if the run was successful
      if (!resp.ok) {
        console.error(`Block ${id} run failed with status: ${resp.status}`)
        // The status will be updated via the polling mechanism, so we don't need to do anything special here
      }
      
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
        <BaseReadonly from={from} fromBlock={fromBlock} fromBlockVersion={(block as any).from_block_version} />
        <div className="flex-1" />
        <button 
          onClick={() => {
            const currentDigest = blockStatus?.digest || ''
            if (currentDigest) {
              onShowCommandsModal(currentDigest, id)
            }
          }}
          className="p-2 rounded-lg text-zinc-600 dark:text-zinc-400 hover:bg-zinc-900/5 dark:hover:bg-white/5 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40"
          title="Docker Commands"
        >
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="size-6">
            <path strokeLinecap="round" strokeLinejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5" />
          </svg>
        </button>
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
                  onForceRebuild(id)
                  setDropdownOpen((prev: Record<string, boolean>) => ({ ...prev, [id]: false }))
                }}
                className="w-full px-3 py-2 text-left text-sm text-amber-700 dark:text-amber-200 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors rounded-lg"
              >
                Force Rebuild
              </button>
              <button 
                onClick={() => {
                  onDeleteBlock(id)
                  setDropdownOpen((prev: Record<string, boolean>) => ({ ...prev, [id]: false }))
                }}
                className="w-full px-3 py-2 text-left text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors rounded-lg"
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
              <div className="ml-1 text-xs text-opacity-80">⌘↵</div>
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
          <button
            className={`px-3 py-2 text-sm rounded-t-lg border-b-2 transition-colors ${tab==='dockerfile' ? 'border-blue-600 text-zinc-900 dark:text-white' : 'border-transparent text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white'}`}
            onClick={()=>setTab('dockerfile')}
          >
            Dockerfile
          </button>
          <button
            className={`px-3 py-2 text-sm rounded-t-lg border-b-2 transition-colors ${tab==='lineage' ? 'border-blue-600 text-zinc-900 dark:text-white' : 'border-transparent text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white'}`}
            onClick={()=>setTab('lineage')}
          >
            Lineage
          </button>
        </div>
        <div className="py-2">
          {tab==='logs' && (
            <div className="p-2 rounded-md bg-zinc-50 dark:bg-zinc-900 max-h-[620px] overflow-y-auto">
              {logs ? (
                <pre className="whitespace-pre-wrap m-0">
                  {logs.split('\n').map((line, index) => {
                    // Check if line contains error indicators
                    const isError = line.includes('Error:') || line.includes('Error Detail:') || line.includes('failed') || line.includes('ERROR') || line.includes('error:')
                    return (
                      <div key={index} className={isError ? 'text-red-600 dark:text-red-400' : ''}>
                        {line}
                      </div>
                    )
                  })}
                </pre>
              ) : (
                <div className="text-zinc-500 dark:text-zinc-400">No logs yet.</div>
              )}
            </div>
          )}
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
                <ul className="m-0 space-y-2">
                  {images.map((im,i)=>(
                    <li key={i} className="text-sm">
                      <div className="flex items-center justify-between rounded-md px-2 py-1 hover:bg-zinc-900/5 dark:hover:bg-white/10 transition-colors">
                        <button
                          className="flex-1 text-left"
                          title="View Dockerfile for this image"
                          onClick={async()=>{
                            onShowDockerfileModal(im.digest, im.tag, im.dockerfile)
                          }}
                        >
                          <code className="mr-2">{im.tag}</code>
                          <code className="mr-2">{im.digest}</code>
                          <span className="text-zinc-500">{im.timestamp}</span>
                        </button>
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => {
                              onShowCommandsModal(im.digest, id)
                            }}
                            className="p-1 rounded text-zinc-600 dark:text-zinc-400 hover:bg-zinc-900/5 dark:hover:bg-white/5"
                            title="Docker Commands"
                          >
                            <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                              <path d="M8 4h8v2H8V4zm0 4h8v2H8V8zm0 4h8v2H8v-2zm0 4h8v2H8v-2z"/>
                            </svg>
                          </button>
                          <button
                            onClick={() => {
                              onShowImageDeleteModal(im.digest, im.tag)
                            }}
                            className="p-1 rounded text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20"
                            title="Delete Image"
                          >
                            <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                              <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                            </svg>
                          </button>
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
          {tab==='dockerfile' && (
            <div className="relative">
              <button className="absolute top-4 right-4 text-xs px-2 py-1 rounded-md bg-zinc-900/80 text-white dark:bg-white/10 dark:text-zinc-100" onClick={()=>navigator.clipboard.writeText(dockerfile)}>Copy</button>
              <pre className="p-3 w-full h-full border border-zinc-200 dark:border-zinc-800 rounded-md bg-white dark:bg-zinc-900 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500/40">
                {dockerfile}
              </pre>
            </div>
          )}
          {tab==='lineage' && (
            <div>
              {lineage.length===0 ? (
                <div className="text-sm text-zinc-600 dark:text-zinc-400">No lineage data available</div>
              ) : (
                <div className="space-y-2">
                  {lineage.map((item, i) => (
                    <div key={i} className="flex items-center gap-3 p-2 rounded-md bg-zinc-50 dark:bg-zinc-800/50">
                      <div className="flex-shrink-0 w-6 h-6 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-xs font-medium text-blue-700 dark:text-blue-300">
                        {i + 1}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium text-zinc-900 dark:text-zinc-100">{item.id}</div>
                        <div className="text-xs text-zinc-500 dark:text-zinc-400">
                          {item.from ? `Image: ${item.from}` : `from_block: ${item.from_block}`}
                          {item.from_block_version && ` @ ${item.from_block_version.substring(0, 12)}...`}
                        </div>
                        {item.digest && (
                          <div className="text-xs text-zinc-400 dark:text-zinc-500 font-mono">
                            {item.digest.substring(0, 12)}... {item.timestamp}
                          </div>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
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

function BasePicker({ id, from, fromBlock, fromBlockVersion, onChange, blocksSuggest }: { id: string; from?: string; fromBlock?: string; fromBlockVersion?: string; onChange: (from?: string, fromBlock?: string, fromBlockVersion?: string)=>void; blocksSuggest: string[] }) {
  const [mode, setMode] = useState<'image'|'block'>(from ? 'image' : 'block')
  const [image, setImage] = useState(from||'')
  const [block, setBlock] = useState(fromBlock||'')
  const [version, setVersion] = useState(fromBlockVersion||'')
  const [availableVersions, setAvailableVersions] = useState<{ digest: string; timestamp: string }[]>([])
  const [loadingVersions, setLoadingVersions] = useState(false)
  
  // Only update mode if we don't have any existing values to avoid overriding user selection
  useEffect(()=>{ 
    if (!from && !fromBlock) {
      setMode('image'); // Default to image mode for new blocks
    } else if (from && !fromBlock) {
      setMode('image');
    } else if (!from && fromBlock) {
      setMode('block');
    }
    setImage(from||''); 
    setBlock(fromBlock||''); 
    setVersion(fromBlockVersion||'') 
  }, [from, fromBlock, fromBlockVersion])

  // Fetch available versions when block changes
  useEffect(() => {
    if (mode === 'block' && block) {
      setLoadingVersions(true)
      fetch(`/api/history?id=${encodeURIComponent(block)}`)
        .then(r => r.ok ? r.json() : [])
        .then((arr: any[]) => {
          setAvailableVersions(arr.map(x => ({ digest: x.digest, timestamp: x.timestamp || '' })))
        })
        .finally(() => setLoadingVersions(false))
    } else {
      setAvailableVersions([])
    }
  }, [mode, block])

  return (
    <div className="flex items-center gap-2 text-xs">
      <div className="flex items-center rounded-md border border-zinc-200 dark:border-zinc-800 overflow-hidden bg-zinc-50 dark:bg-zinc-900/50">
        <button
          className={`px-2.5 py-1.5 text-xs transition-all ${mode==='image' ? 'bg-white dark:bg-zinc-900 text-blue-700 dark:text-blue-200 shadow-[inset_0_-2px_0_rgba(59,130,246,0.25)]' : 'bg-transparent text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100/70 dark:hover:bg-zinc-800/60'}`}
          onClick={()=>{ 
            setMode('image'); 
            onChange(image, undefined, undefined) 
          }}
          type="button"
        >Docker Image</button>
        <button
          className={`px-2.5 py-1.5 text-xs transition-all ${mode==='block' ? 'bg-white dark:bg-zinc-900 text-blue-700 dark:text-blue-200 shadow-[inset_0_-2px_0_rgba(59,130,246,0.25)]' : 'bg-transparent text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100/70 dark:hover:bg-zinc-800/60'}`}
          onClick={()=>{ 
            setMode('block'); 
            onChange(undefined, block, version) 
          }}
          type="button"
        >Previous Block</button>
      </div>
      {mode==='image' ? (
        <input className="border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1 w-56 bg-white dark:bg-zinc-900 text-zinc-800 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-500 focus:outline-none focus:ring-2 focus:ring-blue-500/40"
          placeholder="e.g. alpine:latest, node:18, python:3.11" value={image} onChange={e=>{ setImage(e.target.value); onChange(e.target.value, undefined, undefined) }} />
      ) : (
        <div className="flex items-center gap-2">
          <select className="border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1 w-48 bg-white dark:bg-zinc-900 text-zinc-800 dark:text-zinc-200 focus:outline-none focus:ring-2 focus:ring-blue-500/40"
            value={block} onChange={e=>{ 
              setBlock(e.target.value); 
              setVersion(''); // Reset version when block changes
              onChange(undefined, e.target.value, '') 
            }}>
            <option value="" disabled>{blocksSuggest.length ? 'Select a block to extend…' : 'No blocks available'}</option>
            {blocksSuggest.map(b => (<option key={b} value={b}>{b}</option>))}
          </select>
          {block && (
            <select 
              className="border border-zinc-300 dark:border-zinc-700 rounded-md px-2 py-1 w-40 bg-white dark:bg-zinc-900 text-zinc-800 dark:text-zinc-200 focus:outline-none focus:ring-2 focus:ring-blue-500/40"
              value={version} 
              onChange={e=>{ 
                setVersion(e.target.value); 
                onChange(undefined, block, e.target.value) 
              }}
              disabled={loadingVersions}
              title="Select specific version of the block"
            >
              <option value="">Latest version</option>
              {availableVersions.map((v, i) => (
                <option key={i} value={v.digest}>
                  {v.timestamp ? new Date(v.timestamp).toLocaleString() : v.digest.substring(0, 12)}...
                </option>
              ))}
            </select>
          )}
        </div>
      )}
    </div>
  )
}

function CommandsModal({ 
  isOpen, 
  onClose, 
  imageRef, 
  blockId 
}: { 
  isOpen: boolean; 
  onClose: () => void; 
  imageRef: string; 
  blockId: string; 
}) {
  if (!isOpen) return null

  const commands: { label: string; command: string; description?: string }[] = [
    {
      label: "Run interactive shell",
      command: `docker run -it --rm ${imageRef} /bin/sh`,
    },
    {
      label: "Tag image",
      command: `docker tag ${imageRef} myrepo/${blockId}:latest`,
    },
    {
      label: "Push image",
      command: `docker push myrepo/${blockId}:latest`,
    },
    {
      label: "Inspect image",
      command: `docker inspect ${imageRef}`,
    }
  ]

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="absolute inset-0 flex items-center justify-center p-4">
        <div className="w-full max-w-2xl rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl">
          <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80">
            <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Docker Commands</div>
            <div className="text-xs text-zinc-500 dark:text-zinc-400">Commands for {blockId}</div>
          </div>
          <div className="p-4 space-y-3">
            {commands.map((cmd, i) => (
              <div key={i} className="group relative">
                <div className="text-xs font-medium text-zinc-700 dark:text-zinc-200 mb-1">{cmd.label}</div>
                {cmd.description && <div className="text-xs text-zinc-500 dark:text-zinc-400 mb-2">{cmd.description}</div>}
                <div className="relative">
                  <code className="block px-3 py-2 rounded-md bg-zinc-100 dark:bg-zinc-800 overflow-x-auto text-xs font-mono">
                    {cmd.command}
                  </code>
                  <button 
                    className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity text-xs px-2 py-1 rounded-md bg-zinc-900/80 text-white dark:bg-white/10 dark:text-zinc-100"
                    onClick={() => copyToClipboard(cmd.command)}
                  >
                    Copy
                  </button>
                </div>
              </div>
            ))}
          </div>
          <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
            <button 
              className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" 
              onClick={onClose}
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function DockerfileModal({ 
  isOpen, 
  onClose, 
  image 
}: { 
  isOpen: boolean; 
  onClose: () => void; 
  image: { digest: string; tag: string; dockerfile?: string } | null; 
}) {
  const [dockerfileContent, setDockerfileContent] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (isOpen && image) {
      setLoading(true)
      // First try to use the dockerfile from the image record if available
      if (image.dockerfile) {
        setDockerfileContent(image.dockerfile)
        setLoading(false)
      } else {
        // Fallback to fetching from the API using the digest
        fetch(`/api/dockerfile-by-digest?digest=${encodeURIComponent(image.digest)}`)
          .then(r => {
            if (r.ok) {
              return r.text()
            } else {
              throw new Error(`API returned ${r.status}`)
            }
          })
          .then(content => {
            setDockerfileContent(content || 'Dockerfile not available')
            setLoading(false)
          })
          .catch((error) => {
            console.warn('Failed to fetch Dockerfile from API:', error)
            setDockerfileContent('Dockerfile not available - this image was built before Dockerfile snapshots were implemented')
            setLoading(false)
          })
      }
    }
  }, [isOpen, image])

  if (!isOpen || !image) return null

  const copyToClipboard = () => {
    navigator.clipboard.writeText(dockerfileContent)
  }

  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="absolute inset-0 flex items-center justify-center p-4">
        <div className="w-full max-w-4xl h-[80vh] rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl flex flex-col">
          <div className="px-4 py-3 border-b border-zinc-200/70 dark:border-zinc-800/80 flex items-center justify-between">
            <div>
              <div className="text-sm font-medium text-zinc-700 dark:text-zinc-200">Dockerfile</div>
              <div className="text-xs text-zinc-500 dark:text-zinc-400">
                {image.tag} • {image.digest.substring(0, 12)}...
              </div>
            </div>
            <button 
              className="text-xs px-2 py-1 rounded-md bg-zinc-900/80 text-white dark:bg-white/10 dark:text-zinc-100 hover:bg-zinc-800 dark:hover:bg-white/20 transition-colors" 
              onClick={copyToClipboard}
            >
              Copy
            </button>
          </div>
          <div className="p-4 flex-1 overflow-auto">
            {loading ? (
              <div className="flex items-center justify-center h-full">
                <div className="text-sm text-zinc-600 dark:text-zinc-400">Loading Dockerfile...</div>
              </div>
            ) : (
              <pre className="whitespace-pre-wrap m-0 text-sm font-mono text-zinc-800 dark:text-zinc-200">
                {dockerfileContent || 'Dockerfile not available'}
              </pre>
            )}
          </div>
          <div className="px-4 py-3 flex items-center justify-end gap-2 border-t border-zinc-200/70 dark:border-zinc-800/80">
            <button 
              className="px-3 py-1.5 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" 
              onClick={onClose}
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function AddBlockModal({ 
  isOpen, 
  onClose, 
  onAdd, 
  blocksSuggest 
}: { 
  isOpen: boolean; 
  onClose: () => void; 
  onAdd: (blockId: string, from?: string, fromBlock?: string) => Promise<void>;
  blocksSuggest: string[];
}) {
  const [blockId, setBlockId] = useState('')
  const [mode, setMode] = useState<'image'|'block'>('image')
  const [image, setImage] = useState('alpine:latest')
  const [block, setBlock] = useState('')
  const [version, setVersion] = useState('')
  const [availableVersions, setAvailableVersions] = useState<{ digest: string; timestamp: string }[]>([])
  const [loadingVersions, setLoadingVersions] = useState(false)

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setBlockId('')
      setMode('image')
      setImage('alpine:latest')
      setBlock('')
      setVersion('')
    }
  }, [isOpen])

  // Fetch available versions when block changes
  useEffect(() => {
    if (mode === 'block' && block) {
      setLoadingVersions(true)
      fetch(`/api/history?id=${encodeURIComponent(block)}`)
        .then(r => r.ok ? r.json() : [])
        .then((arr: any[]) => {
          setAvailableVersions(arr.map(x => ({ digest: x.digest, timestamp: x.timestamp || '' })))
        })
        .finally(() => setLoadingVersions(false))
    } else {
      setAvailableVersions([])
    }
  }, [mode, block])

  // Handle CMD+Enter shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (isOpen && e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        handleSubmit()
      }
    }
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown)
      return () => window.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, blockId, mode, image, block, version])

  const handleSubmit = async () => {
    if (!blockId.trim()) return
    
    const from = mode === 'image' ? image : undefined
    const fromBlock = mode === 'block' ? block : undefined
    
    await onAdd(blockId.trim(), from, fromBlock)
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="absolute inset-0 flex items-center justify-center p-4">
        <div className="w-full max-w-lg rounded-xl border border-zinc-200/70 dark:border-zinc-800/80 bg-white/80 dark:bg-zinc-900/80 backdrop-blur shadow-xl">
          <div className="px-6 py-4 border-b border-zinc-200/70 dark:border-zinc-800/80">
            <div className="text-lg font-medium text-zinc-700 dark:text-zinc-200">Add Block</div>
            <div className="text-sm text-zinc-500 dark:text-zinc-400">Create a new build block</div>
          </div>
          <div className="p-6 space-y-6">
            <div>
              <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-200 mb-2">Block Name</label>
              <input 
                value={blockId} 
                onChange={e=>setBlockId(e.target.value)} 
                placeholder="e.g. build, test, deploy" 
                className="w-full rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2.5 text-sm outline-none focus:ring-2 focus:ring-blue-500/40" 
                autoFocus
              />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-200 mb-3">Base Image</label>
              
              {/* Improved mode selector */}
              <div className="flex items-center gap-1 p-1 rounded-lg border border-zinc-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800/50 mb-4">
                <button
                  className={`flex-1 px-3 py-2 text-sm font-medium rounded-md transition-all ${
                    mode === 'image' 
                      ? 'bg-white dark:bg-zinc-900 text-blue-700 dark:text-blue-200 shadow-sm border border-blue-200 dark:border-blue-800' 
                      : 'text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200'
                  }`}
                  onClick={() => setMode('image')}
                >
                  Docker Image
                </button>
                <button
                  className={`flex-1 px-3 py-2 text-sm font-medium rounded-md transition-all ${
                    mode === 'block' 
                      ? 'bg-white dark:bg-zinc-900 text-blue-700 dark:text-blue-200 shadow-sm border border-blue-200 dark:border-blue-800' 
                      : 'text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200'
                  }`}
                  onClick={() => setMode('block')}
                >
                  Previous Block
                </button>
              </div>

              {mode === 'image' ? (
                <div>
                  <input 
                    className="w-full rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2.5 text-sm outline-none focus:ring-2 focus:ring-blue-500/40"
                    placeholder="e.g. alpine:latest, node:18, python:3.11" 
                    value={image} 
                    onChange={e => setImage(e.target.value)} 
                  />
                  <div className="mt-2 text-xs text-zinc-500 dark:text-zinc-400">
                    Popular options: <code className="px-1 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800">alpine:latest</code>, <code className="px-1 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800">node:18</code>, <code className="px-1 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800">python:3.11</code>
                  </div>
                </div>
              ) : (
                <div className="space-y-3">
                  <select 
                    className="w-full rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2.5 text-sm outline-none focus:ring-2 focus:ring-blue-500/40"
                    value={block} 
                    onChange={e => { 
                      setBlock(e.target.value)
                      setVersion('')
                    }}
                  >
                    <option value="" disabled>{blocksSuggest.length ? 'Select a block to extend…' : 'No blocks available'}</option>
                    {blocksSuggest.map(b => (<option key={b} value={b}>{b}</option>))}
                  </select>
                  
                  {block && (
                    <select 
                      className="w-full rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2.5 text-sm outline-none focus:ring-2 focus:ring-blue-500/40"
                      value={version} 
                      onChange={e => setVersion(e.target.value)}
                      disabled={loadingVersions}
                    >
                      <option value="">Latest version</option>
                      {availableVersions.map((v, i) => (
                        <option key={i} value={v.digest}>
                          {v.timestamp ? new Date(v.timestamp).toLocaleString() : v.digest.substring(0, 12)}...
                        </option>
                      ))}
                    </select>
                  )}
                </div>
              )}
            </div>
          </div>
          <div className="px-6 py-4 flex items-center justify-end gap-3 border-t border-zinc-200/70 dark:border-zinc-800/80">
            <button 
              className="px-4 py-2 text-sm font-medium rounded-lg text-zinc-700 dark:text-zinc-200 bg-zinc-900/5 dark:bg-white/5 hover:bg-zinc-900/10 dark:hover:bg-white/10 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40" 
              onClick={onClose}
            >
              Cancel
            </button>
            <button
              className="px-4 py-2 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-500 dark:bg-blue-500/60 dark:hover:bg-blue-400/90 text-white shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/50 active:translate-y-px"
              disabled={!blockId.trim()}
              onClick={handleSubmit}
            >
              Create Block
              <span className="ml-2 text-xs opacity-60">⌘↵</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function BaseReadonly({ from, fromBlock, fromBlockVersion }: { from?: string; fromBlock?: string; fromBlockVersion?: string }) {
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
            {fromBlockVersion && (
              <span className="text-[10px] text-zinc-500 dark:text-zinc-400">
                @ {fromBlockVersion.substring(0, 12)}...
              </span>
            )}
          </>
        )}
      </span>
    </div>
  )
}


