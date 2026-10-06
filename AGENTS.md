# Trabajo en The Date API

- Empieza por `docs/backend-map.md` y abre solo los módulos del cambio.
- Go funcional: decisiones puras en `internal/core`; orquestación con puertos pequeños en `internal/application`; efectos en adaptadores. Evita crear interfaces que simplemente repliquen toda la API de MongoDB.
- No añadas MongoDB, HTTP, variables de entorno o servicios externos al núcleo. Tiempo e identidad deben ser argumentos explícitos.
- Usa el driver oficial MongoDB v2. Centraliza operaciones concurrentes e índices en `internal/mongostore`, con contratos tipados y escrituras condicionales; no uses mutex locales como garantía entre réplicas.
- `cmd/api` solo inicia el proceso. Mantén archivos de implementación por debajo de 200 líneas y separa responsabilidades reales.
- Las claves solo vienen del entorno. No copies secretos a código, pruebas, documentación o logs. Dokploy usa únicamente `DOKPLOY_THE_DATE_API_KEY`.
- Nunca reintentes a ciegas una creación externa ni un envío cuyo resultado es desconocido. Conserva y reconcilia los checkpoints.
- Ejecuta `gofmt`, `go test -race ./...`, `go vet ./...` y `go build ./cmd/api`. Para cambios de concurrencia, ejecuta también las pruebas con `TEST_MONGODB_URI` contra MongoDB local aislado.
- No mezcles escritores de la versión antigua y la nueva tras migrar las reservas. Consulta `docs/rollout.md`.
