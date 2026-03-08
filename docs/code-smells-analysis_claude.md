# Code Smells Analysis

## Metodología

- **Fecha:** 2026-03-06
- **Scope:** `apps/designer/src/`
- **Criterios:** Martin Fowler (*Refactoring* 2ª ed.) + React antipatterns
- **Herramientas:** Inspección manual + revisión estática de tipos TypeScript
- **Archivos analizados:** 19 fuentes `.ts` / `.tsx` (excluye `node_modules`, `dist`, tests)

---

## 🚨 Code Smells Identificados

---

### 1. LARGE COMPONENT (God Component) — Severidad: 🔴

**Ubicación:** `apps/designer/src/components/ConfigPanel.tsx:21`

**Código:**
```typescript
export function ConfigPanel({ selectedNode, onNodeUpdate, allNodes = [] }: ConfigPanelProps) {
  // 668 líneas: gestiona trigger, http, sql, log, transform,
  // file, sftp, s3, smb, mail, rabbitmq, code/script_ts
  // + Live Test + Data Mapper + Monaco Modal
```

**Problema:** `ConfigPanel` tiene **668 líneas** y es responsable de renderizar la UI de configuración para los 11 tipos de nodo distintos, más el trigger. También gestiona Live Test, el modal del mapper y el editor Monaco. Viola el Principio de Responsabilidad Única (SRP) de forma flagrante.

**Impacto:** Cualquier cambio en un tipo de nodo requiere abrir y navegar este fichero monolítico. Los tests unitarios son prácticamente inviables. El renderizado condicional anidado (`data.nodeKind === 'process' && data.type === 'http' && (...)`) genera una explosión de estados y ramas difícil de razonar.

**Refactor sugerido:** Extraer un componente de panel por tipo de nodo: `HttpConfig`, `SqlConfig`, `SftpConfig`, `TriggerConfig`, etc., y un `NodeConfigPanelRouter` que seleccione cuál renderizar. La lógica de Live Test debería ir a `LiveTestPanel`.

---

### 2. DUPLICATE CODE — Script source resolution — Severidad: 🔴

**Ubicación:** `apps/designer/src/components/ConfigPanel.tsx:109`, `182-185`, `210-213`

**Código:**
```typescript
// Instancia 1 — handleScriptChange (línea 109)
if (data.type === 'code') {
  const currentConfig = (data.config as unknown as Record<string, unknown>) || {}
  updateNodeConfigCentralized({ ...currentConfig, script: value })
} else {
  updateNodeDataCentralized({ script: value })
}

// Instancia 2 — handleLiveTest (línea 182)
const script = (data.type as string) === 'script_ts'
  ? (data.script as string || '')
  : (data.type as string) === 'code'
    ? ((data.config as unknown as Record<string, unknown>)?.script as string || '')
    : undefined

// Instancia 3 — currentScript derivation (línea 210)
const currentScript = data.nodeKind === 'process' && (data.type as string) === 'script_ts'
  ? (data.script as string || '')
  : data.nodeKind === 'process' && (data.type as string) === 'code'
    ? ((data.config as unknown as Record<string, unknown>)?.script as string || '')
    : ''
```

**Problema:** La lógica para determinar *dónde* reside el script (en `data.script` o en `data.config.script`) está duplicada en tres lugares distintos del mismo componente. Esto es un clásico *Duplicate Code* de Fowler: la misma decisión, expresada tres veces.

**Impacto:** Si se añade un tercer tipo de nodo con script, hay que actualizar tres funciones. Si la lógica difiere entre instancias (ya difieren: instancia 2 devuelve `undefined` para tipos desconocidos; instancia 3 devuelve `''`), se producen bugs sutiles.

**Refactor sugerido:**
```typescript
function resolveScript(data: NodeData): string {
  if (data.nodeKind !== 'process') return ''
  if ((data.type as string) === 'script_ts') return (data.script as string) ?? ''
  if ((data.type as string) === 'code') {
    return ((data.config as Record<string, unknown>)?.script as string) ?? ''
  }
  return ''
}
```

