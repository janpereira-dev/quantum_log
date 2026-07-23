# Modelo de telemetría de Quantum Log

## Jerarquía de spans

```text
quantum_log.session
└── quantum_log.task
    ├── gen_ai.request
    ├── quantum_log.tool.call
    ├── quantum_log.mcp.request
    ├── quantum_log.command.execute
    ├── quantum_log.file.operation
    └── quantum_log.ledger.append
```

## Recursos mínimos

- `service.name=quantum-log`
- `service.version`
- `service.instance.id`
- `deployment.environment.name`
- atributos de host/proceso solo cuando no comprometan privacidad.

## Atributos de dominio

Usar namespace `quantum_log.*` únicamente cuando no exista una convención oficial estable apropiada.

- `quantum_log.session.id`
- `quantum_log.task.id`
- `quantum_log.operation.id`
- `quantum_log.ledger.entry.id`
- `quantum_log.agent.name`
- `quantum_log.agent.version`
- `quantum_log.data.quality`
- `quantum_log.cost.source`

No adjuntar IDs únicos a métricas. Los IDs pueden vivir en spans y logs bajo política de privacidad.
