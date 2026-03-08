# Code Smells Analysis — Go Services

## Metodología

- **Fecha:** 2026-03-06
- **Scope:** `services/`
- **Criterios:** Martin Fowler (*Refactoring* 2ª ed.) + Go idioms + OWASP
- **Herramientas:** Inspección manual + análisis de patrones de código
- **Archivos analizados:** 38 fuentes `.go` en `services/engine/` y `services/audit-logger/`

---

## 🚨 Code Smells Identificados

---

### 1. DUPLICATE CODE — Closure `getCredential` triplicada — Severidad: 🔴

**Ubicación:** `engine/internal/activities/sftp.go:256-264` · `s3.go:219-227` · `mail.go:94-102`

**Código:**
```go
// sftp.go:256-264 (idéntico en s3.go y mail.go)
getCredential := func(key string) string {
    if authMap, ok := config["auth"].(map[string]interface{}); ok {
        if v, ok := authMap[key].(string); ok {
            return v
        }
    }
    v, _ := config[key].(string)
    return v
}
```

**Problema:** Una closure de 7 líneas de extracción de credenciales está copiada literalmente en tres actividades distintas (`sftp`, `s3`, `mail`). Es precisamente el *Duplicate Code* que Fowler identifica como el "smell más fundamental": la misma lógica en múltiples lugares garantiza que un cambio requiere editar N sitios.

**Impacto:** Si la estrategia de resolución de credenciales cambia (p.ej. soporte para variables de entorno), hay que actualizar tres archivos. Ya existe una ligera divergencia: `smb.go` extrae las credenciales en `extractSMBAuth` (función global) mientras los otros tres usan la closure inline — divergencia que tenderá a ampliarse.

**Refactor sugerido:**
```go
// internal/activities/credentials.go
func getCredential(config map[string]interface{}, key string) string {
    if authMap, ok := config["auth"].(map[string]interface{}); ok {
        if v, ok := authMap[key].(string); ok {
            return v
        }
    }
    v, _ := config[key].(string)
    return v
}
```

---

### 2. DUPLICATE CODE — Patrón `*Put` triplicado (SFTP / SMB / S3) — Severidad: 🔴

**Ubicación:** `sftp.go:152-205` · `smb.go:160-204` · `s3.go:142-207`

**Código:**
```go
// Las tres implementaciones siguen el mismo esqueleto:
func sftpPut / smbPut / s3Put(...) (map[string]interface{}, error) {
    localFolder, _ := config["local_folder"].(string)
    if localFolder == "" { localFolder = "." }

    overwrite := true
    if ow, ok := config["overwrite"].(bool); ok { overwrite = ow }

    var fileNames []string
    if flist, ok := config["files"].([]interface{}); ok {
        for _, f := range flist {
            if s, ok := f.(string); ok { fileNames = append(fileNames, s) }
        }
    }
    // ... iteración + check de existencia + upload + acumular en `uploaded`
    return map[string]interface{}{"files_uploaded": uploaded, "count": len(uploaded)}, nil
}
```

**Problema:** La estructura completa de "upload de ficheros" (leer `localFolder`, `overwrite`, `fileNames` de config; iterar; verificar existencia; subir; devolver `{files_uploaded, count}`) está duplicada en tres actividades. Lo mismo ocurre con el patrón `*Get` (lista remota → filtro regex → descargar → devolver `{files_downloaded, count}`). Son ~50 líneas por triplicado.

**Impacto:** Añadir una feature (e.g. dry-run mode, progress logging) o corregir un bug en la lógica de overwrite requiere tres cambios idénticos. El patrón de compilación anticipada de `regex_filter` más la doble compilación también está triplicado (`sftp.go:63-66,114-116`; `smb.go:67-70,122-124`; `s3.go:60-63,91-93`).

**Refactor sugerido:** Extraer una función genérica de upload basada en callbacks:
```go
type RemoteFS interface {
    Stat(path string) (os.FileInfo, error)
    Create(path string) (io.WriteCloser, error)
    // ...
}
func filePut(fs RemoteFS, config map[string]interface{}, remoteFolder string) (map[string]interface{}, error)
```

