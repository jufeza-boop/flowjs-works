import { useState } from 'react'
import type { DesignerNode } from '../types/designer'
import type { SchemaField } from '../types/mapper'
import type { InputMapping } from '../types/dsl'
import { buildSourceFields } from '../lib/mapper'
import { liveTest } from '../lib/api'
import { DataMapper } from './DataMapper'
import { MonacoModal } from './MonacoModal'

interface ConfigPanelProps {
  selectedNode: DesignerNode | null
  onNodeUpdate: (nodeId: string, updates: Partial<DesignerNode['data']>) => void
  /** All nodes in the canvas — used to build the source field tree */
  allNodes?: DesignerNode[]
}

/** Right panel showing configuration for the selected node */
export function ConfigPanel({ selectedNode, onNodeUpdate, allNodes = [] }: ConfigPanelProps) {
  const [showMapper, setShowMapper] = useState(false)
  const [showMonaco, setShowMonaco] = useState(false)
  const [liveTestInput, setLiveTestInput] = useState('{\n  "body": {}\n}')
  const [liveTestResult, setLiveTestResult] = useState<string | null>(null)
  const [liveTestError, setLiveTestError] = useState<string | null>(null)
  const [liveTestLoading, setLiveTestLoading] = useState(false)

  if (!selectedNode) {
    return (
      <aside className="w-72 bg-gray-50 border-l border-gray-200 flex flex-col">
        <div className="px-4 py-3 border-b border-gray-200 bg-white">
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
            Configuration
          </h2>
        </div>
        <div className="flex-1 flex items-center justify-center p-4">
          <p className="text-sm text-gray-400 text-center">
            Select a node on the canvas to configure it
          </p>
        </div>
      </aside>
    )
  }

  const data = selectedNode.data

  const handleIdChange = (value: string) => {
    onNodeUpdate(selectedNode.id, { ...data, id: value })
  }

  const handleDescriptionChange = (value: string) => {
    if (data.nodeKind === 'process') {
      onNodeUpdate(selectedNode.id, { ...data, description: value })
    }
  }

  const handleScriptChange = (value: string) => {
    if (data.nodeKind === 'process' && data.type === 'script_ts') {
      onNodeUpdate(selectedNode.id, { ...data, script: value })
    }
  }

  const handleInputMappingChange = (key: string, value: string) => {
    if (data.nodeKind === 'process') {
      onNodeUpdate(selectedNode.id, {
        ...data,
        input_mapping: { ...(data.input_mapping ?? {}), [key]: value },
      })
    }
  }

  const handleMapperSave = (mapping: InputMapping) => {
    if (data.nodeKind === 'process') {
      onNodeUpdate(selectedNode.id, { ...data, input_mapping: mapping })
    }
    setShowMapper(false)
  }

  const handleMonacoSave = (script: string) => {
    handleScriptChange(script)
    setShowMonaco(false)
  }

  const handleLiveTest = async () => {
    if (data.nodeKind !== 'process') return
    setLiveTestLoading(true)
    setLiveTestResult(null)
    setLiveTestError(null)
    try {
      const parsedInput = JSON.parse(liveTestInput) as Record<string, unknown>
      const result = await liveTest({
        input_mapping: data.input_mapping ?? {},
        script: data.type === 'script_ts' ? (data.script ?? '') : undefined,
        input_payload: parsedInput,
      })
      setLiveTestResult(JSON.stringify(result.output, null, 2))
    } catch (err) {
      setLiveTestError(err instanceof Error ? err.message : String(err))
    } finally {
      setLiveTestLoading(false)
    }
  }

  // Build source fields for the mapper
  const sourceFields: SchemaField[] = buildSourceFields(allNodes, selectedNode.id)
  const targetKeys = data.nodeKind === 'process' ? Object.keys(data.input_mapping ?? {}) : []

  return (
    <>
      <aside className="w-72 bg-gray-50 border-l border-gray-200 flex flex-col">
        <div className="px-4 py-3 border-b border-gray-200 bg-white">
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
            Configuration
          </h2>
          <p className="text-xs text-gray-400 mt-0.5 truncate">{selectedNode.id}</p>
        </div>
        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          {/* Node ID */}
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Node ID</label>
            <input
              type="text"
              value={data.id}
              onChange={(e) => handleIdChange(e.target.value)}
              className="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400"
            />
          </div>

          {/* Description (process nodes only) */}
          {data.nodeKind === 'process' && (
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">Description</label>
              <input
                type="text"
                value={data.description ?? ''}
                onChange={(e) => handleDescriptionChange(e.target.value)}
                className="w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400"
              />
            </div>
          )}

          {/* Script editor (script_ts nodes) */}
          {data.nodeKind === 'process' && data.type === 'script_ts' && (
            <div>
              <div className="flex items-center justify-between mb-1">
                <label className="block text-xs font-medium text-gray-600">Script</label>
                <button
                  onClick={() => setShowMonaco(true)}
                  className="text-[10px] px-2 py-0.5 bg-gray-700 text-white rounded hover:bg-gray-900 transition-colors"
                  aria-label="Open script in Monaco editor"
                  title="Open in Monaco editor"
                >
                  ⎆ Editor
                </button>
              </div>
              <textarea
                value={data.script ?? ''}
                onChange={(e) => handleScriptChange(e.target.value)}
                rows={6}
                className="w-full text-xs font-mono border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 resize-none"
                placeholder="export default (input) => { return input; }"
              />
            </div>
          )}

          {/* Input mapping */}
          {data.nodeKind === 'process' && (
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-xs font-medium text-gray-600">Input Mapping</label>
                <button
                  onClick={() => setShowMapper(true)}
                  className="text-[10px] px-2 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
                  aria-label="Open visual data mapper"
                  title="Open visual data mapper"
                >
                  ⇄ Visual Mapper
                </button>
              </div>
              {data.input_mapping && (
                <div className="space-y-2">
                  {Object.entries(data.input_mapping).map(([key, value]) => (
                    <div key={key} className="flex gap-1 items-center">
                      <span className="text-xs text-gray-500 w-20 truncate">{key}</span>
                      <input
                        type="text"
                        value={value}
                        onChange={(e) => handleInputMappingChange(key, e.target.value)}
                        className="flex-1 text-xs border border-gray-300 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-blue-400"
                        placeholder="$.path.to.value"
                      />
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Live Test panel (process nodes only) */}
          {data.nodeKind === 'process' && (
            <div className="border border-gray-200 rounded p-3 space-y-2 bg-white">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-gray-600">Live Test</span>
                <button
                  onClick={handleLiveTest}
                  disabled={liveTestLoading}
                  className="text-[10px] px-2 py-1 bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50 transition-colors"
                >
                  {liveTestLoading ? '⏳ Running…' : '▶ Run'}
                </button>
              </div>
              <div>
                <label className="block text-[10px] text-gray-500 mb-1">Input Payload (JSON)</label>
                <textarea
                  value={liveTestInput}
                  onChange={(e) => setLiveTestInput(e.target.value)}
                  rows={4}
                  className="w-full text-[10px] font-mono border border-gray-200 rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-green-400 resize-none"
                />
              </div>
              {liveTestResult && (
                <div>
                  <label className="block text-[10px] text-gray-500 mb-1">Output</label>
                  <pre className="text-[10px] font-mono bg-gray-900 text-green-400 rounded p-2 overflow-auto max-h-40">
                    {liveTestResult}
                  </pre>
                </div>
              )}
              {liveTestError && (
                <div className="text-[10px] text-red-600 font-mono bg-red-50 rounded p-2 overflow-auto max-h-24">
                  {liveTestError}
                </div>
              )}
            </div>
          )}

          {/* Trigger config */}
          {data.nodeKind === 'trigger' && (
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">Type</label>
              <p className="text-xs text-gray-700">{data.type}</p>
              {data.config?.path && (
                <>
                  <label className="block text-xs font-medium text-gray-600 mt-3 mb-1">Path</label>
                  <p className="text-xs text-gray-700 font-mono">{data.config.path}</p>
                </>
              )}
            </div>
          )}
        </div>
      </aside>

      {/* Visual Data Mapper modal */}
      {showMapper && (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/50">
          <div
            className="bg-white rounded-lg shadow-2xl flex flex-col"
            style={{ width: '85vw', height: '80vh' }}
            onClick={(e) => e.stopPropagation()}
          >
            <DataMapper
              sourceFields={sourceFields}
              targetKeys={targetKeys}
              currentMapping={data.nodeKind === 'process' ? (data.input_mapping ?? {}) : {}}
              onSave={handleMapperSave}
              onClose={() => setShowMapper(false)}
            />
          </div>
        </div>
      )}

      {/* Monaco Editor modal (script_ts nodes) */}
      {showMonaco && data.nodeKind === 'process' && data.type === 'script_ts' && (
        <MonacoModal
          value={data.script ?? ''}
          onSave={handleMonacoSave}
          onClose={() => setShowMonaco(false)}
        />
      )}
    </>
  )
}

