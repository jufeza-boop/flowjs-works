import type { DesignerNode } from '../types/designer'

interface ConfigPanelProps {
  selectedNode: DesignerNode | null
  onNodeUpdate: (nodeId: string, updates: Partial<DesignerNode['data']>) => void
}

/** Right panel showing configuration for the selected node */
export function ConfigPanel({ selectedNode, onNodeUpdate }: ConfigPanelProps) {
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

  return (
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
            <label className="block text-xs font-medium text-gray-600 mb-1">Script</label>
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
        {data.nodeKind === 'process' && data.input_mapping && (
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-2">Input Mapping</label>
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
  )
}
