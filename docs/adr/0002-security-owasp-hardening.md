# ADR 0002 — Hardening de Seguridad OWASP Top 10 (2021)

- **Estado:** Aceptado
- **Fecha:** 2026-03-08
- **Scope:** `services/engine` (Go) · `services/audit-logger` (Go)
- **Autores:** Copilot Agent (basado en auditoría OWASP Top 10 — 2021)

---

## Contexto

Se realizó una auditoría de seguridad sobre el código bajo las pautas de **OWASP Top 10 (2021)**. El sistema opera en un entorno de producción con datos sensibles (trazas de ejecución, credenciales cifradas de integraciones, datos de procesos desplegados).

Se identificaron las siguientes vulnerabilidades activas:

| ID | Riesgo | Descripción |
|-----|--------|-------------|
| A02 | 🔴 Alto | Cabecera HSTS ausente; el navegador no fuerza HTTPS |
| A05 | 🔴 Alto | CORS configurado con `Access-Control-Allow-Origin: *` en ambos servicios |
| A05 | 🟡 Medio | Sin cabeceras de seguridad defensivas (X-Frame-Options, X-Content-Type-Options, CSP) |
| A04 | 🟡 Medio | Sin Rate Limiting; cualquier IP puede saturar los endpoints |
| A05 | 🟡 Medio | Mensajes de error exponen detalles internos de la base de datos (`err.Error()` enviado al cliente) |
| A09 | 🟡 Medio | Sin trazas de auditoría de acceso HTTP (IP, método, ruta, código de respuesta) |
| CI  | 🔴 Alto | `golangci-lint-action@v6` no soporta `golangci-lint v2.11.2`; CI permanentemente roto |

---

## Decisiones adoptadas

### 1. Middleware de seguridad centralizado (nuevo paquete `internal/middleware`)

**Problema:** Los servicios `engine` y `audit-logger` tenían su propia implementación ad-hoc de CORS, y carecían de todos los demás controles de seguridad.

**Decisión:** Crear `internal/middleware/security.go` en cada servicio con los siguientes componentes:

- **`CORS(origins []string)`** — Middleware que compara el header `Origin` contra una lista blanca explícita. Nunca emite `Access-Control-Allow-Origin: *`.
- **`AllowedOrigins()`** — Lee `ALLOWED_ORIGINS` (variable de entorno, comma-separated). Llama a `log.Fatalf` si la variable no está definida y `APP_ENV != "development"`, evitando arranques accidentales con CORS abierto en producción.
- **`SecurityHeaders`** — Middleware que inyecta en cada respuesta:
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains` (**A02** HSTS)
  - `X-Frame-Options: DENY` (anticlickjacking, **A05**)
  - `X-Content-Type-Options: nosniff` (**A05**)
  - `X-XSS-Protection: 0` (desactiva filtro legacy; usar CSP)
  - `Referrer-Policy: strict-origin`
  - `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`
- **`RateLimiter`** — Limitador por IP basado en ventana deslizante (100 req/min por IP por defecto). Responde HTTP 429 cuando se supera el límite. Incluye goroutine de limpieza periódica para evitar unbounded memory growth (**A04**).
- **`RequestLogger`** — Middleware que registra cada request con `event=HTTP_REQUEST ip=... method=... path=... status=... ts=...`. Nunca registra contraseñas, tokens completos ni datos financieros (**A09**).
- **`SanitizeError(err, detail)`** — Devuelve el mensaje genérico `detail` en producción, o `detail: <err>` en desarrollo. Protege contra la exposición de errores internos de la base de datos al cliente (**A05**).

**Chain de middleware en ambos servicios:**
```
RequestLogger → RateLimiter → CORS → SecurityHeaders → mux
```

**Consecuencia:** Un único punto de configuración y auditoría para todos los controles de seguridad HTTP.

---

### 2. Restricción CORS: de `*` a lista blanca (A05)

**Problema:** Ambos servicios configuraban `Access-Control-Allow-Origin: *`, lo que permite peticiones cross-origin desde cualquier dominio, incluyendo sitios atacantes.

**Decisión:**
- Leer `ALLOWED_ORIGINS` como variable de entorno (comma-separated list de orígenes permitidos, e.g. `https://app.example.com,https://admin.example.com`).
- El middleware `CORS` emite el header sólo si el `Origin` de la petición está en la lista blanca.
- En `development` sin variable configurada, se usa `http://localhost:5173` como fallback.
- En cualquier otro `APP_ENV`, la ausencia de `ALLOWED_ORIGINS` provoca `log.Fatalf` en el arranque.
- Se eliminan las funciones `corsMiddleware` legacy de ambos servicios.

**Consecuencia:** Las peticiones de orígenes no autorizados no reciben el header CORS, bloqueando efectivamente el acceso cross-origin desde dominios no permitidos.

---

### 3. Cabeceras de Seguridad HTTP (A02 · A05)

**Problema:** Ninguno de los servicios enviaba cabeceras de seguridad estándar.

**Decisión:** El middleware `SecurityHeaders` inyecta obligatoriamente las cabeceras descritas en §1. La cabecera HSTS es crítica para forzar HTTPS en producción (**A02**).

**Consecuencia:** Los navegadores modernos rechazarán peticiones HTTP en claro, cargas en iframe y sniffing de MIME-type.

---

### 4. Rate Limiting por IP (A04)

