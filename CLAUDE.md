# CLAUDE.md

Contexto y reglas para asistentes de IA (Claude u otros) que trabajen en este repositorio. Leelo completo antes de proponer o escribir cambios.

## Proyecto

Sistema integral de gestión de gimnasio: Trabajo Práctico Integrador de Arquitectura de Software, construido como **microservicios**. Incluye membresías, clases y reservas con cupo, asistencia, planes de entrenamiento y nutrición, y un Club de Beneficios con puntos expuesto a partners de otros grupos.

- **Qué hace el sistema:** [SPEC.md](SPEC.md) (aprobado; fuente de verdad del negocio).
- **Cómo está construido:** [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) y [docs/adr/](docs/adr/).
- **Etapa actual:** 03 — contrato público de la API de partners publicado ([docs/contracts/](docs/contracts/README.md)) con mock; **sin código de aplicación todavía**.

## Stack

| Capa | Tecnología |
|---|---|
| Backend | Go 1.22+ con Gin |
| Frontend | React + Vite + TypeScript |
| Datos | PostgreSQL 16 (una instancia, una base por servicio), MongoDB 7, Redis 7, OpenSearch 2 |
| Mensajería | RabbitMQ (exchange topic `gym.events`) |
| Balanceo | Traefik (solo delante de booking-service) |
| Observabilidad | OpenTelemetry, OTel Collector, Jaeger, Prometheus, Loki, Grafana |
| Email local | Mailpit |
| Infra local | Docker Compose + Makefile |
| CI | GitHub Actions |
| Tests | `go test`, testify, testcontainers-go, k6 |

## Servicios

| Servicio | Puerto | Patrón | Datos | Publica | Consume |
|---|---|---|---|---|---|
| `api-gateway` | 8080 | Middlewares | Redis (rate limit) | — | — |
| `members-service` | 8081 | **Capas** | PostgreSQL `members_db` | `membresia.activada`, `membresia.cancelada`, `usuario.desactivado` | — |
| `booking-service` (×2, Traefik) | 8082 | **Hexagonal + CQRS** | PostgreSQL `booking_db` + OpenSearch | `asistencia.registrada`, `clase.actualizada` | `membresia.cancelada`, `usuario.desactivado` |
| `booking-indexer` | — | Consumidor | OpenSearch `clases` | — | `clase.actualizada` |
| `benefits-service` | 8083 | **Hexagonal** | PostgreSQL `benefits_db` | — | `asistencia.registrada` |
| `training-service` | 8084 | **Capas** | MongoDB `training_db` | — | — |
| `notification-worker` | 8085 | Consumidor | MongoDB `notifications_db` | — | `membresia.activada` |
| `web` | 5173 | SPA | — | — | — |

Además (aprobado 2026-10-07): members publica `membresia.cancelada` y `usuario.desactivado`, que consume booking (RN-04, RN-34); training consulta asignaciones a members (RN-30); booking consulta datos de alumnos a members para listados (HU-21).

## Estructura de carpetas

```text
gateway/                     api-gateway
services/<servicio>/         un módulo Go por servicio
  cmd/<binario>/             main.go
  internal/                  código privado del servicio
  migrations/                SQL versionado (servicios con Postgres)
pkg/                         librerías técnicas compartidas (sin dominio)
web/                         frontend
deploy/                      compose, init de bases, rabbitmq, traefik, observabilidad
docs/                        ARCHITECTURE, adr/, api/ (OpenAPI), events/ (JSON Schema)
tests/load/                  k6
tests/e2e/                   flujos del SPEC §11
```

### Patrón CAPAS (members, training)

```text
internal/handler/      HTTP (Gin): parseo, validación de formato, mapeo a RFC 7807
internal/service/      reglas de negocio y orquestación
internal/repository/   acceso a datos (interfaces + implementación)
internal/model/        entidades y DTOs
```

Dependencias solo hacia abajo: handler → service → repository. El handler nunca usa el repository.

### Patrón HEXAGONAL (booking, benefits)

```text
internal/domain/                 entidades, value objects, reglas, errores de dominio
internal/application/            casos de uso + puertos (interfaces de entrada y salida)
internal/adapters/in/{http,amqp}/
internal/adapters/out/{postgres,opensearch,members,outbox}/
```

`domain` **no importa** nada de infraestructura (ni Gin, ni drivers, ni `net/http`, ni `pkg/` técnico). `application` depende solo de `domain` y de sus propios puertos. Los adaptadores implementan los puertos. El cableado se hace en `cmd/`.

## Convenciones

### Errores — RFC 7807

