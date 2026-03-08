# Análisis de Code Smells — flowjs-works

## Resumen Ejecutivo

Este documento recoge los *code smells* identificados en el repositorio **flowjs-works** tras una revisión estática del código fuente. Los smells se clasifican por severidad (**Alta**, **Media**, **Baja**) y se incluye la acción correctiva recomendada para cada uno.

---

## 1. Magic Numbers y Magic Strings

### Severidad: Alta

| Fichero | Línea | Valor | Descripción |
|---------|-------|-------|-------------|
| `internal/engine/executor.go` | 430 | `2 * time.Second` | Intervalo de reintento hardcodeado |
| `internal/activities/http.go` | 39 | `30 * time.Second` | Timeout HTTP por defecto |
| `internal/activities/script.go` | 43 | `5000` | Timeout JS por defecto en ms |
| `internal/activities/mail.go` | 53 | `587` | Puerto SMTP por defecto |
| `internal/activities/sql.go` | 38 | `30` | Timeout SQL por defecto en segundos |
| `internal/activities/file.go` | 46 | `0644` | Permisos de fichero |

**Acción:** Extraer a constantes con nombre descriptivo en cada paquete.

---

## 2. Código Duplicado

### Severidad: Alta

#### 2a. Lógica de enrutamiento de transiciones

La lógica de clasificación y despacho de transiciones (`condTrans`, `noCondTrans`, `successTrans`) está duplicada entre `ExecuteFromNode` y `executeChain` en `executor.go`.

**Acción:** Extraer a función privada `routeTransitions`.

#### 2b. Extracción de listas de strings en actividad de mail

La extracción de `toList` y `ccList` a partir de `[]interface{}` está duplicada en `mail.go`.

**Acción:** Extraer a función privada `extractStringSlice`.

---

## 3. Variable Shadowing

### Severidad: Media

En `process_store.go`, el método `List` declara una variable local `var s ProcessSummary` que oculta (shadow) el receptor del método `s *ProcessStore`.

```go
// Problema: el receptor del método también se llama 's'
func (s *ProcessStore) List(...) {
    for rows.Next() {
        var s ProcessSummary  // ← shadow del receptor
```

**Acción:** Renombrar la variable local a `summary`.

---

## 4. Actividad HTTP no registrada con alias esperado

### Severidad: Alta

Los tests de integración usan el tipo `http_request` (por convención DSL), pero el `ActivityRegistry` sólo registra la actividad bajo el nombre `http`. Esto causa 2 fallos de test pre-existentes:

- `TestExecute_HttpRequestActivityRegistered`
- `TestExecute_HttpRequestMissingURL`

**Acción:** Registrar `HTTPActivity` también bajo el nombre `http_request`.

---

## 5. Política de reintento no respeta el campo `Interval`

### Severidad: Media

`RetryPolicy` define el campo `Interval string` (p.ej. `"2s"`, `"1m"`) pero el ejecutor siempre duerme `2 * time.Second` ignorando dicho campo.

**Acción:** Extraer constante `defaultRetryInterval`. El soporte completo de parsing del `Interval` puede abordarse en un ticket separado.

---

## 6. Formato de código inconsistente

### Severidad: Baja

Varios ficheros (`script.go`, `sql.go`, `file.go`, `rabbitmq.go`, `transform.go`) tienen indentación con espacios en lugar del tabulador estándar de Go.

**Acción:** Ejecutar `gofmt -w` sobre todos los ficheros afectados.

---

## Estado tras Refactorización

Véase `docs/adr/0001-estandares-refactorizacion-agentes.md` para las decisiones de arquitectura adoptadas durante esta refactorización.
