---
name: "quantum-log-otel-orchestrator"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "traces"
  - "metrics"
  - "logs"
  - "OTLP"
  - "Collector"
  - "instrumentación"
  - "correlación"
---

# quantum-log-otel-orchestrator

## Propósito

Orquesta todo trabajo de OpenTelemetry dentro de Quantum Log.

## Cuándo se activa

- traces
- metrics
- logs
- OTLP
- Collector
- instrumentación
- correlación

## Responsabilidades obligatorias

- Clasificar la señal y el objetivo operativo.
- Elegir instrumentación manual, automática o combinación controlada.
- Invocar skills de seguridad, cardinalidad, testing y semconv.
- Garantizar que el ledger siga operativo con telemetría deshabilitada.
- Producir plan, implementación, pruebas y reporte de readiness.

## Flujo de ejecución

1. Cargar contexto del proyecto.
2. Mapear flujo de ejecución y puntos de causalidad.
3. Definir spans, métricas, eventos y recursos necesarios.
4. Validar semantic conventions y atributos propios.
5. Aplicar privacidad y presupuestos.
6. Implementar con shutdown y degradación segura.
7. Ejecutar evals relevantes.
8. Solicitar auditoría de ledger y release gate.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- telemetry-design
- implementation-plan
- test-plan
- production-readiness-report

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Instrumentar por volumen sin pregunta operativa.
- Duplicar spans de librerías.
- Bloquear una operación por fallo del exportador.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