---

### 3. DUPLICATE CODE — Clasificación de transiciones en `executor.go` — Severidad: 🟡

**Ubicación:** `engine/internal/engine/executor.go:204-214` y `291-301`

**Código:**
```go
// Instancia 1 — ExecuteFromNode:204
var condTrans, noCondTrans, successTrans []models.Transition
for _, t := range transMap[startNodeID] {
    switch t.Type {
    case "condition":  condTrans  = append(condTrans, t)
    case "nocondition": noCondTrans = append(noCondTrans, t)
    case "success":    successTrans = append(successTrans, t)
    }
}

// Instancia 2 — executeChain:291 (código idéntico)
var condTrans, noCondTrans, successTrans []models.Transition
for _, t := range transitions {
    switch t.Type { /* ... exactamente igual ... */ }
}
```

**Problema:** El bloque de clasificación de transiciones por tipo aparece dos veces en el mismo archivo. Además, el bloque de *dispatch* condicional/nocondición (`if len(condTrans) > 0 || len(noCondTrans) > 0 { ... } else { ... for successTrans ... }`) también está duplicado en `ExecuteFromNode:216-239` y `executeChain:303-322`.

**Impacto:** Añadir un nuevo tipo de transición (e.g. `"parallel"`) requiere modificar dos sitios. El código de `ExecuteFromNode` es esencialmente `executeChain` con el primer nodo salteado — debería reutilizarlo.

**Refactor sugerido:**
```go
func classifyTransitions(transitions []models.Transition) (cond, noCond, success, errorT []models.Transition) {
    for _, t := range transitions {
        switch t.Type {
        case "condition":   cond    = append(cond, t)
        case "nocondition": noCond  = append(noCond, t)
        case "success":     success = append(success, t)
        case "error":       errorT  = append(errorT, t)
        }
    }
    return
}
```

---

### 4. LONG METHOD — `mailSend` — Severidad: 🟡

**Ubicación:** `engine/internal/activities/mail.go:47-211`

**Código:**
```go
func mailSend(config map[string]interface{}) (map[string]interface{}, error) {
    // 164 líneas:
    // 1. Extraer y validar config (host, port, security, contentType, subject, body)
    // 2. Construir listas to/cc
    // 3. Extraer credenciales
    // 4. Formatear headers del mensaje
    // 5. Switch por seguridad (TLS / STARTTLS / NONE) - cada rama ~20 líneas
    //    con auth → MAIL FROM → N×RCPT TO → DATA → Write
}
```

**Problema:** La función tiene 164 líneas y mezcla validación de config, construcción de mensaje, construcción de lista de destinatarios y tres variantes de protocolo SMTP. Es el *Long Method* de Fowler, agravado por el hecho de que dos de las tres ramas (`TLS` y `STARTTLS`) son casi idénticas.

**Impacto:** Imposible testear unitariamente la construcción del mensaje sin levantar un mock SMTP. Cualquier cambio en la lógica de auth o de recipients requiere editar hasta dos ramas.

**Refactor sugerido:** Extraer `buildSMTPMessage`, `buildRecipients`, y un helper `sendViaSMTPClient` que reciba el `client` ya conectado — las ramas TLS/STARTTLS solo difieren en cómo establecen la conexión, no en el protocolo de envío.

---

### 5. DUPLICATE CODE — Ramas TLS y STARTTLS en `mailSend` — Severidad: 🟡

**Ubicación:** `mail.go:124-156` (TLS) y `mail.go:162-199` (STARTTLS)

**Código:**
```go
// Rama TLS (línea 124) — ~32 líneas
if auth != nil { client.Auth(auth) }
client.Mail(fromUser)
recipients := append(toList, ccList...)
for _, r := range recipients { client.Rcpt(r) }
w, _ := client.Data()
w.Write(msgBytes)
w.Close()

// Rama STARTTLS (línea 162) — prácticamente idéntica
if auth != nil { client.Auth(auth) }
client.Mail(fromUser)
recipients := append(toList, ccList...)
for _, r := range recipients { client.Rcpt(r) }
w, _ := client.Data()
w.Write(msgBytes)
w.Close()
```

