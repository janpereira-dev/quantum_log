---
name: "quantum-log-metrics"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "métrica"
  - "counter"
  - "histogram"
  - "gauge"
  - "SLO"
  - "dashboard"
---

# quantum-log-metrics

## Propósito

Define métricas operativas y contables agregables con cardinalidad limitada.

## Cuándo se activa

- métrica
- counter
- histogram
- gauge
- SLO
- dashboard

## Responsabilidades obligatorias

- Elegir instrumento por semántica, no por comodidad.
- Usar unidades UCUM y descripciones claras.
- Separar métricas operativas de importes contables del ledger.
- Definir atributos de dimensión cerrada.
- Crear vistas o límites cuando el SDK lo permita.

## Flujo de ejecución

1. Escribir la pregunta operacional que responde cada métrica.
2. Definir nombre, tipo, unidad, temporality esperada y dimensiones.
3. Calcular cardinalidad teórica.
4. Instrumentar y probar agregación.
5. Revisar coste y utilidad tras carga real.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- metric-catalog
- cardinality-estimate
- dashboard-requirements

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- IDs en etiquetas.
- Counter para valores que disminuyen.
- Métricas de negocio usadas como ledger autoritativo.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
