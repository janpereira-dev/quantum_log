# Documentación de QUANTUM_LOG

Quantum Log lee los archivos de sesión de Codex que ya tienes en el equipo y
muestra los tokens registrados por proyecto. Importas cuando quieres actualizar
el informe; no necesita un proceso en segundo plano.

[Read in English](../en/README.md)

## Empieza aquí

- [Guía actual en español](simple.md): compilar, probar datos ficticios, importar y exportar.
- [Presentación del proyecto](../../README.md), en inglés.
- [Política de seguridad](../../SECURITY.md), en inglés.

Los tres comandos habituales son `qlog import`, `qlog report` y `qlog export`.
Ejecutar `qlog` muestra el informe. Los costes no están disponibles y las
muestras de uso no equivalen al número de llamadas al modelo.

## Colaboradores

- [Contribución](../../CONTRIBUTING.md): paquetes actuales, comprobaciones y revisión, en inglés.
- [Decisión del producto actual](../architecture/ADR-008-on-demand-product.md), en inglés.
- [Registros de decisiones de arquitectura](../architecture/): decisiones técnicas normativas.

## Producto anterior

La implementación anterior de captura se conserva en `cmd/qlog-legacy`.
Su [guía](guia.md), [referencia CLI](referencia-cli.md),
[arquitectura](arquitectura.md), [operaciones](operaciones.md),
[privacidad y seguridad](privacidad-seguridad.md) y [notas de contribución](contribucion.md)
son material histórico. El [índice histórico](../legacy/README.md) reúne todos
esos documentos. No son los pasos de puesta en marcha del producto actual.