**Problema:** 20+ líneas de lógica SMTP (autenticación → MAIL FROM → RCPT TO × N → DATA → Write) están duplicadas verbatim entre la rama TLS y STARTTLS. La única diferencia entre ambas ramas es cómo se establece la conexión (TLS directo vs STARTTLS negotiation).

**Impacto:** Cualquier bug en el protocolo SMTP debe corregirse en dos lugares. El smoke-test de `append(toList, ccList...)` potencialmente muta `toList` via el slice header (bug latente) y está duplicado.

**Refactor sugerido:**
```go
func smtpSend(client *smtp.Client, auth smtp.Auth, from string, recipients []string, msg []byte) error {
    // una sola implementación compartida por TLS y STARTTLS
}
```

---

### 6. MAGIC NUMBERS — Severidad: 🟡

**Ubicación:** `executor.go:430` · `sftp.go:75,291` · `smb.go:77` · `script.go:43` · `http.go:39`

**Código:**
```go
// executor.go:430 — retry backoff sin nombre
time.Sleep(2 * time.Second)

// sftp.go:75 — TCP dial timeout hardcodeado
conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
// sftp.go:291 — SSH handshake timeout (mismo valor, diferente lugar)
return &ssh.ClientConfig{..., Timeout: 30 * time.Second}, nil

// smb.go:77 — TCP dial timeout idéntico al de sftp
conn, err := net.DialTimeout("tcp", addr, 30*time.Second)

// script.go:43 — timeout por defecto de ejecución JS en ms
timeoutMs := 5000

// http.go:39 — HTTP client timeout
timeout := 30 * time.Second
```

**Problema:** Múltiples timeout y delays están hardcodeados sin constantes nombradas. El valor `30*time.Second` aparece en al menos 4 lugares distintos (sftp TCP, sftp SSH, smb TCP, http client). El `5000` de `script.go` es ms pero no lleva unidad en el nombre.

**Impacto:** Si hay que aumentar el timeout de red en todos los conectores (p.ej. para redes lentas), hay que buscar cada aparición manualmente. El valor `2 * time.Second` del retry es especialmente opaco: ¿es un minimum? ¿un base para backoff exponencial?

**Refactor sugerido:**
```go
const (
    defaultNetDialTimeout = 30 * time.Second
    defaultSSHTimeout     = 30 * time.Second
    defaultHTTPTimeout    = 30 * time.Second
    defaultScriptTimeoutMs = 5_000
    retryBaseInterval     = 2 * time.Second
)
```

---

### 7. GLOBAL MUTABLE STATE — Registros de triggers — Severidad: 🟡

**Ubicación:** `engine/internal/triggers/rest.go:127` · `engine/internal/triggers/soap.go` (patrón análogo)

**Código:**
```go
// rest.go:127 — variable de paquete mutable
var globalRESTRegistry = newRESTRegistry()

// Usada directamente por los trigger handlers:
func (t *restTrigger) Start(ctx context.Context, proc *models.Process) error {
    ...
    globalRESTRegistry.register(path, method, func(...) { ... })
}

// Expuesta hacia el servidor:
func GetRegistryHandler() http.Handler {
    return globalRESTRegistry
}
```

**Problema:** La arquitectura del registro de triggers REST y SOAP se basa en variables globales de paquete (`globalRESTRegistry`, `globalSOAPRegistry`). Este patrón crea un singleton implícito que no puede ser reemplazado en tests, impide tener múltiples instancias del servidor en el mismo proceso, y acopla el paquete `triggers` con el ciclo de vida del programa.

**Impacto:** Los tests de integración del trigger REST no pueden crear aislamiento entre casos de test: el registro global contamina entre tests. Los tests existentes en `manager_test.go` no pueden verificar que el registro HTTP fue actualizado correctamente.

**Refactor sugerido:** Inyectar el registry como dependencia en el `Manager`:
```go
type Manager struct {
    executor     Executor
    restRegistry *restRegistryImpl   // inyectado, no global
    soapRegistry *soapRegistryImpl
    running      map[string]TriggerHandler
    mu           sync.Mutex
}
```

