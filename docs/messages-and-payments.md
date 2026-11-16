# Mensajes y pagos de invitados

Wazend recibe mensajes nuevos de WhatsApp mediante el evento `message`, además de los eventos `poll.vote` y `event.response` existentes. La recepción de texto y adjuntos nunca modifica las respuestas canónicas, acompañantes, aforo o pagos. No hay extracción automática de importes de las capturas ni una subida de comprobantes para invitados. La bandeja comienza a conservar mensajes cuando se habilita su webhook; no importa silenciosamente conversaciones anteriores.

## Asociación y privacidad

El endpoint compartido verifica firma, timestamp y sesión antes de procesar el mensaje una sola vez. Consulta los candidatos en las bases de ambas marcas cuando comparten la sesión. Una invitación citada, un mensaje de esa conversación citado o el código `TD-…` identifican el evento; el teléfono basta únicamente si existe un solo candidato y no hay una cita incompatible. Los grupos, estados y mensajes propios se ignoran. Los remitentes `@lid` se resuelven consultando la sesión autenticada de Wazend.

Un mensaje ambiguo queda privado, sin `eventId`/`guestId`, y recibe una aclaración citando el mensaje original. El claim de envío se persiste antes de contactar Wazend. La aclaración no enumera eventos de otros organizadores. Una respuesta explícita puede asociar un único mensaje pendiente de las últimas 24 horas; con varios pendientes debe citar la aclaración concreta e indicar el código. Nunca se traslada todo el historial del teléfono. La copia entre bases reclama primero un destino único y conserva checkpoints para reintentos.

Los IDs normalizados de sesión y mensaje deduplican webhooks. Cada conversación guarda marcas de lectura y atención independientes: marcar atendido un mensaje antiguo no oculta otro recibido después. La lista devuelve la última entrada por invitado; las conversaciones paginan de 100 en 100 mediante `before`.

## Adjuntos y respuestas

Desde una conversación se pueden recuperar explícitamente los últimos 50 mensajes de Wazend posteriores al envío de su invitación. Se omiten los mensajes ambiguos, propios, de otro evento y anteriores a la invitación. La recuperación es idempotente y no envía aclaraciones ni otros mensajes automáticos sobre el historial. Los adjuntos recuperados se descargan en segundo plano.

Los adjuntos se descargan con la credencial de Wazend solo desde `/api/files/` de su origen configurado, sin redirecciones. Se conservan en el bucket privado de la marca y se sirven tras comprobar acceso al evento. Límite de 10 MiB; imágenes de hasta 40 millones de píxeles. Un worker intenta recuperar archivos pendientes durante siete días. Eliminar una cuenta elimina sus adjuntos asociados, mensajes y estados de conversación.

Los envíos del creador/colaborador tienen `requestId` y un registro persistido antes de enviar. Un timeout se muestra como envío por verificar; el mismo identificador no vuelve a enviar a ciegas. La respuesta incluye marca y evento y puede citar el mensaje del invitado. Enviar una respuesta no marca automáticamente la conversación atendida.

## Pagos, únicamente The Date

`Event.admission` se configura en el formulario inicial y admite costo por persona o invitación/grupo, importe en unidades menores, moneda e instrucciones. Las bodas rechazan este campo. PEN, USD, MXN, COP, EUR, CLP y ARS se guardan explícitamente; no hay conversión entre monedas. Los mensajes de invitación de eventos con costo incluyen estas instrucciones.

Solo propietario/admin accede a `guest-payments`. Para registrar un abono se exige importe, moneda coincidente, fecha válida, medio y confirmación explícita de ingreso verificado. Puede vincularse un adjunto recibido del mismo invitado y evento; también se admite efectivo o transferencia manual sin adjunto. El ID de operación permite reintentos y un adjunto no genera dos abonos. El registro y los cambios de costo compiten con `responseVersion`: después de un pago no se permite cambiar importe, modalidad, moneda ni desactivar el costo. Los invitados con pagos registrados no se eliminan.

Los mensajes y el registro de pagos siguen disponibles después de cerrar el evento; las demás operaciones de cierre conservan sus restricciones. Los pagos de invitados son distintos del pago de activación de la plataforma y no usan Stripe.

## Rutas privadas

- `GET /events/{id}/conversations`
- `GET /events/{id}/conversations/{guestId}?before={messageId}`
- `PATCH /events/{id}/conversations/{guestId}` con `through` y `attended` opcional.
- `POST /events/{id}/conversations/{guestId}` con `requestId`, `text` y `replyTo` opcional.
- `POST /events/{id}/conversations/{guestId}/history`
- `GET /events/{id}/messages/{messageId}/media`
- `GET|POST /events/{id}/guest-payments`

Contrato base de Wazend/WAHA: [recepción y medios](https://waha.devlike.pro/docs/how-to/receive-messages/), [envío con cita](https://waha.devlike.pro/docs/how-to/send-messages/), [resolución de LIDs](https://waha.devlike.pro/docs/how-to/contacts/#api---lids).
