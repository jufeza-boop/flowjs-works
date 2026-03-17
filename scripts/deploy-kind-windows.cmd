@echo off
setlocal EnableDelayedExpansion

title FlowJS-Works — Despliegue local con kind

rem ============================================================================
rem  FlowJS-Works — Script de despliegue local en Kubernetes con kind
rem  Plataforma : Windows 10 / 11
rem  Referencia : docs\guia-despliegue-kubernetes.md  (Secciones 1-10)
rem
rem  Que hace este script:
rem    1. Verifica prerrequisitos (Docker, kind, kubectl, curl)
rem    2. Crea un cluster kind "flowjs-local" (si no existe)
rem    3. Construye las 4 imagenes Docker del proyecto
rem    4. Carga las imagenes en kind  (sin necesidad de un registry externo)
rem    5. Genera una clave AES-256 aleatoria para los secretos
rem    6. Aplica todos los manifiestos Kubernetes via kustomize
rem    7. Parchea los ConfigMaps para permitir CORS desde localhost
rem    8. Inicializa las bases de datos PostgreSQL
rem    9. Espera a que todos los pods esten en estado Running/Ready
rem   10. Lanza port-forwards en ventanas minimizadas para acceder a los servicios
rem   11. Verifica los health checks de engine y audit-logger
rem   12. Ejecuta un smoke test de creacion y replay de un flujo de prueba
rem   13. Muestra el resumen de URLs de acceso
rem
rem  Puertos locales resultantes:
rem    http://localhost:8080  ->  Designer UI
rem    http://localhost:9090  ->  Engine API
rem    http://localhost:8081  ->  Audit Logger API
rem
rem  Prerrequisitos (deben estar en el PATH):
rem    - Docker Desktop  https://www.docker.com/products/docker-desktop/
rem    - kind            https://kind.sigs.k8s.io/docs/user/quick-start/
rem    - kubectl         https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/
rem    - curl            incluido en Windows 10+ build 17063
rem
rem  Instalacion rapida con winget:
rem    winget install Docker.DockerDesktop
rem    winget install Kubernetes.kind
rem    winget install Kubernetes.kubectl
rem ============================================================================

echo.
echo ================================================================
echo   FlowJS-Works ^| Despliegue local en Kubernetes con kind
echo   Referencia: docs\guia-despliegue-kubernetes.md  (Seccion 8)
echo ================================================================
echo.

rem ── Variables de configuracion ───────────────────────────────────────────────

set CLUSTER_NAME=flowjs-local
set NAMESPACE=flowjs

