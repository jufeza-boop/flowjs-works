@echo off
setlocal EnableDelayedExpansion

title FlowJS-Works — Despliegue PRODUCCION en Kubernetes (Cloud)

rem ============================================================================
rem  FlowJS-Works — Script de despliegue en Kubernetes de PRODUCCION (Cloud)
rem  Plataforma : Windows 10 / 11
rem  Referencia : docs\guia-despliegue-kubernetes.md  (Secciones 2-10)
rem
rem  Que hace este script:
rem    0. Verifica prerrequisitos (Docker, kubectl, curl)
rem    1. Configuracion interactiva (registry, tag, dominio, cluster, BD)
rem    2. Login al registro de contenedores
rem    3. Construye las 4 imagenes Docker con las URLs de produccion compiladas
rem    4. Publica las imagenes en el registro real (docker push)
rem    5. Genera secretos de produccion seguros (AES-256, contrasena PostgreSQL)
rem    6. Aplica los manifiestos Kubernetes via kustomize
rem    7. Parchea el cluster: imagenes, secretos AES, CORS, DSN, imagePullPolicy
rem    8. Inicializa las bases de datos (in-cluster o externa)
rem    9. Aplica el recurso Ingress con los dominios configurados
rem   10. Espera a que todos los deployments esten Available (rollout)
rem   11. Verifica los health checks sobre los dominios publicos
rem   12. Smoke test (crear y ejecutar un flujo de prueba)
rem   13. Guarda un resumen de configuracion en deploy-prod-summary.txt
rem
rem  Prerrequisitos (deben estar en el PATH):
rem    - Docker Desktop  https://www.docker.com/products/docker-desktop/
rem    - kubectl 1.28+   https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/
rem    - curl            incluido en Windows 10+ build 17063
rem
rem  Tambien necesitaras la CLI de tu proveedor cloud SI usas ECR o ACR:
rem    - AWS:    aws cli  (winget install Amazon.AWSCLI)
rem    - GCP:    gcloud   (winget install Google.CloudSDK)
rem    - Azure:  az cli   (winget install Microsoft.AzureCLI)
rem
rem  El cluster Kubernetes debe estar ya provisionado y tu contexto kubectl
rem  debe apuntar a el (kubectl config current-context).
rem
rem  Instalacion rapida con winget:
rem    winget install Docker.DockerDesktop
rem    winget install Kubernetes.kubectl
rem ============================================================================

echo.
echo ================================================================
echo   FlowJS-Works ^| Despliegue PRODUCCION en Kubernetes (Cloud)
echo   Referencia: docs\guia-despliegue-kubernetes.md  (Seccion 8)
echo ================================================================
echo.
echo   AVISO: Este script despliega en un cluster de produccion real.
echo   Asegurate de haber revisado todos los valores antes de continuar.
echo.
pause

rem ── Ruta raiz del repositorio ────────────────────────────────────────────────
pushd "%~dp0.." >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo acceder al directorio raiz del repositorio.
    goto :error
)
set REPO_ROOT=%CD%
popd >nul
echo [INFO] Directorio del repositorio: %REPO_ROOT%
echo.

rem ── Namespace Kubernetes (fijo segun los manifiestos) ────────────────────────
set NAMESPACE=flowjs

rem ── Timeout maximo en segundos para esperar pods/rollout ─────────────────────
set POD_TIMEOUT=600

rem ============================================================================
rem  PASO 0 — Verificacion de prerrequisitos
rem ============================================================================
echo.
echo ── PASO 0: Verificando prerrequisitos ──────────────────────────────────────
echo.

where docker >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] 'docker' no encontrado en el PATH.
    echo         Instala Docker Desktop: https://www.docker.com/products/docker-desktop/
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
    echo [AVISO] 'curl' no encontrado. Los health checks usaran PowerShell.
    set USE_CURL=0
) else (
    echo [OK] curl encontrado.
    set USE_CURL=1
)

echo.
echo [OK] Prerrequisitos basicos verificados.
echo.

rem ============================================================================
rem  PASO 1 — Configuracion interactiva
rem  El usuario introduce los valores especificos de su entorno de produccion.
rem  Los valores con [default] se usan si se pulsa Enter sin escribir nada.
rem ============================================================================
echo.
echo ── PASO 1: Configuracion de produccion ────────────────────────────────────
echo.
echo   Introduce los valores para tu entorno. Pulsa Enter para usar el [default].
echo.

rem --- Registro de contenedores ---
echo   Registro de contenedores (ejemplos):
echo     Docker Hub : docker.io/miusuario
echo     GHCR       : ghcr.io/miorg
echo     ECR        : 123456789.dkr.ecr.eu-west-1.amazonaws.com
echo     GCR        : gcr.io/mi-proyecto
echo     ACR        : miregistry.azurecr.io
echo.
set /p "REGISTRY=  Registro [docker.io/miorg]: "
if "!REGISTRY!"=="" set REGISTRY=docker.io/miorg
echo   -> REGISTRY=%REGISTRY%
echo.

rem --- Tag de version ---
for /f "delims=" %%T in ('git -C "%REPO_ROOT%" describe --tags --abbrev=0 2^>nul') do set GIT_TAG=%%T
if "!GIT_TAG!"=="" (
    echo [AVISO] No se encontraron tags en el repositorio. Se usara '1.0.0' como valor por defecto.
    set GIT_TAG=1.0.0
)
set /p "TAG=  Tag de version [!GIT_TAG!]: "
if "!TAG!"=="" set TAG=!GIT_TAG!
echo   -> TAG=%TAG%
echo.

rem --- Contexto kubectl ---
for /f "delims=" %%C in ('kubectl config current-context 2^>nul') do set CURRENT_CTX=%%C
if "!CURRENT_CTX!"=="" set CURRENT_CTX=(ninguno)
echo   Contextos kubectl disponibles:
kubectl config get-contexts --no-headers 2>nul | findstr /v "^$"
echo.
set /p "KUBE_CONTEXT=  Contexto kubectl [!CURRENT_CTX!]: "
if "!KUBE_CONTEXT!"=="" set KUBE_CONTEXT=!CURRENT_CTX!
echo   -> KUBE_CONTEXT=%KUBE_CONTEXT%
echo.

