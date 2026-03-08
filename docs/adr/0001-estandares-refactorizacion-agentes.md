# ADR 0001 — Estándares de Refactorización para Agentes de IA

- **Estado:** Aceptado
- **Fecha:** 2026-03-08
- **Scope:** `services/engine` (Go) · `apps/designer/src` (TypeScript/React)
- **Autores:** Copilot Agent (basado en `docs/code-smells-analysis_claude_go.md`, `docs/code-smells-analysis_claude.md`, `docs/code-smells-analysis.md`)

---

## Contexto

Tras el análisis de *code smells* realizado en las fechas 2025-07-14 y 2026-03-06, se identificaron 15 problemas de calidad en el motor Go y 14 en la capa frontend React/TypeScript. El nivel de deuda técnica acumulada afectaba a la testabilidad, la seguridad y la mantenibilidad del sistema.

Este ADR documenta las decisiones de refactorización adoptadas para eliminar los *smells* más críticos, definiendo además los estándares que los agentes de IA deben seguir en futuras generaciones de código para el proyecto **flowjs-works**.

---

## Decisiones adoptadas

### Go — `services/engine`

#### 1. Función compartida `getCredential` (Smell #1 — Duplicate Code 🔴)

**Problema:** La closure `getCredential` de 7 líneas (lectura de credenciales desde un mapa de configuración, con soporte para `config["auth"]` anidado o claves planas inyectadas por el resolvedor de secretos) estaba copiada literalmente en `sftp.go`, `s3.go` y `mail.go`.

**Decisión:** Extraer la función como función de paquete en el nuevo archivo `internal/activities/credentials.go`. Las tres actividades ahora llaman `getCredential(config, key)`.

**Consecuencia:** Un único punto de cambio si se modifica la estrategia de resolución de credenciales (p.ej., soporte para variables de entorno).

---

#### 2. `gofmt` aplicado (Smell #15 — Formateo inconsistente ⚠)

**Problema:** Cuatro archivos (`script.go`, `sql.go`, `transform.go`, `mail.go`) tenían el cuerpo de las funciones sin indentación de tabs, violando el formato canónico de Go.

**Decisión:** Aplicar `gofmt -w` sobre esos cuatro archivos. Se establece como requisito de CI que todos los archivos Go estén formateados antes de ser enviados.

**Estándar futuro:** Los agentes DEBEN ejecutar `gofmt` o asegurarse de que el código generado sea compatible con el formateador estándar de Go.

---

#### 3. Seguridad: clave AES insegura por defecto (Smell #10 — Security 🟡)

**Problema:** `aesKeyFromEnv` en `cmd/server/main.go` arrancaba el servidor con una clave AES hardcodeada pública cuando `SECRETS_AES_KEY` no estaba configurada. Esto comprometía todos los secretos almacenados en entornos que hubieran omitido la configuración por error.

**Decisión:** Añadir verificación del entorno mediante `APP_ENV`. Cuando `APP_ENV != "development"`, el servidor llama a `log.Fatalf` si la clave no está presente o tiene menos de 32 bytes. Solo en modo `development` se permite la clave de fallback (con un `WARNING` en el log).

**Consecuencia:** Despliegues sin configurar la variable serán detectados en el arranque en lugar de en tiempo de uso. El modo `development` conserva su ergonomía.

---

#### 4. Función `classifyTransitions` (Smell #3 — Duplicate Code 🟡)

**Problema:** El bloque de clasificación de transiciones por tipo (`condition`, `nocondition`, `success`, `error`) estaba duplicado en dos funciones de `executor.go` (`ExecuteFromNode` y `executeChain`).

**Decisión:** Extraer `classifyTransitions(transitions []models.Transition) (cond, noCond, success, errorT []models.Transition)` como función de paquete privada en `executor.go`. Ambas funciones llaman a este helper.

**Consecuencia:** Añadir un nuevo tipo de transición (p.ej. `"parallel"`) requiere un único cambio.

---

#### 5. Conversión redundante `interface{}(val)` eliminada (Smell #14 — Ruido de código ⚠)

**Problema:** En `models/context.go` se realizaba una conversión explícita a `interface{}` que en Go siempre es implícita: `current = interface{}(val)`.

**Decisión:** Sustituir por `current = val`. La regla `unconvert` de `golangci-lint` detecta y rechaza este patrón en el futuro.

---

#### 6. `UnmarshalJSON` no-op eliminado (Smell #13 — Dead Code ⚠)

**Problema:** `process.go` implementaba un `UnmarshalJSON` para `Process` que era funcionalmente idéntico al comportamiento por defecto de `encoding/json`. El método añadía ruido conceptual sin aportar valor.

