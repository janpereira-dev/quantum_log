---
name: "quantum-log-ledger-correlation"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "ledger_entry_id"
  - "trace_id"
  - "span_id"
  - "correlación"
  - "integridad"
---

# quantum-log-ledger-correlation

## Propósito

Relaciona entradas append-only con trazas sin convertir OTel en fuente de verdad.

## Cuándo se activa

- ledger_entry_id
- trace_id
- span_id
- correlación
- integridad

## Responsabilidades obligatorias

- Persistir identificadores de correlación como metadatos no autoritativos.
- Asociar una entrada con la operación que la generó.
- Mantener idempotencia aunque cambie el trace ID tras un reintento.
- Evitar que sampling o pérdida OTLP afecten la contabilidad.
- Definir correcciones y reversiones como nuevas entradas.

## Flujo de ejecución

1. Identificar operación de dominio.
2. Crear o reutilizar operation ID.
3. Abrir span y persistir ledger entry.
4. Adjuntar ledger entry ID al span tras commit.
5. Registrar el trace/span en metadatos permitidos.
6. Probar reintentos, duplicados y exporter offline.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- correlation-contract
- persistence-sequence
- idempotency-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Usar trace ID como clave idempotente.
- Editar una entrada para añadir datos tardíos.
- Considerar exportación OTLP parte del commit.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
