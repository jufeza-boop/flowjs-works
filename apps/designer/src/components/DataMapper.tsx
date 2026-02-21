import { useState, useCallback, useRef, useEffect } from 'react'
import type { SchemaField, MappingConnection } from '../types/mapper'
import type { InputMapping } from '../types/dsl'
import { buildInputMapping } from '../lib/mapper'

interface FieldNodeProps {
  field: SchemaField
  depth: number
  onSelect: (field: SchemaField) => void
  isSelected: boolean
}

/** Renders a single field row in the source or target tree */
function FieldNode({ field, depth, onSelect, isSelected }: FieldNodeProps) {
  const [expanded, setExpanded] = useState(depth === 0)
  const hasChildren = field.children && field.children.length > 0

  const typeColor: Record<SchemaField['type'], string> = {
    string: 'text-green-600',
    number: 'text-blue-600',
    boolean: 'text-purple-600',
    object: 'text-orange-600',
    array: 'text-yellow-600',
    unknown: 'text-gray-400',
  }

  return (
    <div>
      <div
        className={`flex items-center gap-1 px-2 py-0.5 rounded cursor-pointer text-xs hover:bg-gray-100 ${
          isSelected ? 'bg-blue-50 ring-1 ring-blue-400' : ''
        }`}
        style={{ paddingLeft: `${8 + depth * 12}px` }}
        onClick={() => onSelect(field)}
      >
        {hasChildren ? (
          <button
            className="text-gray-400 w-3 text-center"
            onClick={(e) => { e.stopPropagation(); setExpanded((v) => !v) }}
          >
            {expanded ? '▾' : '▸'}
          </button>
        ) : (
          <span className="w-3" />
        )}
        <span className="text-gray-700 font-mono">{field.key}</span>
        <span className={`ml-1 text-[10px] ${typeColor[field.type]}`}>{field.type}</span>
      </div>
      {expanded && hasChildren && field.children!.map((child) => (
        <FieldNode
          key={child.path}
          field={child}
          depth={depth + 1}
          onSelect={onSelect}
          isSelected={isSelected && false /* parent selected, not child */}
        />
      ))}
    </div>
  )
}

interface ConnectionLineProps {
  sourceY: number
  targetY: number
  containerWidth: number
}

/** SVG arrow connecting a source field row to a target field row */
function ConnectionLine({ sourceY, targetY, containerWidth }: ConnectionLineProps) {
  const midX = containerWidth / 2
  const d = `M ${midX - 2} ${sourceY} C ${midX + 20} ${sourceY}, ${midX - 20} ${targetY}, ${midX + 2} ${targetY}`
  return (
    <path
      d={d}
      stroke="#3b82f6"
      strokeWidth={1.5}
      fill="none"
      markerEnd="url(#arrow)"
    />
  )
}

interface DataMapperProps {
  /** All available source fields (from previous nodes) */
  sourceFields: SchemaField[]
  /** Target keys defined in the current node's input_mapping */
  targetKeys: string[]
  /** Current input_mapping to pre-populate connections */
  currentMapping: InputMapping
  onSave: (mapping: InputMapping) => void
  onClose: () => void
}

/**
 * Visual Data Mapper — two-column layout with SVG connection lines.
 * Left column: source field tree; Right column: target key list.
 * Users click a source field then a target key to create a mapping connection.
 */
