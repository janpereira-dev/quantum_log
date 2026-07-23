---
name: "quantum-log-logs-events"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "log"
  - "event"
  - "diagnóstico"
  - "error estructurado"
  - "correlación"
---

# quantum-log-logs-events

## Propósito

Estructura logs y eventos correlacionados sin duplicar el ledger ni filtrar contenido.

## Cuándo se activa

- log
- event
- diagnóstico
- error estructurado
- correlación

## Responsabilidades obligatorias

- Diferenciar log diagnóstico, evento OTel y entrada de ledger.
- Incluir trace/span IDs mediante integración estructurada.
- Aplicar severidad coherente.
- Evitar cuerpos no acotados y stack traces repetidos.
- Usar eventos para hechos puntuales dentro de una operación.

## Flujo de ejecución

1. Clasificar el hecho.
2. Definir nombre de evento estable.
3. Sanear atributos y body.
4. Emitir dentro del contexto activo.
5. Probar correlación y redacción.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- event-catalog
- logging-policy
- correlation-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Copiar prompts en logs.
- Emitir la misma excepción en todas las capas.
- Usar texto libre como dimensión.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
