---
name: "quantum-log-api-ipc-observability"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "API"
  - "HTTP"
  - "gRPC"
  - "IPC"
  - "named pipe"
  - "unix socket"
  - "WebSocket"
---

# quantum-log-api-ipc-observability

## Propósito

Instrumenta HTTP, gRPC, sockets, pipes e IPC de la aplicación local.

## Cuándo se activa

- API
- HTTP
- gRPC
- IPC
- named pipe
- unix socket
- WebSocket

## Responsabilidades obligatorias

- Usar instrumentación oficial del transporte cuando sea suficiente.
- Normalizar rutas y métodos.
- Propagar contexto en envelopes IPC internos.
- Medir colas, backpressure y tamaño sin registrar payloads.
- Separar errores de transporte, validación y negocio.

## Flujo de ejecución

1. Identificar frontera cliente/servidor.
2. Aplicar middleware/instrumentación.
3. Definir atributos propios mínimos para IPC no estándar.
4. Probar cancelación, timeout y reconexión.
5. Validar cardinalidad de rutas y códigos.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- interface-telemetry-contract
- middleware-plan
- ipc-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Ruta concreta como nombre de span.
- Payload completo en atributos.
- Doble instrumentación del mismo request.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
