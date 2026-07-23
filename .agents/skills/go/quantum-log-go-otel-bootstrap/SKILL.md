---
name: "quantum-log-go-otel-bootstrap"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "bootstrap Go"
  - "TracerProvider"
  - "MeterProvider"
  - "LoggerProvider"
  - "shutdown"
---

# quantum-log-go-otel-bootstrap

## Propósito

Implementa el ciclo de vida OTel en Go con modos disabled, debug y OTLP.

## Cuándo se activa

- bootstrap Go
- TracerProvider
- MeterProvider
- LoggerProvider
- shutdown

## Responsabilidades obligatorias

- Crear providers y propagadores desde configuración explícita.
- Usar batch processors en producción y exportadores de consola solo en desarrollo.
- Registrar recursos de servicio sin filtrar datos privados.
- Implementar shutdown idempotente, ordenado y acotado por timeout.
- Proveer no-op real cuando telemetría esté deshabilitada.

## Flujo de ejecución

1. Definir `TelemetryConfig` validada.
2. Construir exporters por señal.
3. Construir providers y establecer globals de forma controlada.
4. Devolver una función `Shutdown(ctx)` compuesta e idempotente.
5. Probar disabled mode, exporter failure y shutdown timeout.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- bootstrap-code
- configuration-schema
- lifecycle-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Inicialización en `init()` difícil de probar.
- Usar `context.Background()` dentro de operaciones activas.
- Ignorar errores de flush/shutdown.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