---

### 8. NEW HTTP CLIENT PER REQUEST (connection pool starvation) — Severidad: 🟡

**Ubicación:** `engine/internal/activities/http.go:44-47`

**Código:**
```go
func (a *HTTPActivity) Execute(input, config map[string]interface{}, ...) (...) {
    // ...
    timeout := 30 * time.Second
    // Se crea un nuevo cliente en cada invocación de Execute:
    client := &http.Client{
        Timeout: timeout,
    }
    // ...
    resp, err := client.Do(req)
```

**Problema:** En Go, `http.Client` contiene internamente un `http.Transport` con un pool de conexiones TCP. Al crear un nuevo `http.Client` en cada invocación, se crea también un nuevo `Transport` vacío. Esto desactiva el HTTP keep-alive y la reutilización de conexiones, resultando en un nuevo handshake TCP (y TLS) por cada nodo HTTP en el flujo.

**Impacto:** En flows con múltiples nodos HTTP hacia el mismo host (común en integraciones), el throughput cae drásticamente porque nunca se reutilizan conexiones. Puede agotar puertos efímeros bajo carga.

**Refactor sugerido:**
```go
type HTTPActivity struct {
    client *http.Client // compartido y reutilizado
}

func NewHTTPActivity() *HTTPActivity {
    return &HTTPActivity{
        client: &http.Client{Timeout: defaultHTTPTimeout},
    }
}
```

---

### 9. PRIMITIVE OBSESSION — `ExecutionContext.Nodes` — Severidad: 🟡

**Ubicación:** `engine/internal/models/context.go:19` · `SetNodeOutput:37` · `SetNodeStatus:44`

**Código:**
```go
// El estado de cada nodo es un map genérico accesible por clave string
type ExecutionContext struct {
    Nodes map[string]map[string]interface{} `json:"nodes"`
}

// Acceso por clave magic string "output" y "status":
func (ctx *ExecutionContext) SetNodeOutput(nodeID string, output map[string]interface{}) {
    if ctx.Nodes[nodeID] == nil {
        ctx.Nodes[nodeID] = make(map[string]interface{})
    }
    ctx.Nodes[nodeID]["output"] = output  // clave magic string
}

func (ctx *ExecutionContext) SetNodeStatus(nodeID string, status string) {
    ctx.Nodes[nodeID]["status"] = status  // clave magic string
}
```

**Problema:** El tipo del estado de nodo (`map[string]interface{}`) es un *Primitive Obsession*: acumula dos conceptos distintos (`output` y `status`) en un mapa genérico accesible por magic strings. No hay garantía en tiempo de compilación de que `"output"` o `"status"` existan; un typo ("outpt") pasaría silenciosamente.

**Impacto:** Todo consumidor del estado de nodo (triggers, handlers, audit logs) debe hacer type assertion manual sobre el map. El campo `NodeExecution` en `process.go` define un struct equivalente pero no se usa en `ExecutionContext`.

**Refactor sugerido:**
```go
type NodeState struct {
    Output map[string]interface{} `json:"output,omitempty"`
    Status string                 `json:"status"`
}

type ExecutionContext struct {
    Nodes map[string]*NodeState `json:"nodes"`
}
```

---

### 10. SECURITY CODE SMELL — AES Key insegura por defecto — Severidad: 🟡

**Ubicación:** `engine/cmd/server/main.go:598-607`

**Código:**
```go
func aesKeyFromEnv(envKey string) []byte {
    v := os.Getenv(envKey)
    if len(v) >= 32 {
        return []byte(v[:32])
    }
    // Dev fallback — never use in production
    const devKey = "flowjs-dev-key-00000000000000000"
    log.Printf("engine-server: WARNING — using insecure dev AES key; set %s in production", envKey)
    return []byte(devKey[:32])
}
```

**Problema:** Cuando `SECRETS_AES_KEY` no está configurada, el servidor arranca silenciosamente con una clave AES hardcodeada y conocida públicamente. El código es correcto en intención (hay un log de WARNING) pero inseguro en implementación: un despliegue que olvide configurar la variable seguirá funcionando, cifrando todos los secretos con una llave que cualquiera puede extraer del binario.