rem --- Dominios de produccion ---
echo   Dominios de produccion (el Ingress Controller debe estar instalado):
echo     Si no tienes dominio aun, puedes usar la IP del LoadBalancer mas tarde.
echo.
set /p "DESIGNER_DOMAIN=  Dominio del Designer UI [flowjs.mi-empresa.com]: "
if "!DESIGNER_DOMAIN!"=="" set DESIGNER_DOMAIN=flowjs.mi-empresa.com

set /p "ENGINE_DOMAIN=  Dominio del Engine API [api.flowjs.mi-empresa.com]: "
if "!ENGINE_DOMAIN!"=="" set ENGINE_DOMAIN=api.flowjs.mi-empresa.com

set /p "AUDIT_DOMAIN=  Dominio del Audit Logger [audit.flowjs.mi-empresa.com]: "
if "!AUDIT_DOMAIN!"=="" set AUDIT_DOMAIN=audit.flowjs.mi-empresa.com

echo   -> DESIGNER_DOMAIN=%DESIGNER_DOMAIN%
echo   -> ENGINE_DOMAIN=%ENGINE_DOMAIN%
echo   -> AUDIT_DOMAIN=%AUDIT_DOMAIN%
echo.

rem --- Protocolo (http o https) ---
set /p "PROTO=  Protocolo [https]: "
if "!PROTO!"=="" set PROTO=https
echo   -> PROTO=%PROTO%
echo.

rem --- Base de datos ---
echo   PostgreSQL:
echo     in-cluster  = usa el pod PostgreSQL del manifiesto deploy/k8s/postgres/
echo     external    = usa una base de datos gestionada (RDS, Cloud SQL, Azure DB...)
echo.
set /p "DB_MODE=  Modo de base de datos (in-cluster / external) [in-cluster]: "
if "!DB_MODE!"=="" set DB_MODE=in-cluster
echo   -> DB_MODE=%DB_MODE%
echo.

if /i "!DB_MODE!"=="external" (
    echo   DSN de la base de datos de AUDITORIA (flowjs_audit):
    echo   Formato: host=HOST port=5432 user=USER password=PASS dbname=flowjs_audit sslmode=require
    echo.
    set /p "AUDIT_DSN=  POSTGRES_DSN audit []: "
    if "!AUDIT_DSN!"=="" (
        echo [ERROR] El DSN del Audit Logger es obligatorio en modo external.
        goto :error
    )

    echo.
    echo   DSN / DATABASE_URL de la base de datos de CONFIGURACION (flowjs_config):
    echo   Formato: postgres://USER:PASS@HOST:5432/flowjs_config?sslmode=require
    echo.
    set /p "CONFIG_DSN=  DATABASE_URL config []: "
    if "!CONFIG_DSN!"=="" (
        echo [ERROR] El DATABASE_URL de config es obligatorio en modo external.
        goto :error
    )
) else (
    rem In-cluster: se generara una contrasena segura automaticamente
    set AUDIT_DSN=
    set CONFIG_DSN=
)