**Decisión:** Eliminar el método y el import de `encoding/json` asociado en `process.go`. La estructura `NodeExecution` (previamente huérfana) se conserva como tipo canónico para representar el resultado de ejecución de un nodo, alineándose con el propósito original de diseño.

---

#### 7. `HTTPActivity` con cliente HTTP compartido (Smell #8 — Performance 🟡)

**Problema:** `HTTPActivity.Execute` creaba un nuevo `http.Client` (y por tanto un nuevo `http.Transport` sin pool de conexiones) en cada invocación. Esto desactivaba el HTTP keep-alive y resultaba en un handshake TCP/TLS extra por cada nodo HTTP.

**Decisión:**
- Añadir `client *http.Client` como campo de `HTTPActivity`.
- Crear el constructor `NewHTTPActivity()` que inicializa el cliente con `defaultHTTPTimeout`.
- Permitir override por petición solo cuando `config["timeout"]` está presente (se crea un cliente local para ese único request).
- Actualizar `activity.go` y los tests para usar `NewHTTPActivity()`.

---

#### 8. Constantes con nombre para magic numbers (Smell #6 — Magic Numbers 🟡)

**Problema:** Múltiples timeouts y delays estaban hardcodeados como literales numéricos sin nombre: `30*time.Second` en 4 lugares distintos, `5000` ms en `script.go`, `2*time.Second` en el retry de `executor.go`.

**Decisión:** Declarar las siguientes constantes tipadas:
- `defaultHTTPTimeout`, `defaultNetDialTimeout`, `defaultSSHTimeout` = `30 * time.Second` — en `activities/http.go` (paquete `activities`).
- `defaultScriptTimeoutMs` = `5_000` — en `activities/http.go`.
- `retryBaseInterval` = `2 * time.Second` — en `engine/executor.go` (paquete `engine`).

Sustituir todos los literales correspondientes por las constantes nombradas. Eliminar las importaciones de `"time"` de `sftp.go` y `smb.go` que quedaron sin usos tras el cambio.

---

### TypeScript/React — `apps/designer/src`

#### 9. Utilidad `toErrorMessage` (Smell #3 frontend — Duplicate Code 🔴)

**Problema:** El patrón `err instanceof Error ? err.message : String(err)` estaba repetido ≥12 veces en 4 archivos diferentes.

**Decisión:** Crear `lib/errors.ts` exportando `toErrorMessage(err: unknown): string`. Sustituir todas las instancias en `App.tsx`, `ProcessManager.tsx`, `SecretsManager.tsx`, `ExecutionHistory.tsx` y `ConfigPanel.tsx`.

**Consecuencia:** Enriquecer el mensaje de error en el futuro (stack trace en dev, código HTTP) requiere una única modificación.

---

#### 10. `DEFAULT_DEFINITION` única (Smell #7 frontend — Duplicate Code 🟡)

**Problema:** La constante `DEFAULT_DEFINITION: FlowDefinition` estaba copiada literalmente en `App.tsx` y `lib/serializer.ts`, incluyendo el magic number `30000` para el timeout.

**Decisión:** Exportar `DEFAULT_DEFINITION` desde `lib/serializer.ts` como `export const`. Importarla en `App.tsx` en lugar de redefinirla. Eliminar la copia local de `App.tsx`.

---

#### 11. `showSaveSuccess` — eliminar duplicación del setTimeout (Smell #4 frontend — Duplicate Code 🟡)

**Problema:** El patrón `setSaveMsg('Saved ✓'); setTimeout(() => setSaveMsg(null), 3000)` estaba copiado en dos handlers de `App.tsx` (`handleSave` y `handleSaveAsConfirm`).

**Decisión:** Extraer un callback `showSaveSuccess` (usando `useCallback`) que encapsula ambas llamadas. Definir la constante `TOAST_DURATION_MS = 3_000` con nombre descriptivo. Ambos handlers llaman a `showSaveSuccess()`.

---

#### 12. `isSelected={false}` explícito en DataMapper (Smell #9 frontend — Dead Logic ⚠)

**Problema:** `DataMapper.tsx` pasaba `isSelected={isSelected && false}` a los nodos hijo, una expresión que siempre evalúa a `false` independientemente del valor de `isSelected`.

**Decisión:** Reemplazar por `isSelected={false}` con un comentario que explique la intención: solo el nodo raíz puede estar seleccionado.

---

#### 13. `nodeCounterRef` en lugar de variable de módulo (Smell #6 frontend — Global Mutable State 🟡)

