---
name: "quantum-log-security-privacy"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "PII"
  - "secreto"
  - "prompt"
  - "redaction"
  - "path"
  - "header"
  - "privacy"
---

# quantum-log-security-privacy

## Propósito

Evita captura y exportación de secretos, PII y contenido de trabajo.

## Cuándo se activa

- PII
- secreto
- prompt
- redaction
- path
- header
- privacy

## Responsabilidades obligatorias

- Aplicar minimización en origen.
- Clasificar campos antes de instrumentarlos.
- Mantener captura de contenido bajo opt-in explícito y temporal.
- Redactar headers, variables, rutas y argumentos sensibles.
- Revisar almacenamiento local de telemetría y permisos.

## Flujo de ejecución

1. Crear inventario de datos.
2. Clasificar sensibilidad y necesidad.
3. Eliminar atributos innecesarios.
4. Aplicar hash/redaction donde proceda.
5. Ejecutar fixtures de secretos.
6. Bloquear release ante fuga.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- data-inventory
- capture-policy
- privacy-test-report

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Confiar solo en redacción del backend.
- Hash sin salt cuando permite diccionario.
- Captura permanente para depuración.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