---

### 3. DUPLICATE CODE — Error serialization pattern — Severidad: 🟡

**Ubicación:** `App.tsx:118,141,157` · `ProcessManager.tsx:63,83,99,115,174` · `SecretsManager.tsx:54,87,100` · `ExecutionHistory.tsx:124,274,303,309`

**Código:**
```typescript
// Repetido ~12 veces en el codebase
err instanceof Error ? err.message : String(err)
```

**Problema:** El patrón de serializar errores desconocidos a string se repite al menos 12 veces en 4 ficheros. Es un *Duplicate Code* estructural que dispersa la misma lógica de presentación.

**Impacto:** Si se necesita enriquecer el mensaje de error (añadir stack trace en dev, código de error HTTP, etc.), hay que modificar 12 puntos.

**Refactor sugerido:**
```typescript
// lib/errors.ts
export function toErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}
```

---

### 4. DUPLICATE CODE — `setTimeout` save feedback — Severidad: 🟡

**Ubicación:** `apps/designer/src/App.tsx:138-139` y `193-194`

**Código:**
```typescript
// handleSave (línea 138)
setSaveMsg('Saved ✓')
setTimeout(() => setSaveMsg(null), 3000)

// handleSaveAsConfirm (línea 193)
setSaveMsg('Saved ✓')
setTimeout(() => setSaveMsg(null), 3000)
```

**Problema:** El patrón "mostrar mensaje de éxito y borrarlo tras 3 segundos" se duplica literalmente en dos handlers del mismo componente. El magic number `3000` también aparece duplicado.

**Impacto:** Cambiar el timeout o el mensaje requiere dos ediciones. Pequeño ahora; se amplifica si se añaden más acciones con feedback.

**Refactor sugerido:** Extraer una función helper `showSaveSuccess()` que encapsule ambas llamadas, o bien un hook `useFeedbackMessage`.

---

### 5. DUPLICATE CSS CLASS STRINGS — Primitive Obsession — Severidad: 🟡

**Ubicación:** `ConfigPanel.tsx:216-218` y `SecretsManager.tsx:106-110`

**Código:**
```typescript
// ConfigPanel.tsx
const inputClass = "w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400"
const selectClass = "w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 bg-white"
const labelClass = "block text-xs font-medium text-gray-600 mb-1"

// SecretsManager.tsx (idéntico)
const inputClass = "w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400"
const selectClass = "w-full text-xs border border-gray-300 rounded px-2 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-400 bg-white"
const labelClass = "block text-xs font-medium text-gray-600 mb-1"
```

**Problema:** Las mismas tres strings de clases Tailwind están copiadas literalmente en dos componentes. Si el design system cambia (ring color, border radius, tamaño de fuente), requiere ediciones en múltiples ficheros.

**Impacto:** Inconsistencia visual si uno se actualiza y el otro no. Señal de que no hay un sistema de design tokens compartido.

**Refactor sugerido:** Extraer a un fichero compartido `lib/classNames.ts` o usando clases CSS personalizadas en `tailwind.config`.

---

### 6. GLOBAL MUTABLE STATE — Severidad: 🟡

**Ubicación:** `apps/designer/src/components/DesignerCanvas.tsx:101`

**Código:**
```typescript
let nodeCounter = 1  // variable de módulo mutable

// Usada en onDrop para generar ids únicos:
const id = `${type}_${Date.now()}_${nodeCounter++}`
```

**Problema:** `nodeCounter` es una variable mutable a nivel de módulo (no de componente). En React con HMR (Hot Module Replacement), el módulo no se re-evalúa en cada recarga, por lo que el contador persiste entre recargas en desarrollo. Además, si en el futuro hubiera múltiples instancias del canvas, compartirían el mismo contador, generando IDs duplicados.

**Impacto:** IDs de nodo potencialmente no únicos en escenarios de HMR o múltiples instancias. Dificulta el testing (el output no es determinista).

