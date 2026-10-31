# Puesta en marcha y límites operativos

## Cuentas independientes por marca

`ISOLATE_PRODUCTS=true` monta `/products/general/` y `/products/wedding/`.
Cada servicio usa su propia base (`MONGODB_GENERAL_DATABASE` y
`MONGODB_WEDDING_DATABASE`) y una clave de sesión derivada distinta. No hay
fallback a usuarios ni sesiones de la base compartida anterior. Esta activación
se ha diseñado para iniciar ambas marcas sin usuarios: no migra ni elimina los
datos anteriores. No usarla como migración automática de cuentas existentes.

Configura `GOOGLE_GENERAL_CLIENT_ID` y `GOOGLE_WEDDING_CLIENT_ID` con clientes
de proyectos Google distintos. El cliente y el portal deben corresponder al
producto de la URL. La misma persona puede crear dos cuentas independientes.
El acceso administrativo y los afiliados también quedan separados.

En el frontend activa `API_PRODUCT_ISOLATION=true` y configura los dos pares
`GOOGLE_GENERAL_CLIENT_ID/SECRET` y `GOOGLE_WEDDING_CLIENT_ID/SECRET`. Los
callbacks autorizados son `https://crea.thedate.now/api/auth/google/callback`
y `https://studio.save.thedate.now/api/auth/google/callback`, respectivamente.
Despliega primero la API y después los frontends. Los workers de publicación
incluyen la ruta del producto en la URL de API de cada contenedor.

Los endpoints raíz de Stripe y Wazend se conservan: verifican las firmas en
cada base independiente y solo aplican cambios a los registros correspondientes.
No habilitan autenticación compartida en la raíz. Los certificados y los buckets
de fotos se conservan; la pertenencia a eventos se comprueba en la base de cada
marca.

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


## Profesionales y direcciones de invitación (octubre 2026)

- El profesional paga desde su cuenta: boda USD20 con comisión USD5; evento general USD10 con comisión USD2. No existe descuento profesional ni pago separado para clientes. Gestiona múltiples eventos y añade a sus clientes como colaboradores; las comisiones se consultan solo en su cuenta. Se conservan las cotizaciones anteriores por sesión para reconciliar pagos ya iniciados.
- Comisión por evento Stripe live, disponible siete días después del cargo. Las escrituras condicionales evitan abonos duplicados. Reembolsos reducen la comisión proporcionalmente; disputas la retienen. Un ajuste puede dejar saldo negativo si ya se gastó o retiró. Saldo, pruebas y cortesías no generan comisión. Las compras nuevas de cuentas normales no generan comisión, aunque tengan un referido. Las cotizaciones y balances históricos se conservan para reconciliación.
- Retiro mínimo USD10, con liquidación administrativa. El saldo cubre eventos completos sin pago mixto. Una reserva persistida permite reintentar sin descontar otra vez; cancelar antes del débito bloquea intentos antiguos mediante un marcador.
- El índice único parcial `paid_invitation_address` reserva nombres solo de eventos pagados. Se crea antes de retirar el índice general antiguo. Si un pago concurrente pierde el nombre solicitado, recibe una dirección alternativa editable antes de publicar. Nunca se cambia una dirección publicada.
- Validar con proveedores falsos y MongoDB aislado, sin cobros live ni envíos reales.