rem Nombres de imagen: deben coincidir exactamente con los valores en deploy/k8s/*.yaml
rem Los manifiestos usan "flowjs-*:latest" con imagePullPolicy: IfNotPresent,
rem que es compatible con kind load docker-image.
set IMG_ENGINE=flowjs-engine:latest
set IMG_AUDIT=flowjs-audit-logger:latest
set IMG_ORCH=flowjs-orchestrator:latest
set IMG_DESIGNER=flowjs-designer:latest

rem Puertos de acceso local (port-forward)
set PORT_DESIGNER=8080
set PORT_ENGINE=9090
set PORT_AUDIT=8081

rem ALLOWED_ORIGINS para modo local: el navegador accede al Designer en localhost:8080
set LOCAL_ORIGINS=http://localhost:%PORT_DESIGNER%

rem Timeout maximo en segundos para esperar pods
set POD_TIMEOUT=300

rem ── Ruta raiz del repositorio ────────────────────────────────────────────────
rem   Este script vive en <raiz>\scripts\  ->  %~dp0.. es la raiz del repo.
pushd "%~dp0.." >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo acceder al directorio raiz del repositorio.
    goto :error
)
set REPO_ROOT=%CD%
popd >nul

echo [INFO] Directorio del repositorio: %REPO_ROOT%
echo.

rem ============================================================================
rem  PASO 0 — Verificacion de prerrequisitos
rem ============================================================================
echo ── PASO 0: Verificando prerrequisitos ──────────────────────────────────────
echo.

where docker >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] 'docker' no encontrado en el PATH.
    echo         Instala Docker Desktop: https://www.docker.com/products/docker-desktop/
    echo         O con winget: winget install Docker.DockerDesktop
    goto :error
)
echo [OK] docker encontrado.

docker info >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Docker no esta en ejecucion.
    echo         Inicia Docker Desktop y espera a que el icono de la bandeja sea estable.
    goto :error
)
echo [OK] Docker esta en ejecucion.

where kind >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] 'kind' no encontrado en el PATH.
    echo         Descarga: https://kind.sigs.k8s.io/docs/user/quick-start/
    echo         O con winget: winget install Kubernetes.kind
    goto :error
)
echo [OK] kind encontrado.

where kubectl >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] 'kubectl' no encontrado en el PATH.
    echo         Descarga: https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/
    echo         O con winget: winget install Kubernetes.kubectl
    goto :error
)
echo [OK] kubectl encontrado.

where curl >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [AVISO] 'curl' no encontrado. El smoke test (Paso 12) usara PowerShell.
    set USE_CURL=0
) else (
    echo [OK] curl encontrado.
    set USE_CURL=1
)

echo.
echo [OK] Todos los prerrequisitos estan disponibles.
echo.
pause

rem ============================================================================
rem  PASO 1 — Crear el cluster kind
rem  (equivale a Paso 1 de la guia: preparar el entorno de Kubernetes local)
rem ============================================================================
echo.
echo ── PASO 1: Creando cluster kind '%CLUSTER_NAME%' ───────────────────────────
echo.

kind get clusters 2>nul | findstr /i "%CLUSTER_NAME%" >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo [INFO] El cluster '%CLUSTER_NAME%' ya existe. Se reutilizara.
    echo        Si quieres empezar desde cero ejecuta:
    echo          kind delete cluster --name %CLUSTER_NAME%
    echo        y luego vuelve a ejecutar este script.
) else (
    echo [INFO] Creando cluster kind '%CLUSTER_NAME%'...

    rem Escribir la configuracion del cluster kind a un fichero temporal.
    rem Se habilitan puertos extra por si en el futuro se quieren usar NodePorts.
    set KIND_CFG=%TEMP%\flowjs-kind-config.yaml
    (
        echo apiVersion: kind.x-k8s.io/v1alpha4
        echo kind: Cluster
        echo name: %CLUSTER_NAME%
        echo nodes:
        echo - role: control-plane
        echo   kubeadmConfigPatches:
        echo   - ^|
        echo     kind: InitConfiguration
        echo     nodeRegistration:
        echo       kubeletExtraArgs:
        echo         node-labels: "ingress-ready=true"
    ) > "!KIND_CFG!"

    kind create cluster --config "!KIND_CFG!" --name "%CLUSTER_NAME%"
    if %ERRORLEVEL% neq 0 (
        echo [ERROR] No se pudo crear el cluster kind.
        goto :error
    )
    del "!KIND_CFG!" >nul 2>&1
    echo [OK] Cluster kind '%CLUSTER_NAME%' creado.
)

rem Asegurarse de que kubectl apunta al cluster correcto
kubectl config use-context "kind-%CLUSTER_NAME%" >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo establecer el contexto kubectl para 'kind-%CLUSTER_NAME%'.
    goto :error
)
echo [OK] Contexto kubectl establecido: kind-%CLUSTER_NAME%
echo.

rem ============================================================================
rem  PASO 2 — Construir las imagenes Docker
rem  (equivale a Paso 2 de la guia)
rem ============================================================================
echo.
echo ── PASO 2: Construyendo imagenes Docker ────────────────────────────────────
echo.
echo [INFO] Las imagenes se construyen con el tag 'latest' para que coincidan
echo        con los manifiestos de deploy/k8s/ (imagePullPolicy: IfNotPresent).
echo.

pushd "%REPO_ROOT%" >nul

rem --- Engine ---
echo [INFO] Construyendo imagen: %IMG_ENGINE%
docker build -t %IMG_ENGINE% -f services\engine\Dockerfile services\engine
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Engine.
    goto :error
)
echo [OK] %IMG_ENGINE% construida.
echo.

rem --- Audit Logger ---
echo [INFO] Construyendo imagen: %IMG_AUDIT%
docker build -t %IMG_AUDIT% -f services\audit-logger\Dockerfile services\audit-logger
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Audit Logger.
    goto :error
)
echo [OK] %IMG_AUDIT% construida.
echo.

rem --- Orchestrator ---
echo [INFO] Construyendo imagen: %IMG_ORCH%
docker build -t %IMG_ORCH% -f services\orchestrator\Dockerfile services\orchestrator
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Orchestrator.
    goto :error
)
echo [OK] %IMG_ORCH% construida.
echo.

rem --- Designer (SPA React) ---
rem Las variables VITE_* se baquean en el bundle durante el build.
rem Los valores por defecto en api.ts son exactamente los puertos de port-forward
rem (localhost:9090 para engine, localhost:8080 para audit-logger), por lo que
rem no es necesario pasar --build-arg aqui.
rem Si el Dockerfile del designer admite ARGs, se pueden sobreescribir asi:
rem   docker build --build-arg VITE_ENGINE_API_URL=http://localhost:9090 ...
echo [INFO] Construyendo imagen: %IMG_DESIGNER%
echo        (VITE_ENGINE_API_URL=http://localhost:%PORT_ENGINE%, VITE_AUDIT_API_URL=http://localhost:%PORT_AUDIT%)
docker build -t %IMG_DESIGNER% -f apps\designer\Dockerfile apps\designer
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Designer.
    goto :error
)
echo [OK] %IMG_DESIGNER% construida.
echo.

popd >nul

rem ============================================================================
rem  PASO 3 — Cargar imagenes en kind
rem  (sustituye al Paso 3 de la guia: "push a un registry")
rem  kind load docker-image hace que el cluster use las imagenes locales
rem  sin necesidad de un registry externo ni credenciales.
rem ============================================================================
echo.
echo ── PASO 3: Cargando imagenes en el cluster kind ────────────────────────────
echo.

echo [INFO] Cargando %IMG_ENGINE% en kind...
kind load docker-image %IMG_ENGINE% --name %CLUSTER_NAME%
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo cargar %IMG_ENGINE% en kind.
    goto :error
)
echo [OK] %IMG_ENGINE% cargada.

echo [INFO] Cargando %IMG_AUDIT% en kind...
kind load docker-image %IMG_AUDIT% --name %CLUSTER_NAME%
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo cargar %IMG_AUDIT% en kind.
    goto :error
)
echo [OK] %IMG_AUDIT% cargada.

echo [INFO] Cargando %IMG_ORCH% en kind...
kind load docker-image %IMG_ORCH% --name %CLUSTER_NAME%
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo cargar %IMG_ORCH% en kind.
    goto :error
)
echo [OK] %IMG_ORCH% cargada.

echo [INFO] Cargando %IMG_DESIGNER% en kind...
kind load docker-image %IMG_DESIGNER% --name %CLUSTER_NAME%
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo cargar %IMG_DESIGNER% en kind.
    goto :error
)
echo [OK] %IMG_DESIGNER% cargada.
echo.

rem ============================================================================
rem  PASO 4 — Generar clave AES-256 aleatoria
rem  (equivale a Paso 4.1 de la guia: configurar secretos)
rem  Se usa PowerShell para generar 32 bytes aleatorios en base64 (44 chars).
rem  Este valor siempre cumple el requisito de longitud >= 32 del engine.
rem ============================================================================
echo.
echo ── PASO 4: Generando clave AES-256 para los secretos ───────────────────────
echo.

for /f "delims=" %%K in ('powershell -NoProfile -Command "[System.Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))"') do set AES_KEY=%%K

if "!AES_KEY!"=="" (
    echo [ERROR] No se pudo generar la clave AES con PowerShell.
    echo         Asegurate de tener PowerShell 5.1+ instalado.
    goto :error
)
echo [OK] Clave AES generada (44 caracteres base64, 256 bits).
echo        (el valor no se muestra por seguridad)
echo.

rem ============================================================================
rem  PASO 5 — Aplicar manifiestos Kubernetes via kustomize
rem  (equivale al Paso 6 de la guia: "kubectl apply -k deploy/k8s/")
rem ============================================================================
echo.
echo ── PASO 5: Aplicando manifiestos Kubernetes ────────────────────────────────
echo.

pushd "%REPO_ROOT%" >nul

kubectl apply -k deploy\k8s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al aplicar los manifiestos con kustomize.
    echo         Comprueba que deploy\k8s\kustomization.yaml es correcto.
    goto :error
)
popd >nul
echo.
echo [OK] Manifiestos aplicados. Namespace '%NAMESPACE%' creado con todos los recursos.
echo.

rem ============================================================================
rem  PASO 6 — Parchear secretos y configuracion para entorno local
rem  (equivale a Paso 4.1 y 4.3 de la guia, adaptado a kind/localhost)
rem  a) Actualizar SECRETS_AES_KEY con la clave generada en el Paso 4
rem  b) Actualizar ALLOWED_ORIGINS en engine y audit-logger para CORS local
rem     (los manifiestos usan "http://designer" que no resuelve desde el browser)
rem ============================================================================
echo.
echo ── PASO 6: Parchando secretos y CORS para localhost ────────────────────────
echo.

echo [INFO] Actualizando engine-secret con la clave AES generada...
kubectl -n %NAMESPACE% patch secret engine-secret ^
    --type=merge ^
    -p "{\"stringData\":{\"SECRETS_AES_KEY\":\"!AES_KEY!\"}}"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo parchear el secret engine-secret.
    goto :error
)
echo [OK] engine-secret actualizado.

echo [INFO] Actualizando ALLOWED_ORIGINS del engine a '%LOCAL_ORIGINS%'...
kubectl -n %NAMESPACE% patch configmap engine-config ^
    --type=merge ^
    -p "{\"data\":{\"ALLOWED_ORIGINS\":\"%LOCAL_ORIGINS%\"}}"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo parchear engine-config.
    goto :error
)
echo [OK] engine-config parcheado.

echo [INFO] Actualizando ALLOWED_ORIGINS del audit-logger a '%LOCAL_ORIGINS%'...
kubectl -n %NAMESPACE% patch configmap audit-logger-config ^
    --type=merge ^
    -p "{\"data\":{\"ALLOWED_ORIGINS\":\"%LOCAL_ORIGINS%\"}}"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo parchear audit-logger-config.
    goto :error
)
echo [OK] audit-logger-config parcheado.

echo [INFO] Reiniciando deployments para que tomen la nueva configuracion...
kubectl -n %NAMESPACE% rollout restart deployment/engine
kubectl -n %NAMESPACE% rollout restart deployment/audit-logger
echo [OK] Rollout restart enviado.
echo.

rem ============================================================================
rem  PASO 7 — Esperar a que PostgreSQL este listo
rem  (requisito previo para inicializar las bases de datos)
rem ============================================================================
echo.
echo ── PASO 7: Esperando a que PostgreSQL este listo ───────────────────────────
echo.

echo [INFO] Esperando al pod de PostgreSQL (timeout: %POD_TIMEOUT%s)...
kubectl -n %NAMESPACE% wait pod ^
    --selector=app=postgres ^
    --for=condition=Ready ^
    --timeout=%POD_TIMEOUT%s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] PostgreSQL no estuvo listo en %POD_TIMEOUT% segundos.
    echo         Comprueba los eventos: kubectl -n %NAMESPACE% describe pod -l app=postgres
    goto :error
)
echo [OK] PostgreSQL esta listo.

rem Obtener el nombre del pod de postgres para el kubectl exec
for /f "delims=" %%P in ('kubectl -n %NAMESPACE% get pod -l app=postgres -o jsonpath={.items[0].metadata.name} 2^>nul') do set POSTGRES_POD=%%P
if "!POSTGRES_POD!"=="" (
    echo [ERROR] No se encontro el pod de PostgreSQL.
    goto :error
)
echo [INFO] Pod PostgreSQL: !POSTGRES_POD!
echo.

rem ============================================================================
rem  PASO 8 — Inicializar las bases de datos
rem  (equivale al Paso 5 de la guia)
rem  Se copian los scripts SQL al pod y se ejecutan en orden.
rem ============================================================================
echo.
echo ── PASO 8: Inicializando bases de datos ────────────────────────────────────
echo.

echo [INFO] Copiando scripts SQL al pod...
kubectl -n %NAMESPACE% cp "%REPO_ROOT%\init-db\01-init.sql" "!POSTGRES_POD!:/tmp/01-init.sql"
kubectl -n %NAMESPACE% cp "%REPO_ROOT%\init-db\02-audit-schema.sql" "!POSTGRES_POD!:/tmp/02-audit-schema.sql"
kubectl -n %NAMESPACE% cp "%REPO_ROOT%\init-db\03-config-schema.sql" "!POSTGRES_POD!:/tmp/03-config-schema.sql"
echo [OK] Scripts copiados.

echo [INFO] Ejecutando 01-init.sql  (crea flowjs_audit y flowjs_config)...
kubectl -n %NAMESPACE% exec "!POSTGRES_POD!" -- psql -U admin -f /tmp/01-init.sql
if %ERRORLEVEL% neq 0 (
    echo [AVISO] El script 01-init.sql reporto errores (puede ser normal si las DBs ya existen).
)

echo [INFO] Ejecutando 02-audit-schema.sql  (tablas executions y activity_logs)...
kubectl -n %NAMESPACE% exec "!POSTGRES_POD!" -- psql -U admin -f /tmp/02-audit-schema.sql
if %ERRORLEVEL% neq 0 (
    echo [AVISO] El script 02-audit-schema.sql reporto errores (puede ser normal si las tablas ya existen).
)

echo [INFO] Ejecutando 03-config-schema.sql  (tablas processes y secrets)...
kubectl -n %NAMESPACE% exec "!POSTGRES_POD!" -- psql -U admin -f /tmp/03-config-schema.sql
if %ERRORLEVEL% neq 0 (
    echo [AVISO] El script 03-config-schema.sql reporto errores (puede ser normal si las tablas ya existen).
)
echo [OK] Bases de datos inicializadas.
echo.

rem ============================================================================
rem  PASO 9 — Esperar a que todos los pods esten en Running/Ready
rem  (equivale al Paso 7 de la guia: "verificar el estado del despliegue")
rem ============================================================================
echo.
echo ── PASO 9: Esperando a que todos los pods esten listos ─────────────────────
echo.

echo [INFO] Esperando a todos los deployments (timeout: %POD_TIMEOUT%s)...
kubectl -n %NAMESPACE% wait deployment ^
    --all ^
    --for=condition=Available ^
    --timeout=%POD_TIMEOUT%s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No todos los deployments alcanzaron el estado Available.
    echo.
    echo         Estado actual de los pods:
    kubectl -n %NAMESPACE% get pods
    echo.
    echo         Consulta la seccion 13 (Resolucion de problemas) de la guia.
    goto :error
)
echo.
echo [OK] Todos los pods estan en estado Running/Ready:
kubectl -n %NAMESPACE% get pods
echo.

rem ============================================================================
rem  PASO 10 — Iniciar port-forwards
rem  (equivale al Paso 8, Opcion A de la guia: port-forward para pruebas locales)
rem  Cada port-forward se abre en una ventana CMD minimizada independiente.
rem  Cierra esas ventanas cuando hayas terminado de usar el entorno.
rem ============================================================================
echo.
echo ── PASO 10: Iniciando port-forwards ────────────────────────────────────────
echo.
echo [INFO] Se abriran 3 ventanas CMD minimizadas con los port-forwards.
echo        Cierralas cuando termines de usar el entorno.
echo.

echo [INFO] Port-forward Designer UI  -> http://localhost:%PORT_DESIGNER%
START "FlowJS Designer :%PORT_DESIGNER%" /MIN cmd /k "kubectl -n %NAMESPACE% port-forward svc/designer %PORT_DESIGNER%:80"

echo [INFO] Port-forward Engine API   -> http://localhost:%PORT_ENGINE%
START "FlowJS Engine :%PORT_ENGINE%" /MIN cmd /k "kubectl -n %NAMESPACE% port-forward svc/engine %PORT_ENGINE%:%PORT_ENGINE%"

echo [INFO] Port-forward Audit Logger -> http://localhost:%PORT_AUDIT%
START "FlowJS AuditLogger :%PORT_AUDIT%" /MIN cmd /k "kubectl -n %NAMESPACE% port-forward svc/audit-logger %PORT_AUDIT%:8080"

echo.
echo [INFO] Esperando 8 segundos para que los port-forwards se establezcan...
timeout /t 8 /nobreak >nul
echo.

rem ============================================================================
rem  PASO 11 — Verificar health checks
rem  (equivale al Paso 9 de la guia)
rem ============================================================================
echo.
echo ── PASO 11: Verificando health checks ──────────────────────────────────────
echo.

echo [INFO] Comprobando /health del Engine (http://localhost:%PORT_ENGINE%/health)...
set HEALTH_OK=0
for /l %%i in (1,1,10) do (
    if !HEALTH_OK! equ 0 (
        rem Capturar el codigo HTTP en una variable usando for /f dentro del bucle
        for /f "delims=" %%C in ('curl -s -o nul -w "%%{http_code}" http://localhost:%PORT_ENGINE%/health 2^>nul') do set HTTP_CODE=%%C
        if "!HTTP_CODE!"=="200" (
            set HEALTH_OK=1
            echo [OK] Engine /health respondio 200 OK.
        ) else (
            echo [INFO] Intento %%i/10 (codigo: !HTTP_CODE!) ... esperando al engine...
            timeout /t 3 /nobreak >nul
        )
    )
)
if !HEALTH_OK! equ 0 (
    echo [AVISO] El engine no respondio 200 en /health. Comprueba los logs:
    echo         kubectl -n %NAMESPACE% logs -l app=engine --tail=30
)

echo [INFO] Comprobando /health del Audit Logger (http://localhost:%PORT_AUDIT%/health)...
set HEALTH_OK=0
for /l %%i in (1,1,10) do (
    if !HEALTH_OK! equ 0 (
        for /f "delims=" %%C in ('curl -s -o nul -w "%%{http_code}" http://localhost:%PORT_AUDIT%/health 2^>nul') do set HTTP_CODE=%%C
        if "!HTTP_CODE!"=="200" (
            set HEALTH_OK=1
            echo [OK] Audit Logger /health respondio 200 OK.
        ) else (
            echo [INFO] Intento %%i/10 (codigo: !HTTP_CODE!) ... esperando al audit-logger...
            timeout /t 3 /nobreak >nul
        )
    )
)
if !HEALTH_OK! equ 0 (
    echo [AVISO] El audit-logger no respondio 200 en /health. Comprueba los logs:
    echo         kubectl -n %NAMESPACE% logs -l app=audit-logger --tail=30
)
echo.

rem ============================================================================
rem  PASO 12 — Smoke test: crear y ejecutar un flujo de prueba
rem  (equivale al Paso 10 de la guia)
rem  Se usa PowerShell Invoke-RestMethod para evitar problemas de escape de
rem  caracteres especiales de JSON en CMD de Windows.
rem ============================================================================
echo.
echo ── PASO 12: Smoke test — flujo 'hola-mundo-kind' ───────────────────────────
echo.
echo [INFO] Creando flujo de prueba via POST /api/v1/processes ...

powershell -NoProfile -Command ^
    "$body = @{ definition = @{ id = 'hola-mundo-kind'; version = '1.0.0'; name = 'Hola Mundo (kind)' }; trigger = @{ id = 'trg_manual'; type = 'manual' }; nodes = @(@{ id = 'log_saludo'; type = 'log'; input_mapping = @{ message = '$.trigger.body.nombre' }; config = @{ level = 'INFO' } }) } | ConvertTo-Json -Depth 10; try { $r = Invoke-RestMethod -Method POST -Uri 'http://localhost:%PORT_ENGINE%/api/v1/processes' -ContentType 'application/json' -Body $body; Write-Host '[OK] Flujo creado:' ($r | ConvertTo-Json -Compress) } catch { Write-Host '[AVISO] No se pudo crear el flujo:' $_.Exception.Message }"

echo.
echo [INFO] Ejecutando replay del flujo 'hola-mundo-kind'...

powershell -NoProfile -Command ^
    "$body = @{ trigger_data = @{ nombre = 'FlowJS en kind' } } | ConvertTo-Json; try { $r = Invoke-RestMethod -Method POST -Uri 'http://localhost:%PORT_ENGINE%/api/v1/processes/hola-mundo-kind/replay' -ContentType 'application/json' -Body $body; Write-Host '[OK] Ejecucion lanzada:' ($r | ConvertTo-Json -Compress) } catch { Write-Host '[AVISO] El replay retorno:' $_.Exception.Message }"

echo.
echo [INFO] Consultando historial de ejecuciones en Audit Logger...

powershell -NoProfile -Command ^
    "try { $r = Invoke-RestMethod -Uri 'http://localhost:%PORT_AUDIT%/executions'; Write-Host '[OK] Ejecuciones registradas:' ($r | Measure-Object).Count } catch { Write-Host '[AVISO] No se pudo consultar el historial:' $_.Exception.Message }"

echo.

rem ============================================================================
rem  RESUMEN FINAL
rem ============================================================================
echo.
echo ================================================================
echo   DESPLIEGUE COMPLETADO
echo ================================================================
echo.
echo   Cluster kind : %CLUSTER_NAME%
echo   Namespace    : %NAMESPACE%
echo.
echo   Servicios accesibles (port-forward activo):
echo   ┌───────────────────────────────────────────────────────────┐
echo   │  Designer UI    ->  http://localhost:%PORT_DESIGNER%                 │
echo   │  Engine API     ->  http://localhost:%PORT_ENGINE%                 │
echo   │  Audit Logger   ->  http://localhost:%PORT_AUDIT%                 │
echo   └───────────────────────────────────────────────────────────┘
echo.
echo   Comandos utiles:
echo   ─────────────────────────────────────────────────────────────
echo   Ver todos los pods:
echo     kubectl -n %NAMESPACE% get pods
echo.
echo   Ver logs del engine en tiempo real:
echo     kubectl -n %NAMESPACE% logs -l app=engine -f
echo.
echo   Ver logs del audit-logger en tiempo real:
echo     kubectl -n %NAMESPACE% logs -l app=audit-logger -f
echo.
echo   Ver eventos del namespace:
echo     kubectl -n %NAMESPACE% get events --sort-by=.lastTimestamp
echo.
echo   Eliminar el cluster cuando termines:
echo     kind delete cluster --name %CLUSTER_NAME%
echo.
echo ================================================================
echo   Consulta docs\guia-despliegue-kubernetes.md para mas detalles:
echo   - Seccion  9: Hot-Reload de flujos en produccion
echo   - Seccion 10: Monitorizacion con Prometheus
echo   - Seccion 13: Resolucion de problemas
echo ================================================================
echo.

endlocal
exit /b 0

rem ============================================================================
rem  :error — Salida con error centralizada
rem ============================================================================
:error
echo.
echo ================================================================
echo   [ERROR] El script termino con errores.
echo.
echo   Sugerencias:
echo     - Revisa los mensajes anteriores para identificar el fallo.
echo     - Consulta la seccion 13 (Resolucion de problemas) de:
echo         docs\guia-despliegue-kubernetes.md
echo     - Para ver el estado del cluster:
echo         kubectl -n %NAMESPACE% get pods
echo         kubectl -n %NAMESPACE% get events --sort-by=.lastTimestamp
echo     - Para eliminar el cluster y empezar de nuevo:
echo         kind delete cluster --name %CLUSTER_NAME%
echo ================================================================
echo.
endlocal
exit /b 1