rem --- Ingress Controller ---
echo.
echo   Tipo de Ingress Controller instalado en el cluster:
echo     nginx    = nginx-ingress-controller (anotaciones nginx.ingress.kubernetes.io/*)
echo     traefik  = Traefik v2/v3
echo     none     = sin Ingress; usar LoadBalancer directamente
echo.
set /p "INGRESS_CLASS=  Ingress class (nginx / traefik / none) [nginx]: "
if "!INGRESS_CLASS!"=="" set INGRESS_CLASS=nginx
echo   -> INGRESS_CLASS=%INGRESS_CLASS%
echo.

rem --- TLS ---
if /i not "!INGRESS_CLASS!"=="none" (
    set /p "TLS_ENABLED=  Habilitar TLS / cert-manager (s/n) [s]: "
    if "!TLS_ENABLED!"=="" set TLS_ENABLED=s
    if /i "!TLS_ENABLED!"=="s" (
        set /p "TLS_ISSUER=  Nombre del ClusterIssuer de cert-manager [letsencrypt-prod]: "
        if "!TLS_ISSUER!"=="" set TLS_ISSUER=letsencrypt-prod
        echo   -> TLS_ISSUER=%TLS_ISSUER%
    ) else (
        set TLS_ISSUER=
    )
)
echo.

rem --- Confirmacion ---
echo ────────────────────────────────────────────────────────────────
echo   Resumen de configuracion:
echo     REGISTRY        = !REGISTRY!
echo     TAG             = !TAG!
echo     KUBE_CONTEXT    = !KUBE_CONTEXT!
echo     DESIGNER_DOMAIN = !DESIGNER_DOMAIN!
echo     ENGINE_DOMAIN   = !ENGINE_DOMAIN!
echo     AUDIT_DOMAIN    = !AUDIT_DOMAIN!
echo     PROTO           = !PROTO!
echo     DB_MODE         = !DB_MODE!
echo     INGRESS_CLASS   = !INGRESS_CLASS!
echo ────────────────────────────────────────────────────────────────
echo.
set /p "CONFIRM=  Continuar con estos valores? (s/n) [s]: "
if "!CONFIRM!"=="" set CONFIRM=s
if /i not "!CONFIRM!"=="s" (
    echo [INFO] Despliegue cancelado por el usuario.
    endlocal
    exit /b 0
)
echo.

rem Establecer el contexto kubectl seleccionado
kubectl config use-context "!KUBE_CONTEXT!" >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo establecer el contexto kubectl '!KUBE_CONTEXT!'.
    echo         Comprueba: kubectl config get-contexts
    goto :error
)
echo [OK] Contexto kubectl activo: !KUBE_CONTEXT!

rem Nombres de imagen con registry y tag de produccion
set IMG_ENGINE=!REGISTRY!/flowjs-engine:!TAG!
set IMG_AUDIT=!REGISTRY!/flowjs-audit-logger:!TAG!
set IMG_ORCH=!REGISTRY!/flowjs-orchestrator:!TAG!
set IMG_DESIGNER=!REGISTRY!/flowjs-designer:!TAG!

rem URLs publicas de los servicios (usadas en VITE_* build-args y ALLOWED_ORIGINS)
set ENGINE_URL=!PROTO!://!ENGINE_DOMAIN!
set AUDIT_URL=!PROTO!://!AUDIT_DOMAIN!
set DESIGNER_URL=!PROTO!://!DESIGNER_DOMAIN!

rem ============================================================================
rem  PASO 2 — Login al registro de contenedores
rem  Para registros en la nube puede ser necesario usar la CLI del proveedor
rem  antes de este script (aws ecr get-login-password | docker login ...).
rem ============================================================================
echo.
echo ── PASO 2: Login al registro de contenedores ───────────────────────────────
echo.
echo [INFO] Intentando docker login en !REGISTRY! ...
echo        (Si usas ECR/GCR/ACR, ya deberias haberte autenticado con la CLI del
echo         proveedor antes de ejecutar este script. Pulsa Enter para omitir
echo         el login de Docker Hub si ya estas autenticado.)
echo.

rem Detectar si es un registro cloud que gestiona su propio login
echo !REGISTRY! | findstr /i "amazonaws.com" >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo [INFO] Registro AWS ECR detectado.
    echo        Asegurate de haber ejecutado:
    echo          aws ecr get-login-password --region ^<region^> ^| docker login --username AWS --password-stdin !REGISTRY!
    echo        Si ya lo hiciste, pulsa Enter para continuar.
    pause
    goto :login_done
)

echo !REGISTRY! | findstr /i "gcr.io\|pkg.dev" >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo [INFO] Registro Google GCR/Artifact Registry detectado.
    echo        Asegurate de haber ejecutado:
    echo          gcloud auth configure-docker
    echo        Si ya lo hiciste, pulsa Enter para continuar.
    pause
    goto :login_done
)

echo !REGISTRY! | findstr /i "azurecr.io" >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo [INFO] Registro Azure ACR detectado.
    echo        Asegurate de haber ejecutado:
    echo          az acr login --name ^<nombre-del-acr^>
    echo        Si ya lo hiciste, pulsa Enter para continuar.
    pause
    goto :login_done
)

rem Docker Hub o GHCR: login interactivo
set /p "REGISTRY_USER=  Usuario del registro (Docker Hub / GHCR): "
if "!REGISTRY_USER!"=="" (
    echo [AVISO] No se introdujo usuario. Omitiendo docker login.
    echo         Asegurate de estar ya autenticado con: docker login !REGISTRY!
) else (
    docker login "!REGISTRY!" --username "!REGISTRY_USER!"
    if %ERRORLEVEL% neq 0 (
        echo [ERROR] docker login fallo.
        goto :error
    )
    echo [OK] Login correcto en !REGISTRY!.
)

:login_done
echo.

rem ============================================================================
rem  PASO 3 — Construir las imagenes Docker con las URLs de produccion
rem  Las variables VITE_* se compilan dentro del bundle JS en tiempo de build.
rem  Deben apuntar a las URLs publicas que el NAVEGADOR del usuario usara.
rem  (Paso 2 de la guia de despliegue)
rem ============================================================================
echo.
echo ── PASO 3: Construyendo imagenes Docker para produccion ────────────────────
echo.
echo [INFO] VITE_ENGINE_API_URL = !ENGINE_URL!
echo [INFO] VITE_AUDIT_API_URL  = !AUDIT_URL!
echo.

pushd "%REPO_ROOT%" >nul

rem --- Engine ---
echo [INFO] Construyendo %IMG_ENGINE% ...
docker build -t "!IMG_ENGINE!" ^
    -f services\engine\Dockerfile ^
    services\engine
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Engine.
    goto :error
)
echo [OK] %IMG_ENGINE% construida.
echo.

rem --- Audit Logger ---
echo [INFO] Construyendo !IMG_AUDIT! ...
docker build -t "!IMG_AUDIT!" ^
    -f services\audit-logger\Dockerfile ^
    services\audit-logger
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Audit Logger.
    goto :error
)
echo [OK] !IMG_AUDIT! construida.
echo.

rem --- Orchestrator ---
echo [INFO] Construyendo !IMG_ORCH! ...
docker build -t "!IMG_ORCH!" ^
    -f services\orchestrator\Dockerfile ^
    services\orchestrator
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Orchestrator.
    goto :error
)
echo [OK] !IMG_ORCH! construida.
echo.

rem --- Designer (SPA React — las VITE_* URLs se hornean en el bundle) ---
echo [INFO] Construyendo !IMG_DESIGNER! ...
echo        (VITE_ENGINE_API_URL=!ENGINE_URL!)
echo        (VITE_AUDIT_API_URL=!AUDIT_URL!)
docker build -t "!IMG_DESIGNER!" ^
    --build-arg "VITE_ENGINE_API_URL=!ENGINE_URL!" ^
    --build-arg "VITE_AUDIT_API_URL=!AUDIT_URL!" ^
    -f apps\designer\Dockerfile ^
    apps\designer
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al construir la imagen del Designer.
    goto :error
)
echo [OK] !IMG_DESIGNER! construida.

popd >nul
echo.

rem ============================================================================
rem  PASO 4 — Publicar las imagenes en el registro de contenedores
rem  (Paso 3 de la guia de despliegue)
rem ============================================================================
echo.
echo ── PASO 4: Publicando imagenes en el registro ──────────────────────────────
echo.

echo [INFO] Publicando !IMG_ENGINE! ...
docker push "!IMG_ENGINE!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al publicar la imagen del Engine.
    echo         Comprueba que tienes permisos de escritura en !REGISTRY!
    goto :error
)
echo [OK] !IMG_ENGINE! publicada.

echo [INFO] Publicando !IMG_AUDIT! ...
docker push "!IMG_AUDIT!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al publicar la imagen del Audit Logger.
    goto :error
)
echo [OK] !IMG_AUDIT! publicada.

echo [INFO] Publicando !IMG_ORCH! ...
docker push "!IMG_ORCH!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al publicar la imagen del Orchestrator.
    goto :error
)
echo [OK] !IMG_ORCH! publicada.

echo [INFO] Publicando !IMG_DESIGNER! ...
docker push "!IMG_DESIGNER!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al publicar la imagen del Designer.
    goto :error
)
echo [OK] !IMG_DESIGNER! publicada.
echo.

rem ============================================================================
rem  PASO 5 — Generar secretos de produccion
rem  (Paso 4.1 y 4.2 de la guia)
rem  - AES-256: 32 bytes aleatorios -> base64 (44 chars), siempre > 32 bytes
rem  - DB_PASSWORD: 24 bytes aleatorios -> base64
rem  Ambos valores se inyectan en el cluster via kubectl patch; nunca se
rem  guardan en los YAML del repositorio.
rem ============================================================================
echo.
echo ── PASO 5: Generando secretos de produccion ────────────────────────────────
echo.

for /f "delims=" %%K in ('powershell -NoProfile -Command "[System.Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))"') do set AES_KEY=%%K
if "!AES_KEY!"=="" (
    echo [ERROR] No se pudo generar la clave AES con PowerShell.
    goto :error
)
echo [OK] Clave AES-256 generada (256 bits, base64).

if /i "!DB_MODE!"=="in-cluster" (
    rem Generar contrasena segura para PostgreSQL
    for /f "delims=" %%P in ('powershell -NoProfile -Command "[System.Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(24))"') do set DB_PASSWORD=%%P
    if "!DB_PASSWORD!"=="" (
        echo [ERROR] No se pudo generar la contrasena de PostgreSQL.
        goto :error
    )
    echo [OK] Contrasena de PostgreSQL generada (192 bits, base64).

    rem Construir los DSNs in-cluster con las credenciales generadas
    set DB_USER=flowjs_admin
    set AUDIT_DSN=host=postgres port=5432 user=!DB_USER! password=!DB_PASSWORD! dbname=flowjs_audit sslmode=disable
    set CONFIG_DSN=postgres://!DB_USER!:!DB_PASSWORD!@postgres:5432/flowjs_config?sslmode=disable

    rem Base64 para el Secret de postgres (formato kubectl)
    for /f "delims=" %%U in ('powershell -NoProfile -Command "[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes(\"!DB_USER!\"))"') do set DB_USER_B64=%%U
    for /f "delims=" %%W in ('powershell -NoProfile -Command "[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes(\"!DB_PASSWORD!\"))"') do set DB_PASS_B64=%%W
)
echo.

rem ============================================================================
rem  PASO 6 — Aplicar los manifiestos Kubernetes base via kustomize
rem  (Paso 6 de la guia de despliegue)
rem ============================================================================
echo.
echo ── PASO 6: Aplicando manifiestos Kubernetes ────────────────────────────────
echo.

pushd "%REPO_ROOT%" >nul
kubectl apply -k deploy\k8s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Fallo al aplicar los manifiestos con kustomize.
    goto :error
)
popd >nul
echo.
echo [OK] Manifiestos base aplicados. Namespace '!NAMESPACE!' creado/actualizado.
echo.

rem ============================================================================
rem  PASO 7 — Parchear el cluster para produccion
rem  Actualizar: imagenes con tag real, AES key, ALLOWED_ORIGINS, DSNs,
rem  contrasena de PostgreSQL e imagePullPolicy: Always.
rem  (Paso 4 de la guia de despliegue)
rem ============================================================================
echo.
echo ── PASO 7: Aplicando configuracion de produccion al cluster ────────────────
echo.

rem 7.1 — Actualizar referencias de imagen en los Deployments
echo [INFO] 7.1 Actualizando imagen del Engine a !IMG_ENGINE! ...
kubectl -n !NAMESPACE! set image deployment/engine engine="!IMG_ENGINE!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo actualizar la imagen del engine.
    goto :error
)

echo [INFO] 7.1 Actualizando imagen del Audit Logger a !IMG_AUDIT! ...
kubectl -n !NAMESPACE! set image deployment/audit-logger audit-logger="!IMG_AUDIT!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo actualizar la imagen del audit-logger.
    goto :error
)

echo [INFO] 7.1 Actualizando imagen del Orchestrator a !IMG_ORCH! ...
kubectl -n !NAMESPACE! set image deployment/orchestrator orchestrator="!IMG_ORCH!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo actualizar la imagen del orchestrator.
    goto :error
)

echo [INFO] 7.1 Actualizando imagen del Designer a !IMG_DESIGNER! ...
kubectl -n !NAMESPACE! set image deployment/designer designer="!IMG_DESIGNER!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo actualizar la imagen del designer.
    goto :error
)
echo [OK] Imagenes actualizadas a !TAG!.

rem 7.2 — imagePullPolicy: Always (necesario en produccion con tags fijos)
echo [INFO] 7.2 Estableciendo imagePullPolicy=Always en todos los deployments...
for %%D in (engine audit-logger orchestrator designer) do (
    kubectl -n !NAMESPACE! patch deployment %%D ^
        --type=json ^
        -p "[{\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/imagePullPolicy\",\"value\":\"Always\"}]" ^
        >nul 2>&1
)
echo [OK] imagePullPolicy=Always aplicado.

rem 7.3 — Clave AES-256 de produccion
echo [INFO] 7.3 Aplicando clave AES-256 de produccion...
kubectl -n !NAMESPACE! patch secret engine-secret ^
    --type=merge ^
    -p "{\"stringData\":{\"SECRETS_AES_KEY\":\"!AES_KEY!\"}}"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo parchear engine-secret.
    goto :error
)
echo [OK] SECRETS_AES_KEY actualizada.

rem 7.4 — ALLOWED_ORIGINS para CORS en produccion
rem El engine y el audit-logger deben aceptar peticiones del dominio del Designer.
echo [INFO] 7.4 Actualizando ALLOWED_ORIGINS a '!DESIGNER_URL!' ...
kubectl -n !NAMESPACE! patch configmap engine-config ^
    --type=merge ^
    -p "{\"data\":{\"ALLOWED_ORIGINS\":\"!DESIGNER_URL!\"}}"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo parchear engine-config (ALLOWED_ORIGINS).
    goto :error
)
kubectl -n !NAMESPACE! patch configmap audit-logger-config ^
    --type=merge ^
    -p "{\"data\":{\"ALLOWED_ORIGINS\":\"!DESIGNER_URL!\"}}"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo parchear audit-logger-config (ALLOWED_ORIGINS).
    goto :error
)
echo [OK] ALLOWED_ORIGINS actualizado.

rem 7.5 — DSN del Audit Logger
echo [INFO] 7.5 Actualizando POSTGRES_DSN del Audit Logger...
kubectl -n !NAMESPACE! patch secret audit-logger-secret ^
    --type=merge ^
    -p "{\"stringData\":{\"POSTGRES_DSN\":\"!AUDIT_DSN!\"}}"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo parchear audit-logger-secret.
    goto :error
)
echo [OK] POSTGRES_DSN actualizado.

rem 7.6 — DATABASE_URL del Engine y Orchestrator
if not "!CONFIG_DSN!"=="" (
    echo [INFO] 7.6 Actualizando DATABASE_URL del Engine y Orchestrator...
    kubectl -n !NAMESPACE! patch configmap engine-config ^
        --type=merge ^
        -p "{\"data\":{\"DATABASE_URL\":\"!CONFIG_DSN!\"}}"
    kubectl -n !NAMESPACE! patch configmap orchestrator-config ^
        --type=merge ^
        -p "{\"data\":{\"DATABASE_URL\":\"!CONFIG_DSN!\"}}"
    echo [OK] DATABASE_URL actualizado.
)

rem 7.7 — Credenciales PostgreSQL in-cluster
if /i "!DB_MODE!"=="in-cluster" (
    echo [INFO] 7.7 Actualizando credenciales de PostgreSQL in-cluster...
    kubectl -n !NAMESPACE! patch secret postgres-secret ^
        --type=merge ^
        -p "{\"data\":{\"POSTGRES_USER\":\"!DB_USER_B64!\",\"POSTGRES_PASSWORD\":\"!DB_PASS_B64!\"}}"
    if %ERRORLEVEL% neq 0 (
        echo [ERROR] No se pudo parchear postgres-secret.
        goto :error
    )
    echo [OK] Credenciales de PostgreSQL actualizadas.
)

rem 7.8 — APP_ENV = production en todos los ConfigMaps
echo [INFO] 7.8 Verificando APP_ENV=production...
kubectl -n !NAMESPACE! patch configmap engine-config --type=merge -p "{\"data\":{\"APP_ENV\":\"production\"}}" >nul 2>&1
kubectl -n !NAMESPACE! patch configmap audit-logger-config --type=merge -p "{\"data\":{\"APP_ENV\":\"production\"}}" >nul 2>&1
echo [OK] APP_ENV=production confirmado.

rem 7.9 — Reiniciar deployments para que tomen toda la configuracion nueva
echo [INFO] 7.9 Reiniciando deployments...
kubectl -n !NAMESPACE! rollout restart deployment/engine
kubectl -n !NAMESPACE! rollout restart deployment/audit-logger
kubectl -n !NAMESPACE! rollout restart deployment/orchestrator
kubectl -n !NAMESPACE! rollout restart deployment/designer
echo [OK] Rollout restart enviado a todos los deployments.
echo.

rem ============================================================================
rem  PASO 8 — Inicializar las bases de datos
rem  Solo para modo in-cluster. En modo external las bases de datos se asumen
rem  ya inicializadas o se inicializan manualmente con los scripts de init-db/.
rem  (Paso 5 de la guia de despliegue)
rem ============================================================================
echo.
echo ── PASO 8: Inicializando bases de datos ────────────────────────────────────
echo.

if /i "!DB_MODE!"=="external" (
    echo [INFO] Modo external: omitiendo inicializacion in-cluster.
    echo        Asegurate de haber ejecutado los scripts init-db/*.sql contra
    echo        tu base de datos gestionada antes de continuar.
    echo        Scripts disponibles en: %REPO_ROOT%\init-db\
    pause
    goto :skip_db_init
)

rem Esperar al pod de PostgreSQL
echo [INFO] Esperando al pod de PostgreSQL (timeout: !POD_TIMEOUT!s)...

rem Defensivo: si DB_USER no fue establecido (modo in-cluster esperado), usar 'admin' como fallback
if "!DB_USER!"=="" set DB_USER=admin
kubectl -n !NAMESPACE! wait pod ^
    --selector=app=postgres ^
    --for=condition=Ready ^
    --timeout=!POD_TIMEOUT!s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] PostgreSQL no estuvo listo en !POD_TIMEOUT! segundos.
    echo         kubectl -n !NAMESPACE! describe pod -l app=postgres
    goto :error
)

for /f "delims=" %%P in ('kubectl -n !NAMESPACE! get pod -l app=postgres -o jsonpath={.items[0].metadata.name} 2^>nul') do set POSTGRES_POD=%%P
if "!POSTGRES_POD!"=="" (
    echo [ERROR] No se encontro el pod de PostgreSQL.
    goto :error
)
echo [OK] Pod PostgreSQL: !POSTGRES_POD!

echo [INFO] Copiando scripts SQL al pod...
kubectl -n !NAMESPACE! cp "%REPO_ROOT%\init-db\01-init.sql" "!POSTGRES_POD!:/tmp/01-init.sql"
kubectl -n !NAMESPACE! cp "%REPO_ROOT%\init-db\02-audit-schema.sql" "!POSTGRES_POD!:/tmp/02-audit-schema.sql"
kubectl -n !NAMESPACE! cp "%REPO_ROOT%\init-db\03-config-schema.sql" "!POSTGRES_POD!:/tmp/03-config-schema.sql"
echo [OK] Scripts copiados.

echo [INFO] Ejecutando 01-init.sql ...
kubectl -n !NAMESPACE! exec "!POSTGRES_POD!" -- psql -U !DB_USER! -f /tmp/01-init.sql
if %ERRORLEVEL% neq 0 (
    echo [AVISO] 01-init.sql reporto errores (puede ser normal si las BDs ya existen).
)

echo [INFO] Ejecutando 02-audit-schema.sql ...
kubectl -n !NAMESPACE! exec "!POSTGRES_POD!" -- psql -U !DB_USER! -f /tmp/02-audit-schema.sql
if %ERRORLEVEL% neq 0 (
    echo [AVISO] 02-audit-schema.sql reporto errores (puede ser normal).
)

echo [INFO] Ejecutando 03-config-schema.sql ...
kubectl -n !NAMESPACE! exec "!POSTGRES_POD!" -- psql -U !DB_USER! -f /tmp/03-config-schema.sql
if %ERRORLEVEL% neq 0 (
    echo [AVISO] 03-config-schema.sql reporto errores (puede ser normal).
)
echo [OK] Bases de datos inicializadas.

:skip_db_init
echo.

rem ============================================================================
rem  PASO 9 — Aplicar recurso Ingress con los dominios de produccion
rem  (Paso 8 Opcion B de la guia de despliegue)
rem  Se genera un ingress.yaml temporal con los dominios configurados y
rem  se aplica al cluster.
rem ============================================================================
echo.
echo ── PASO 9: Aplicando Ingress de produccion ─────────────────────────────────
echo.

if /i "!INGRESS_CLASS!"=="none" (
    echo [INFO] INGRESS_CLASS=none: omitiendo Ingress.
    echo        Recuerda parchear los servicios a tipo LoadBalancer manualmente:
    echo          kubectl -n !NAMESPACE! patch svc designer -p "{\"spec\":{\"type\":\"LoadBalancer\"}}"
    echo          kubectl -n !NAMESPACE! patch svc engine   -p "{\"spec\":{\"type\":\"LoadBalancer\"}}"
    goto :skip_ingress
)

rem Generar el YAML del Ingress en un fichero temporal
set INGRESS_TMP=%TEMP%\flowjs-ingress-prod.yaml

rem Construir anotaciones TLS y spec segun el ingress controller
if /i "!INGRESS_CLASS!"=="traefik" (
    set INGRESS_ANNOTATION_CLASS=traefik
) else (
    set INGRESS_ANNOTATION_CLASS=nginx
)

rem Escribir el YAML base del Ingress
(
    echo apiVersion: networking.k8s.io/v1
    echo kind: Ingress
    echo metadata:
    echo   name: flowjs-ingress
    echo   namespace: !NAMESPACE!
    echo   annotations:
    echo     kubernetes.io/ingress.class: "!INGRESS_ANNOTATION_CLASS!"
    if /i "!INGRESS_CLASS!"=="nginx" (
        echo     nginx.ingress.kubernetes.io/proxy-body-size: "16m"
        echo     nginx.ingress.kubernetes.io/proxy-read-timeout: "120"
        echo     nginx.ingress.kubernetes.io/proxy-send-timeout: "120"
    )
    if /i not "!TLS_ISSUER!"=="" (
        echo     cert-manager.io/cluster-issuer: "!TLS_ISSUER!"
    )
    echo spec:
    if /i not "!TLS_ISSUER!"=="" (
        echo   tls:
        echo   - hosts:
        echo     - !DESIGNER_DOMAIN!
        echo     secretName: flowjs-designer-tls
        echo   - hosts:
        echo     - !ENGINE_DOMAIN!
        echo     secretName: flowjs-engine-tls
        echo   - hosts:
        echo     - !AUDIT_DOMAIN!
        echo     secretName: flowjs-audit-tls
    )
    echo   rules:
    echo   - host: !DESIGNER_DOMAIN!
    echo     http:
    echo       paths:
    echo       - path: /
    echo         pathType: Prefix
    echo         backend:
    echo           service:
    echo             name: designer
    echo             port:
    echo               number: 80
    echo   - host: !ENGINE_DOMAIN!
    echo     http:
    echo       paths:
    echo       - path: /
    echo         pathType: Prefix
    echo         backend:
    echo           service:
    echo             name: engine
    echo             port:
    echo               number: 9090
    echo   - host: !AUDIT_DOMAIN!
    echo     http:
    echo       paths:
    echo       - path: /
    echo         pathType: Prefix
    echo         backend:
    echo           service:
    echo             name: audit-logger
    echo             port:
    echo               number: 8080
) > "!INGRESS_TMP!"

echo [INFO] Aplicando Ingress (controller: !INGRESS_CLASS!)...
kubectl apply -f "!INGRESS_TMP!"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] No se pudo aplicar el Ingress.
    echo         Revisa si el Ingress Controller esta instalado en el cluster.
    del "!INGRESS_TMP!" >nul 2>&1
    goto :error
)
del "!INGRESS_TMP!" >nul 2>&1
echo [OK] Ingress aplicado correctamente.
echo.
echo [INFO] Ingress rules creados:
echo          !DESIGNER_DOMAIN!  ->  svc/designer:80
echo          !ENGINE_DOMAIN!   ->  svc/engine:9090
echo          !AUDIT_DOMAIN!    ->  svc/audit-logger:8080
echo.

:skip_ingress

rem ============================================================================
rem  PASO 10 — Esperar a que todos los deployments completen el rollout
rem  (Paso 7 de la guia de despliegue)
rem ============================================================================
echo.
echo ── PASO 10: Esperando rollout de todos los deployments ──────────────────────
echo.

echo [INFO] Aguardando rollout del engine...
kubectl -n !NAMESPACE! rollout status deployment/engine --timeout=!POD_TIMEOUT!s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] El engine no completo el rollout.
    echo         kubectl -n !NAMESPACE! describe deployment engine
    echo         kubectl -n !NAMESPACE! logs -l app=engine --tail=50
    goto :error
)
echo [OK] engine: rollout completo.

echo [INFO] Aguardando rollout del audit-logger...
kubectl -n !NAMESPACE! rollout status deployment/audit-logger --timeout=!POD_TIMEOUT!s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] El audit-logger no completo el rollout.
    goto :error
)
echo [OK] audit-logger: rollout completo.

echo [INFO] Aguardando rollout del orchestrator...
kubectl -n !NAMESPACE! rollout status deployment/orchestrator --timeout=!POD_TIMEOUT!s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] El orchestrator no completo el rollout.
    goto :error
)
echo [OK] orchestrator: rollout completo.

echo [INFO] Aguardando rollout del designer...
kubectl -n !NAMESPACE! rollout status deployment/designer --timeout=!POD_TIMEOUT!s
if %ERRORLEVEL% neq 0 (
    echo [ERROR] El designer no completo el rollout.
    goto :error
)
echo [OK] designer: rollout completo.

echo.
echo [OK] Todos los deployments estan Available:
kubectl -n !NAMESPACE! get pods
echo.

rem ============================================================================
rem  PASO 11 — Verificar health checks sobre los dominios publicos
rem  (Paso 9 de la guia de despliegue)
rem  Solo se intenta si los dominios son alcanzables desde esta maquina.
rem  Es normal que fallen si los DNS o el LoadBalancer aun no han propagado.
rem ============================================================================
echo.
echo ── PASO 11: Verificando health checks ──────────────────────────────────────
echo.
echo [INFO] Probando /health del Engine en !ENGINE_URL!/health ...
echo        (Si el DNS aun no ha propagado, esto puede fallar. Es normal.)
echo.

set HEALTH_OK=0
for /l %%i in (1,1,6) do (
    if !HEALTH_OK! equ 0 (
        for /f "delims=" %%C in ('curl -sk -o nul -w "%%{http_code}" "!ENGINE_URL!/health" 2^>nul') do set HTTP_CODE=%%C
        if "!HTTP_CODE!"=="200" (
            set HEALTH_OK=1
            echo [OK] Engine /health respondio 200 OK.
        ) else (
            echo [INFO] Intento %%i/6 (codigo: !HTTP_CODE!) ... reintentando en 10s...
            timeout /t 10 /nobreak >nul
        )
    )
)
if !HEALTH_OK! equ 0 (
    echo [AVISO] Engine no respondio 200 en !ENGINE_URL!/health
    echo         Puede ser normal si el DNS/LB no ha propagado todavia.
    echo         Comprueba manualmente cuando este disponible.
)

set HEALTH_OK=0
for /l %%i in (1,1,6) do (
    if !HEALTH_OK! equ 0 (
        for /f "delims=" %%C in ('curl -sk -o nul -w "%%{http_code}" "!AUDIT_URL!/health" 2^>nul') do set HTTP_CODE=%%C
        if "!HTTP_CODE!"=="200" (
            set HEALTH_OK=1
            echo [OK] Audit Logger /health respondio 200 OK.
        ) else (
            echo [INFO] Intento %%i/6 (codigo: !HTTP_CODE!) ... reintentando en 10s...
            timeout /t 10 /nobreak >nul
        )
    )
)
if !HEALTH_OK! equ 0 (
    echo [AVISO] Audit Logger no respondio 200 en !AUDIT_URL!/health
    echo         Puede ser normal si el DNS/LB no ha propagado todavia.
)
echo.

rem ============================================================================
rem  PASO 12 — Smoke test: crear y ejecutar un flujo de prueba
rem  (Paso 10 de la guia de despliegue)
rem  Se omite si el engine no fue alcanzable en el Paso 11.
rem ============================================================================
echo.
echo ── PASO 12: Smoke test — flujo 'hola-mundo-prod' ───────────────────────────
echo.
echo [INFO] Creando flujo de prueba en !ENGINE_URL!/api/v1/processes ...

powershell -NoProfile -Command ^
    "$body = @{ definition = @{ id = 'hola-mundo-prod'; version = '!TAG!'; name = 'Hola Mundo (prod)' }; trigger = @{ id = 'trg_manual'; type = 'manual' }; nodes = @(@{ id = 'log_saludo'; type = 'log'; input_mapping = @{ message = '$.trigger.body.nombre' }; config = @{ level = 'INFO' } }) } | ConvertTo-Json -Depth 10; try { $r = Invoke-RestMethod -Method POST -Uri '!ENGINE_URL!/api/v1/processes' -ContentType 'application/json' -Body $body; Write-Host '[OK] Flujo creado:' ($r | ConvertTo-Json -Compress) } catch { Write-Host '[AVISO] No se pudo crear el flujo (puede ser normal si el DNS no ha propagado):' $_.Exception.Message }"

echo.
echo [INFO] Ejecutando replay del flujo 'hola-mundo-prod'...
powershell -NoProfile -Command ^
    "$body = @{ trigger_data = @{ nombre = 'FlowJS en produccion' } } | ConvertTo-Json; try { $r = Invoke-RestMethod -Method POST -Uri '!ENGINE_URL!/api/v1/processes/hola-mundo-prod/replay' -ContentType 'application/json' -Body $body; Write-Host '[OK] Ejecucion lanzada:' ($r | ConvertTo-Json -Compress) } catch { Write-Host '[AVISO] Replay:' $_.Exception.Message }"

echo.
echo [INFO] Consultando historial de ejecuciones en el Audit Logger...
powershell -NoProfile -Command ^
    "try { $r = Invoke-RestMethod -Uri '!AUDIT_URL!/executions'; Write-Host '[OK] Ejecuciones registradas:' ($r | Measure-Object).Count } catch { Write-Host '[AVISO] Historial:' $_.Exception.Message }"

echo.

rem ============================================================================
rem  PASO 13 — Guardar resumen de configuracion en fichero de texto
rem  Este fichero NO se debe commitear al repositorio (esta en .gitignore).
rem  Contiene las credenciales generadas — guardalo en un lugar seguro.
rem ============================================================================
echo.
echo ── PASO 13: Guardando resumen de configuracion ──────────────────────────────
echo.

set SUMMARY_FILE=%REPO_ROOT%\deploy-prod-summary.txt
(
    echo FlowJS-Works — Resumen de Despliegue de Produccion
    echo Fecha    : %DATE% %TIME%
    echo Contexto : !KUBE_CONTEXT!
    echo Namespace: !NAMESPACE!
    echo Tag      : !TAG!
    echo.
    echo === Imagenes desplegadas ===
    echo   Engine      : !IMG_ENGINE!
    echo   AuditLogger : !IMG_AUDIT!
    echo   Orchestrator: !IMG_ORCH!
    echo   Designer    : !IMG_DESIGNER!
    echo.
    echo === URLs de acceso ===
    echo   Designer UI  : !DESIGNER_URL!
    echo   Engine API   : !ENGINE_URL!
    echo   Audit Logger : !AUDIT_URL!
    echo.
    echo === Secretos generados (guardar en lugar seguro) ===
    echo   SECRETS_AES_KEY : !AES_KEY!
    if /i "!DB_MODE!"=="in-cluster" (
        echo   DB_USER         : !DB_USER!
        echo   DB_PASSWORD     : !DB_PASSWORD!
        echo   AUDIT_DSN       : !AUDIT_DSN!
        echo   CONFIG_DSN      : !CONFIG_DSN!
    ) else (
        echo   AUDIT_DSN       : (externo - proporcionado por el usuario)
        echo   CONFIG_DSN      : (externo - proporcionado por el usuario)
    )
    echo.
    echo === Configuracion de Ingress ===
    echo   Controller : !INGRESS_CLASS!
    if /i not "!TLS_ISSUER!"=="" (
        echo   TLS Issuer : !TLS_ISSUER!
    )
    echo.
    echo === Comandos utiles ===
    echo   Ver pods:          kubectl -n !NAMESPACE! get pods
    echo   Logs engine:       kubectl -n !NAMESPACE! logs -l app=engine -f
    echo   Logs audit:        kubectl -n !NAMESPACE! logs -l app=audit-logger -f
    echo   Rollback engine:   kubectl -n !NAMESPACE! rollout undo deployment/engine
    echo   Hot-reload flujo:  curl -X POST !ENGINE_URL!/api/v1/processes/^<id^>/reload
) > "!SUMMARY_FILE!"

echo [OK] Resumen guardado en: !SUMMARY_FILE!
echo      IMPORTANTE: Este fichero contiene credenciales. Guardalo en un lugar
echo      seguro (gestor de secretos) y NO lo subas al repositorio.
echo.

rem ============================================================================
rem  RESUMEN FINAL EN CONSOLA
rem ============================================================================
echo.
echo ================================================================
echo   DESPLIEGUE DE PRODUCCION COMPLETADO
echo ================================================================
echo.
echo   Cluster  : !KUBE_CONTEXT!
echo   Namespace: !NAMESPACE!
echo   Tag      : !TAG!
echo.
echo   Servicios accesibles:
echo   ┌───────────────────────────────────────────────────────────┐
echo   │  Designer UI    ->  !DESIGNER_URL!
echo   │  Engine API     ->  !ENGINE_URL!
echo   │  Audit Logger   ->  !AUDIT_URL!
echo   └───────────────────────────────────────────────────────────┘
echo.
echo   Comandos utiles:
echo   ─────────────────────────────────────────────────────────────
echo   Ver todos los pods:
echo     kubectl -n !NAMESPACE! get pods
echo.
echo   Ver logs en tiempo real:
echo     kubectl -n !NAMESPACE! logs -l app=engine -f
echo     kubectl -n !NAMESPACE! logs -l app=audit-logger -f
echo.
echo   Rollback a la version anterior:
echo     kubectl -n !NAMESPACE! rollout undo deployment/engine
echo     kubectl -n !NAMESPACE! rollout undo deployment/audit-logger
echo.
echo   Historial de rollouts:
echo     kubectl -n !NAMESPACE! rollout history deployment/engine
echo.
echo   Ver Ingress y su IP/hostname asignado:
echo     kubectl -n !NAMESPACE! get ingress
echo.
echo ================================================================
echo   Consulta docs\guia-despliegue-kubernetes.md para mas detalles:
echo   - Seccion  9: Hot-Reload de flujos sin interrupcion
echo   - Seccion 10: Monitorizacion con Prometheus
echo   - Seccion 13: Resolucion de problemas comunes
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
echo   [ERROR] El script de despliegue termino con errores.
echo.
echo   Sugerencias:
echo     - Revisa los mensajes anteriores para identificar el fallo.
echo     - Consulta la seccion 13 (Resolucion de problemas) de:
echo         docs\guia-despliegue-kubernetes.md
echo     - Para ver el estado actual del cluster:
echo         kubectl -n %NAMESPACE% get pods
echo         kubectl -n %NAMESPACE% get events --sort-by=.lastTimestamp
echo     - Para hacer rollback a la version anterior:
echo         kubectl -n %NAMESPACE% rollout undo deployment/engine
echo         kubectl -n %NAMESPACE% rollout undo deployment/audit-logger
echo ================================================================
echo.
endlocal
exit /b 1