export function DataMapper({ sourceFields, targetKeys, currentMapping, onSave, onClose }: DataMapperProps) {
  // Build initial connections from existing mapping
  const resolveField = (path: string, fields: SchemaField[]): SchemaField | undefined => {
    for (const f of fields) {
      if (f.path === path) return f
      if (f.children) {
        const found = resolveField(path, f.children)
        if (found) return found
      }
    }
    return undefined
  }

  const initialConnections: MappingConnection[] = Object.entries(currentMapping)
    .map(([targetKey, sourcePath]) => {
      const sourceField = resolveField(sourcePath, sourceFields)
      if (!sourceField) return null
      return { sourceField, targetKey } satisfies MappingConnection
    })
    .filter((c): c is MappingConnection => c !== null)

  const [connections, setConnections] = useState<MappingConnection[]>(initialConnections)
  const [pendingSource, setPendingSource] = useState<SchemaField | null>(null)
  const [newTargetKey, setNewTargetKey] = useState('')
  const svgRef = useRef<SVGSVGElement>(null)

  // Effective target keys = predefined ones + any keys already in connections
  const allTargetKeys = Array.from(
    new Set([...targetKeys, ...connections.map((c) => c.targetKey)]),
  )

  const handleSourceSelect = useCallback((field: SchemaField) => {
    setPendingSource((prev) => (prev?.path === field.path ? null : field))
  }, [])

  const handleTargetSelect = useCallback((targetKey: string) => {
    if (!pendingSource) return
    setConnections((prev) => {
      // Replace existing connection for this target key
      const filtered = prev.filter((c) => c.targetKey !== targetKey)
      return [...filtered, { sourceField: pendingSource, targetKey }]
    })
    setPendingSource(null)
  }, [pendingSource])

  const handleRemoveConnection = useCallback((targetKey: string) => {
    setConnections((prev) => prev.filter((c) => c.targetKey !== targetKey))
  }, [])

  const handleAddTargetKey = () => {
    const key = newTargetKey.trim()
    if (!key) return
    setNewTargetKey('')
    if (pendingSource) {
      setConnections((prev) => {
        const filtered = prev.filter((c) => c.targetKey !== key)
        return [...filtered, { sourceField: pendingSource, targetKey: key }]
      })
      setPendingSource(null)
    }
  }

  const handleSave = () => {
    onSave(buildInputMapping(connections))
  }

  // Row heights for SVG lines (approximate based on rendered DOM)
  const [rowHeights, setRowHeights] = useState<Record<string, number>>({})
  const sourceRowRefs = useRef<Map<string, HTMLDivElement>>(new Map())
  const targetRowRefs = useRef<Map<string, HTMLDivElement>>(new Map())
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!containerRef.current) return
    const containerTop = containerRef.current.getBoundingClientRect().top
    const heights: Record<string, number> = {}
    sourceRowRefs.current.forEach((el, path) => {
      const rect = el.getBoundingClientRect()
      heights[`src:${path}`] = rect.top - containerTop + rect.height / 2
    })
    targetRowRefs.current.forEach((el, key) => {
      const rect = el.getBoundingClientRect()
      heights[`tgt:${key}`] = rect.top - containerTop + rect.height / 2
    })
    setRowHeights(heights)
  }, [connections, allTargetKeys])

  const svgWidth = containerRef.current?.clientWidth ?? 600

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-gray-200 bg-gray-50">
        <h2 className="text-sm font-semibold text-gray-700">Visual Data Mapper</h2>
        <div className="flex gap-2">
          <button
            onClick={handleSave}
            className="text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
          >
            Save Mapping
          </button>
          <button
            onClick={onClose}
            className="text-xs px-3 py-1.5 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 transition-colors"
          >
            Close
          </button>
        </div>
      </div>

      {pendingSource && (
        <div className="px-4 py-1.5 bg-blue-50 border-b border-blue-200 text-xs text-blue-700">
          Selected: <span className="font-mono font-semibold">{pendingSource.path}</span> — now click a target key
        </div>
      )}

      {/* Three-pane layout: Source | SVG Bridge | Target */}
      <div className="flex flex-1 overflow-hidden relative" ref={containerRef}>
        {/* Source Column */}
        <div className="w-5/12 border-r border-gray-200 overflow-y-auto bg-white">
          <div className="px-3 py-2 border-b border-gray-100 bg-gray-50">
            <span className="text-[11px] font-semibold text-gray-500 uppercase tracking-wider">Source</span>
          </div>
          {sourceFields.map((field) => (
            <div
              key={field.path}
              ref={(el) => { if (el) sourceRowRefs.current.set(field.path, el) }}
            >
              <FieldNode
                field={field}
                depth={0}
                onSelect={handleSourceSelect}
                isSelected={pendingSource?.path === field.path}
              />
            </div>
          ))}
        </div>

        {/* SVG Bridge (connection lines) */}
        <svg
          ref={svgRef}
          className="absolute inset-0 pointer-events-none overflow-visible"
          width={svgWidth}
          height="100%"
        >
          <defs>
            <marker id="arrow" markerWidth="6" markerHeight="6" refX="6" refY="3" orient="auto">
              <path d="M0,0 L0,6 L6,3 z" fill="#3b82f6" />
            </marker>
          </defs>
          {connections.map((conn) => {
            const srcY = rowHeights[`src:${conn.sourceField.path}`]
            const tgtY = rowHeights[`tgt:${conn.targetKey}`]
            if (srcY === undefined || tgtY === undefined) return null
            return (
              <ConnectionLine
                key={`${conn.sourceField.path}->${conn.targetKey}`}
                sourceY={srcY}
                targetY={tgtY}
                containerWidth={svgWidth}
              />
            )
          })}
        </svg>

        {/* Target Column */}
        <div className="w-7/12 overflow-y-auto bg-white">
          <div className="px-3 py-2 border-b border-gray-100 bg-gray-50">
            <span className="text-[11px] font-semibold text-gray-500 uppercase tracking-wider">Target (Input Mapping Keys)</span>
          </div>

          <div className="p-2 space-y-1">
            {allTargetKeys.map((key) => {
              const conn = connections.find((c) => c.targetKey === key)
              return (
                <div
                  key={key}
                  ref={(el) => { if (el) targetRowRefs.current.set(key, el) }}
                  className={`flex items-center justify-between px-2 py-1 rounded text-xs cursor-pointer border ${
                    conn ? 'border-blue-300 bg-blue-50' : 'border-gray-200 hover:bg-gray-50'
                  }`}
                  onClick={() => handleTargetSelect(key)}
                >
                  <div>
                    <span className="font-mono text-gray-800">{key}</span>
                    {conn && (
                      <span className="ml-2 text-[10px] text-blue-500 font-mono truncate max-w-[160px] inline-block align-middle">
                        ← {conn.sourceField.path}
                      </span>
                    )}
                  </div>
                  {conn && (
                    <button
                      className="text-gray-400 hover:text-red-500 ml-2"
                      onClick={(e) => { e.stopPropagation(); handleRemoveConnection(key) }}
                      title="Remove connection"
                    >
                      ✕
                    </button>
                  )}
                </div>
              )
            })}

            {/* Add new target key */}
            <div className="flex gap-1 mt-2">
              <input
                type="text"
                value={newTargetKey}
                onChange={(e) => setNewTargetKey(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') handleAddTargetKey() }}
                placeholder="new_key"
                className="flex-1 text-xs border border-dashed border-gray-300 rounded px-2 py-1 focus:outline-none focus:border-blue-400 font-mono"
              />
              <button
                onClick={handleAddTargetKey}
                className="text-xs px-2 py-1 bg-gray-100 hover:bg-gray-200 text-gray-600 rounded border border-gray-300"
              >
                + Add
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
