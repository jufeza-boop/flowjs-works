# Análisis de Code Smells — Go Engine (Claude)

> Análisis enfocado en el motor de ejecución Go (`services/engine`).
> Complementa el documento general `code-smells-analysis.md`.

---

## 1. Magic Numbers — Constantes sin nombre

### Problema

El código usa literales numéricos dispersos sin nombre semántico:

```go
// executor.go — intervalo de reintento hardcodeado
time.Sleep(2 * time.Second)

// http.go — timeout por defecto
timeout := 30 * time.Second

// script.go — timeout JS
timeoutMs := 5000

// mail.go — puerto SMTP
port := 587

// sql.go — timeout SQL
timeoutSec := 30

// file.go — permisos UNIX
f, err := os.OpenFile(path, flag, 0644)
```

### Impacto

- Dificulta búsqueda/modificación centralizada.
- Riesgo de inconsistencias al cambiar un valor en un solo sitio.

### Corrección

```go
// En cada paquete, al principio del fichero:
const (
    defaultHTTPTimeoutSec    = 30
    defaultScriptTimeoutMs   = 5_000
    defaultSMTPPort          = 587
    defaultSQLTimeoutSec     = 30
    defaultFilePermission    = 0o644
    defaultRetryIntervalSecs = 2
)
```

---

## 2. Código Duplicado — Enrutamiento de Transiciones

### Problema

La lógica de clasificación y despacho de transiciones aparece dos veces:

```go
// executeChain (línea ~291)
var condTrans, noCondTrans, successTrans []models.Transition
for _, t := range transitions {
    switch t.Type {
    case "condition":  condTrans = append(...)
    case "nocondition": noCondTrans = append(...)
    case "success":    successTrans = append(...)
    }
}
// ... dispatch logic ...

// ExecuteFromNode (línea ~204) — idéntico
var condTrans, noCondTrans, successTrans []models.Transition
for _, t := range transMap[startNodeID] { ... }
```

### Corrección

Extraer función privada:

```go
// routeTransitions clasifica las transiciones de un nodo por tipo.
func routeTransitions(transitions []models.Transition) (cond, noCond, success, errTrans []models.Transition) {
    for _, t := range transitions {
        switch t.Type {
        case "condition":   cond    = append(cond, t)
        case "nocondition": noCond  = append(noCond, t)
        case "success":     success = append(success, t)
        case "error":       errTrans = append(errTrans, t)
        }
    }
    return
}
```

---

## 3. Variable Shadowing — process_store.go

### Problema

```go
func (s *ProcessStore) List(ctx context.Context, statusFilter string) ([]ProcessSummary, error) {
    ...
    for rows.Next() {
        var s ProcessSummary   // ← oculta el receptor 's *ProcessStore'
        if err := rows.Scan(&s.ID, ...); err != nil { ... }
        result = append(result, s)
    }
}
```

### Impacto

- Confusión al leer el código.
- Riesgo de errores al acceder accidentalmente a métodos del store en lugar de campos del summary.

### Corrección

```go
var summary ProcessSummary
if err := rows.Scan(&summary.ID, &summary.Version, &summary.Name,
    &summary.Status, &summary.TriggerType, &summary.UpdatedAt); err != nil {
    ...
}
result = append(result, summary)
```

---

## 4. Actividad HTTP con tipo incorrecto en Registry

### Problema

`NewActivityRegistry()` registra la actividad HTTP bajo el nombre `"http"`, pero los tests y parte de la documentación DSL usan `"http_request"`. Esto causa fallos de test:

```
FAIL: TestExecute_HttpRequestActivityRegistered
FAIL: TestExecute_HttpRequestMissingURL
```

### Corrección

Registrar el alias adicional:

```go
registry.Register(&HTTPActivity{})
registry.Register(&HTTPRequestActivity{}) // alias: type HTTPRequestActivity = HTTPActivity
```

O bien añadir un segundo `Name()` mediante un tipo separado:

```go
// HTTPRequestActivity es un alias de HTTPActivity con el nombre "http_request".
type HTTPRequestActivity struct{ HTTPActivity }
func (a *HTTPRequestActivity) Name() string { return "http_request" }
```

---

## 5. Política de Reintento Ignora el Campo `Interval`

### Problema

```go
// RetryPolicy tiene:
type RetryPolicy struct {
    MaxAttempts int    `json:"max_attempts"`
    Interval    string `json:"interval"`   // ← nunca se usa
    Type        string `json:"type"`
}

// El ejecutor siempre duerme 2 s fijo:
time.Sleep(2 * time.Second)
```

### Corrección Mínima

Extraer constante para claridad; el parsing completo del campo `Interval` como deuda técnica documentada:

```go
const defaultRetryIntervalSecs = 2

time.Sleep(defaultRetryIntervalSecs * time.Second)
```

---

## 6. Context No Propagado en executeNode

### Problema

`executeNode` no recibe ni propaga `context.Context`. Esto impide cancelaciones externas y limita la observabilidad.

### Impacto: Baja (mejora futura)

La corrección completa requiere cambiar la firma de la interfaz `Activity.Execute` e impacta todos los adaptadores. Se deja como deuda técnica documentada en el ADR.

---

## 7. Formato de Código (gofmt)

Los ficheros `script.go`, `sql.go`, `file.go`, `rabbitmq.go` y `transform.go` usan espacios en lugar de tabuladores (violación de `gofmt`).

**Corrección:** `gofmt -w ./...` desde la raíz del módulo.
