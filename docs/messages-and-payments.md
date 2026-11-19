# Mensajes y pagos de invitados

Wazend recibe mensajes nuevos de WhatsApp mediante el evento `message`, además de los eventos `poll.vote` y `event.response` existentes. La recepción de texto y adjuntos nunca modifica las respuestas canónicas, acompañantes, aforo o pagos. No hay extracción automática de importes de las capturas ni una subida de comprobantes para invitados. La bandeja comienza a conservar mensajes cuando se habilita su webhook; no importa silenciosamente conversaciones anteriores.

## Asociación y privacidad

El endpoint compartido verifica firma, timestamp y sesión antes de procesar el mensaje una sola vez. Consulta los candidatos en las bases de ambas marcas cuando comparten la sesión. Una invitación o mensaje de esa conversación citado identifica el evento. Si el teléfono corresponde a una sola invitación, se asocia sin preguntas, incluso después del cierre. Las invitaciones nuevas no incluyen códigos técnicos. Los grupos, estados y mensajes propios se ignoran. Los remitentes `@lid` se resuelven consultando la sesión autenticada de Wazend.

Un mensaje ambiguo queda privado, sin `eventId`/`guestId`, y recibe una encuesta nativa de WhatsApp para elegir el evento con un toque. Las opciones muestran título, fecha y marca, únicamente de invitaciones enviadas a ese teléfono; se paginan cuando exceden el límite de la encuesta. Cada encuesta identifica un mensaje o imagen concreto. Se verifica sesión, firma, remitente y selección antes de asociar ese mensaje, y se vuelve a comprobar que el invitado conserve acceso al evento. La elección no cambia RSVP ni registra pagos. Los reintentos no reenvían la encuesta y una elección ya aplicada no mueve el archivo a otro evento. Las respuestas a las aclaraciones antiguas admiten el título exacto si es inequívoco y nunca generan una cadena de nuevas preguntas. Los resultados de envío desconocidos no se reintentan a ciegas.

Los IDs normalizados de sesión y mensaje deduplican webhooks. Cada conversación guarda marcas de lectura y atención independientes: marcar atendido un mensaje antiguo no oculta otro recibido después. La lista devuelve la última entrada por invitado; las conversaciones paginan de 100 en 100 mediante `before`.

## Adjuntos y respuestas

Al abrir una conversación se sincronizan automáticamente los últimos 50 mensajes de Wazend posteriores al envío de su invitación, con actualización periódica y al volver a la ventana. Se omiten los mensajes ambiguos, propios, de otro evento y anteriores a la invitación. La recuperación es idempotente y no envía aclaraciones ni otros mensajes automáticos sobre el historial. Los adjuntos recuperados se descargan en segundo plano.

Los adjuntos se descargan con la credencial de Wazend solo desde `/api/files/` de su origen configurado, sin redirecciones. Si Wazend almacena los archivos en S3, `WAZEND_MEDIA_BASE_URL` debe indicar explícitamente el origen HTTPS y prefijo del bucket permitido; esas descargas nunca reciben la clave de la API. Se conservan en el bucket privado de la marca y se sirven tras comprobar acceso al evento. Límite de 10 MiB; imágenes de hasta 40 millones de píxeles. Un worker intenta recuperar archivos pendientes durante siete días. Eliminar una cuenta elimina sus adjuntos asociados, mensajes y estados de conversación.

Los envíos del creador/colaborador tienen `requestId` y un registro persistido antes de enviar. Un timeout se muestra como envío por verificar; el mismo identificador no vuelve a enviar a ciegas. La respuesta incluye marca y evento y puede citar el mensaje del invitado. Enviar una respuesta no marca automáticamente la conversación atendida.

## Pagos, únicamente The Date

`Event.admission` se configura en el formulario inicial y admite costo por persona o invitación/grupo, importe en unidades menores, moneda e instrucciones. Las bodas rechazan este campo. PEN, USD, MXN, COP, EUR, CLP y ARS se guardan explícitamente; no hay conversión entre monedas. Los mensajes de invitación de eventos con costo incluyen estas instrucciones.

Solo propietario/admin accede a `guest-payments`. Para registrar un abono se exige importe, moneda coincidente, fecha válida, medio y confirmación explícita de ingreso verificado. Puede vincularse un adjunto recibido del mismo invitado y evento; también se admite efectivo o transferencia manual sin adjunto. El ID de operación permite reintentos y un adjunto no genera dos abonos. El registro y los cambios de costo compiten con `responseVersion`: después de un pago no se permite cambiar importe, modalidad, moneda ni desactivar el costo. Los invitados con pagos registrados no se eliminan.

Los mensajes y el registro de pagos siguen disponibles después de cerrar el evento; las demás operaciones de cierre conservan sus restricciones. Retirar la web pública y liberar su dirección no elimina la asociación WhatsApp. Los eventos finalizados participan en la detección de ambigüedad del mismo teléfono y aparecen en la encuesta de selección junto con los activos. Si solo existe una invitación, no se solicita selección. Los pagos de invitados son distintos del pago de activación de la plataforma y no usan Stripe.

## Rutas privadas

- `GET /events/{id}/conversations`
- `GET /events/{id}/conversations/{guestId}?before={messageId}`
- `PATCH /events/{id}/conversations/{guestId}` con `through` y `attended` opcional.
- `POST /events/{id}/conversations/{guestId}` con `requestId`, `text` y `replyTo` opcional.
- `POST /events/{id}/conversations/{guestId}/history`
- `GET /events/{id}/messages/{messageId}/media`
- `GET|POST /events/{id}/guest-payments`

Contrato base de Wazend/WAHA: [recepción y medios](https://waha.devlike.pro/docs/how-to/receive-messages/), [envío con cita](https://waha.devlike.pro/docs/how-to/send-messages/), [resolución de LIDs](https://waha.devlike.pro/docs/how-to/contacts/#api---lids).


Confirmar Sí mantiene el envío personal de la lista de regalos cuando está habilitada. El webhook procesa su cola al confirmar, además del worker de respaldo. Eliminar al invitado limpia la marca de seguimiento anterior para que una nueva invitación y confirmación puedan generar su propio envío. Un envío de regalos en curso bloquea temporalmente la eliminación.

Las nuevas confirmaciones Sí reciben el enlace personal si Regalos está habilitado, incluso con la lista vacía. Quienes recibieron el enlace antes del primer regalo reciben un único aviso cuando hay regalos disponibles. Las confirmaciones anteriores pendientes conservan su política de envío y los estados enviados o inciertos no se reinician. Tal vez no envía regalos hasta convertirse en Sí.

Desde la encuesta, el enlace web o una corrección del organizador, el cambio a Sí prepara por separado los avisos de acompañantes pendientes, de regalos habilitados y, en The Date, la solicitud de comprobante si hay saldo. El envío vuelve a comprobar la respuesta y la elegibilidad, y conserva su resultado aunque otro aviso falle. Los mensajes automáticos quedan en la conversación, con el identificador del proveedor para asociar una respuesta citada.

Una imagen o PDF que cita la solicitud de pago se indica como posible comprobante. Un adjunto elegible recibido en una conversación con saldo pendiente se indica para revisión. Los adjuntos ambiguos siguen el flujo de privacidad de selección de evento. Ninguna de estas etiquetas registra un abono.
