---
name: "quantum-log-agent-session-observability"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "agente"
  - "sesión"
  - "task"
  - "subagente"
  - "tool call"
  - "workflow"
---

# quantum-log-agent-session-observability

## Propósito

Instrumenta sesiones, tareas, planes, subagentes y herramientas de IA.

## Cuándo se activa

- agente
- sesión
- task
- subagente
- tool call
- workflow

## Responsabilidades obligatorias

- Normalizar estados de sesión y tarea entre proveedores.
- Representar delegaciones de subagentes con causalidad explícita.
- Registrar nombre/version del adaptador sin inventar datos del proveedor.
- Medir duración y resultado de herramientas.
- Mantener contenido desactivado por defecto.

## Flujo de ejecución

1. Mapear eventos nativos del adaptador.
2. Convertirlos al modelo Session/Task/Operation.
3. Crear spans y ledger entries donde corresponda.
4. Adjuntar uso y coste con calidad del dato.
5. Cerrar estados incompletos ante crash o timeout.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- adapter-mapping
- session-trace-model
- recovery-policy

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Asumir que todos los agentes exponen tokens.
- Crear un nuevo modelo canónico por proveedor.
- Guardar transcripciones completas por defecto.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
