# Modelo de dominio mínimo

## Entidades

### Session
Agrupa una interacción continua con un agente o entorno.

### Task
Unidad de trabajo solicitada por el usuario o planificador.

### Operation
Acción medible dentro de una tarea: llamada de modelo, herramienta, MCP, comando, lectura, escritura o persistencia.

### UsageRecord
Consumo informado o inferido: tokens, caracteres, bytes, caché, razonamiento y llamadas.

### CostRecord
Importe original, conversión, versión de precios y calidad del cálculo.

### LedgerEntry
Registro append-only con hash, enlace anterior, payload normalizado y referencias de correlación.

### Correction / Reversal / Tombstone
Entradas compensatorias. Nunca editan silenciosamente el hecho original.

## Identificadores

- `session_id`: estable durante la sesión.
- `task_id`: estable para la unidad de trabajo.
- `operation_id`: único e idempotente por operación capturada.
- `ledger_entry_id`: creado al persistir.
- `trace_id` y `span_id`: identificadores OTel, no sustituyen IDs de dominio.

## Calidad del dato

Todo valor de uso o coste debe indicar procedencia:

- `reported`: entregado explícitamente por proveedor/agente.
- `estimated`: calculado mediante tabla de precios o tokenizer conocido.
- `inferred`: aproximación indirecta con menor confianza.
- `unavailable`: no puede obtenerse de forma defendible.
