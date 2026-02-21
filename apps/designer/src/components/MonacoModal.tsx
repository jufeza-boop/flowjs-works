import { useRef } from 'react'
import Editor from '@monaco-editor/react'

interface MonacoModalProps {
  /** Initial script content */
  value: string
  /** Called when the user clicks Save */
  onSave: (value: string) => void
  onClose: () => void
}

/**
 * Full-screen modal wrapping the Monaco Editor for TypeScript/JavaScript script editing.
 * The edited code is persisted to the node's `script` property in the DSL.
 */
export function MonacoModal({ value, onSave, onClose }: MonacoModalProps) {
  const currentValueRef = useRef<string>(value)

  const handleEditorChange = (val: string | undefined) => {
    currentValueRef.current = val ?? ''
  }

  const handleSave = () => {
    onSave(currentValueRef.current)
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-lg shadow-2xl flex flex-col"
        style={{ width: '80vw', height: '75vh' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-2.5 border-b border-gray-200 bg-gray-50 rounded-t-lg">
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold text-gray-700">Script Editor</span>
            <span className="text-xs text-gray-400 bg-gray-200 px-1.5 py-0.5 rounded font-mono">TypeScript</span>
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleSave}
              className="text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
            >
              Save Script
            </button>
            <button
              onClick={onClose}
              className="text-xs px-3 py-1.5 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>

        {/* Monaco Editor */}
        <div className="flex-1 overflow-hidden rounded-b-lg">
          <Editor
            height="100%"
            defaultLanguage="typescript"
            defaultValue={value}
            onChange={handleEditorChange}
            theme="vs-dark"
            options={{
              fontSize: 13,
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              wordWrap: 'on',
              formatOnPaste: true,
              tabSize: 2,
            }}
          />
        </div>
      </div>
    </div>
  )
}
