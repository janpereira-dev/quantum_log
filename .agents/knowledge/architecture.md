# Arquitectura objetivo de observabilidad

## Flujo principal

```text
Agente / IDE / CLI
    ↓ evento nativo
Adaptador Quantum Log
    ↓ evento canónico
Normalizador y política de privacidad
    ↓
Ledger Core ───────────────→ almacenamiento append-only
    │
    └─ telemetría no bloqueante
          ↓
       OTel SDK Go
          ↓
  disabled | debug local | OTLP
          ↓
 Collector opcional → backend elegido por el usuario
```

## Separación de responsabilidades

### Ledger

Registra hechos contables duraderos: identidad de sesión, operación, modelo, uso, coste, resultado, fuente, calidad del dato y cadena de integridad.

### OpenTelemetry

Describe ejecución y salud: latencia, causalidad, dependencias, fallos, colas, recursos y propagación de contexto.

### Regla de desacoplamiento

La escritura del ledger se confirma antes de considerar el éxito de la exportación de telemetría. Un fallo OTLP solo genera diagnóstico y métricas internas; no revierte la entrada contable.

## Límites de transacción

1. Recibir evento.
2. Validar y sanear.
3. Normalizar e identificar idempotencia.
4. Persistir entrada o detectar duplicado.
5. Emitir telemetría asociada.
6. Confirmar al adaptador.

La emisión de telemetría puede empezar antes para medir latencia, pero su flush no forma parte de la transacción de negocio.
