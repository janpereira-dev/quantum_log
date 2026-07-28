# Privacidad y seguridad

QUANTUM_LOG está diseñado para retener evidencia local de uso sin retener contenido sensible de agentes. Es límite del sistema, no permiso para ingerir datos privados arbitrarios ni exponer servicios locales sin cuidado.

[Read in English](../en/privacy-security.md)

## Política de almacenamiento

Datos permanecen locales por defecto. Eventos raw son append-only y se encadenan por source/session. Antes de hashing o importación, sanitización elimina contenido de prompt y respuesta, argumentos y resultados de herramientas, secretos, valores de autorización, API keys, tokens y familias relacionadas de claves sensibles.

Sistema intencionalmente no almacena contenido sensible. En particular:

- `qlog run` nunca persiste argumentos de comando, valores de entorno ni salida de proceso.
- Manejo de hooks Claude Code reduce payloads a metadatos de ciclo de vida seguros; campos transcript y prompt no se retienen.
- Ingestión de plugins y OTLP elimina payload sensible y valores de atributos antes de almacenamiento.

Sanitización reduce riesgo; no vuelve segura para compartir toda entrada externa. Limita acceso a collector e inspecciona exportaciones antes de distribuirlas.

## Calidad de captura

Calidad de captura es explícita porque procedencia de datos afecta qué pueden concluir usuarios.

| Etiqueta | Significado | No infieras |
| --- | --- | --- |
| `otel_reported` | OTLP aceptado proporcionó campos de uso. | Que todo proveedor emitió uso completo. |
| `agent_reported` | Integración de agente proporcionó campos de uso. | Totales facturados por proveedor o evidencia de transcript completo. |
| `lifecycle_only` | Evento de ciclo de vida/proceso fue capturado. | Cualquier conteo de tokens o costo. |
| `unavailable` | No hay evidencia de uso soportada. | Uso real cero. |

Reportes y exportaciones preservan estas etiquetas. QUANTUM_LOG nunca inventa conteos de tokens para evidencia ausente.

## Estado de Copilot

M4 está `IN_PROGRESS`. Captura de Copilot VS Code es experimental hasta que evidencia real de extremo a extremo registre uso originado en Copilot en SQLite. Instalación exitosa de configuración, comprobación de estado de collector o datos genéricos importados no bastan.

`qlog adapter verify copilot-vscode` requiere model call local OTLP reciente de Copilot con tokens `otel_reported`. Hasta que esa etapa pase contra evidencia real, describe integración como experimental y no verificada.

## Modelo de amenazas

QUANTUM_LOG aborda estos riesgos locales:

| Riesgo | Control |
| --- | --- |
| Contenido sensible llega a ingestión | Sanitización antes de importación y hashing; payloads acotados de plugin/hook. |
| Evento de libro mayor es alterado | Verificación de cadena SHA-256 de source/session. |
| Historial de libro mayor es truncado o diverge | Anchors externos exportados y `anchor check`. |
| Acceso SQLite concurrente produce diagnósticos inseguros | Bloqueos cooperativos de quiescencia/writer y comprobaciones WAL. |
| Collector local se expone accidentalmente | Loopback por defecto; opt-in explícito fuera de loopback. |
| Atribución se adivina desde metadatos débiles | Política explícita de resolución de proyecto y estado sin atribuir. |

No elimina riesgos de host comprometido, usuario local autorizado, secretos ya presentes en metadatos que sanitizador no reconoce, almacenamiento de backup inseguro ni exposición deliberada de collector a red. Aplica controles de acceso de sistema operativo y protege backups y archivos anchor.

## Exportación y compartición seguras

Usa:

```bash
qlog export --format csv --redact-paths > usage.csv
```

Revisa campos exportados, etiquetas de calidad de captura, nombres de proyecto, timestamps, metadatos de proveedor/modelo y datos de asignación antes de compartir. Redactar rutas no redacta todos campos sensibles para negocio.

No compartas archivos raw de base de datos, sidecars WAL/SHM, archivos anchor sin revisar, configuración de adaptador ni diagnósticos con rutas específicas del entorno salvo que destinatarios estén autorizados y contenido haya sido revisado.

## Respuesta de seguridad

Para problema de seguridad, sigue [SECURITY.md](../../SECURITY.md). Preserva evidencia en su lugar, evita acciones destructivas de recuperación y no incluyas secretos ni contenido sensible de agentes en reportes públicos.
