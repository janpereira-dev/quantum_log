---
name: "quantum-log-token-cost-accounting"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "tokens"
  - "coste"
  - "precio"
  - "moneda"
  - "usage"
  - "cached tokens"
---

# quantum-log-token-cost-accounting

## Propósito

Normaliza tokens y costes con procedencia, versión de precio y moneda verificable.

## Cuándo se activa

- tokens
- coste
- precio
- moneda
- usage
- cached tokens

## Responsabilidades obligatorias

- Preservar valores reportados sin recalcularlos silenciosamente.
- Calcular estimaciones solo con tabla de precios versionada.
- Etiquetar calidad y método de cálculo.
- Registrar conversión de moneda con fuente y timestamp.
- Conciliar totales de operación, tarea y sesión.

## Flujo de ejecución

1. Extraer uso nativo.
2. Normalizar categorías de tokens.
3. Asignar data quality.
4. Resolver versión de precios y moneda.
5. Calcular y redondear sin perder precisión contable.
6. Comparar sumas con niveles superiores.
7. Emitir métricas agregadas sin IDs.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- usage-record
- cost-record
- reconciliation-report

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Presentar estimado como cobro real.
- Usar `float64` para importes finales sin política decimal.
- Convertir moneda sin guardar tasa.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
