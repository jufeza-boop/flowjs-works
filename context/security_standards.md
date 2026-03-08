# Estándares de Seguridad (Security Standards)

## 1. Gestión de Variables de Entorno y Secretos
- **Cero Secretos Hardcodeados:** Las credenciales, DSNs o API Keys NUNCA deben estar en el código fuente. Deben manejarse mediante archivos `.env` (excluidos en `.gitignore`).
- **Validación Temprana:** Toda configuración de entorno debe ser validada en tiempo de inicio (startup) usando esquemas estrictos de **Zod**. Si falta una variable crítica, la app no debe arrancar.
- **Flujos / Nodos:** Las credenciales de integración en el DSL no deben guardarse inline. Usar apuntadores `secret_ref: "<secret_id>"` resueltos en tiempo de ejecución.

## 2. Mitigación OWASP Top 10
- **A01 Broken Access Control:** La validación de permisos DEBE hacerse siempre en el backend (Control Plane/Data Plane). Verificar si el token es válido, si el rol tiene permiso y si el usuario es dueño del recurso.
- **A02 Cryptographic Failures:** Usar encriptación fuerte. Las contraseñas (si aplican) deben hashearse con `bcrypt` (12+ rounds). Todo el tráfico debe usar HTTPS forzado mediante el header HSTS.
- **A03 Injection:** NUNCA concatenar input de usuario en consultas a base de datos. Usar siempre consultas parametrizadas (Prepared Statements).
- **A04 & A07 Auth/Design:** Implementar *Rate Limiting* en endpoints sensibles (ej. 5 intentos/15min) y bloqueo de cuentas para prevenir ataques de fuerza bruta . Las sesiones deben ser cortas (ej. 15 min para Access Tokens).
- **A05 Security Misconfiguration:** 
  - Configurar políticas CORS restrictivas apuntando solo a dominios propios.
  - Nunca enviar *stack traces* completos al frontend en producción.
  - Utilizar **Helmet.js** para aplicar cabeceras de seguridad (`X-Frame-Options`, `X-Content-Type-Options`, `Content-Security-Policy`).
- **A09 Logging:** Loguear eventos de seguridad críticos (login exitoso/fallido, cambios de permisos). NUNCA loguear contraseñas, tokens completos ni datos personales/financieros.

## 3. Manejo de Tokens (JWT)
- Almacenar los tokens JWT de sesión utilizando **Cookies `httpOnly`**, configuradas como `secure: true` y `sameSite: 'strict'` para prevenir ataques XSS y CSRF. Evitar totalmente el uso de `localStorage` para tokens.
