---
name: "quantum-log-performance-resilience"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "performance"
  - "overhead"
  - "queue"
  - "retry"
  - "backpressure"
  - "offline"
---

# quantum-log-performance-resilience

## Propósito

Mantiene bajo overhead, backpressure controlado y degradación segura.

## Cuándo se activa

- performance
- overhead
- queue
- retry
- backpressure
- offline

## Responsabilidades obligatorias

- Medir coste de instrumentación y exportación.
- Evitar bloqueos síncronos en el camino crítico.
- Definir límites de cola y política de descarte de telemetría.
- Preservar ledger ante fallos de red/exportador.
- Implementar flush y shutdown con timeout.

## Flujo de ejecución

1. Establecer benchmark sin OTel.
2. Medir con OTel disabled/local/OTLP.
3. Simular backend lento y caído.
4. Verificar memoria, CPU y latencia.
5. Ajustar batch/queue/sampling.
6. Documentar trade-offs.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- benchmark-report
- degradation-policy
- resource-budget

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Cola ilimitada.
- Reintentos infinitos.
- Flush sin deadline en cierre de aplicación.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
