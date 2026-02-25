import { useState, type DragEvent } from 'react'
import type { PaletteCategory, NodeTypeKey } from '../types/designer'

const PALETTE_CATEGORIES: PaletteCategory[] = [
  {
    label: 'Triggers',
    items: [
      { type: 'trg_cron',     label: 'Cron',     description: 'Scheduled trigger',       icon: '⏰', color: 'bg-green-600' },
      { type: 'trg_rest',     label: 'REST',     description: 'HTTP webhook',             icon: '🌐', color: 'bg-green-500' },
      { type: 'trg_soap',     label: 'SOAP',     description: 'SOAP endpoint',            icon: '📡', color: 'bg-green-400' },
      { type: 'trg_rabbitmq', label: 'RabbitMQ', description: 'Queue consumer',           icon: '🐇', color: 'bg-green-500' },
      { type: 'trg_mcp',      label: 'MCP',      description: 'Model Context Protocol',   icon: '🤖', color: 'bg-emerald-500' },
      { type: 'trg_manual',   label: 'Manual',   description: 'Manual trigger',           icon: '👆', color: 'bg-teal-500' },
    ],
  },
  {
    label: 'Activities',
    items: [
      { type: 'http',      label: 'HTTP',      description: 'External HTTP call',    icon: '🌐', color: 'bg-blue-400' },
      { type: 'sftp',      label: 'SFTP',      description: 'SFTP file transfer',    icon: '📂', color: 'bg-teal-500' },
      { type: 's3',        label: 'S3',        description: 'AWS S3 storage',        icon: '☁️', color: 'bg-yellow-500' },
      { type: 'smb',       label: 'SMB',       description: 'SMB file share',        icon: '🗂️', color: 'bg-gray-500' },
      { type: 'mail',      label: 'Mail',      description: 'Send/receive email',    icon: '📧', color: 'bg-red-400' },
      { type: 'rabbitmq',  label: 'RabbitMQ',  description: 'Message producer',      icon: '🐇', color: 'bg-orange-400' },
      { type: 'sql',       label: 'SQL',       description: 'Database query',        icon: '🗄️', color: 'bg-orange-500' },
      { type: 'code',      label: 'Code',      description: 'JS/TS script',          icon: '📜', color: 'bg-purple-500' },
      { type: 'log',       label: 'Log',       description: 'Log a message',         icon: '📋', color: 'bg-gray-400' },
      { type: 'transform', label: 'Transform', description: 'Data transformation',   icon: '🔄', color: 'bg-indigo-500' },
      { type: 'file',      label: 'File',      description: 'Local file operation',  icon: '📄', color: 'bg-lime-500' },
    ],
  },
]

/** Drag-and-drop source panel listing available node types, grouped by category */
export function NodePalette() {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})

  const onDragStart = (event: DragEvent<HTMLDivElement>, nodeType: NodeTypeKey) => {
    event.dataTransfer.setData('application/reactflow', nodeType)
    event.dataTransfer.effectAllowed = 'move'
  }

  const toggleCategory = (label: string) => {
    setCollapsed((prev) => ({ ...prev, [label]: !prev[label] }))
  }

  return (
    <aside className="w-56 bg-gray-50 border-r border-gray-200 flex flex-col">
      <div className="px-4 py-3 border-b border-gray-200 bg-white">
        <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Node Palette</h2>
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {PALETTE_CATEGORIES.map((category) => (
          <div key={category.label}>
            <button
              onClick={() => toggleCategory(category.label)}
              className="w-full flex items-center justify-between text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2 hover:text-gray-700 transition-colors"
            >
              <span>{category.label}</span>
              <span>{collapsed[category.label] ? '▶' : '▼'}</span>
            </button>
            {!collapsed[category.label] && (
              <div className="space-y-2">
                {category.items.map((item) => (
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
            )}
          </div>
        ))}
      </div>
    </aside>
  )
}
