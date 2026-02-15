# Roadmap de Desarrollo: JSON-Flow (MVP)

## Fase 1: Core Engine (Headless)
- [ ] Implementación del Parser de DSL en Go.
- [ ] Motor de ejecución de Scripts (Goja Integration).
- [ ] Nodo HTTP y Logger básico.
- [ ] Pruebas unitarias de paso de contexto (Payload propagation).

## Fase 2: Visual Designer (No-Code UI)
- [ ] Canvas interactivo con React Flow.
- [ ] Serializador: UI Graph -> JSON DSL.
- [ ] Librería de componentes para nodos (Icons, Inputs, Outputs).

## Fase 3: Observabilidad (Auditoría)
- [ ] Esquema de base de datos Postgres (JSONB).
- [ ] Servicio de logging asíncrono en el Runner.
- [ ] UI de trazabilidad: "Step-by-step Execution Viewer".

## Fase 4: Data Mapper Avanzado
- [ ] Interfaz de mapeo visual (Source -> Target).
- [ ] Integración de Monaco Editor para transformaciones TS/JS.
- [ ] Previsualización de datos en tiempo real.

## Fase 5: Infraestructura Cloud-Native
- [ ] Containerización de Runners (Docker).
- [ ] API de gestión (Control Plane).
- [ ] Implementación de triggers avanzados (Scheduler, Webhook).


Estrategia de Trabajo: "Interface-First"
Para que la IA no alucine, tú definirás siempre los Contratos (Interfaces/Types) en archivos .ts o .go y le pedirás a la IA que implemente la lógica interna.

Fase 1: El Corazón (Motor Runner en Go) - Semanas 1-2
Objetivo: Un binario que reciba un JSON por CLI y ejecute dos nodos secuenciales.

Tarea 1.1: Definir las estructuras de datos (Structs) del DSL en Go a partir del JSON que ya diseñamos.

Tarea 1.2: Implementar el "Orquestador Lineal": una función que recorra el array nodes y pase el output de uno al input del siguiente.

Tarea 1.3: Integrar Goja (motor JS en Go).

Prompt para la IA: "Crea un wrapper en Go para Goja que reciba un objeto JSON y un string de código JS, lo ejecute en un entorno seguro y devuelva el JSON resultante".

Tarea 1.4: Crear el primer nodo real: HTTP Request.

Fase 2: El Diseñador Visual (React + React Flow) - Semanas 3-4
Objetivo: Una UI donde puedas arrastrar nodos y exportar el JSON DSL.

Tarea 2.1: Setup de React + Tailwind + React Flow.

Tarea 2.2: Crear la "Paleta de Nodos": componentes visuales para Trigger, HTTP, Script y DB.

Tarea 2.3: Lógica de exportación: Convertir el grafo de React Flow al formato JSON de nuestro DSL.

Prompt para la IA: "Escribe un helper en TypeScript que recorra los nodes y edges de React Flow y genere un JSON siguiendo este esquema [adjuntar esquema DSL]".

Fase 3: Auditoría y Persistencia (Postgres) - Semanas 5-6
Objetivo: Que cada ejecución deje rastro y se pueda consultar.

Tarea 3.1: Setup de Docker Compose con PostgreSQL.

Tarea 3.2: Implementar el Audit Logger asíncrono.

Prompt para la IA: "Crea un servicio en Go que escuche eventos en un canal interno y realice inserciones en lote (batch insert) en una tabla de Postgres optimizada para JSONB".

Tarea 3.3: Pantalla de "Execution History" en el frontend para ver los payloads de entrada/salida de cada nodo.

Fase 4: El Data Mapper (La Joya de la Corona) - Semanas 7-8
Objetivo: La interfaz para unir campos con flechas y lógica JS.

Tarea 4.1: Crear el componente de "Mapeador de dos columnas".

Tarea 4.2: Integrar Monaco Editor (el motor de VS Code) en una ventana modal para escribir el JS de cada nodo.

Tarea 4.3: Sistema de "Live Test": Un botón en la UI que envíe el mapeo actual al Runner y muestre el resultado instantáneamente.

Fase 5: Microservicios y Despliegue - Semanas 9+
Objetivo: Convertir el Runner en un servicio que escuche peticiones reales.

Tarea 5.1: Crear el Manager API que guarde los flujos en DB.

Tarea 5.2: Dockerizar el Runner para que pueda levantarse una instancia por cada flujo (aislamiento).

Tarea 5.3: Implementar el "Control de Replays" desde la UI.