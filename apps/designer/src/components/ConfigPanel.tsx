import { useState, useEffect, useRef, useCallback } from 'react'
import { useReactFlow } from '@xyflow/react'
import type { DesignerNode, NodeData } from '../types/designer'
import type { SchemaField } from '../types/mapper'
import type { InputMapping, HttpNodeConfig } from '../types/dsl'
import type { SecretMeta } from '../types/secrets'
import { buildSourceFields } from '../lib/mapper'
import { liveTest, listSecrets } from '../lib/api'
import { DataMapper } from './DataMapper'
import { MonacoModal } from './MonacoModal'

interface ConfigPanelProps {
  selectedNode: DesignerNode | null
  onNodeUpdate: (nodeId: string, updates: Partial<DesignerNode['data']>) => void
  allNodes?: DesignerNode[]
}

const NODES_WITH_SECRET = ['sftp', 's3', 'smb', 'mail', 'rabbitmq', 'sql', 'http']

/** Right panel showing configuration for the selected node */
export function ConfigPanel({ selectedNode, onNodeUpdate, allNodes = [] }: ConfigPanelProps) {
  const { setNodes } = useReactFlow()
  const [showMapper, setShowMapper] = useState(false)
  const [showMonaco, setShowMonaco] = useState(false)
  const [liveTestInput, setLiveTestInput] = useState('{\n  "body": {}\n}')
  const [liveTestResult, setLiveTestResult] = useState<string | null>(null)
  const [liveTestError, setLiveTestError] = useState<string | null>(null)
  const [liveTestLoading, setLiveTestLoading] = useState(false)
  const [headerRows, setHeaderRows] = useState<Array<{ uid: number; key: string; value: string }>>([])
  const [availableSecrets, setAvailableSecrets] = useState<SecretMeta[]>([])
  const uidCounterRef = useRef(0)
  const nextUid = () => ++uidCounterRef.current

  // Load secrets list once on mount so the secret_ref dropdown is always populated
  useEffect(() => {
    listSecrets()
      .then(setAvailableSecrets)
      .catch(() => { /* silently ignore — user may not have secrets yet */ })
  }, [])

  useEffect(() => {
    if (selectedNode?.data.nodeKind === 'process' && selectedNode.data.type === 'http') {
      const config = (selectedNode.data.config as HttpNodeConfig) || {}
      setHeaderRows(
        Object.entries(config.headers || {}).map(([key, value]) => ({
          uid: nextUid(),
          key,
          value,
        }))
      )
    } else {
      setHeaderRows([])
    }
  }, [selectedNode?.id])

  const updateNodeDataCentralized = useCallback((updates: Partial<DesignerNode['data']>) => {
    if (!selectedNode) return
    const selectedNodeId = selectedNode.id
    setNodes((nds) => nds.map((node) =>
      node.id === selectedNodeId
        ? { ...node, data: { ...node.data, ...updates } }
        : node
    ))
    onNodeUpdate(selectedNodeId, updates)
  }, [selectedNode, setNodes, onNodeUpdate])

  const updateNodeConfigCentralized = useCallback((updatedConfig: Record<string, unknown>) => {
    if (!selectedNode) return
    const selectedNodeId = selectedNode.id
    setNodes((nds) => nds.map((node) =>
      node.id === selectedNodeId
        ? { ...node, data: { ...node.data, config: updatedConfig } as unknown as NodeData }
        : node
    ))
    onNodeUpdate(selectedNodeId, { ...selectedNode.data, config: updatedConfig } as Partial<NodeData>)
  }, [selectedNode, setNodes, onNodeUpdate])

  const syncHeaders = useCallback((rows: Array<{ key: string; value: string }>) => {
    if (!selectedNode || selectedNode.data.nodeKind !== 'process') return
    const headers = rows.reduce<Record<string, string>>((acc, row) => {
      if (row.key) acc[row.key] = row.value
      return acc
    }, {})
    const currentConfig = (selectedNode.data.config as HttpNodeConfig) || {}
    updateNodeConfigCentralized({ ...(currentConfig as unknown as Record<string, unknown>), headers } as Record<string, unknown>)
  }, [selectedNode, updateNodeConfigCentralized])

  if (!selectedNode) {
    return (
      <aside className="w-72 bg-gray-50 border-l border-gray-200 flex flex-col">
        <div className="px-4 py-3 border-b border-gray-200 bg-white">
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Configuration</h2>
        </div>
        <div className="flex-1 flex items-center justify-center p-4">
          <p className="text-sm text-gray-400 text-center">Select a node on the canvas to configure it</p>
        </div>
      </aside>
    )
  }

  const data = selectedNode.data

  const handleIdChange = (value: string) => updateNodeDataCentralized({ id: value })

  const handleDescriptionChange = (value: string) => {
    if (data.nodeKind === 'process') updateNodeDataCentralized({ description: value })
  }

  const handleScriptChange = (value: string) => {
    if (data.nodeKind === 'process' && ((data.type as string) === 'script_ts' || data.type === 'code')) {
      if (data.type === 'code') {
        const currentConfig = (data.config as unknown as Record<string, unknown>) || {}
        updateNodeConfigCentralized({ ...currentConfig, script: value })
      } else {
        updateNodeDataCentralized({ script: value })
      }
    }
  }

  const handleInputMappingChange = (key: string, value: string) => {
    if (data.nodeKind === 'process') {
      updateNodeDataCentralized({
        input_mapping: { ...(data.input_mapping || {}), [key]: value },
      })
    }
  }

  const handleMapperSave = (mapping: InputMapping) => {
    if (data.nodeKind === 'process') updateNodeDataCentralized({ input_mapping: mapping })
    setShowMapper(false)
  }

  const handleMonacoSave = (script: string) => {
    handleScriptChange(script)
    setShowMonaco(false)
  }

  const handleHttpConfigChange = (field: keyof HttpNodeConfig, value: string | number) => {
    if (data.nodeKind !== 'process') return
    const currentConfig = (data.config as HttpNodeConfig) || {}
    updateNodeConfigCentralized({ ...currentConfig, [field]: value } as Record<string, unknown>)
  }

  const handleHeaderChange = (uid: number, field: 'key' | 'value', val: string) => {
    const newRows = headerRows.map((row) => (row.uid === uid ? { ...row, [field]: val } : row))
    setHeaderRows(newRows)
    const timeoutId = setTimeout(() => { syncHeaders(newRows) }, 300)
    return () => clearTimeout(timeoutId)
  }

  const handleAddHeader = () => {
    const newRows = [...headerRows, { uid: nextUid(), key: '', value: '' }]
    setHeaderRows(newRows)
    syncHeaders(newRows)
  }

  const handleRemoveHeader = (uid: number) => {
    const newRows = headerRows.filter((row) => row.uid !== uid)
    setHeaderRows(newRows)
    syncHeaders(newRows)
  }

  const handleConfigFieldChange = (field: string, value: unknown) => {
    if (data.nodeKind !== 'process') return
    const currentConfig = (data.config as unknown as Record<string, unknown>) || {}
    updateNodeConfigCentralized({ ...currentConfig, [field]: value })
  }

  const handleTriggerConfigChange = (field: string, value: unknown) => {
    if (data.nodeKind !== 'trigger') return
    const currentConfig = (data.config as unknown as Record<string, unknown>) || {}
    updateNodeDataCentralized({ config: { ...currentConfig, [field]: value } as never })
  }

  const handleLiveTest = async () => {
    if (data.nodeKind !== 'process') return
    setLiveTestLoading(true)
    setLiveTestResult(null)
    setLiveTestError(null)
    try {
      const parsedInput = JSON.parse(liveTestInput) as Record<string, unknown>
      const script = (data.type as string) === 'script_ts'
        ? (data.script as string || '')
        : (data.type as string) === 'code'
          ? ((data.config as unknown as Record<string, unknown>)?.script as string || '')
          : undefined
      const result = await liveTest({
        input_mapping: data.input_mapping || {},
        script,
        input_payload: parsedInput,
        node_type: data.type as string,
        config: (data.config as Record<string, unknown> | undefined) ?? undefined,
      })
      setLiveTestResult(JSON.stringify(result.output, null, 2))
    } catch (err) {
      setLiveTestError(err instanceof Error ? err.message : String(err))
    } finally {
      setLiveTestLoading(false)
    }
  }

  const sourceFields: SchemaField[] = buildSourceFields(allNodes, selectedNode.id)
  const targetKeys = data.nodeKind === 'process' ? Object.keys(data.input_mapping || {}) : []

  const cfg = (data.nodeKind === 'process' || data.nodeKind === 'trigger')
    ? (data.config as unknown as Record<string, unknown>) || {}
    : {}

  const needsSecretRef = data.nodeKind === 'process' && NODES_WITH_SECRET.includes(data.type as string)
  const currentScript = data.nodeKind === 'process' && (data.type as string) === 'script_ts'
    ? (data.script as string || '')
    : data.nodeKind === 'process' && (data.type as string) === 'code'
      ? ((data.config as unknown as Record<string, unknown>)?.script as string || '')
      : ''

  const inputClass = "w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400"
  const selectClass = "w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 bg-white"
  const labelClass = "block text-xs font-medium text-gray-600 mb-1"

  return (
    <>
      <aside className="w-72 bg-gray-50 border-l border-gray-200 flex flex-col">
        <div className="px-4 py-3 border-b border-gray-200 bg-white">
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Configuration</h2>
          <p className="text-xs text-gray-400 mt-0.5 truncate">{selectedNode.id}</p>
        </div>
        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          {/* Node ID */}
          <div>
            <label className={labelClass}>Node ID</label>
            <input type="text" value={data.id || ''} onChange={(e) => handleIdChange(e.target.value)} className={inputClass} />
          </div>

          {/* Secret Ref */}
          {needsSecretRef && (
            <div>
              <label className={labelClass}>Secret Ref</label>
              <select
                value={(data.secret_ref as string) || ''}
                onChange={(e) => updateNodeDataCentralized({ secret_ref: e.target.value || undefined })}
                className={selectClass}
              >
                <option value="">— none —</option>
                {availableSecrets.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} ({s.type})
                  </option>
                ))}
              </select>
              {availableSecrets.length === 0 && (
                <p className="text-xs text-gray-400 mt-1">No secrets found. Add them in the Secrets panel.</p>
              )}
            </div>
          )}

          {/* Description (process nodes only) */}
          {data.nodeKind === 'process' && (
            <div>
              <label className={labelClass}>Description</label>
              <input type="text" value={data.description as string || ''} onChange={(e) => handleDescriptionChange(e.target.value)} className={inputClass} />
            </div>
          )}

          {/* Trigger config */}
          {data.nodeKind === 'trigger' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">Trigger — {data.type as string}</label>

              {data.type === 'cron' && (
                <div>
                  <label className={labelClass}>Cron Expression</label>
                  <input type="text" value={(cfg.expression as string) || ''} onChange={(e) => handleTriggerConfigChange('expression', e.target.value)} className={inputClass} placeholder="0 * * * *" />
                </div>
              )}

              {data.type === 'rest' && (
                <>
                  <div>
                    <label className={labelClass}>Path</label>
                    <input type="text" value={(cfg.path as string) || ''} onChange={(e) => handleTriggerConfigChange('path', e.target.value)} className={inputClass} placeholder="/v1/flow" />
                  </div>
                  <div>
                    <label className={labelClass}>Method</label>
                    <select value={(cfg.method as string) || 'POST'} onChange={(e) => handleTriggerConfigChange('method', e.target.value)} className={selectClass}>
                      {['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((m) => <option key={m}>{m}</option>)}
                    </select>
                  </div>
                </>
              )}

              {data.type === 'soap' && (
                <div>
                  <label className={labelClass}>Path</label>
                  <input type="text" value={(cfg.path as string) || ''} onChange={(e) => handleTriggerConfigChange('path', e.target.value)} className={inputClass} placeholder="/ws/flow" />
                </div>
              )}

              {data.type === 'rabbitmq' && (
                <>
                  <div>
                    <label className={labelClass}>AMQP URL</label>
                    <input type="text" value={(cfg.url_amqp as string) || ''} onChange={(e) => handleTriggerConfigChange('url_amqp', e.target.value)} className={inputClass} placeholder="amqp://localhost" />
                  </div>
                  <div>
                    <label className={labelClass}>Queue</label>
                    <input type="text" value={(cfg.queue as string) || ''} onChange={(e) => handleTriggerConfigChange('queue', e.target.value)} className={inputClass} placeholder="flow-queue" />
                  </div>
                </>
              )}

              {data.type === 'mcp' && (
                <div>
                  <label className={labelClass}>Version</label>
                  <input type="text" value={(cfg.version as string) || ''} onChange={(e) => handleTriggerConfigChange('version', e.target.value)} className={inputClass} placeholder="1.0" />
                </div>
              )}
            </div>
          )}

          {/* HTTP node config */}
          {data.nodeKind === 'process' && data.type === 'http' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">HTTP Request</label>
              <div>
                <label className={labelClass}>URL</label>
                <input type="text" value={(cfg.url as string) || ''} onChange={(e) => handleHttpConfigChange('url', e.target.value)} className={inputClass} placeholder="https://api.example.com" />
              </div>
              <div>
                <label className={labelClass}>Method</label>
                <select value={(cfg.method as string) || 'GET'} onChange={(e) => handleHttpConfigChange('method', e.target.value)} className={selectClass}>
                  {['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((m) => <option key={m} value={m}>{m}</option>)}
                </select>
              </div>
              <div>
                <label className={labelClass}>Timeout (s)</label>
                <input type="number" min={0} value={(cfg.timeout as number) ?? 30} onChange={(e) => { const n = parseInt(e.target.value, 10); if (!isNaN(n) && n >= 0) handleHttpConfigChange('timeout', n) }} className={inputClass} />
              </div>
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className={labelClass}>Headers</label>
                  <button onClick={handleAddHeader} className="text-[10px] px-2 py-0.5 bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors">+ Add</button>
                </div>
                <div className="space-y-1">
                  {headerRows.map((row) => (
                    <div key={row.uid} className="flex gap-1 items-center">
                      <input type="text" value={row.key} onChange={(e) => handleHeaderChange(row.uid, 'key', e.target.value)} className="w-24 text-xs border border-gray-300 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-blue-400" placeholder="Key" />
                      <input type="text" value={row.value} onChange={(e) => handleHeaderChange(row.uid, 'value', e.target.value)} className="flex-1 text-xs border border-gray-300 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-blue-400" placeholder="Value" />
                      <button onClick={() => handleRemoveHeader(row.uid)} className="text-[10px] px-1.5 py-1 bg-red-100 text-red-600 rounded hover:bg-red-200 transition-colors" aria-label="Remove header">✕</button>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* SQL node config */}
          {data.nodeKind === 'process' && data.type === 'sql' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">SQL</label>
              <div>
                <label className={labelClass}>Engine</label>
                <select value={(cfg.engine as string) || 'postgres'} onChange={(e) => handleConfigFieldChange('engine', e.target.value)} className={selectClass}>
                  {['postgres', 'mysql', 'oracle'].map((e) => <option key={e}>{e}</option>)}
                </select>
              </div>
              <div>
                <label className={labelClass}>Host</label>
                <input type="text" value={(cfg.host as string) || ''} onChange={(e) => handleConfigFieldChange('host', e.target.value)} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>Database</label>
                <input type="text" value={(cfg.database as string) || ''} onChange={(e) => handleConfigFieldChange('database', e.target.value)} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>Query</label>
                <textarea rows={4} value={(cfg.query as string) || ''} onChange={(e) => handleConfigFieldChange('query', e.target.value)} className="w-full text-xs font-mono border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 resize-none" />
              </div>
            </div>
          )}

          {/* Log node config */}
          {data.nodeKind === 'process' && data.type === 'log' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">Log</label>
              <div>
                <label className={labelClass}>Level</label>
                <select value={(cfg.level as string) || 'INFO'} onChange={(e) => handleConfigFieldChange('level', e.target.value)} className={selectClass}>
                  {['ERROR', 'WARNING', 'INFO', 'DEBUG'].map((l) => <option key={l}>{l}</option>)}
                </select>
              </div>
              <div>
                <label className={labelClass}>Message</label>
                <input type="text" value={(cfg.message as string) || ''} onChange={(e) => handleConfigFieldChange('message', e.target.value)} className={inputClass} placeholder="Log message or $.trigger.body" />
              </div>
            </div>
          )}

          {/* Transform node config */}
          {data.nodeKind === 'process' && data.type === 'transform' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">Transform</label>
              <div>
                <label className={labelClass}>Transform Type</label>
                <select value={(cfg.transform_type as string) || 'json2csv'} onChange={(e) => handleConfigFieldChange('transform_type', e.target.value)} className={selectClass}>
                  {['json2csv', 'xml2json', 'json2xml'].map((t) => <option key={t}>{t}</option>)}
                </select>
              </div>
            </div>
          )}

          {/* File node config */}
          {data.nodeKind === 'process' && data.type === 'file' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">File</label>
              <div>
                <label className={labelClass}>Operation</label>
                <select value={(cfg.operation as string) || 'read'} onChange={(e) => handleConfigFieldChange('operation', e.target.value)} className={selectClass}>
                  {['create', 'delete', 'read'].map((o) => <option key={o}>{o}</option>)}
                </select>
              </div>
              <div>
                <label className={labelClass}>Path</label>
                <input type="text" value={(cfg.path as string) || ''} onChange={(e) => handleConfigFieldChange('path', e.target.value)} className={inputClass} placeholder="/tmp/file.txt" />
              </div>
              {cfg.operation === 'create' && (
                <div>
                  <label className={labelClass}>Content</label>
                  <textarea rows={4} value={(cfg.content as string) || ''} onChange={(e) => handleConfigFieldChange('content', e.target.value)} className="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 resize-none" />
                </div>
              )}
            </div>
          )}

          {/* SFTP node config */}
          {data.nodeKind === 'process' && data.type === 'sftp' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">SFTP</label>
              <div>
                <label className={labelClass}>Server</label>
                <input type="text" value={(cfg.server as string) || ''} onChange={(e) => handleConfigFieldChange('server', e.target.value)} className={inputClass} placeholder="sftp.example.com" />
              </div>
              <div>
                <label className={labelClass}>Port</label>
                <input type="number" min={1} max={65535} value={(cfg.port as number) ?? 22} onChange={(e) => { const n = parseInt(e.target.value, 10); if (!isNaN(n)) handleConfigFieldChange('port', n) }} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>Folder</label>
                <input type="text" value={(cfg.folder as string) || ''} onChange={(e) => handleConfigFieldChange('folder', e.target.value)} className={inputClass} placeholder="/files" />
              </div>
              <div>
                <label className={labelClass}>Method</label>
                <select value={(cfg.method as string) || 'get'} onChange={(e) => handleConfigFieldChange('method', e.target.value)} className={selectClass}>
                  {['get', 'put'].map((m) => <option key={m}>{m}</option>)}
                </select>
              </div>
              {cfg.method === 'get' && (
                <div>
                  <label className={labelClass}>Regex Filter</label>
                  <input type="text" value={(cfg.regex_filter as string) || ''} onChange={(e) => handleConfigFieldChange('regex_filter', e.target.value)} className={inputClass} placeholder=".*\.csv" />
                </div>
              )}
              {cfg.method === 'put' && (
                <>
                  <div className="flex items-center gap-2">
                    <input type="checkbox" id="sftp-overwrite" checked={(cfg.overwrite as boolean) ?? true} onChange={(e) => handleConfigFieldChange('overwrite', e.target.checked)} />
                    <label htmlFor="sftp-overwrite" className={labelClass + ' mb-0'}>Overwrite existing files</label>
                  </div>
                  <div className="flex items-center gap-2">
                    <input type="checkbox" id="sftp-create-folder" checked={(cfg.create_folder as boolean) ?? false} onChange={(e) => handleConfigFieldChange('create_folder', e.target.checked)} />
                    <label htmlFor="sftp-create-folder" className={labelClass + ' mb-0'}>Create folder if missing</label>
                  </div>
                </>
              )}
            </div>
          )}

          {/* S3 node config */}
          {data.nodeKind === 'process' && data.type === 's3' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">S3</label>
              <div>
                <label className={labelClass}>Bucket</label>
                <input type="text" value={(cfg.bucket as string) || ''} onChange={(e) => handleConfigFieldChange('bucket', e.target.value)} className={inputClass} placeholder="my-bucket" />
              </div>
              <div>
                <label className={labelClass}>Region</label>
                <input type="text" value={(cfg.region as string) || ''} onChange={(e) => handleConfigFieldChange('region', e.target.value)} className={inputClass} placeholder="us-east-1" />
              </div>
              <div>
                <label className={labelClass}>Folder / Prefix</label>
                <input type="text" value={(cfg.folder as string) || ''} onChange={(e) => handleConfigFieldChange('folder', e.target.value)} className={inputClass} placeholder="/" />
              </div>
              <div>
                <label className={labelClass}>Method</label>
                <select value={(cfg.method as string) || 'get'} onChange={(e) => handleConfigFieldChange('method', e.target.value)} className={selectClass}>
                  {['get', 'put'].map((m) => <option key={m}>{m}</option>)}
                </select>
              </div>
              {cfg.method === 'get' && (
                <div>
                  <label className={labelClass}>Regex Filter</label>
                  <input type="text" value={(cfg.regex_filter as string) || ''} onChange={(e) => handleConfigFieldChange('regex_filter', e.target.value)} className={inputClass} placeholder=".*\.json" />
                </div>
              )}
              {cfg.method === 'put' && (
                <div className="flex items-center gap-2">
                  <input type="checkbox" id="s3-overwrite" checked={(cfg.overwrite as boolean) ?? true} onChange={(e) => handleConfigFieldChange('overwrite', e.target.checked)} />
                  <label htmlFor="s3-overwrite" className={labelClass + ' mb-0'}>Overwrite existing objects</label>
                </div>
              )}
            </div>
          )}

          {/* SMB node config */}
          {data.nodeKind === 'process' && data.type === 'smb' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">SMB</label>
              <div>
                <label className={labelClass}>Server</label>
                <input type="text" value={(cfg.server as string) || ''} onChange={(e) => handleConfigFieldChange('server', e.target.value)} className={inputClass} placeholder="fileserver" />
              </div>
              <div>
                <label className={labelClass}>Port</label>
                <input type="number" min={1} max={65535} value={(cfg.port as number) ?? 445} onChange={(e) => { const n = parseInt(e.target.value, 10); if (!isNaN(n)) handleConfigFieldChange('port', n) }} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>Share</label>
                <input type="text" value={(cfg.share as string) || ''} onChange={(e) => handleConfigFieldChange('share', e.target.value)} className={inputClass} placeholder="shared" />
              </div>
              <div>
                <label className={labelClass}>Folder</label>
                <input type="text" value={(cfg.folder as string) || ''} onChange={(e) => handleConfigFieldChange('folder', e.target.value)} className={inputClass} placeholder="/files" />
              </div>
              <div>
                <label className={labelClass}>Method</label>
                <select value={(cfg.method as string) || 'get'} onChange={(e) => handleConfigFieldChange('method', e.target.value)} className={selectClass}>
                  {['get', 'put'].map((m) => <option key={m}>{m}</option>)}
                </select>
              </div>
              {cfg.method === 'get' && (
                <div>
                  <label className={labelClass}>Regex Filter</label>
                  <input type="text" value={(cfg.regex_filter as string) || ''} onChange={(e) => handleConfigFieldChange('regex_filter', e.target.value)} className={inputClass} placeholder=".*\.xml" />
                </div>
              )}
              {cfg.method === 'put' && (
                <div className="flex items-center gap-2">
                  <input type="checkbox" id="smb-overwrite" checked={(cfg.overwrite as boolean) ?? true} onChange={(e) => handleConfigFieldChange('overwrite', e.target.checked)} />
                  <label htmlFor="smb-overwrite" className={labelClass + ' mb-0'}>Overwrite existing files</label>
                </div>
              )}
            </div>
          )}

          {/* Mail node config */}
          {data.nodeKind === 'process' && data.type === 'mail' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">Mail</label>
              <div>
                <label className={labelClass}>Host</label>
                <input type="text" value={(cfg.host as string) || ''} onChange={(e) => handleConfigFieldChange('host', e.target.value)} className={inputClass} placeholder="smtp.example.com" />
              </div>
              <div>
                <label className={labelClass}>Port</label>
                <input type="number" value={(cfg.port as number) ?? 587} onChange={(e) => handleConfigFieldChange('port', parseInt(e.target.value, 10))} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>Action</label>
                <select value={(cfg.action as string) || 'send'} onChange={(e) => handleConfigFieldChange('action', e.target.value)} className={selectClass}>
                  {['send', 'receive'].map((a) => <option key={a}>{a}</option>)}
                </select>
              </div>
              {cfg.action === 'send' && (
                <>
                  <div>
                    <label className={labelClass}>To (comma-separated)</label>
                    <textarea rows={2} value={Array.isArray(cfg.to) ? (cfg.to as string[]).join(', ') : (cfg.to as string) || ''} onChange={(e) => handleConfigFieldChange('to', e.target.value.split(',').map((s) => s.trim()))} className="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 resize-none" placeholder="user@example.com" />
                  </div>
                  <div>
                    <label className={labelClass}>Subject</label>
                    <input type="text" value={(cfg.subject as string) || ''} onChange={(e) => handleConfigFieldChange('subject', e.target.value)} className={inputClass} />
                  </div>
                  <div>
                    <label className={labelClass}>Body</label>
                    <textarea rows={3} value={(cfg.body as string) || ''} onChange={(e) => handleConfigFieldChange('body', e.target.value)} className="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 resize-none" />
                  </div>
                </>
              )}
            </div>
          )}

          {/* RabbitMQ activity node config */}
          {data.nodeKind === 'process' && data.type === 'rabbitmq' && (
            <div className="space-y-3">
              <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider">RabbitMQ</label>
              <div>
                <label className={labelClass}>AMQP URL</label>
                <input type="text" value={(cfg.url_amqp as string) || ''} onChange={(e) => handleConfigFieldChange('url_amqp', e.target.value)} className={inputClass} placeholder="amqp://localhost" />
              </div>
              <div>
                <label className={labelClass}>Exchange</label>
                <input type="text" value={(cfg.exchange as string) || ''} onChange={(e) => handleConfigFieldChange('exchange', e.target.value)} className={inputClass} placeholder="" />
              </div>
              <div>
                <label className={labelClass}>Routing Key</label>
                <input type="text" value={(cfg.routing_key as string) || ''} onChange={(e) => handleConfigFieldChange('routing_key', e.target.value)} className={inputClass} placeholder="flow.event" />
              </div>
            </div>
          )}

          {/* Code / script_ts node config */}
          {data.nodeKind === 'process' && ((data.type as string) === 'code' || (data.type as string) === 'script_ts') && (
            <div>
              <div className="flex items-center justify-between mb-1">
                <label className={labelClass}>Script</label>
                <button onClick={() => setShowMonaco(true)} className="text-[10px] px-2 py-0.5 bg-gray-700 text-white rounded hover:bg-gray-900 transition-colors" title="Open in Monaco editor">⎆ Editor</button>
              </div>
              <textarea
                value={currentScript}
                onChange={(e) => handleScriptChange(e.target.value)}
                rows={6}
                className="w-full text-xs font-mono border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 resize-none"
                placeholder="export default (input) => { return input; }"
              />
            </div>
          )}

          {/* Input mapping (process nodes) */}
          {data.nodeKind === 'process' && (
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className={labelClass}>Input Mapping</label>
                <button onClick={() => setShowMapper(true)} className="text-[10px] px-2 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors" title="Open visual data mapper">⇄ Visual Mapper</button>
              </div>
              {data.input_mapping && (
                <div className="space-y-2">
                  {Object.entries(data.input_mapping).map(([key, value]) => (
                    <div key={key} className="flex gap-1 items-center">
                      <span className="text-xs text-gray-500 w-20 truncate">{key}</span>
                      <input type="text" value={value} onChange={(e) => handleInputMappingChange(key, e.target.value)} className="flex-1 text-xs border border-gray-300 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-blue-400" placeholder="$.path.to.value" />
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Live Test (process nodes) */}
          {data.nodeKind === 'process' && (
            <div className="border border-gray-200 rounded p-3 space-y-2 bg-white">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-gray-600">Live Test</span>
                <button onClick={handleLiveTest} disabled={liveTestLoading} className="text-[10px] px-2 py-1 bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50 transition-colors">
                  {liveTestLoading ? '⏳ Running…' : '▶ Run'}
                </button>
              </div>
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">Input Payload (JSON)</label>
                <textarea value={liveTestInput} onChange={(e) => setLiveTestInput(e.target.value)} rows={4} className="w-full text-[10px] font-mono border border-gray-200 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-green-400 resize-none" />
              </div>
              {liveTestResult && (
                <div>
                  <label className="block text-[10px] text-gray-500 mb-1">Output</label>
                  <pre className="text-[10px] font-mono bg-gray-900 text-green-400 rounded p-2 overflow-auto max-h-40">{liveTestResult}</pre>
                </div>
              )}
              {liveTestError && (
                <div className="text-[10px] text-red-600 font-mono bg-red-50 rounded p-2 overflow-auto max-h-24">{liveTestError}</div>
              )}
            </div>
          )}
        </div>
      </aside>

      {showMapper && (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/50">
          <div className="bg-white rounded-lg shadow-2xl flex flex-col" style={{ width: '85vw', height: '80vh' }} onClick={(e) => e.stopPropagation()}>
            <DataMapper sourceFields={sourceFields} targetKeys={targetKeys} currentMapping={data.nodeKind === 'process' ? (data.input_mapping || {}) : {}} onSave={handleMapperSave} onClose={() => setShowMapper(false)} />
          </div>
        </div>
      )}

      {showMonaco && data.nodeKind === 'process' && ((data.type as string) === 'script_ts' || data.type === 'code') && (
        <MonacoModal value={currentScript} onSave={handleMonacoSave} onClose={() => setShowMonaco(false)} />
      )}
    </>
  )
}
