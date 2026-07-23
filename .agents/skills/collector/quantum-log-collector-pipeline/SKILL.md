---
name: "quantum-log-collector-pipeline"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "Collector"
  - "receiver"
  - "processor"
  - "exporter"
  - "OTLP"
  - "tail sampling"
---

# quantum-log-collector-pipeline

## Propósito

Mantiene un Collector opcional, mínimo, seguro y observable.

## Cuándo se activa

- Collector
- receiver
- processor
- exporter
- OTLP
- tail sampling

## Responsabilidades obligatorias

- Mantener pipelines mínimos para traces, metrics y logs.
- Aplicar memory limiter y batch con valores validados.
- Redactar y filtrar antes de exportar.
- Configurar retry/queue según criticidad no contable.
- Exponer health y telemetría interna de forma local/segura.

## Flujo de ejecución

1. Partir de plantilla mínima.
2. Añadir solo componentes necesarios.
3. Validar configuración con la versión fijada.
4. Probar backend caído y reinicio.
5. Verificar que Quantum Log funciona sin Collector.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- collector-config
- component-inventory
- failure-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Collector obligatorio.
- Tail sampling distribuido sin afinidad de trace.
- Endpoints públicos sin autenticación.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
