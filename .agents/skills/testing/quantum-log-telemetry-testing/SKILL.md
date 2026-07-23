---
name: "quantum-log-telemetry-testing"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "test"
  - "eval"
  - "span exporter"
  - "contract test"
  - "trace-based testing"
---

# quantum-log-telemetry-testing

## Propósito

Prueba contratos de telemetría, correlación, privacidad y degradación.

## Cuándo se activa

- test
- eval
- span exporter
- contract test
- trace-based testing

## Responsabilidades obligatorias

- Usar exporters in-memory/fake para tests deterministas.
- Comprobar árboles, atributos, estados y métricas.
- Probar Collector opcional por integración.
- Ejecutar fixtures con secretos y cardinalidad hostil.
- Evitar assertions acopladas a datos irrelevantes.

## Flujo de ejecución

1. Seleccionar eval por flujo.
2. Ejecutar operación con IDs controlados.
3. Recolectar spans/metrics/logs.
4. Validar contrato y ausencia de fugas.
5. Simular fallo y recuperación.
6. Emitir resultado bloqueante.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- test-suite
- eval-results
- regression-baseline

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Solo probar que existe un span.
- Depender de backend externo en unit tests.
- Snapshots gigantes imposibles de mantener.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
