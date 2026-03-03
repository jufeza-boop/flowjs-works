## Reglas Generales de Generación de Código
1. **No alucinar:** Si desconoces una propiedad del DSL o una librería específica de Go, pregunta antes de inventar.
2. **Interface-First:** Define siempre los contratos (Interfaces en Go, Types en TypeScript) antes de generar la implementación de la lógica.
3. **Manejo de Errores en Go:** Prioridad absoluta a la estabilidad. Nunca generes código sin manejar los errores (`if err != nil`).
4. **Sintaxis JSONPath:** Todas las referencias a datos dentro de los flujos deben usar la sintaxis JSONPath estricta (ej. `$.trigger.body` o `$.nodes.ID.output`).
5. **Tipado Estricto:** En el frontend, utiliza TypeScript estricto. Está prohibido el uso de `any`.
6. **Microcopy y Textos:** Cuando generes textos para la UI, el tono debe ser humano, útil, claro y conciso (no robótico). Si hay errores, proporciona soluciones accionables.
7. **Documentación como Código:** Si creas una nueva función de utilidad compleja, genera su documentación inline usando JSDoc/TSDoc explicando el "por qué" y añadiendo un `@example`.
