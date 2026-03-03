# Estándares de Calidad (Quality Standards)

## 1. Estrategia de Testing y Cobertura
- **Pirámide de Testing:** Escribir más tests unitarios, suficientes de integración y menos E2E.
- **User-Centric Testing:** Testear comportamiento, no detalles de implementación. Priorizar queries como `getByRole` por encima de `getByTestId`.
- **Patrón AAA:** Todo test debe estructurarse claramente en Arrange, Act, Assert.
- **Strategic Coverage (100/80/0):** 
  - **CORE (100%):** Lógica de negocio crítica y cálculos.
  - **IMPORTANT (80%):** Componentes de UI visibles e interacciones principales del usuario.
  - **INFRASTRUCTURE (0%):** Tipos, interfaces y constantes que se validan estáticamente mediante TypeScript.

## 2. Calidad del Código y Deuda Técnica
- **Boy Scout Rule:** Deja el código más limpio de como lo encontraste. Refactoriza pequeños *smells* constantemente.
- **Code Smells:** Prestar atención y evitar números mágicos (extraer a constantes), código duplicado y funciones con alta complejidad ciclomática (>15).
- **Quality Gates:**  
  - *Pre-commit:* Ejecuta Linter, Unit Tests rápidos y Build.
  - *Pre-push:* Ejecuta validación de Coverage y tests.

## 3. Documentación (Docs as Code)
- La documentación debe vivir en el mismo repositorio que el código y actualizarse en el mismo Pull Request [28, 29].
- **APIs:** Documentar usando la especificación OpenAPI (Swagger), preferiblemente generada de forma automática desde los tipos de TypeScript.
- **Componentes React:** Usar Storybook para documentar variantes, estados normales y casos extremos (`EdgeCases`) de cada componente.
- **Decisiones de Arquitectura:** Utilizar ADRs (Architecture Decision Records) inmutables ubicados en `docs/adr/` para documentar el "por qué" de las elecciones técnicas importantes.

## 4. Rendimiento y UX (Frontend)
- **Performance Percibida:** Mostrar *skeleton screens* instantáneos y proporcionar feedback inmediato (< 100ms) ante interacciones del usuario.
- **Accesibilidad (A11Y):** Cumplir con WCAG 2.1 Nivel AA. Es obligatorio el uso de HTML semántico (`<button>` en lugar de `<div>`), navegación funcional solo con teclado y foco visible.
