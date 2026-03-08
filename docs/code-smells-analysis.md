# Code Smells Analysis

## Metodología
- **Fecha:** 2025-07-14
- **Scope:** `/apps/designer/src`, `/services/engine`
- **Criterios:** Martin Fowler (Refactoring book) + React antipatterns + Go best practices
- **Herramientas:** Inspección manual + ESLint/SonarJS (configurado en `apps/designer/eslint.config.js`)

---

## 🚨 Code Smells Identificados

---

### 1. LARGE COMPONENT / GOD COMPONENT - Severidad: 🔴

**Ubicación:** `apps/designer/src/App.tsx` (componente principal)

**Código:**
```typescript
// App.tsx concentra routing, layout, estado global y lógica de negocio
function App() {
  // Múltiples useStates, lógica de serialización DSL, gestión de nodos...
}
```

**Problema:** El componente `App` actúa como un "God Component": gestiona el estado del grafo React Flow, la serialización al DSL JSON, el routing y el layout. Viola el principio de Responsabilidad Única (SRP).

**Impacto:** Dificulta el testing unitario, aumenta la complejidad cognitiva, y hace que Fast Refresh de Vite sea menos efectivo (full reloads frecuentes).

**Refactor sugerido:**
```typescript
// Extraer responsabilidades:
// hooks/useFlowGraph.ts → estado y operaciones del grafo
// hooks/useFlowSerializer.ts → conversión ReactFlow ↔ DSL JSON
// components/FlowCanvas.tsx → renderizado del canvas
// App.tsx → solo composición y routing
```

---

### 2. PROPS DRILLING - Severidad: 🔴

**Ubicación:** `apps/designer/src/components/` (nodos del canvas)

**Código:**
```typescript
// Patrón detectado: pasar props de configuración de nodo a través de 3+ niveles
<FlowCanvas 
  onNodeSelect={onNodeSelect}
  selectedNode={selectedNode}
  onConfigChange={onConfigChange}
  // ... más props pasadas hacia abajo
/>
```

**Problema:** La configuración del nodo seleccionado y los callbacks se pasan manualmente a través de múltiples capas de componentes React Flow, creando acoplamiento fuerte entre capas no relacionadas.

**Impacto:** Cambiar la firma de un callback requiere modificar todos los componentes intermediarios. Viola el principio "Open/Closed".

**Refactor sugerido:**
```typescript
// Usar Context API o Zustand para estado del nodo seleccionado
// hooks/useNodeSelection.ts
const NodeSelectionContext = createContext<NodeSelectionState | null>(null);

export function useNodeSelection() {
  const ctx = useContext(NodeSelectionContext);
  if (!ctx) throw new Error('useNodeSelection must be inside NodeSelectionProvider');
  return ctx;
}
```

---

### 3. MAGIC NUMBERS / MAGIC STRINGS - Severidad: 🟡

**Ubicación:** `apps/designer/src/` (varios archivos de nodos y transiciones)

**Código:**
```typescript
// Strings de tipo de nodo hardcodeados en múltiples lugares
if (node.type === 'http_request') { ... }
if (node.type === 'script_ts') { ... }
if (transition.type === 'success') { ... }
```

**Problema:** Los tipos de nodo y transición están duplicados como strings literales en toda la codebase. Un typo introduce un bug silencioso en runtime.

**Impacto:** Refactorizar el nombre de un tipo de nodo requiere un `find & replace` global propenso a errores. No hay validación en tiempo de compilación.

**Refactor sugerido:**
```typescript
// types/dsl.constants.ts
export const NODE_TYPES = {
  HTTP_REQUEST: 'http_request',
  SCRIPT_TS: 'script_ts',
  LOGGER: 'logger',
  TRANSFORM: 'transform',
} as const;

export type NodeType = typeof NODE_TYPES[keyof typeof NODE_TYPES];

export const TRANSITION_TYPES = {
  SUCCESS: 'success',
  ERROR: 'error',
  CONDITION: 'condition',
} as const;
```

---

### 4. DUPLICATE CODE - Renderizado de Nodos - Severidad: 🟡

**Ubicación:** `apps/designer/src/components/nodes/`

