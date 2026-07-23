---
name: "quantum-log-production-readiness"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "release"
  - "producción"
  - "readiness"
  - "PR"
  - "gate"
---

# quantum-log-production-readiness

## Propósito

Consolida gates de producción para cada cambio observable.

## Cuándo se activa

- release
- producción
- readiness
- PR
- gate

## Responsabilidades obligatorias

- Verificar todos los guardrails bloqueantes.
- Exigir evidencia de funcionamiento con OTel deshabilitado.
- Exigir presupuesto de cardinalidad y rendimiento.
- Verificar rollback/configuración segura.
- Emitir decisión GO/NO-GO con hallazgos trazables.

## Flujo de ejecución

1. Recopilar resultados de tests y auditores.
2. Comprobar configuración y flags.
3. Ejecutar smoke tests de modos.
4. Revisar cambios de contrato/migración.
5. Emitir gate y acciones.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- go-no-go-report
- blocking-findings
- release-evidence

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Aceptar “funciona en local” como evidencia.
- Ignorar overhead.
- Liberar captura de contenido activada.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