**Refactor sugerido:** Usar `useRef` dentro del componente:
```typescript
const nodeCounterRef = useRef(1)
// en onDrop:
const id = `${type}_${Date.now()}_${nodeCounterRef.current++}`
```

---

### 7. DEFAULT DEFINITION DUPLICADA — Severidad: 🟡

**Ubicación:** `apps/designer/src/App.tsx:21-31` y `apps/designer/src/lib/serializer.ts:6-16`

**Código:**
```typescript
// App.tsx
const DEFAULT_DEFINITION: FlowDefinition = {
  id: 'new-flow', version: '1.0.0', name: 'New Flow', description: '',
  settings: { persistence: 'full', timeout: 30000, error_strategy: 'stop_and_rollback' },
}

// serializer.ts — idéntico
const DEFAULT_DEFINITION: FlowDefinition = {
  id: 'new-flow', version: '1.0.0', name: 'New Flow', description: '',
  settings: { persistence: 'full', timeout: 30000, error_strategy: 'stop_and_rollback' },
}
```

**Problema:** La constante `DEFAULT_DEFINITION` está copiada literalmente en dos ficheros distintos, incluyendo el magic number `30000` para el timeout.

**Impacto:** Si se cambia el valor de timeout por defecto o la estrategia de error, hay que acordarse de actualizar ambas copias. Es fácil que diverjan.

**Refactor sugerido:** Exportar la constante desde `serializer.ts` o desde un módulo `lib/defaults.ts` e importarla donde se necesite.

---

### 8. MAGIC NUMBERS — Severidad: ⚠

**Ubicación:** `App.tsx:29,139,194` · `ConfigPanel.tsx:147` · `DesignerCanvas.tsx:61-63`

**Código:**
```typescript
// App.tsx:29 — timeout de flow en ms sin nombre
settings: { persistence: 'full', timeout: 30000, ... }

// App.tsx:139 (y 194) — duración del toast sin nombre
setTimeout(() => setSaveMsg(null), 3000)

// ConfigPanel.tsx:147 — debounce de headers sin nombre
const timeoutId = setTimeout(() => { syncHeaders(newRows) }, 300)

// DesignerCanvas.tsx:61-63 — puerto y defaults de nodo sql
sql: { engine: 'postgres', host: 'localhost', port: 5432, database: 'mydb', query: 'SELECT 1' },
sftp: { server: 'sftp.example.com', port: 22, folder: '/files', method: 'get' },
mail: { host: 'smtp.example.com', port: 587, action: 'send' },
```

**Problema:** Múltiples valores literales numéricos sin nombre descriptivo: `30000` (timeout del flow), `3000` (duración del toast), `300` (debounce), puertos por defecto (`5432`, `22`, `587`) embebidos directamente en arrays de configuración.

**Impacto:** Al leer el código no está claro qué significa `3000` sin contexto. Cambiar la duración del toast implica buscar todas las ocurrencias.

**Refactor sugerido:**
```typescript
const TOAST_DURATION_MS = 3_000
const HEADER_DEBOUNCE_MS = 300
const DEFAULT_FLOW_TIMEOUT_MS = 30_000
```

---

### 9. DEAD / ALWAYS-FALSE LOGIC — Severidad: ⚠

**Ubicación:** `apps/designer/src/components/DataMapper.tsx:55`

**Código:**
```typescript
{expanded && hasChildren && field.children!.map((child) => (
  <FieldNode
    key={child.path}
    field={child}
    depth={depth + 1}
    onSelect={onSelect}
    isSelected={isSelected && false /* parent selected, not child */}
  />
))}
```

**Problema:** La expresión `isSelected && false` siempre evalúa a `false`, independientemente del valor de `isSelected`. El prop `isSelected` que se pasa a los nodos hijo nunca será `true`. El comentario reconoce la intención, pero deja el código con lógica muerta confusa.

**Impacto:** Si en el futuro se quiere que un nodo hijo pueda mostrarse como seleccionado, la lógica está incorrectamente hardcodeada. Confunde al lector sobre cuál es el comportamiento intencionado.

