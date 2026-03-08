# Análisis de Code Smells — Frontend Designer (Claude)

> Análisis enfocado en la aplicación React/TypeScript (`apps/designer`).
> Complementa el documento general `code-smells-analysis.md`.

---

## 1. Estado del Frontend

La aplicación Designer está construida con React + Vite + TypeScript estricto. En términos generales el código frontend está bien estructurado: no se usa `any`, se utilizan tipos explícitos y los componentes son relativamente cohesivos.

No se han encontrado smells críticos en el frontend. Los puntos de mejora identificados son de severidad baja.

---

## 2. Smells Identificados

### 2.1 Función Inline sin Nombre en `useRef` (Baja)

**Fichero:** `ConfigPanel.tsx`, línea 32

```tsx
const uidCounterRef = useRef(0)
const nextUid = () => ++uidCounterRef.current
```

La función `nextUid` es una closure sin memoización. Si el componente se re-renderiza frecuentemente, cada render crea una nueva función. Mejor usar `useCallback`:

```tsx
const nextUid = useCallback(() => ++uidCounterRef.current, [])
```

### 2.2 Literal de String para Tipos de Nodo (Baja)

**Fichero:** `ConfigPanel.tsx`, línea 18

```tsx
const NODES_WITH_SECRET = ['sftp', 's3', 'smb', 'mail', 'rabbitmq', 'sql', 'http']
```

El array de tipos de nodo con secreto está hardcodeado y puede desincronizarse con el DSL. Debería derivarse de los tipos del DSL o del registro de actividades del motor.

**Recomendación:** Centralizar los tipos de nodo en `types/dsl.ts` como unión de literales:

```ts
export const NODE_TYPES_WITH_SECRET = ['sftp', 's3', 'smb', 'mail', 'rabbitmq', 'sql', 'http'] as const
export type NodeTypeWithSecret = typeof NODE_TYPES_WITH_SECRET[number]
```

### 2.3 Componente `ConfigPanel` con Alta Responsabilidad (Media)

`ConfigPanel.tsx` gestiona múltiples responsabilidades: renderizado del formulario de configuración, estado de live-test, gestión de headers HTTP y selección de secretos. Con el tiempo podría crecer a un componente difícil de mantener (God Component).

**Recomendación:** Extraer hooks personalizados:
- `useLiveTest(nodeId)` para la lógica de prueba en vivo
- `useHeaderRows(config)` para la gestión de headers HTTP

### 2.4 Error Silenciado en `useEffect` (Baja)

**Fichero:** `ConfigPanel.tsx`, línea 37-38

```tsx
.catch(() => { /* silently ignore — user may not have secrets yet */ })
```

Los errores silenciados dificultan el diagnóstico en producción.

**Recomendación:** Loguear en modo desarrollo:

```tsx
.catch((err) => {
  if (import.meta.env.DEV) {
    console.warn('[ConfigPanel] Failed to load secrets:', err)
  }
})
```

---

## 3. Aspectos Positivos

- **Tipado estricto:** No se usa `any` en ningún fichero.
- **Separación de tipos:** Los tipos DSL, designer y mapper están separados en `types/`.
- **Hooks personalizados:** Ya existe separación en `lib/mapper.ts` y `lib/api.ts`.
- **Testing:** Existe `lib/mapper.test.ts`.

---

## 4. Recomendaciones Futuras

| Prioridad | Acción |
|-----------|--------|
| Media | Extraer `useLiveTest` y `useHeaderRows` de `ConfigPanel` |
| Baja | Centralizar constante `NODES_WITH_SECRET` en `types/dsl.ts` |
| Baja | Añadir `useCallback` a `nextUid` en `ConfigPanel` |
| Baja | Loguear errores suprimidos en modo DEV |
