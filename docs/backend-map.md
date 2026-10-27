# Mapa del backend

## Capas y puntos de entrada

| Cambio | Archivos iniciales |
| --- | --- |
| Inicio, límites HTTP y apagado | `cmd/api/main.go`, `internal/httpapi/server.go` |
| Rutas y errores JSON/orígenes | `internal/httpapi/routes.go`, `http.go` |
| Autenticación y Google | `internal/httpapi/auth.go`, `google.go` |
| Políticas de bodas/eventos | `internal/core/products.go`, `event_validation.go`, `place.go` |
| Diseño y secciones | `internal/core/design.go`, `internal/httpapi/design.go` |
| Datos privados de eventos / DTO público | `internal/httpapi/events.go`, `public_event.go` |
| Reservas y cambios de capacidad | `internal/core/responses.go` → `internal/application/responses.go` → `internal/mongostore/responses.go` |
| Invitación y acceso de parejas | `internal/core/couples.go` → `internal/application/couples.go` → `internal/mongostore/couples.go` |
| Publicación por invitación | `internal/httpapi/deployment.go`, `internal/application/deployment.go`, `internal/mongostore/deployments.go` |
| Adaptador Dokploy | `internal/dokploy/client.go`, `provision.go` |
| Pago y firma Stripe | `internal/httpapi/billing.go`, `stripe_webhook.go`, `payment_ledger.go` |
| WhatsApp y firmas webhook | `internal/httpapi/wazend.go`, `wazend_webhook.go` |
| Fotos y límites de tamaño/píxeles | `internal/httpapi/storage.go` |
| Índices y protección de intentos | `internal/mongostore/indexes.go`, `rate_limits.go` |

## Núcleo funcional y efectos

`core` contiene datos y decisiones deterministas. No consulta MongoDB, servicios HTTP ni el entorno. `application` depende de interfaces pequeñas de repositorio/proveedor y recibe el reloj. `mongostore` usa el driver oficial v2 y convierte los comandos tipados a BSON. `dokploy` encapsula transporte y autenticación del proveedor. `httpapi` es la capa de efectos de entrada: valida la sesión, traduce solicitudes/respuestas y conecta casos de uso. Los CRUD sencillos y las integraciones Stripe/S3/Wazend conservan orquestación directa en esta capa; evita añadir abstracciones que solo oculten una llamada.

No se presenta esta arquitectura como una garantía de perfección: sus invariantes críticos están probados y sus límites operativos están en `rollout.md`.

## Diferencias de producto

| Regla | Save the Date | The Date |
| --- | --- | --- |
| Tipo | `wedding` | `general` |
| Dominio | `<slug>.save.thedate.now` | `<slug>.thedate.now` |
| Acceso creador inicial | planner | organizer |
| Modalidad virtual | No | Sí |
| Accesos de pareja | Hasta dos colaboradores, incluidos enlaces pendientes | Hasta dos colaboradores, incluidos enlaces pendientes |
| Precio Stripe de pruebas | USD 25 | USD 5 |

Toda cuenta válida puede crear bodas y eventos independientes, con varios eventos por propietario. La pertenencia como colaborador permite editar diseño/fotos y gestionar invitados/mesas del evento asignado; solo propietario/admin maneja envío WhatsApp, pago y publicación. Los demos permanecen privados.

## Concurrencia en MongoDB standalone

Las respuestas canónicas y sus recibos están dentro del documento del evento. Un contador `responseVersion` protege conjuntamente reserva y capacidad. La actualización usa comparación de versión, y vuelve a calcular si otra instancia escribió primero. Invitaciones/cuentas de pareja usan `coupleVersion` con el mismo principio. No se requieren transacciones entre documentos para estas invariantes.

La colección `guests` conserva datos personales y del envío; su estado antiguo de RSVP sirve únicamente como fuente de migración. La lectura pública/privada combina el registro de invitado con la respuesta canónica. No lean `guests.response` directamente para estadísticas nuevas.

## Referencias

