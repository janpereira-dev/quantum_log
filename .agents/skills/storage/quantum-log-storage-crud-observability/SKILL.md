---
name: "quantum-log-storage-crud-observability"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "CRUD"
  - "base de datos"
  - "persistencia"
  - "query"
  - "migration"
  - "integridad"
---

# quantum-log-storage-crud-observability

## Propósito

Instrumenta persistencia, consulta, corrección, reversión, tombstone y verificación.

## Cuándo se activa

- CRUD
- base de datos
- persistencia
- query
- migration
- integridad

## Responsabilidades obligatorias

- Reformular UPDATE/DELETE del ledger como operaciones compensatorias.
- Instrumentar transacciones y consultas con convenciones de base de datos aplicables.
- No capturar sentencias o parámetros sensibles por defecto.
- Medir bloqueo, latencia, filas y errores con cardinalidad controlada.
- Verificar que la telemetría no cambie la transacción.

## Flujo de ejecución

1. Clasificar operación de almacenamiento.
2. Definir span de cliente o interno según driver/frontera.
3. Usar resumen de consulta estable.
4. Adjuntar resultado después de commit.
5. Probar rollback, lock, corrupción y recuperación.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- storage-span-map
- query-sanitization-policy
- transaction-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- SQL completo con datos.
- UPDATE directo de una entrada.
- Span por fila en operaciones masivas.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
