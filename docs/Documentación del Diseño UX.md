# Diseño del Visual Data Mapper - JSON-Flow

## 1. Filosofía de Diseño
El Mapper debe ser "Code-Aware". Esto significa que aunque sea visual, no oculta la realidad del dato. Todo mapeo visual genera por debajo un bloque de código TypeScript que el desarrollador puede editar manualmente si lo desea.

## 2. Componentes de la Interfaz
* **Source Explorer:** Árbol jerárquico de salidas previas. Soporta búsqueda difusa (fuzzy search) de campos.
* **Target Canvas:** Representación visual del JSON Schema de destino. Permite definir campos como "requeridos" visualmente.
* **Transformation Layer (The Bridge):**
    * **Direct Link:** Conexión 1 a 1.
    * **Function Link:** Conexión que pasa por un transformador JS.
    * **Constants:** Permite asignar valores fijos (Hardcoded) directamente en el destino.

## 3. Funciones Avanzadas de Mapeo
* **Auto-Cast:** Conversión automática entre tipos de datos compatibles.
* **Array Mapping Mode:** Interfaz especializada para manejar la iteración de listas, permitiendo mapear el "item" actual del array.
* **Conditional Mapping:** Posibilidad de asignar un valor solo si se cumple una condición (ej: `if (status === 'ACTIVE')`).

## 4. Integración con el Runtime
El resultado del mapeador se compila en el DSL como una propiedad `input_mapping` que contiene:
1. Las referencias JSONPath.
2. Los fragmentos de código TS inyectados.