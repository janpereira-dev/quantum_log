---
name: "quantum-log-genai-mcp-observability"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "GenAI"
  - "LLM"
  - "modelo"
  - "MCP"
  - "prompt"
  - "completion"
  - "tool"
---

# quantum-log-genai-mcp-observability

## Propósito

Aplica convenciones GenAI/MCP fijadas y extensiones propias controladas.

## Cuándo se activa

- GenAI
- LLM
- modelo
- MCP
- prompt
- completion
- tool

## Responsabilidades obligatorias

- Consultar el commit/schema GenAI fijado antes de implementar.
- Usar convenciones oficiales cuando existan y estén aceptadas.
- Aislar atributos experimentales detrás de versión de contrato.
- Medir latencia, time-to-first-token y uso cuando estén disponibles.
- Sanear prompts, respuestas y argumentos de herramientas.

## Flujo de ejecución

1. Resolver versión de semconv GenAI.
2. Mapear request/response del proveedor.
3. Definir qué contenido se omite, hashea o captura mediante opt-in.
4. Instrumentar cliente, agente y MCP sin duplicar spans.
5. Validar estabilidad y compatibilidad.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- genai-semconv-map
- mcp-span-contract
- content-capture-policy

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Copiar atributos de un blog sin verificar versión.
- Capturar argumentos secretos.
- Duplicar spans del SDK del proveedor y del adaptador.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
