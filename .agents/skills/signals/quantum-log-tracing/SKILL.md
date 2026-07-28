---
name: "quantum-log-tracing"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "span"
  - "trace"
  - "latencia"
  - "dependencia"
  - "causalidad"
---

# quantum-log-tracing

## Propósito

Diseña trazas causales para sesiones, tareas, agentes, almacenamiento e IPC.

## Cuándo se activa

- span
- trace
- latencia
- dependencia
- causalidad

## Responsabilidades obligatorias

- Definir un span solo cuando represente una operación o frontera útil.
- Mantener nombres estables y de baja cardinalidad.
- Establecer SpanKind correcto para servidor, cliente, productor, consumidor o interno.
- Registrar errores sin convertir cancelaciones esperadas en fallos.
- Correlacionar spans con IDs de dominio.

## Flujo de ejecución

1. Dibujar árbol de causalidad esperado.
2. Elegir raíces de sesión/tarea sin crear trazas infinitas.
3. Definir atributos y eventos mínimos.
4. Instrumentar preservando `context.Context`.
5. Validar parentesco, estado y ausencia de PII.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- trace-contract
- span-table
- trace-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Nombre de span con UUID o prompt.
- Un span por función trivial.
- Crear raíces nuevas cuando existe contexto remoto válido.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