**Problema:** `DesignerCanvas.tsx` usaba `let nodeCounter = 1` como variable de módulo mutable para generar IDs únicos. Con HMR de Vite, el módulo no se re-evalúa, por lo que el contador persiste entre recargas de desarrollo. En tests o con múltiples instancias del canvas, el contador sería compartido.

**Decisión:** Reemplazar `let nodeCounter` por `const nodeCounterRef = useRef(1)` dentro del componente `DesignerCanvas`. El ID se genera con `nodeCounterRef.current++`.

---

#### 14. Clases CSS compartidas en `lib/classNames.ts` (Smell #5 frontend — Primitive Obsession 🟡)

**Problema:** Tres strings de clases Tailwind (`inputClass`, `selectClass`, `labelClass`) estaban copiados literalmente en `ConfigPanel.tsx` y `SecretsManager.tsx`.

**Decisión:** Crear `lib/classNames.ts` que exporta las tres constantes. `ConfigPanel.tsx` y `SecretsManager.tsx` importan estas constantes en lugar de definirlas localmente. Se eliminan las definiciones locales de ambos archivos.

---

#### 15. `resolveScript` — eliminación de triplicación lógica (Smell #2 frontend — Duplicate Code 🔴)

**Problema:** La lógica para determinar dónde reside el script de un nodo (`data.script` para `script_ts`, `data.config.script` para `code`) estaba duplicada en tres lugares de `ConfigPanel.tsx`: `handleScriptChange`, `handleLiveTest` y la derivación de `currentScript`.

**Decisión:** Extraer `resolveScript(data: DesignerNode['data']): string` como función de módulo antes del componente `ConfigPanel`. Las tres instancias de la lógica duplicada son reemplazadas por llamadas a `resolveScript(data)`.

---

## Consecuencias generales

### Positivas
- La cobertura de tests existente sigue pasando al 100% (81/81 tests de frontend, tests de Go sin cambios de comportamiento).
- Los *smells* de severidad 🔴 y 🟡 más impactantes han sido eliminados.
- La vulnerabilidad de seguridad activa (AES key insegura) ha sido corregida.
- El rendimiento de conexiones HTTP mejora gracias al pool compartido.
- Los agentes de IA que generen nuevo código para actividades (SFTP, SMB, S3, Mail) deben llamar a `getCredential(config, key)` en lugar de duplicar la closure.

### Pendientes (backlog)
- **`ConfigPanel` — Descomposición en sub-componentes** (668 líneas): El componente sigue siendo un *God Component*. El refactor requiere extraer `HttpConfig`, `SqlConfig`, `SftpConfig`, `TriggerConfig`, etc. Alto ROI pero mayor riesgo de regresión visual.
- **`FlowContext` (React Context)** para eliminar Props Drilling y el Data Clump `{nodes, edges, definition}`.
- **`NodeExecution` como tipo canónico** en `ExecutionContext.Nodes` para eliminar el Primitive Obsession con `map[string]interface{}`.
- **Abstraer el patrón put/get** en una interface `RemoteFS` (SFTP, SMB, S3).
- **Deprecar `script_ts`** como tipo de nodo y migrar DSLs existentes a `code` para eliminar las ramas condicionales residuales.
- ~~**CI lint gate**~~: ✅ Implementado en `.github/workflows/lint.yml` + `.golangci.yml`. El workflow ejecuta `gofmt -l`, `golangci-lint` (unconvert, gocyclo ≤10, errcheck, dupl, staticcheck) y `eslint --max-warnings 0` en cada push/PR a `main`.

---

## Estándares para agentes de IA (reglas derivadas)

Con base en los *smells* encontrados y las correcciones aplicadas, se establecen las siguientes reglas obligatorias para los agentes de IA que generen código en este repositorio:

1. **No duplicar `getCredential`**: Usar siempre la función del paquete `activities/credentials.go`.
2. **No crear `http.Client` por request**: Usar `NewHTTPActivity()` o un campo compartido.
3. **Usar `toErrorMessage` siempre**: No repetir `err instanceof Error ? err.message : String(err)`.
4. **Importar `DEFAULT_DEFINITION` desde `lib/serializer.ts`**: No redefinir la constante.
5. **Usar constantes para timeouts**: No usar literales numéricos para timeouts o durations.
6. **No variables de módulo mutables en React**: Usar `useRef` dentro del componente.
7. **Importar CSS class strings de `lib/classNames.ts`**: No duplicar en componentes individuales.
8. **Aplicar `gofmt`**: Todo código Go generado debe estar correctamente formateado.
9. **Verificar `APP_ENV` para claves de seguridad**: Los modos inseguros de desarrollo deben fallar explícitamente en producción.
10. **Extraer helpers para lógica duplicada**: Si la misma lógica aparece en más de dos lugares, extraer a función reutilizable.
