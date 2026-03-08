# 🤖 Guía de Estándares para Agentes de IA - flowjs-works

Este documento define las reglas de oro y estándares de calidad que todo agente de IA debe seguir al generar código para el ecosistema **flowjs-works**.

> 📚 **Extended knowledge base**: See `context/` folder for detailed architecture rules, DB schema, API specs, and DSL reference.

## 1. Contexto del Proyecto
**flowjs-works** es una plataforma iPaaS de alto rendimiento diseñada para sustituir herramientas legacy (como TIBCO BW) mediante una arquitectura ligera de microservicios basada en JSON.

- **Lenguaje Motor:** Go (Golang) para eficiencia máxima.
- **Transformación:** JavaScript/TypeScript (vía Goja/V8).
- **Persistencia:** PostgreSQL con JSONB para auditoría profunda.
- **Arquitectura:** Microservicio independiente por cada flujo (Data Plane) y un Control Plane central.

## 2. Reglas Generales de Comportamiento
1. **Prioridad de Estabilidad:** Nunca generes código sin manejar errores (`if err != nil`).
2. **Contexto de Datos:** Todas las referencias a datos deben usar la sintaxis JSONPath (ej. `$.trigger` o `$.nodes.ID.output`).
3. **No-Alucinación:** Si no conoces una librería de Go o una propiedad del DSL, pregunta en lugar de inventar.
4. **Interface-First:** Define siempre los contratos (Interfaces en Go, Types en TS) antes de la implementación lógica.

## 3. Estándares de Código (Go Engine)
- **Estructura:** Seguir la estructura de carpetas: `/cmd` (puntos de entrada) e `/internal` (lógica privada).
- **Concurrencia:** Usa Goroutines de forma responsable; cada nodo de actividad debe ser thread-safe.
- **Linter:** El código debe cumplir con `golangci-lint`. Evita funciones con complejidad ciclomática superior a 10.
- **Auditoría:** Cada actividad terminada debe emitir obligatoriamente un evento de auditoría asíncrono vía NATS.

## 4. Estándares de Testing (Calidad Obligatoria)
- **Cobertura Mínima:** 80% en lógica de negocio y motores de transformación.
- **Librerías:** Usa `testify/assert` y `testify/require` para aserciones legibles.
- **Mocks:** Es obligatorio usar mocks para llamadas externas (HTTP, DB, NATS). No dependas de servicios reales en tests unitarios.
- **Casos de Borde:** Incluye siempre tests para:
    - Payloads JSON mal formados.
    - Timeouts de red.
    - Referencias a nodos inexistentes en el contexto.

## 5. Estándares de UI (Designer App)
- **Componentes:** Usa React con Tailwind CSS.
- **Estado:** Gestión de flujos mediante React Flow; asegura que el grafo sea siempre serializable a JSON DSL.
- **Tipado:** TypeScript estricto. Prohibido el uso de `any`.

## 6. Definition of Done (DoD)
Para que una tarea se considere completada, el agente debe verificar:
- [ ] El código compila sin warnings.
- [ ] Se han incluido los archivos `_test.go` o `.test.ts` correspondientes.
- [ ] La lógica de resolución de variables `$.` funciona para el nuevo nodo.
- [ ] Se han actualizado las definiciones de tipos en el paquete compartido.
- [ ] Se ha documentado brevemente la nueva funcionalidad en los comentarios del código.

## 7. Instrucciones para Replay y Resiliencia
Al implementar nodos, recuerda que el sistema debe permitir el "Replay".
- Las actividades deben ser, en la medida de lo posible, idempotentes.
- El estado (`input_data` y `output_data`) debe ser serializable a JSON para guardarse en la `Audit DB`.

## 8. Estándares Derivados de Refactorización (ADR 0001)
Reglas obligatorias derivadas del análisis de *code smells*. Ver `docs/adr/0001-estandares-refactorizacion-agentes.md` para contexto completo.

### Go — `services/engine`
1. **No duplicar `getCredential`:** Usar siempre `getCredential(config, key)` del paquete `internal/activities/credentials.go`. Nunca copiar la closure en actividades individuales.
2. **No crear `http.Client` por request:** Usar `NewHTTPActivity()` o un campo `client *http.Client` compartido en la struct. No instanciar `http.Client` dentro de `Execute()`.
3. **Usar constantes para timeouts y durations:** No usar literales numéricos (`30*time.Second`, `5000`, etc.). Usar las constantes nombradas del paquete (`defaultHTTPTimeout`, `defaultScriptTimeoutMs`, `retryBaseInterval`).
4. **Aplicar `gofmt`:** Todo código Go generado debe estar correctamente formateado. El CI rechaza archivos sin formato canónico de tabs.
5. **Verificar `APP_ENV` para claves de seguridad:** Cuando `APP_ENV != "development"`, fallar en arranque (`log.Fatalf`) si falta una clave de seguridad o tiene menos de 32 bytes. Solo en `development` se permite fallback con advertencia explícita.

### TypeScript/React — `apps/designer/src`
6. **Usar `toErrorMessage` siempre:** No repetir `err instanceof Error ? err.message : String(err)`. Importar y usar `toErrorMessage(err)` de `lib/errors.ts`.
7. **Importar `DEFAULT_DEFINITION` desde `lib/serializer.ts`:** No redefinir la constante en otros archivos (ej. `App.tsx`).
8. **No variables de módulo mutables en React:** Usar `useRef` dentro del componente en lugar de `let counter` a nivel de módulo para contadores o estado mutable por instancia.
9. **Importar CSS class strings de `lib/classNames.ts`:** No duplicar strings de clases Tailwind (`inputClass`, `selectClass`, `labelClass`) en componentes individuales.

### General
10. **Extraer helpers para lógica duplicada:** Si la misma lógica aparece en más de dos lugares, extraer a función reutilizable antes de añadir una tercera copia.