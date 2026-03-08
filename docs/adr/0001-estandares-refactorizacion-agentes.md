# ADR 0001 — Estándares de Refactorización: Eliminación de Code Smells en el Motor Go

## Estado

**Aceptado** — Implementado en la rama `copilot/update-code-smells-analysis-docs` (2026-03-08)

---

## Contexto

Tras una revisión estática del código fuente del motor de ejecución (`services/engine`) se identificaron varios *code smells* que reducen la legibilidad, mantenibilidad y estabilidad del sistema. Los hallazgos quedan documentados en:

- `docs/code-smells-analysis.md` — visión general
- `docs/code-smells-analysis_claude_go.md` — análisis Go detallado
- `docs/code-smells-analysis_claude.md` — análisis frontend React/TS

La política de calidad definida en `docs/quality_standards.md` exige:
- **Boy Scout Rule:** dejar el código más limpio de como se encontró.
- **Code Smells:** evitar números mágicos, código duplicado y alta complejidad ciclomática.
- **Decisiones de Arquitectura:** documentar el "por qué" mediante ADRs en `docs/adr/`.

---

## Decisión

Se adoptan los siguientes estándares de refactorización y se aplican de forma inmediata al motor Go.

### 1. Magic Numbers → Constantes con Nombre

**Problema:** Literales numéricos dispersos sin semántica explícita dificultan búsquedas y modificaciones centralizadas.

**Resolución:** Cada paquete declara sus propias constantes con prefijo descriptivo al inicio del fichero principal:

| Paquete | Constante | Valor | Significado |
|---------|-----------|-------|-------------|
| `engine` | `defaultRetryIntervalSecs` | `2` | Segundos de espera entre reintentos |
| `activities` (http) | `defaultHTTPTimeoutSec` | `30` | Timeout HTTP por defecto |
| `activities` (script) | `defaultScriptTimeoutMs` | `5_000` | Timeout JS por defecto en ms |
| `activities` (mail) | `defaultSMTPPort` | `587` | Puerto SMTP por defecto |
| `activities` (sql) | `defaultSQLTimeoutSec` | `30` | Timeout SQL por defecto |
| `activities` (file) | `defaultFilePermission` | `0o644` | Permisos UNIX de fichero |

**Nota sobre `RetryPolicy.Interval`:** El campo `Interval string` del modelo `RetryPolicy` existe en el DSL pero no se utiliza en el ejecutor (siempre se usa `defaultRetryIntervalSecs`). El soporte completo de parsing de duraciones (`"2s"`, `"1m"`) queda como deuda técnica a abordar en un ticket separado.

### 2. Registro de Alias `http_request`

**Problema:** Los tests `TestExecute_HttpRequestActivityRegistered` y `TestExecute_HttpRequestMissingURL` usaban el tipo `http_request` que no estaba registrado en el `ActivityRegistry`, causando 2 fallos de test permanentes.

**Resolución:** Se añade el tipo `HTTPRequestActivity` en `activities/http.go`:

```go
// HTTPRequestActivity es un alias de HTTPActivity registrado como "http_request"
// por compatibilidad retroactiva con flujos DSL y tests que usan ese tipo.
type HTTPRequestActivity struct{ HTTPActivity }

func (a *HTTPRequestActivity) Name() string { return "http_request" }
```

El tipo se registra en `NewActivityRegistry()` junto a `HTTPActivity`.

**Consecuencia:** Ambos nombres (`http` y `http_request`) son válidos en el DSL. Esta dualidad se documenta aquí para evitar confusión futura.

### 3. Eliminación de Código Duplicado en el Ejecutor

**Problema:** La lógica de enrutamiento de transiciones (`condTrans` / `noCondTrans` / `successTrans`) estaba duplicada entre `ExecuteFromNode` y `executeChain`.

**Resolución:** Se extrae el método privado `dispatchTransitions` en `executor.go`:

```go
func (e *ProcessExecutor) dispatchTransitions(
    transitions []models.Transition,
    nodeMap map[string]*models.Node,
    transMap map[string][]models.Transition,
    ctx *models.ExecutionContext,
    visited map[string]bool,
) error
```

Ambas funciones ahora delegan en este helper. La lógica es idéntica: evaluar condiciones en orden → si ninguna coincide, seguir ramas `nocondition` → si no hay condiciones, seguir ramas `success`.

### 4. Extracción de Helper `extractStringSlice` en Mail

**Problema:** La extracción de `toList` y `ccList` de `[]interface{}` estaba duplicada en `mailSend`.

**Resolución:** Se extrae la función privada `extractStringSlice(v interface{}) []string` reutilizada para ambos campos.

### 5. Corrección de Variable Shadowing en `process_store.go`

**Problema:** El método `List` declaraba `var s ProcessSummary` que ocultaba el receptor `s *ProcessStore`.

**Resolución:** La variable local se renombra a `summary`.

### 6. Formato `gofmt` Uniforme

**Problema:** Los ficheros `script.go`, `sql.go`, `file.go`, `rabbitmq.go` y `transform.go` usaban indentación con espacios en lugar de tabuladores.

**Resolución:** Se reescriben con indentación correcta (tabuladores, estándar Go).

---

## Alternativas Consideradas

### A. No hacer nada
Descartado. Los fallos de test activos (`http_request`) indican un contrato roto con el DSL que debe corregirse.

### B. Renombrar `HTTPActivity.Name()` a `"http_request"` y eliminar `"http"`
Descartado. Cambiaría el tipo en el DSL y rompería flujos existentes que usen `"http"`.

### C. Usar un único tipo con múltiples nombres vía mapa en el Registry
Más general pero sobrediseñado para el problema actual. Se puede plantear en una iteración posterior si proliferan los aliases.

---

## Consecuencias

### Positivas
- Todos los tests pasan (0 fallos).
- El código del ejecutor es más corto y fácil de auditar.
- Las constantes facilitan cambios centralizados de timeouts sin búsqueda por el código.
- Formato `gofmt` uniforme en todos los ficheros del paquete `activities`.

### Negativas / Riesgos
- `RetryPolicy.Interval` sigue sin utilizarse. Si un operador configura `"interval": "30s"` en el DSL, el motor ignorará ese valor y usará 2 segundos. **Mitigación:** Añadir un aviso en el log cuando `Interval != ""` y `Interval != "2s"`.
- Dos nombres válidos para el mismo adaptador HTTP (`http` y `http_request`). **Mitigación:** Documentado aquí y en el README del engine.

---

## Deuda Técnica Documentada

| ID | Descripción | Prioridad |
|----|-------------|-----------|
| DT-001 | Parsear `RetryPolicy.Interval` (p.ej. `"2s"`, `"1m"`) | Media |
| DT-002 | Propagar `context.Context` en `Activity.Execute` | Baja |
| DT-003 | Migrar `log.Printf` a `slog` (logging estructurado) | Baja |
| DT-004 | Reutilizar `http.Client` entre peticiones (connection pool) | Baja |
