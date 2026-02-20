import type { DragEvent } from 'react'
import type { PaletteItem, NodeTypeKey } from '../types/designer'

const PALETTE_ITEMS: PaletteItem[] = [
  {
    type: 'trigger',
    label: 'HTTP Trigger',
    description: 'Webhook entry point',
    icon: '⚡',
    color: 'bg-green-500',
  },
  {
    type: 'script_ts',
    label: 'Script TS',
    description: 'TypeScript transformation',
    icon: '📜',
    color: 'bg-purple-500',
  },
  {
    type: 'http_request',
    label: 'HTTP Request',
    description: 'External HTTP call',
    icon: '🌐',
    color: 'bg-blue-400',
  },
  {
    type: 'sql_insert',
    label: 'SQL Insert',
    description: 'Database insert operation',
    icon: '🗄️',
    color: 'bg-orange-500',
  },
]

/** Drag-and-drop source panel listing available node types */
export function NodePalette() {
  const onDragStart = (event: DragEvent<HTMLDivElement>, nodeType: NodeTypeKey) => {
    event.dataTransfer.setData('application/reactflow', nodeType)
    event.dataTransfer.effectAllowed = 'move'
  }

  return (
    <aside className="w-56 bg-gray-50 border-r border-gray-200 flex flex-col">
      <div className="px-4 py-3 border-b border-gray-200 bg-white">
        <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
          Node Palette
        </h2>
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {PALETTE_ITEMS.map((item) => (
          <div
            key={item.type}
            draggable
            onDragStart={(e) => onDragStart(e, item.type)}
            className="rounded-lg border border-gray-200 bg-white shadow-sm cursor-grab active:cursor-grabbing hover:shadow-md transition-shadow"
          >
            <div className={`flex items-center gap-2 ${item.color} px-3 py-1.5 rounded-t-md`}>
              <span className="text-white text-sm">{item.icon}</span>
              <span className="text-white text-xs font-semibold">{item.label}</span>
            </div>
            <p className="px-3 py-1.5 text-xs text-gray-500">{item.description}</p>
          </div>
        ))}
      </div>
    </aside>
  )
}
