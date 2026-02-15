# Diseño de Arquitectura: JSON-Flow (Microservices-Based)

## 1. Visión General
La arquitectura de JSON-Flow se divide en dos planos principales: el **Control Plane** (Gestión y Diseño) y el **Data Plane** (Ejecución). El sistema sigue un modelo de "Microservicio por Flujo", donde cada proceso diseñado se despliega como una unidad de computación independiente y aislada.

---

## 2. Componentes del Sistema

### A. Control Plane (Capa de Gestión)
Es el punto central de administración. No ejecuta los flujos, pero gestiona su ciclo de vida.
* **Designer UI (React):** Interfaz visual donde el usuario construye el grafo del proceso y escribe las transformaciones en TS/JS.
* **Manager API (Node.js/Go):** Orquestador de la plataforma. Gestiona usuarios, proyectos, credenciales y el versionado del DSL.
* **Provisioner / Orchestrator:** Encargado de hablar con el orquestador de contenedores (Kubernetes/Docker) para levantar o bajar las instancias de los flujos.
* **Node Registry:** Catálogo que contiene las definiciones de los nodos (propiedades, iconos, esquemas de entrada/salida).

### B. Data Plane (Capa de Ejecución)
Compuesta por múltiples **Runners**. Cada Runner es un microservicio dedicado a un único flujo.
* **Runtime Engine (Go):** Binario ligero que interpreta el JSON DSL.
* **JS/TS Sandbox (Goja/V8):** Entorno aislado para ejecutar las funciones de transformación sin comprometer al host.
* **Connector Libs:** Drivers embebidos para protocolos (HTTP, SOAP, SQL, gRPC, etc.).
* **In-Memory Context:** Objeto que mantiene el estado temporal del flujo mientras se ejecuta un mensaje.

### C. Capa de Persistencia y Mensajería
* **Config DB (PostgreSQL):** Almacena las definiciones de los flujos (JSON DSL).
* **Audit DB (PostgreSQL + JSONB):** Base de datos optimizada para el historial de ejecuciones (logs tipo TIBCO).
* **Internal Bus (NATS/Redis):** Canal de comunicación asíncrono para enviar eventos de auditoría desde los Runners al sistema de logging sin añadir latencia al flujo.

---

## 3. Diagrama de Flujo de Datos y Comunicación



1.  **Diseño:** El usuario guarda un flujo -> Se almacena el DSL en **Config DB**.
2.  **Despliegue:** El **Manager** ordena al **Provisioner** crear un Pod de Kubernetes con la imagen del `Runner`.
3.  **Arranque:** El `Runner` carga su ID de flujo, descarga el DSL y activa sus **Triggers** (ej. abre un puerto HTTP).
4.  **Ejecución:** Al recibir un mensaje, el `Runner` procesa cada nodo.
5.  **Auditoría:** Al finalizar cada nodo, el `Runner` dispara un evento asíncrono al **Bus Interno**.
6.  **Persistencia:** Un **Audit Worker** consume el bus y guarda la traza en la **Audit DB**.

---

## 4. Matriz de Responsabilidades por Microservicio

| Servicio | Lenguaje | Rol Principal | Persistencia |
| :--- | :--- | :--- | :--- |
| **Manager API** | Node.js/Go | Gestión de flujos y Auth | PostgreSQL (Config) |
| **Runner (Flow)** | Go | Ejecución y Transformación | Memoria Volátil |
| **Audit Logger** | Go/Rust | Escritura masiva de trazas | PostgreSQL (Audit) |
| **Gateway** | Kong/Nginx | Enrutamiento de Webhooks | N/A |

---

## 5. Estrategia de Aislamiento y Resiliencia
* **Aislamiento de Recursos:** Al ser microservicios independientes, un "Out of Memory" en el Flujo A no afecta al Flujo B.
* **Stateful Audit:** Si el Runner muere, la última traza guardada en la Audit DB permite saber exactamente dónde se quedó el proceso para un posible "Replay".
* **Hot-Reload:** Las actualizaciones de flujos se gestionan mediante despliegues "Rolling Update" de Kubernetes, asegurando que no haya pérdida de mensajes.