**Refactor sugerido:** Ser explícito:
```typescript
isSelected={false} // solo el nodo raíz puede estar seleccionado
```

---

### 10. DATA CLUMP — `{nodes, edges, definition}` — Severidad: ⚠

**Ubicación:** `App.tsx:101,136,189,295,313,319` · `ProcessManager.tsx:79`

**Código:**
```typescript
// El trío aparece junto en 6+ lugares
serializeGraph(nodes, edges, definition)              // App.tsx:101, 136, 189; ProcessManager.tsx:79
<ExportButton nodes={nodes} edges={edges} definition={definition} />   // App.tsx:295
<ProcessManager nodes={nodes} edges={edges} definition={definition} /> // App.tsx:313
```

**Problema:** El trío `{nodes, edges, definition}` siempre aparece junto como grupo de datos cohesivo. Esto es el *Data Clump* de Fowler: datos que viajan siempre juntos deberían encapsularse.

**Impacto:** Si se añade un campo al "proyecto" (e.g., `tags`, `metadata`), hay que actualizar todas las firmas donde aparece el trío. Los cambios se propagan como *Shotgun Surgery*.

**Refactor sugerido:** Encapsular en un tipo o en un contexto React:
```typescript
interface FlowProject { nodes: Node<NodeData>[]; edges: Edge[]; definition: FlowDefinition }
// o un React Context para evitar props drilling
```

---

### 11. TYPE CASTING HELL — Primitive Obsession — Severidad: ⚠

**Ubicación:** `ConfigPanel.tsx:43,72,84-85,110,112,140,165,171-172,181,183-185,192,206,213` · `serializer.ts:33,51` · `deserializer.ts:39,52` · `ActivityNode.tsx:27`

**Código:**
```typescript
// ConfigPanel.tsx — ejemplos representativos
const config = (selectedNode.data.config as HttpNodeConfig) || {}
updateNodeConfigCentralized({ ...currentConfig, headers } as Record<string, unknown>)
onNodeUpdate(selectedNodeId, { ...selectedNode.data, config: updatedConfig } as Partial<NodeData>)
const currentConfig = (data.config as unknown as Record<string, unknown>) || {}

// deserializer.ts:39
data: { nodeKind: 'trigger', ... } as unknown as NodeData,

// ActivityNode.tsx:27
const nodeData = data as unknown as NodeData
```

**Problema:** El type `NodeData = Record<string, unknown> & (...)` obliga a hacer casting constante con `as unknown as X` en todo el codebase porque TypeScript no puede narrowear a través de la intersección. Hay al menos 15 casts explícitos solo en `ConfigPanel`.

**Impacto:** Los casts silencian al compilador, eliminando la protección de tipos. Un renombrado de campo en el DSL no generará errores en tiempo de compilación. El código es frágil.

**Refactor sugerido:** Usar `type guards` o discriminated unions más precisas y type narrowing en lugar de casts. Por ejemplo, una función `isProcessNode(node): node is DesignerNode & { data: FlowNode & { nodeKind: 'process' } }`.

---

### 12. INCONSISTENT ABSTRACTION — `script_ts` vs `code` — Severidad: ⚠

**Ubicación:** `ConfigPanel.tsx:110,182,210` · `DesignerCanvas.tsx:56` · múltiples condicionales

**Código:**
```typescript
// Dos tipos de nodo de script con lógica diferenciada en todo el codebase
(data.type as string) === 'script_ts' || data.type === 'code'

// DesignerCanvas.tsx — solo 'code' está en activityDefaults (no script_ts):
code: { script: 'export default (input) => input' },
// (script_ts no aparece en activityDefaults ni en TYPE_MAP)
```

**Problema:** Existen dos tipos `code` y `script_ts` representando conceptos similares (nodo de scripting), con diferente almacenamiento del script (`data.script` vs `data.config.script`). El tipo `script_ts` parece un residuo de una refactorización incompleta: aparece en `ConfigPanel` y en los tipos DSL pero **no está en `TYPE_MAP` ni en `activityDefaults`** de `DesignerCanvas`.

