# Especificación de Persistencia y Trazabilidad

## 1. Modelo de Datos de Auditoría
El sistema utiliza **PostgreSQL con extensiones JSONB** para garantizar la trazabilidad completa de los mensajes. El diseño se basa en dos niveles:

* **Executions:** Registro de alto nivel de cada vez que un flujo es disparado. Incluye metadatos de rendimiento y estado global.
* **Activity Logs:** Registro granular de cada actividad/nodo procesado. Almacena de forma inmutable los payloads de entrada (`input_data`) y salida (`output_data`).

## 2. Capacidades de Búsqueda Avanzada
Gracias a los índices GIN, el sistema permite realizar consultas como:
* "Buscar todas las ejecuciones donde el JSON de salida contenía `order_id: 12345`".
* "Listar flujos que fallaron en el nodo de conexión a SAP en las últimas 2 horas".

## 3. Gestión de Replay
El Replay se implementa mediante la clonación de payloads históricos:
1. **Full Replay:** Reinicio del flujo desde el Trigger original.
2. **Partial Replay:** Reinicio desde un nodo intermedio utilizando el estado persistido en el último punto de éxito (Checkpoint).

## 4. Estrategia de Escalabilidad
* **Asynchronous Logging:** Los Runners no escriben en DB; emiten eventos a un bus de mensajes (NATS/Redis).
* **Partitioning:** Las tablas de logs se particionan dinámicamente por fecha.
* **Archiving:** Integración nativa con S3 para almacenamiento de logs históricos de bajo costo.