- Toda respuesta de error es `application/problem+json` con `type`, `title`, `status`, `detail`, `instance`, `code` y `correlation_id`.
- `code` sale del catálogo de [ARCHITECTURE §12.1](docs/ARCHITECTURE.md#121-errores--rfc-7807-applicationproblemjson) (ej. `SIN_CUPO`, `MEMBRESIA_NO_VIGENTE`, `SALDO_INSUFICIENTE`). No inventar códigos nuevos sin agregarlos allí.
- Usar `pkg/problem`. Nunca devolver errores internos, SQL o stack traces al cliente.

### Logs

- `log/slog` en JSON a stdout. Campos: `service`, `level`, `msg`, `correlation_id`, `trace_id`, `span_id` y, si hay, `user_id`.
- **Prohibido loguear:** emails, DNI, contraseñas, JWT, API keys, cuerpos completos de requests.
- Niveles: `ERROR` (requiere acción), `WARN` (degradación), `INFO` (hechos de negocio y arranque), `DEBUG` (desactivado por defecto).

### Nombres

- Dominio en **español** según el glosario del SPEC, sin tildes en identificadores (`Reserva`, `Membresia`, `CuentaBeneficios`). Términos técnicos en inglés (`Repository`, `Handler`).
- Endpoints: `/api/v1/<recurso-plural-kebab>`; API de partners: `/partner-api/v1/...`; internos: `/internal/v1/...` (el gateway no los expone).
- JSON y columnas: `snake_case`. **Excepción:** el contrato público para partners ([docs/contracts/](docs/contracts/README.md)) usa inglés y `camelCase` (ADR-008). Eventos: `<entidad>.<participio>`. Variables de entorno: `UPPER_SNAKE` con prefijo del servicio.
- Fechas en APIs y eventos: RFC 3339 UTC. Reglas de negocio ("hoy", "vence hoy", "10 minutos antes"): zona `America/Argentina/Buenos_Aires`, siempre con un `Clock` inyectable.

### Commits y ramas

Conventional Commits y GitHub Flow; ver [CONTRIBUTING.md](CONTRIBUTING.md). Ramas `feature/etapa-NN-nombre`.

## Comandos

> Se crean en la etapa de implementación. Si un comando no existe todavía, no lo inventes: avisá.

| Comando | Qué hace |
|---|---|
| `make up` | Levanta todo el stack con Docker Compose |
| `make up-core` | Stack sin observabilidad (máquinas con poca RAM) |
| `make down` | Baja el stack |
| `make test` | Tests unitarios y de integración de todos los módulos |
| `make lint` | `gofmt`, `go vet`, `golangci-lint`, lint del frontend |
| `make logs s=<servicio>` | Logs de un servicio |
| `make load-test` | Pruebas k6 |

## Reglas que toda IA debe respetar en este repo

1. **El SPEC manda.** Antes de implementar, identificá las HU/RN involucradas y citá sus IDs en el código de tests y en el PR. Si algo no está en el SPEC o lo contradice, **preguntá**; no lo resuelvas por tu cuenta.
2. **No cambies decisiones de arquitectura en silencio.** Tecnología, límites de servicio, patrones, puertos y contratos solo cambian con un ADR nuevo aprobado por el equipo. Si ves un problema, explicalo primero.
3. **Cada servicio accede solo a su base.** Nunca agregues una conexión, query o join a la base de otro servicio. Datos ajenos: por API interna o por eventos.
4. **Respetá el patrón de cada servicio.** Nada de lógica de negocio en handlers ni en adaptadores; nada de infraestructura en `domain/`.
5. **`pkg/` es solo técnico.** Nunca pongas reglas, entidades ni DTOs de dominio en `pkg/`.
6. **Publicar = outbox.** Todo evento se escribe en la tabla `outbox` en la misma transacción que el cambio. Nunca publiques directo al broker desde un caso de uso.
7. **Consumir = idempotente.** Deduplicá por `event_id` en la misma transacción que el efecto y hacé ACK después del efecto.
8. **Las invariantes críticas las garantiza la base** (cupo, unicidad, superposición, saldo ≥ 0) además del código. No las debilites para simplificar.
9. **Escrituras sensibles aceptan `Idempotency-Key`** (reservar, cancelar, canjear, asistencia; obligatoria en la API de partners).
10. **Identidad solo desde el gateway.** Los servicios leen `X-User-Id` / `X-User-Role`; nunca decodifican el JWT ni confían en datos de identidad del body.
11. **Tests obligatorios** para toda regla de negocio, con los casos límite del SPEC §10 cuando apliquen. Integración con testcontainers, no mocks de la base.
12. **Contratos versionados.** Cambios en APIs públicas, API de partners o eventos se documentan en `docs/contracts/` (API de partners: correr `make contract-lint` y actualizar el CHANGELOG de la guía), `docs/api/` o `docs/events/` y siguen las reglas de versionado de ARCHITECTURE §7.1.
13. **Sin secretos en el repo.** Solo `.env.example`. Nunca commitees `.env`, claves ni API keys.
14. **No loguees datos personales** (ver Logs).
15. **No hagas commits ni push sin que te lo pidan**, y nunca sobre `main`.
16. **Escribí en español** la documentación, mensajes de error al usuario y descripciones de PR; Conventional Commits en español.
17. **Mantené la documentación al día:** si un cambio afecta ARCHITECTURE, un ADR o este archivo, actualizalo en el mismo PR.