**Problema:** Cualquier IP podía enviar peticiones ilimitadas a los endpoints del motor (incluyendo `/v1/flow`, que ejecuta procesos DSL), exponiéndolo a ataques de DoS o fuerza bruta.

**Decisión:** Implementar `RateLimiter` con ventana deslizante (100 req/min por IP). Cuando se supera el límite:
- Se responde HTTP 429 Too Many Requests.
- Se emite un log de seguridad con `event=RATE_LIMITED`.
- El límite y la ventana son configurables para tests mediante `NewRateLimiterWithConfig`.

**Consecuencia:** Protección básica contra saturación y credential stuffing. Para entornos de alta carga, considerar Redis-backed rate limiting en futura versión.

---

### 5. Sanitización de Errores (A05)

**Problema:** Múltiples handlers en ambos servicios enviaban `err.Error()` directamente al cliente, exponiendo detalles de errores de PostgreSQL (nombres de tablas, consultas, credenciales de conexión).

**Decisión:** 
- Usar `middleware.SanitizeError(err, detail)` que devuelve el mensaje genérico en producción.
- Los detalles completos del error se registran siempre en el log del servidor mediante `log.Printf`.
- En modo `development` (`APP_ENV=development`) se devuelve el error completo para depuración.

**Consecuencia:** El cliente sólo recibe mensajes genéricos en producción ("failed to list secrets", "failed to query executions"). Los operadores siguen teniendo acceso al detalle completo en los logs del servidor.

---

### 6. Logging de Eventos de Seguridad (A09)

**Problema:** No existía un registro de acceso HTTP que permitiera detectar patrones anómalos (accesos masivos, IPs sospechosas, intentos de enumeración).

**Decisión:** 
- El middleware `RequestLogger` registra cada petición con formato estructurado: `event=HTTP_REQUEST ip=<ip> method=<method> path=<path> status=<status> ts=<RFC3339>`.
- El middleware `RateLimiter` registra `event=RATE_LIMITED` cuando bloquea una IP.
- La función `SecurityLog` garantiza el formato consistente y puede ser reemplazada por un logger estructurado (slog, zap) en el futuro sin cambiar los call-sites.
- **Nunca se registran**: contraseñas, tokens completos, payloads de usuario, datos financieros.

**Consecuencia:** Trazabilidad completa de accesos para análisis forense y detección de anomalías.

---

### 7. Shutdown graceful en engine-server (A09 · estabilidad)

**Problema:** El servidor del motor no manejaba señales del sistema (`SIGINT`/`SIGTERM`), lo que impedía liberar recursos de forma ordenada al detener el proceso.

**Decisión:** Añadir manejo de señales con `context.WithTimeout` de 10 segundos para llamar `server.Shutdown`. Esto garantiza que el `RateLimiter` y demás recursos se limpian correctamente.

**Consecuencia:** El motor puede actualizarse sin forzar cierres abruptos de conexiones en curso.

---

### 8. Fix CI: actualización de `golangci-lint-action` (Bug CI)

**Problema:** El workflow `.github/workflows/lint.yml` usaba `golangci-lint-action@v6` pero especificaba la versión `v2.11.2` de golangci-lint. La acción v6 no soporta la serie v2 del linter, provocando que todos los jobs de lint fallaran con el error `invalid version string 'v2.11.2'`. Esto dejó el CI permanentemente roto.

**Decisión:** Actualizar la acción a `golangci-lint-action@v7`, que es la primera versión con soporte oficial para `golangci-lint v2.x`.

**Consecuencia:** Los jobs `Go lint (services/engine)` y `Go lint (services/audit-logger)` pueden ejecutarse correctamente.

---

## Vulnerabilidades NO cubiertas en este ADR (backlog)

| ID OWASP | Descripción | Razón de exclusión |
|----------|-------------|-------------------|
| A01 | Autenticación/Autorización de endpoints API | Los servicios son APIs internas del Control Plane. La autenticación de usuarios está fuera del scope actual del motor (no existe módulo de identidad). Se recomienda añadir un API-key gate o integrar con el Identity Provider del gateway (Kong/Nginx) en una futura iteración. |
| A03 | SQL Injection | Las consultas ya usan Prepared Statements / consultas parametrizadas en todos los handlers. No se detectaron concatenaciones directas de input. |
| A07 | Bloqueo de cuenta / lockout | No existen endpoints de autenticación en estos servicios (login, reset password). Aplica al módulo de identidad futuro. |

---

## Consecuencias generales

### Positivas
- Wildcard CORS eliminado de todos los servicios.
- Cabeceras de seguridad HSTS y anti-clickjacking activas en producción.
- Rate limiting básico protege contra DoS y fuerza bruta.
- Los mensajes de error ya no exponen detalles de la infraestructura.
- Todos los accesos HTTP quedan registrados con IP, método, ruta y código de respuesta.
- El CI vuelve a funcionar tras la corrección de `golangci-lint-action`.

### Pendientes (backlog)
- **Autenticación API**: Añadir API-key middleware o integrar con el Identity Provider del gateway.
- **Rate limiting distribuido**: Para escalado horizontal, reemplazar el limitador in-memory por Redis-backed (e.g. `go-redis/redis_rate`).
- **Rotación automática de `SECRETS_AES_KEY`**: El sistema actual no soporta re-cifrado online al rotar la clave.
- **TLS termination**: Aunque HSTS se envía, el servidor Go actualmente no configura TLS directamente. Se asume terminación TLS en el reverse proxy (Kong/Nginx). Documentar este requisito de despliegue.
