# Especificación de Requisitos de Software (ERS) - Proyecto: "JSON-Flow"

## 1. Introducción
El sistema "JSON-Flow" es una plataforma de integración iPaaS de alto rendimiento diseñada para desarrolladores (Low-Code). Su objetivo es sustituir la pesadez de herramientas legacy basadas en XML (como TIBCO BW) por una arquitectura de microservicios ligera, nativa en JSON y extensible mediante JavaScript/TypeScript.

---

## 2. Requisitos Funcionales (RF)

### 2.1 Core del Motor (Runtime)
* **RF-01: Dualidad de Ejecución:** El motor debe permitir flujos **síncronos** (petición-respuesta inmediata) y **asíncronos** (procesamiento en segundo plano/event-driven).
* **RF-02: Orquestación de Pasos:** Capacidad de ejecutar nodos de forma secuencial, paralela (fork/join) y condicional.
* **RF-03: Persistencia de Estado (Stateful):** Cada payload generado por una actividad debe persistirse en una base de datos antes de pasar a la siguiente, garantizando la recuperación ante fallos.
* **RF-04: Gestión de Reintentos:** Configuración de políticas de reintento (backoff, jitter) a nivel de nodo individual.

### 2.2 Capacidad de Transformación (The Mapper)
* **RF-05: Motor de Scripting JS/TS:** Integración de un entorno de ejecución para JavaScript/TypeScript puro para realizar transformaciones de datos.
* **RF-06: Funciones de Agregación:** Capacidad de combinar múltiples fuentes de datos (outputs de nodos previos) en una estructura única.
* **RF-07: Lógica Estructural:** Soporte para transformación compleja:
    * Mapeo de Arrays a Objetos y viceversa.
    * Filtrado dinámico de colecciones.
    * Creación de estructuras anidadas profundas.

### 2.3 Conectividad y Protocolos
* **RF-08: Triggers (Disparadores):** Soporte nativo para:
    * **Webhooks:** Endpoints REST/SOAP.
    * **Tiempo:** Scheduler tipo Cron.
    * **Mensajería:** Consumidores de colas (RabbitMQ, Kafka).
    * **Polling:** Detección de cambios en Bases de Datos (SQL/NoSQL).
    * **File System:** Observadores de cambios en S3, SFTP y protocolos SMB/Samba.
* **RF-09: Catálogo de Actividades:** Nodos predefinidos para:
    * Comunicaciones: HTTP, gRPC, GraphQL, SOAP.
    * Datos: SQL (Postgres, MySQL, Oracle), NoSQL (Mongo, Redis).
    * Transferencia: SFTP, S3, sharepoint.
    * Especiales: MCP (Model Context Protocol) para IA y Nodos de Código personalizado.

### 2.4 Interfaz de Usuario (UX/UI)
* **RF-10: Diseñador Visual:** Lienzo de dibujo técnico (drag-and-drop) para el diseño de flujos.
* **RF-11: Editor de Transformación:** IDE integrado con resaltado de sintaxis y autocompletado para el mapeo en JS/TS.
* **RF-12: Gestor de Extensiones:** Interfaz visual para que el usuario suba y configure nuevos nodos (Custom Nodes) sin necesidad de recompilar el core.

---

## 3. Requisitos No Funcionales (RNF)

### 3.1 Arquitectura y Despliegue
* **RNF-01: Arquitectura de Microservicios:** Cada flujo debe poder desplegarse de forma independiente (aislamiento total de recursos).
* **RNF-02: Alto Rendimiento:** El motor debe priorizar una latencia mínima y un bajo consumo de memoria (Runtime ligero).
* **RNF-03: Escalabilidad Horizontal:** Capacidad de escalar instancias de flujos específicos según la carga detectada en las colas o triggers.

### 3.2 Persistencia y Auditoría
* **RNF-04: Auditoría Completa (TIBCO Style):** Registro obligatorio de:
    * Payload de entrada y salida por cada nodo.
    * Estado de la ejecución (Success/Error/Warning).
    * Tiempos de latencia interna por actividad.
* **RNF-05: Almacenamiento JSONB:** Uso de bases de datos preparadas para grandes volúmenes de JSON para permitir búsquedas dentro de los históricos de ejecución.

### 3.3 Extensibilidad
* **RNF-06: Hot-Reload:** Posibilidad de actualizar la lógica de un flujo sin afectar las ejecuciones en curso (Graceful Shutdown/Restart).

---

## 4. Stack Tecnológico de Referencia
* **Lenguaje Motor:** Go (Golang) o Rust (para eficiencia máxima).
* **Entorno JS:** V8 Engine o Goja (Go JavaScript Engine).
* **Base de Datos:** PostgreSQL (con optimización JSONB) o MongoDB.
* **Orquestación de Contenedores:** Kubernetes (K8s) nativo.
* **Frontend:** React.js con React Flow.