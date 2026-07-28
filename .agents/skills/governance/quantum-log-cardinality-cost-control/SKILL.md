---
name: "quantum-log-cardinality-cost-control"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "cardinalidad"
  - "series"
  - "sampling"
  - "volumen"
  - "coste OTel"
---

# quantum-log-cardinality-cost-control

## Propósito

Controla explosión de series, volumen OTLP y coste de observabilidad.

## Cuándo se activa

- cardinalidad
- series
- sampling
- volumen
- coste OTel

## Responsabilidades obligatorias

- Calcular cardinalidad teórica y observada.
- Prohibir IDs, rutas dinámicas y mensajes libres en métricas.
- Definir límites de atributos y tamaños.
- Aplicar sampling solo a telemetría, nunca al ledger.
- Detectar crecimiento por nuevos proveedores/modelos.

## Flujo de ejecución

1. Inventariar dimensiones.
2. Calcular producto cartesiano máximo.
3. Clasificar dimensiones cerradas, acotadas o abiertas.
4. Eliminar o transformar abiertas.
5. Ejecutar carga y medir series/bytes.
6. Comparar con presupuesto.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- cardinality-budget
- volume-estimate
- sampling-policy

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- trace_id en métricas.
- error.message como label.
- Proveedor/modelo sin normalización ni límite.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
