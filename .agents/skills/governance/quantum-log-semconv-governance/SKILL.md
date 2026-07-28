---
name: "quantum-log-semconv-governance"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "semantic conventions"
  - "atributo OTel"
  - "deprecated"
  - "schema URL"
  - "stability opt-in"
---

# quantum-log-semconv-governance

## Propósito

Gobierna versiones, estabilidad y extensiones de semantic conventions.

## Cuándo se activa

- semantic conventions
- atributo OTel
- deprecated
- schema URL
- stability opt-in

## Responsabilidades obligatorias

- Resolver versión desde `sources.lock.yaml`.
- Usar atributos oficiales apropiados antes de crear `quantum_log.*`.
- Registrar estabilidad de cada dominio utilizado.
- Separar upgrade de semconv de cambios funcionales.
- Mantener compatibilidad o migración explícita.

## Flujo de ejecución

1. Identificar dominio semántico.
2. Consultar especificación fijada.
3. Comprobar estabilidad y migración.
4. Definir extensión propia versionada si no existe convención.
5. Validar nombres, tipos, unidades y SpanKind.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- semconv-decision
- attribute-map
- migration-note

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Copiar atributos obsoletos.
- Mezclar dos generaciones de HTTP/db semconv.
- Crear namespace propio para conceptos estándar.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