**Impacto:** `script_ts` es código legado que crea paths condicionales en 3 funciones. No puede arrastrarse al canvas (no está en `TYPE_MAP`), pero sí puede llegar via `deserializeGraph` desde DSLs existentes. El comportamiento es inconsistente.

**Refactor sugerido:** Deprecar `script_ts`, migrar DSLs existentes a `code` y eliminar todas las ramas condicionales que lo distinguen.

---

### 13. FEATURE ENVY — `ProcessManager` como orquestador de serialización — Severidad: ⚠

**Ubicación:** `apps/designer/src/components/ProcessManager.tsx:75-86`

**Código:**
```typescript
const handleSaveCurrent = useCallback(async () => {
  setSaving(true)
  setActionError(null)
  try {
    const dsl: FlowDSL = serializeGraph(nodes, edges, definition) // mismo patrón que App.tsx
    await saveProcess(dsl)
    await reload()
  } ...
}, [nodes, edges, definition, reload])
```

**Problema:** `ProcessManager` recibe `nodes`, `edges` y `definition` para serializar el grafo — la misma operación que hace `App.tsx`. El componente de gestión de despliegues está haciendo trabajo que pertenece a la capa de orquestación (`App.tsx` o un hook). Este es el smell *Feature Envy*: un método interesado en los datos de otra clase.

**Impacto:** La lógica de "serializar y guardar" está duplicada entre `App.tsx` y `ProcessManager.tsx`. Si cambia la API de `saveProcess`, hay dos lugares que actualizar.

**Refactor sugerido:** Elevar `handleSaveCurrent` a `App.tsx` y pasarlo como callback a `ProcessManager`, igual que `onEditProcess`. Alternativamente, un custom hook `useFlowPersistence`.

---

### 14. PROPS DRILLING — `{nodes, edges, definition}` — Severidad: ⚠

**Ubicación:** `App.tsx:295,313-314` (render tree)

**Código:**
```typescript
<ExportButton nodes={nodes} edges={edges} definition={definition} />
<ProcessManager nodes={nodes} edges={edges} definition={definition} onEditProcess={handleEditProcess} />
```

**Problema:** El trío de estado del grafo se pasa como props a múltiples componentes hijos (`ExportButton`, `ProcessManager`). Este es el antipatrón React de *Props Drilling*: datos que atraviesan la jerarquía de componentes sin ser utilizados en los niveles intermedios.

**Impacto:** Añadir un nuevo atributo al proyecto requiere modificar la firma de múltiples componentes aunque no les importe ese dato.

**Refactor sugerido:** Un `FlowContext` (React Context) que exponga el estado del grafo actual, reduciendo el acoplamiento entre `App` y sus consumidores.

---

## Resumen por severidad

| Severidad | Cantidad | Smells |
|-----------|----------|--------|
| 🔴 Alta   | 2        | Large Component (`ConfigPanel`), Duplicate Code (script resolution) |
| 🟡 Media  | 6        | Duplicate Code (error pattern, CSS classes, save feedback, DEFAULT_DEFINITION), Global Mutable State, Data Clump |
| ⚠ Baja   | 6        | Magic Numbers, Dead Logic, Type Casting Hell, Inconsistent Abstraction, Feature Envy, Props Drilling |

## Prioridad de refactoring recomendada

1. **`ConfigPanel.tsx` — Descomposición en sub-componentes** (alto ROI: reduce ~500 líneas del componente más complejo)
2. **Extraer `toErrorMessage` y `DEFAULT_DEFINITION`** (cambios rápidos con alto impacto en DRY)
3. **Eliminar `script_ts`** (elimina ramas condicionales duplicadas y aclarece el modelo)
4. **`FlowContext`** (resuelve Props Drilling y Data Clump de una vez)
5. **Tipar correctamente `NodeData`** (elimina la cascada de `as unknown as`)
