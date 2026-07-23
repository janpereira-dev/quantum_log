---
name: "quantum-log-project-context"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "cualquier cambio transversal"
  - "nuevos módulos"
  - "decisiones de arquitectura"
  - "cambios de persistencia"
---

# quantum-log-project-context

## Propósito

Carga y protege el contexto arquitectónico y los invariantes de Quantum Log.

## Cuándo se activa

- cualquier cambio transversal
- nuevos módulos
- decisiones de arquitectura
- cambios de persistencia

## Responsabilidades obligatorias

- Leer `.agents/project.yaml` y los documentos de knowledge antes de proponer cambios.
- Distinguir claramente ledger, telemetría, diagnóstico y exportación.
- Detectar contradicciones con local-first, append-only, privacidad y bajo consumo.
- Actualizar decisiones mediante ADR cuando cambie un invariante.

## Flujo de ejecución

1. Identificar módulo y frontera de responsabilidad.
2. Enumerar invariantes afectados.
3. Clasificar el cambio como reversible, migratorio o rupturista.
4. Seleccionar skills y agentes especializados.
5. Emitir restricciones obligatorias antes de implementar.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- context-assessment
- invariant-impact
- required-agents
- blocking-risks

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Convertir OTel en dependencia de disponibilidad.
- Modificar entradas existentes.
- Duplicar modelos de dominio por adaptador.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