**Código:**
```typescript
// HttpNode.tsx
function HttpNode({ data }: NodeProps) {
  return (
    <div className="node-container border rounded-lg p-3 bg-white shadow">
      <div className="node-header flex items-center gap-2">
        <span className="node-icon">🌐</span>
        <span className="node-title font-semibold">{data.label}</span>
      </div>
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}

// ScriptNode.tsx — estructura casi idéntica
function ScriptNode({ data }: NodeProps) {
  return (
    <div className="node-container border rounded-lg p-3 bg-white shadow">
      <div className="node-header flex items-center gap-2">
        <span className="node-icon">⚙️</span>
        <span className="node-title font-semibold">{data.label}</span>
      </div>
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
```

**Problema:** El esqueleto visual de cada nodo está duplicado. Cambiar el diseño base (border, padding, shadows) requiere editar N archivos.

**Impacto:** Alto coste de mantenimiento UI. Inconsistencias visuales entre tipos de nodo.

**Refactor sugerido:**
```typescript
// components/nodes/BaseNode.tsx
interface BaseNodeProps {
  icon: string;
  label: string;
  children?: React.ReactNode;
}

export default function BaseNode({ icon, label, children }: BaseNodeProps) {
  return (
    <div className="node-container border rounded-lg p-3 bg-white shadow">
      <div className="node-header flex items-center gap-2">
        <span className="node-icon">{icon}</span>
        <span className="node-title font-semibold">{label}</span>
      </div>
      {children}
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}

// HttpNode.tsx — ahora delgado
export default function HttpNode({ data }: NodeProps) {
  return <BaseNode icon="🌐" label={data.label} />;
}
```

---

### 5. FEATURE ENVY (Go Engine) - Severidad: 🔴

**Ubicación:** `services/engine/internal/` (actividades que acceden directamente al contexto de ejecución)

**Código:**
```go
// Una actividad accede y manipula directamente campos internos del ExecutionContext
func (a *HttpActivity) Execute(ctx *ExecutionContext) error {
    // Accede a ctx.nodes, ctx.trigger, ctx.resolveVar... directamente
    url := ctx.nodes[a.prevNodeID].output["url"]
    headers := ctx.trigger.headers
    ctx.nodes[a.ID].output = result
    // ...
}
```

**Problema:** La actividad `HttpActivity` tiene demasiado interés en las entrañas de `ExecutionContext`. Viola la Ley de Demeter: una actividad debería hablar solo con su interfaz inmediata, no con los campos internos del contexto.

**Impacto:** Cambiar la estructura interna de `ExecutionContext` rompe todas las actividades. Imposible testear una actividad sin instanciar el contexto completo.

**Refactor sugerido:**
```go
// Definir una interfaz que exponga solo lo necesario
type ActivityContext interface {
    ResolveVar(path string) (interface{}, error)  // Resuelve $.nodes.X.output
    SetOutput(nodeID string, output map[string]interface{})
    GetInput(mapping map[string]string) (map[string]interface{}, error)
}

// La actividad solo depende de la interfaz
func (a *HttpActivity) Execute(ctx ActivityContext) error {
    input, err := ctx.GetInput(a.InputMapping)
    if err != nil {
        return fmt.Errorf("http activity input: %w", err)
    }
    // ...
}
```

---

### 6. LONG METHOD (Go Engine) - Severidad: 🟡

**Ubicación:** `services/engine/internal/runner/runner.go` (función de ejecución principal)

**Código:**
```go
// Función que supera las 60-80 líneas mezclando: parsing, validación,
// resolución de variables, ejecución y auditoría
func (r *Runner) ExecuteProcess(def *ProcessDefinition) error {
    // 1. Validar definición
    // 2. Ordenar nodos
    // 3. Para cada nodo: resolver input_mapping
    // 4. Ejecutar actividad
    // 5. Evaluar transiciones
    // 6. Emitir evento de auditoría
    // Todo en un solo método de >80 líneas
}
```

**Problema:** El método `ExecuteProcess` tiene complejidad ciclomática elevada (estimada >10), mezcla múltiples niveles de abstracción y es difícil de testear en partes.

**Impacto:** Cualquier cambio en la lógica de transiciones o auditoría requiere entender todo el método. Viola el límite de complejidad ciclomática ≤10 definido en [AGENTS.md](../AGENTS.md).

