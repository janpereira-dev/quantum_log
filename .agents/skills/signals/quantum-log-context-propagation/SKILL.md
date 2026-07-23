---
name: "quantum-log-context-propagation"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "context.Context"
  - "propagación"
  - "goroutine"
  - "IPC"
  - "traceparent"
  - "baggage"
---

# quantum-log-context-propagation

## Propósito

Mantiene contexto entre goroutines, HTTP, IPC, CLI, colas y MCP.

## Cuándo se activa

- context.Context
- propagación
- goroutine
- IPC
- traceparent
- baggage

## Responsabilidades obligatorias

- Propagar W3C Trace Context en fronteras compatibles.
- Definir envelope interno para IPC y procesos locales.
- No propagar baggage sensible o de alta cardinalidad.
- Distinguir link de parent cuando la causalidad no es jerárquica.
- Preservar cancelación y deadlines.

## Flujo de ejecución

1. Inventariar fronteras de proceso y concurrencia.
2. Definir mecanismo inject/extract.
3. Propagar contexto explícitamente.
4. Probar fan-out, colas y reintentos.
5. Ejecutar debugger de contexto perdido.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- propagation-map
- carrier-contract
- concurrency-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Guardar contextos en structs de larga vida.
- Usar baggage como base de datos.
- Separar una goroutine sin preservar contexto cuando pertenece a la operación.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