- Implementación de referencia: `lucast1574/nexode-backend`, `src/core/dokploy/dokploy.service.ts` y `src/modules/compute/compute.service.ts`.
- [Driver oficial de MongoDB](https://github.com/mongodb/mongo-go-driver) y [transacciones](https://www.mongodb.com/docs/drivers/go/current/crud/transactions/).
- [API de aplicaciones Dokploy](https://docs.dokploy.com/docs/api/reference-application), [dominios](https://docs.dokploy.com/docs/api/reference-domain).
- [Verificación de identidad Google](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token).

## Diseño gratuito, lienzos y herramientas pagadas

`core/flyer.go` define lienzos por sección y valida coordenadas finitas, tamaños, giros, tipografías/colores permitidos y propiedad de todas las imágenes. `core/design.go` acepta modos `sections` y `flyer`; eventos antiguos sin modo siguen usando secciones. `httpapi/design.go` guarda el diseño sin comprobar pago; los demos también son borradores persistentes. `public_event.go` incluye los lienzos explícitamente en la vista pública.

Crear/importar invitados requiere `paid` y un evento real. El importador admite hasta 500 filas previamente validadas, omite teléfonos existentes y usa IDs deterministas para reintentos seguros. Fallos parciales pueden reintentarse sin borrar filas existentes. Enviar por WhatsApp además requiere `publishedAt`.

`wazend_config.go` selecciona configuración por producto (`WAZEND_WEDDING_*`, `WAZEND_GENERAL_*`), con fallback compatible a `WAZEND_*`. Cada mensaje/evento incluye Save the Date o The Date. El webhook comprueba firma y sesión del producto al que pertenece el invitado; no basta con tener una firma válida de otro producto.

Inspección del 6 de octubre de 2026: la sesión configurada S0047 en LATAM funciona, engine GOWS, nombre de perfil Wazend. El webhook `event.response` hacia esta API tiene HMAC. La sesión también tiene otra integración; no se modificó su nombre/configuración ni se enviaron mensajes reales. Para nombres de remitente separados deben conectarse cuentas/sesiones WhatsApp propias por marca y configurar sus respectivos perfiles; el nombre no es una propiedad que pueda cambiarse por mensaje. Véase [perfil WAHA](https://waha.devlike.pro/docs/how-to/profile/).

Frontend y API se publican siguiendo `rollout.md`; la actualización del renderer reconcilia los contenedores existentes. Stripe sigue en modo de pruebas.

## Personas y plano de mesas

`core/party.go` valida personas y convierte una invitación en slots estables: titular `guestID~0`, acompañantes `guestID~1…`. `Guest.Seats` es el máximo permitido (1–20); la respuesta canónica guarda el número real y los nombres. RSVP explícito `companions: []` confirma solo al titular. Respuestas nativas de WhatsApp conservan la lista registrada; si aún no se registró, reservan conservadoramente el máximo invitado. El enlace personal enviado por WhatsApp permite registrar nombres de acompañantes; no se interpreta texto libre como una lista.

`capacityUnlimited` se exige como opción explícita en los formularios; eventos antiguos siguen limitados por `capacity`. Las reservas cuentan acompañantes y temporales no vencidos. Reducir aforo por debajo de reservas o personas asignadas falla.

`GET/PATCH /events/{id}/seating` requiere acceso al evento real pagado. `core/seating.go` valida formas, posiciones, nombres únicos, capacidad por mesa, personas existentes/no rechazadas y aforo. El límite operativo es 100 mesas de hasta 50 personas / 5000 asignaciones. El documento del evento contiene un plano privado versionado.

`mongostore/seating.go` compara `responseVersion` y `seating.version`, y al guardar incrementa ambas. RSVP limpia asientos de acompañantes eliminados o invitados que declinan, incrementando las mismas versiones en una sola escritura. Una respuesta no puede sobrescribir un plano nuevo desde un snapshot antiguo; un plano antiguo tampoco puede recuperar asientos liberados. Conflictos devuelven 409 para revisión humana.

Pruebas: `core/party_test.go`, `mongostore/seating_test.go` y `httpapi/seating_test.go` cubren aforo ilimitado/finito, límites +1, limpieza de asientos, versiones, permisos de pareja/terceros y pago. Se ejecutan con MongoDB 7 aislado y race detector.

## Plantillas por producto

`core/templates.json` define 30 entradas: 12 plantillas de lienzo por producto y 6 de secciones antiguas para compatibilidad. Las entradas de lienzo incluyen categoría y geometría de composición. `core/design.go` también acepta los motivos SVG editables frame/arch/leaf/sunburst. `core/templates.go` valida `templateId` contra el `kind`, estilo y modo del evento; una plantilla de eventos no se puede guardar en una boda. `mongostore/templates.go` sincroniza por ID la colección `invitationTemplates` al iniciar y completa únicamente IDs ausentes en eventos antiguos. El índice compuesto `kind/mode` identifica el catálogo. `GET /templates` requiere sesión; el diseño y la vista pública conservan `templateId`. Las pruebas `core/templates_test.go` y `httpapi/templates_test.go` cubren catálogo, compatibilidad, rechazo cruzado, persistencia y migración sin sobrescribir una elección guardada.

## Personalización y versiones del renderer

`core/guest_name.go` compone nombre y apellido. `guest_name` es el único binding permitido y solo puede usarse en textos. Secciones guardan `guestText`; flyers usan sus elementos habituales. `GET /public/rsvp/{token}` entrega el DTO público del evento publicado y pagado con el nombre del titular asociado al token, sin teléfonos, pagos, propietario ni otros invitados. WhatsApp usa ese mismo nombre y enlace. Editar conserva tokens, mensajes, respuestas y URL; cambiar el slug publicado se rechaza.

`dokploy/renderer.go` resuelve por HEAD el digest del repositorio configurado por el operador. `httpapi/renderer.go` comprueba al iniciar y cada minuto. `mongostore/renderer.go` persiste en `runtimeConfig` y vuelve a poner en cola despliegues sin lease activo conservando recursos y publicación original. Los errores requieren reintento explícito con la imagen nueva. La disponibilidad HTTP exige evento e imagen para no aceptar el contenedor anterior.

Configuración opcional: `DOKPLOY_RENDERER_IMAGE_REPOSITORY` y `DOKPLOY_RENDERER_MANIFEST_URL`. Si no está disponible se conserva la imagen existente. El repositorio debe ser exclusivo del renderer del frontend. Pruebas Mongo aisladas verifican leases y preservación de recursos/publicación; pruebas HTTP cubren edición después de enviar y permisos.

## Administración, acceso por correo y afiliados

- `httpapi/admin*.go`: panel común de ambas marcas, roles, cortesías, retiros y `adminAudit`. La sesión consulta el rol actual en Mongo; cambiar roles incrementa `tokenVersion`. El administrador principal y la propia cuenta no pueden demoverse. Un administrador delegado puede usar Google o correo/contraseña; solo un administrador existente puede asignar ese rol. La cuenta principal conserva su identidad Google protegida.
- Admin crea eventos completos de cortesía sin checkout. Las cortesías para otros propietarios requieren motivo y evento sin checkout activo; se excluyen de ingresos y afiliados. Estado/rol/fuente de pago no se aceptan desde los formularios de evento.
- `access_email.go`, `access_members.go`, `mail/`: correo HTML según marca, TLS verificado, límite atómico de dos colaboradores/pending, enlaces aleatorios de 48 hex con hash persistido y duración de siete días. Google o contraseña con correo exacto más enlace secreto. Cada cuenta crea sus propios eventos; revocar membresía no elimina la cuenta. Los campos `couple*` se mantienen como compatibilidad BSON; nuevas rutas `collaborators` y `access-invites`. Ya no se crean contraseñas para parejas.
- `core/finance.go`, `mongostore/affiliates.go`, `httpapi/affiliates.go`: comisión de 10% del primer pago real por referido, centavos enteros, retiro mínimo USD50. Atribución solo al crear cuenta por código válido de afiliado activo; autorreferencias descartadas. Pagos de prueba tienen conversión separada y cero saldo real. Marcador y saldo se actualizan juntos en el afiliado; reservas/rechazos de retiros tienen CAS para evitar doble gasto.
- Stripe verifica firma, modo, checkout registrado, importe/currency y PaymentIntent/charge consultados al proveedor antes de activar/abonar. Reembolsos y disputas reversan comisión de forma monotónica; replays no recuperan crédito descontado. Disputa ganada requiere revisión para restituir crédito, no autoabono; pagos manuales requieren referencia. El panel no transfiere dinero.
- SMTP de la integración existente está configurado con nombre de remitente The Date/Save the Date y dirección autorizada existente. Se comprobó TLS/auth sin enviar correos reales. Las aplicaciones están en Rangel Tech: su clave autorizada accede; la dedicada a The Date devuelve acceso denegado.

## Enlace final de publicación

`dokploy.PublicPageReady` comprueba por HTTPS la página raíz real, respuesta 200 HTML y marcador `data-invitation-event` del evento correcto. Solo solicita el host derivado del producto/subdominio; no sigue redirecciones. `httpapi/publication_view.go` oculta el host mientras publica/verifica, incluso si el contenedor ya está ready. Solo expone el enlace tras comprobar la página y `publishedAt`. La comprobación no publica eventos ni recrea recursos; el worker conserva sus checkpoints e identificación de imagen.

## Perfil y muestras personales

`core/profile.go` valida nombres y URLs HTTPS de fotos de Google. OAuth conserva `picture` únicamente desde el ID token verificado y hosts de Google permitidos; las cuentas ya conectadas reciben la foto al volver a iniciar sesión. `PATCH /profile` solo acepta nombre. `POST /profile/avatar` limita bytes/píxeles, decodifica JPEG/PNG/WebP y vuelve a codificar PNG sin metadatos; la colección privada `profilePhotos` usa el ID de la sesión. `GET /profile/avatar` sirve únicamente la foto propia con no-store y nosniff. No acepta roles, permisos, correo ni identificadores ajenos.

La lista del administrador conserva acceso a eventos reales para gestión, pero excluye demos de otros propietarios. La galería personal muestra eventos propios y compartidos; una muestra por producto se crea mediante el ID determinista existente, sin borrar diseños guardados.

`mongostore/user_roles.go` cambia rol y revoca sesiones mediante CAS de rol/versión. Las cuentas antiguas sin `tokenVersion` equivalen exclusivamente a versión cero; los escritores obsoletos no pueden sobrescribir una versión nueva. La lista administrativa indica protección del rol, sin exponer identidad interna.

`mongostore/affiliate_defaults.go` activa enlaces para cuentas elegibles al registrarse, iniciar sesión y en la migración de inicio para cuentas antiguas. Las escrituras condicionales preservan el código, saldo, conversiones y retiros. Las cuentas administradoras mantienen la exclusión de comisiones; los pagos de prueba y cortesías siguen sin saldo real.

`core/session_policy.go` decide de forma pura qué método corresponde a la identidad actual, con propietario y subject fijado como argumentos; el adaptador JWT sigue validando firma, audiencia, expiración, formato y versión revocable.

## Música y audio

`core/music.go` valida fuentes (`mp3`, `spotify`, `youtube`), las referencias de audio del evento y encabezados ID3/MPEG. `httpapi/music.go` recibe MP3 binario hasta 10 MiB, usa el almacenamiento S3 existente por producto y conserva claves inmutables. `mongostore.AttachAudio` limita atómicamente a 20 archivos por evento; cambiar de canción no reemplaza el audio publicado hasta guardar el diseño. Las versiones anteriores permanecen en almacenamiento y requieren una política futura de retención.

El PATCH de diseño valida `music` contra las claves propias; el DTO público incluye solo la canción seleccionada, sin exponer la biblioteca de borradores. El audio público requiere evento pagado/publicado y coincidencia con `music.value`; el privado exige acceso al evento. `http.ServeContent` sirve rangos, con MIME fijo y `nosniff`. No se descargan pistas de Spotify/YouTube ni se extrae su audio: se conservan referencias para sus reproductores oficiales. No hay migración de datos ni nuevas credenciales/servicios.

## Elementos de ubicación y biblioteca de símbolos

`core/symbols.go` valida las referencias `lucide:<nombre>` contra 1903 nombres empotrados, sincronizados con el sprite local del frontend mediante `npm run symbols:sync`. Los motivos anteriores siguen siendo válidos. Los elementos de texto con binding `location` admiten `locationStyle` (`icon`, `label`, `address`, `icon-label`, `icon-address`, `label-address`, `all`), dirección de hasta 500 bytes y enlace Google Maps de hasta 2048 bytes. Son datos del diseño, conservados en BSON y en el DTO público a través de `FlyerElement`; el guardado no modifica el lugar canónico del evento. Los campos específicos de ubicación se rechazan en otros tipos de binding y los enlaces se validan con `GoogleMapsURL`.

## Lista de regalos por evento

`linkpreview/structured.go` reconoce Product JSON-LD en arrays, grafos y objetos
de imagen como respaldo de Open Graph, validando las imágenes con las mismas
restricciones HTTPS/red pública. La lectura HTML está limitada al primer MiB;
una página grande no descarta los metadatos del encabezado. No se ejecuta JavaScript
ni se consultan URLs de los datos estructurados. Tiendas que bloquean solicitudes
siguen permitiendo completar nombre e imagen manualmente en el frontend.

`core/wishlist.go` define productos, validación y decisiones puras de reserva: un producto por invitación, un titular por producto, reintentos idempotentes y liberación por el titular mientras no esté recibido. Editar la presentación nunca acepta asignaciones aportadas por el cliente y no permite quitar/cambiar un producto comprometido sin liberar la reserva. `Event.Wishlist` queda fuera del JSON general; la vista pública de la invitación solo expone si está activada. La lista pública propia devuelve disponibilidad y la elección del visitante, nunca nombres/IDs de otros invitados.

`application/wishlist.go` y `mongostore/wishlist.go` realizan compare-and-swap por `wishlist.version` en el documento del evento. Pago/publicación/opción activada se verifican tanto al decidir como al escribir una reserva pública. Cambios concurrentes del organizador retornan 409 y no pisan reservas. `httpapi/wishlist.go` permite preparar el borrador antes de pagar a propietarios/colaboradores; `public_wishlist.go` verifica el token de RSVP, evento real, pago, publicación y activación. Los límites por invitado/evento están en `gift_limits.go`.

`internal/linkpreview` lee Open Graph/título para nombre e imagen, con edición manual de respaldo. Solo acepta HTTPS público, verifica todas las IP resueltas y conecta directamente a una IP validada, también tras cada redirección; no usa proxies del entorno ni cookies/credenciales. Limita tiempo, cabeceras, HTML (1 MB), imágenes (2 MB y 40 MP) y formatos raster. No admite SVG ejecutable ni destinos locales/privados. Las imágenes requieren acceso al evento o token válido y se devuelven como bytes, sin revelar el código de invitado al comercio.

Rutas privadas: `GET/PATCH /events/{id}/wishlist`, `POST .../metadata`, `PATCH .../reservations/{gift}` y `GET .../images/{gift}`. Rutas personales sin login: `GET/POST /public/wishlist/{token}` y `GET .../images/{gift}`. Reservas, datos de invitados y secciones existentes se mantienen en campos separados; no requiere migración masiva ni otro servicio. Pruebas concurrentes usan MongoDB aislado y múltiples instancias del servicio para verificar un solo ganador por regalo y una sola elección por titular.

## Entrada y avisos de celebración

`core/celebration.go` valida la apertura opcional (`classic`, `arch`, `ticket`, colores y textos acotados) y los avisos/dress code. `updateDesign` persiste `opening` y `celebration` cuando se envían; omitirlos no borra opciones existentes. El DTO público incluye ambos campos. Los bindings `dress_code` y `adults_notice` son textos editables, no reglas de aforo ni validación de edades.

`WhatsAppCelebration` añade el mensaje de adultos exclusivamente cuando se eligieron `adultsOnly` y `whatsappNotice`; no cambia el estado de envíos anteriores. La validación de fotos permite únicamente el marcador estático `template-photo`, además de las fotos propias; no acepta rutas, URLs ni archivos de otros eventos. El renderer resuelve ese marcador al SVG local. Publicar esta API antes del nuevo editor/renderer.

`EventInput.celebration` valida y conserva la elección de invitados desde la creación; `CommitEvent` actualiza ese campo solo cuando fue enviado, sin borrar opciones con clientes antiguos. Apertura admite título independiente, tipografía acotada y símbolo de sello validado (o `none`). `core/dress_code.go` valida las siete combinaciones de icono/imagen/texto y la propiedad de `dressPhotoKey` incluso cuando la imagen está oculta; estos campos se rechazan fuera del binding `dress_code`.

El binding `gift_link` acepta exclusivamente elementos de texto con el destino interno exacto `/regalos` o una URL HTTPS sin credenciales. No se consulta el destino desde el servidor. Las pruebas HTTP/Mongo guardan todas las plantillas de lienzo de ambos productos en eventos demo, preservando su condición de demo, apertura y campos de dress code/regalos.

`ImageBackground` conserva clave de foto propia, ajuste cover/contain, posición top/center/bottom e intensidad entre 0 y 1. Puede existir en el evento como imagen continua, en un lienzo o en la apertura. `ValidDesign` comprueba siempre la propiedad, incluso para fondos ocultos por una excepción de color. El DTO público incluye la imagen común; el renderer resuelve las claves con los endpoints de fotos existentes. `clearBackgroundImage` borra explícitamente la imagen común, mientras que omitir ambos campos conserva el fondo para clientes antiguos. Los fondos no necesitan nuevas rutas ni servicios de almacenamiento.
