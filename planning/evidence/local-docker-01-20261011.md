# DOCKER-01 — evidencia enfocada de empaquetado y configuración

Fecha: 2026-10-11. Código base validado en `9cc3657c75175b7ebbf1d3122c14a9280eb702d3`; los ajustes de revisión y sus pruebas se validaron en `432e9544f12f1daea43fae1968096c25ed5d31e1`. Este documento se actualiza en el commit de documentación siguiente. Cuenta GitHub efectiva verificada: `HernanMEC`. LOCAL-1 fue aceptado explícitamente por el propietario dentro del alcance sintético ratificado; #246 está cerrada y Hecho en Projects. Este corte inicia LOCAL-DOCKER y no modifica el comportamiento de negocio.

## Cambios incluidos

- Dockerfile con targets separados `api` y `mock`: el primero compila Backend y netprobe en Go sin ejecutar Node; el segundo compila TypeScript y servidor mock Go.
- Backend y migrador reciben configuración PostgreSQL parametrizada, incluidos host/puerto/base/usuario y `DATABASE_SSLMODE`, `DATABASE_SSLROOTCERT`, `DATABASE_SSLCERT`, `DATABASE_SSLKEY`. Los secretos continúan por archivos. `DATABASE_SSLMODE` conserva default `disable`; certificados cliente requieren cert y key juntos.
- Compose parameteriza nombre del proyecto, puertos publicados, base y volumen nombrado. Defaults de desarrollo conservan `espacigo`, puertos 8080/8081/8025 y `espacigo_pgdata`.
- `backend` usa `espacigo_runtime`; `migrate` permanece proceso one-shot separado con `espacigo_admin`.
- `scripts/local-docker-stack.sh` ofrece config/build/up/down/status para un proyecto dedicado por defecto `espacigo_docker01`, puertos 18080/18081/18025, volumen `espacigo_docker01_pgdata`, secretos aleatorios propios y directorios privados independientes. `down` preserva volumen y secretos.

## Comprobaciones ejecutadas

```sh
go test ./cmd/api -run 'TestLoadConfig' -count=1
bash -n scripts/local-docker-stack.sh scripts/test-local-docker-01.sh scripts/dev-env.sh
git diff --check
scripts/test-local-docker-01.sh
```

Resultados: PASS. El test renderizó las configuraciones de desarrollo y aislada; verificó nombres, puertos, volumen, ubicación/permisos de secretos, directorios TLS, configuración TLS para backend/migrador, roles separados y red/credenciales API-only del mock. Rechazó colisión con los puertos de desarrollo y duplicación de puertos aislados. Compiló por separado ambos targets: el log de `api` no contiene etapas `mock-assets-build` ni `mock-build`; el target `mock` ejecutó ambas.

Correcciones de revisión de #271: el helper compara rutas resueltas antes de `prepare` o de invocar Docker. Pruebas con el directorio raíz apuntando mediante symlink al `.local` activo y con un subdirectorio de evidencia enlazado al directorio real de desarrollo confirman rechazo; una instantánea de modos/metadatos y hashes locales de secretos demuestra que no hubo cambios y el Docker shim no recibió llamadas. Un `LOCAL_DATABASE_RUNTIME_USER=espacigo_admin` incompatible también se rechaza antes de crear el estado aislado o invocar Compose. El servicio Backend de Compose fija `espacigo_runtime`.

No se levantaron servicios, no se aplicaron migraciones, no se tocó `espacigo_pgdata`, secretos ni datos del entorno actual. Docker reutilizó las capas disponibles; al salir, el test retiró únicamente sus dos tags temporales de imagen. No se eliminaron cachés.

La aceptación de #246 está registrada en [GitHub](https://github.com/HernanEspinozaDev/espaciGo/issues/246#issuecomment-6112922353). Los Issue padres con criterios productivos o diferidos permanecen abiertos. DOCKER-02 (#269) y DOCKER-03 (#270) siguen bloqueados hasta completar sus dependencias; la validación integral corresponde a esos cortes.