**Refactor sugerido:**
```go
func (r *Runner) ExecuteProcess(def *ProcessDefinition) error {
    if err := r.validateDefinition(def); err != nil {
        return fmt.Errorf("validate: %w", err)
    }
    nodes, err := r.topologicalSort(def.Nodes)
    if err != nil {
        return fmt.Errorf("sort nodes: %w", err)
    }
    for _, node := range nodes {
        if err := r.executeNode(node, def); err != nil {
            return fmt.Errorf("node %s: %w", node.ID, err)
        }
    }
    return nil
}

// Métodos privados con responsabilidad única:
// r.validateDefinition(), r.executeNode(), r.evaluateTransitions(), r.emitAudit()
```

---

### 7. PRIMITIVE OBSESSION - Severidad: ⚠

**Ubicación:** `apps/designer/src/` (definición de nodos del grafo)

**Código:**
```typescript
// El estado del grafo usa tipos primitivos string para IDs y configuraciones
interface FlowNode {
  id: string;          // ¿Es un NodeID? ¿Un UUID? ¿Un slug?
  type: string;        // Debería ser NodeType (discriminated union)
  data: Record<string, unknown>;  // Demasiado genérico
}
```

**Problema:** Usar `string` para IDs sin tipo nominal permite asignar cualquier string donde se espera un `NodeID`. El campo `data` con `Record<string, unknown>` pierde toda la semántica de configuración específica por tipo de nodo.

**Impacto:** El compilador TypeScript no puede detectar configuraciones incorrectas de nodo. Viola la regla de "no `any`" del proyecto (aquí `unknown` genérico tiene el mismo efecto práctico).

**Refactor sugerido:**
```typescript
// types/dsl.ts - Discriminated unions por tipo de nodo
type NodeID = string & { readonly __brand: 'NodeID' };

interface HttpNodeData {
  type: 'http_request';
  label: string;
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';
  url: string;
  headers?: Record<string, string>;
}

interface ScriptNodeData {
  type: 'script_ts';
  label: string;
  script: string;
  inputMapping: Record<string, string>;
}

type NodeData = HttpNodeData | ScriptNodeData | LoggerNodeData | TransformNodeData;

interface FlowNode {
  id: NodeID;
  data: NodeData;
}
```

---

### 8. DEAD CODE - Importaciones no usadas - Severidad: ⚠

**Ubicación:** `apps/designer/src/` (varios componentes)

**Código:**
```typescript
// Importaciones que ESLint/TypeScript marcan como no utilizadas
import { useEffect, useState, useCallback, useMemo } from 'react'; // useCallback y useMemo no se usan
import type { Edge, Node, Connection } from '@xyflow/react'; // Connection importada pero no referenciada
```

**Problema:** Importaciones muertas aumentan el bundle innecesariamente y confunden a futuros desarrolladores sobre las dependencias reales del módulo.

**Impacto:** Aumenta el tiempo de análisis del compilador y puede inhibir el tree-shaking de Vite.

**Refactor sugerido:** Activar la regla `@typescript-eslint/no-unused-vars` en [eslint.config.js](../apps/designer/eslint.config.js) como `error` (actualmente solo `warn`) y ejecutar `eslint --fix` para eliminarlas automáticamente.

---

## 📊 Resumen de Hallazgos

| # | Smell | Severidad | Área | Prioridad de Refactor |
|---|-------|-----------|------|----------------------|
| 1 | Large/God Component | 🔴 Alto | Frontend | Alta |
| 2 | Props Drilling | 🔴 Alto | Frontend | Alta |
| 3 | Magic Strings/Numbers | 🟡 Medio | Frontend/Backend | Media |
| 4 | Duplicate Code (Nodos UI) | 🟡 Medio | Frontend | Media |
| 5 | Feature Envy (Actividades) | 🔴 Alto | Backend (Go) | Alta |
| 6 | Long Method (Runner) | 🟡 Medio | Backend (Go) | Media |
| 7 | Primitive Obsession (IDs/Tipos) | ⚠ Bajo | Frontend | Baja |
| 8 | Dead Code (Imports) | ⚠ Bajo | Frontend | Baja |

## 🔧 Próximos Pasos Recomendados

1. **Inmediato:** Refactorizar `ActivityContext` interface (smell #5) — bloquea la testabilidad del motor.
2. **Sprint próximo:** Extraer `BaseNode` component (smell #4) y `NODE_TYPES` constants (smell #3).
3. **Backlog:** Migrar estado al Context/Zustand para eliminar props drilling (smell #2).
4. **CI:** Activar `no-unused-vars` como `error` en ESLint para prevenir smell #8 en futuro.