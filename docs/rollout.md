# Puesta en marcha y límites operativos

## Estado inspeccionado en Dokploy

Revisión de solo lectura el 6 de octubre de 2026: el proyecto `thedate` existe en Rangel Tech; su MongoDB usa `mongo:7`, una réplica, sin `replicaSet` en la configuración inspeccionada. La organización dedicada The Date no devolvió proyectos. Esta revisión no leyó datos de usuarios ni modificó infraestructura.

El código usa la clave dedicada The Date exclusivamente para crear nuevos despliegues. La ubicación actual de la API/base no cambia automáticamente.

## Antes de activar esta versión

1. Haz snapshot/restauración verificable de MongoDB. Prueba primero con una copia aislada.
2. Revisa eventos antiguos con slugs ahora reservados (`crea`, `studio`, `api`, etc.) y reasigna cualquier colisión de forma controlada. Detén todos los escritores antiguos. Despliega la nueva API sin solapar versiones: la migración perezosa toma respuestas antiguas de `guests`, y después solo el registro de reservas del evento es canónico. No vuelvas al binario antiguo tras aceptar respuestas nuevas sin una migración inversa.
3. Los JWT antiguos quedan invalidados al cambiar su formato/issuer/audience. Todos deberán iniciar sesión de nuevo. Una identidad Google con Gmail/Workspace que reclama su correo elimina el password local y revoca sesiones anteriores. Correos externos sin `hd` no se enlazan automáticamente con cuentas locales. Admin exige correo autoritativo o un `GOOGLE_ADMIN_SUB` configurado explícitamente.
4. Publica una imagen del frontend que incluya `INVITATION_*`, proxy de aislamiento y `/api/invitation-health`. Usa su digest SHA256 en `DOKPLOY_INVITATION_IMAGE`; debe ser accesible desde los nodos Dokploy. Las credenciales del registro se administran en Dokploy, no se copian al contenedor de invitación.
5. Configura `DOKPLOY_URL`, `DOKPLOY_THE_DATE_API_KEY` e `INVITATION_API_URL` en el backend. DNS de ambos productos debe apuntar al ingress. El certificado individual requiere validación del dominio en Dokploy.
6. Verifica un evento de prueba completo: crear, diseñar, pagar con Stripe test, publicar, observar despliegue ready, ver invitación/fotos y responder RSVP. No uses Stripe live: continúa bloqueado.

## Publicación

`POST /events/{id}/publish` responde 202 con `{eventId, phase, host}`. Encola un proyecto y una aplicación por invitación. El worker crea/reconcilia recursos por nombre determinista y persiste identificadores entre pasos. Configura imagen, variables sin secretos, recursos de 0.5 CPU/512 MiB, dominio HTTPS y despliegue. Solo marca el evento publicado cuando Dokploy y el endpoint del contenedor confirman la identidad esperada.

`GET /events/{id}/deployment` permite consultar el estado. Un fallo puede reintentarse con publish, conservando recursos. Un lease evita que varias réplicas procesen simultáneamente la misma invitación; recupera tras vencer si el proceso muere. No hay borrado automático de proyectos ni de invitaciones publicadas. Un fallo puede dejar recursos parciales recuperables; no crea repetidamente recursos con nombres nuevos.

## WhatsApp y operación

El envío procesa hasta cinco invitados por solicitud, devuelve `remaining` y `uncertain`, y reclama cada invitado antes de llamar al proveedor. Una respuesta HTTP de éxito requiere un message ID persistido. Un timeout/fallo incierto conserva el claim y evita reenviar automáticamente: debe reconciliarse con Wazend antes de limpiar el claim o registrar el ID. No se garantiza exactamente una vez frente a un proveedor externo sin idempotencia/reconciliación propia. La UI informa pendientes e inciertos.

El recibo del webhook se registra atómicamente junto con la respuesta del evento. El mensaje adicional para pedir el motivo de Maybe sigue siendo best effort; no afecta la reserva ya confirmada.

## Límites y siguiente evolución

- Máximo 40 fotos de hasta 10 MiB, 12 000 píxeles por dimensión y 40 millones de píxeles. Revisar políticas S3 de retención y reconciliar objetos huérfanos ante fallos de compensación.
- Las reservas/recibos viven en un documento MongoDB: para eventos grandes o historiales extensos, migrar a un replica set y transacciones entre documentos antes de alcanzar el límite BSON de 16 MiB.
- Listas de eventos/invitados aún retornan arrays completos. Paginación y jobs de envío con reconciliación automática son mejoras posteriores; no afirmar soporte de escala ilimitada.
- Los límites de intentos por cuenta son compartidos en MongoDB. El límite global usa `RemoteAddr`; detrás de un proxy puede compartir bucket entre clientes. No se confía en `X-Forwarded-For` sin una política explícita de proxies confiables.
- Recuperación de contraseña, verificación por correo externo y MFA requieren flujos adicionales. La sesión dura 24 horas; puede revocarse incrementando `users.tokenVersion`.

## Verificación

```sh
go test -race ./...
go vet ./...
go build ./cmd/api
# Base local aislada; el runner crea/elimina solo bases aleatorias thedate_test_*.
TEST_MONGODB_URI=mongodb://localhost:27017 go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Las pruebas de integración usan MongoDB 7 standalone y proveedores HTTP falsos. Verifican seguridad de sesiones, pagos firmados, concurrencia, replays, accesos de pareja, envío único y recuperación de publicación. No sustituyen una validación del despliegue en Dokploy real.