**Impacto:** Todos los secretos almacenados (credenciales SFTP, AWS keys, tokens, contraseñas SMTP) quedan comprometidos si se lleva accidentalmente la configuración de dev a prod. Este es un *Insecure Default Configuration* del OWASP Top 10.

**Refactor sugerido:** Hacer el arranque fatal cuando la clave no está configurada fuera del modo `dev` explícito:
```go
if len(v) < 32 {
    if os.Getenv("APP_ENV") != "development" {
        log.Fatalf("engine-server: SECRETS_AES_KEY must be set and >= 32 bytes in non-dev environments")
    }
    // solo entonces usar devKey...
}
```

---

### 11. DEAD CODE — `NodeExecution` struct sin uso — Severidad: ⚠

**Ubicación:** `engine/internal/models/process.go:84-90`

**Código:**
```go
// NodeExecution represents the result of executing a node
type NodeExecution struct {
    NodeID string                 `json:"node_id"`
    Status string                 `json:"status"` // success, error, warning
    Output map[string]interface{} `json:"output"`
    Error  string                 `json:"error,omitempty"`
}
```

**Problema:** `NodeExecution` define exactamente los campos que debería tener el estado de nodo en el `ExecutionContext` (ver smell #9), pero nunca se instancia en el código Go. Es un struct huérfano que fue probablemente diseñado con la intención correcta pero nunca conectado.

**Impacto:** Contribuye a la confusión conceptual (¿por qué hay dos formas de representar el resultado de un nodo?). El compilador no lo detecta porque no hay código que lo use, pero pasa lint.

**Refactor sugerido:** Eliminar `NodeExecution` o adoptarlo como reemplazo del `map[string]interface{}` en `ExecutionContext.Nodes` (refactor del smell #9).

---

### 12. DEAD CODE — `contextFromCtx` placeholder — Severidad: ⚠

**Ubicación:** `engine/internal/activities/s3.go:257-260`

**Código:**
```go
// contextFromCtx returns a Go context.Context for use in external API calls.
// Currently returns context.Background(); can be extended to propagate deadlines.
func contextFromCtx(_ *fmodels.ExecutionContext) context.Context {
    return context.Background()
}
```

**Problema:** Esta función recibe un `*models.ExecutionContext` y lo ignora completamente (`_`), devolviendo siempre un `context.Background()`. El `ProcessSettings.Timeout` definido en el DSL nunca se propaga a las llamadas externas de S3. El comentario lo reconoce pero la función actual es dead code funcional: no aporta ningún valor sobre llamar directamente a `context.Background()`.

**Impacto:** Los timeouts configurados en el DSL no se respetan para operaciones S3. Un bucket lento o no respondiente puede bloquear un nodo indefinidamente. El mismo problema aplica a SQL (`sql.go` sí maneja timeout pero desde `config["timeout"]`, no desde `ProcessSettings`).

**Refactor sugerido:** Propagar el timeout del DSL:
```go
func contextFromCtx(ctx *fmodels.ExecutionContext) context.Context {
    // TODO: extraer timeout de ProcessSettings cuando esté disponible en ExecutionContext
    return context.Background()
}
```
O bien conectarlo realmente: añadir `Timeout int` a `ExecutionContext` y usar `context.WithTimeout`.

---

### 13. NO-OP `UnmarshalJSON` — Severidad: ⚠

**Ubicación:** `engine/internal/models/process.go:92-101`

**Código:**
```go
// UnmarshalJSON custom unmarshaling to handle the process structure
func (p *Process) UnmarshalJSON(data []byte) error {
    type Alias Process
    aux := &struct {
        *Alias
    }{
        Alias: (*Alias)(p),
    }
    return json.Unmarshal(data, &aux)
}
```

**Problema:** Esta es una implementación estándar del patrón de alias para evitar recursión infinita en `UnmarshalJSON` — pero aquí no hay ninguna lógica custom. El método no hace nada distinto a lo que haría el unmarshal por defecto. Es likely que fue copiado de una plantilla sin necesidad.

**Impacto:** Código noise que confunde a los lectores (¿por qué se necesita custom unmarshal?). El método extra crea overhead al llamar a `json.Unmarshal` indirectamente. Si en el futuro alguien añade lógica custom, el patrón ya está en su lugar — pero en el estado actual es dead code.

**Refactor sugerido:** Eliminar el método; `encoding/json` hará exactamente lo mismo por defecto.

---

### 14. REDUNDANT TYPE CONVERSION — `interface{}(val)` — Severidad: ⚠

**Ubicación:** `engine/internal/models/context.go:114`

**Código:**
```go
case map[string]map[string]interface{}:
    val, ok := v[part]
    if !ok {
        return nil, fmt.Errorf("path not found: %s at part %s", path, part)
    }
    // Esta conversión es siempre implícita en Go y es ruido:
    current = interface{}(val)
```

**Problema:** En Go, la conversión explícita a `interface{}` nunca es necesaria — cualquier valor satisface `interface{}` implícitamente. Esta línea es equivalente a `current = val`. Es un código noise que sugiere que el autor no estaba seguro de las reglas de interfaz de Go, o que copió/pegó sin limpiar.

**Impacto:** Menor: solo confusión lectora. Pero indica que este codigo no fue revisado por `golangci-lint` (la regla `unconvert` detecta esto).

**Refactor sugerido:** `current = val`

---

### 15. GOFMT NO APLICADO — Formateo inconsistente — Severidad: ⚠

**Ubicación:** `engine/internal/activities/script.go` · `sql.go` · `transform.go` · `mail.go`

**Código:**
```go
// script.go — 0 indentación, sin tabs:
func executeScript(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
scriptCode, ok := config["script"]
if !ok {
return nil, fmt.Errorf("script not found in config")
}
```

**Problema:** Cuatro archivos del paquete `activities` no tienen `gofmt` aplicado. El cuerpo de las funciones empieza en la columna 0 sin indentación de tabs. El estándar Go es no negociable: `gofmt` es una herramienta oficial que produce un formato canónico único.

**Impacto:** Diffs enormes cuando alguien aplica `gofmt` por primera vez. Dificulta la integración con editores y linters. Indica ausencia de un `pre-commit` hook o CI lint check. El resto de los archivos del paquete sí están formateados correctamente, lo que sugiere que estos cuatro fueron añadidos en una sesión diferente sin pre-commit.

**Refactor sugerido:** `gofmt -w services/engine/internal/activities/script.go sql.go transform.go mail.go` y añadir `gofmt` al CI.

---

## Resumen por severidad

| Severidad | Cantidad | Smells |
|-----------|----------|--------|
| 🔴 Alta   | 2        | `getCredential` triplicada, Patrón put/get triplicado |
| 🟡 Media  | 8        | Clasificación transiciones duplicada, Long Method `mailSend`, Ramas TLS/STARTTLS duplicadas, Magic Numbers, Global Mutable State, HTTP Client por request, Primitive Obsession `ExecutionContext`, AES Key por defecto insegura |
| ⚠ Baja   | 5        | Dead Code `NodeExecution`, Dead Code `contextFromCtx`, No-op `UnmarshalJSON`, Conversión redundante `interface{}(val)`, `gofmt` sin aplicar |

## Prioridad de refactoring recomendada

1. **Extraer `getCredential` a función compartida** (mínimo esfuerzo, máximo impacto en DRY)
2. **`gofmt` + CI lint gate** (automatizable, coste cero a largo plazo)
3. **AES Key insegura** (riesgo de seguridad activo)
4. **`NodeExecution` + refactor `ExecutionContext.Nodes`** (elimina dead code y mejora type safety)
5. **Abstraer patrón put/get en FileActivity interface** (mayor esfuerzo, mayor ganancia)
6. **Extraer `classifyTransitions`** (pequeño, elimina la duplicación en executor)
7. **Resolver `contextFromCtx`** (conectar timeout del DSL a operaciones externas)
8. **HTTP Client como campo de `HTTPActivity`** (mejora de performance directo)
