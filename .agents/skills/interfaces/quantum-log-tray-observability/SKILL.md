---
name: "quantum-log-tray-observability"
version: "1.0.0"
project: "quantum-log"
kind: "skill"
language: "es"
blocking_capable: true
triggers:
  - "tray"
  - "Windows"
  - "macOS"
  - "Linux"
  - "startup"
  - "toggle"
  - "estado local"
---

# quantum-log-tray-observability

## Propósito

Observa la tray app sin rastrear innecesariamente la interacción del usuario.

## Cuándo se activa

- tray
- Windows
- macOS
- Linux
- startup
- toggle
- estado local

## Responsabilidades obligatorias

- Medir arranque, conexión al servicio, estado del Collector y colas pendientes.
- Registrar cambios de configuración relevantes como eventos auditables.
- Evitar telemetría invasiva de clics o navegación.
- Medir consumo de proceso y reinicios inesperados.
- Distinguir estado visual de estado real del servicio.

## Flujo de ejecución

1. Definir máquinas de estado.
2. Mapear transiciones operativas.
3. Instrumentar startup, connect, toggle y export.
4. Añadir health snapshot local.
5. Probar servicio ausente y reconexión.

## Entradas mínimas

- Contexto actualizado de `.agents/project.yaml`.
- Flujo o módulo afectado.
- Restricciones de privacidad y rendimiento.
- Versión fijada en `sources.lock.yaml` cuando intervengan OTel o semantic conventions.

## Salidas contractuales

- tray-state-model
- health-metrics
- reconnect-tests

Cada salida debe indicar: decisiones, supuestos, archivos afectados, pruebas, riesgos bloqueantes y deuda diferida.

## Gates

- El ledger funciona con telemetría desactivada.
- Ningún dato sensible se captura sin política explícita.
- Los atributos de métricas tienen cardinalidad acotada.
- Los errores de exportación no cambian el resultado de negocio.
- Todo dato de coste declara su calidad y fuente.
- Los cambios de contrato incluyen migración o compatibilidad.

## Antipatrones bloqueados

- Registrar cada clic.
- Mostrar telemetría activa cuando el servicio está caído.
- Usar la UI como fuente de verdad.

## Colaboración

Invoca al agente principal que corresponda y delega análisis acotados en subagentes. Los subagentes no pueden aprobar su propio trabajo ni levantar un gate bloqueante.

## Definición de terminado

- Implementación compilable y formateada.
- Tests unitarios y de integración pertinentes.
- Eval contractual superado.
- Guardrails aplicables superados.
- Documentación y contrato actualizados.
- Reporte GO/NO-GO emitido cuando el cambio vaya a release.
