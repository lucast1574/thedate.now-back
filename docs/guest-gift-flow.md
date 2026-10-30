# Invitation and gift navigation

The authored gift_link element always resolves to this event's wishlist. Template gift sections remain visible even with no products. From-scratch elements default to /regalos; editing the element changes its presentation, not a manually pasted destination. No additional gift section is injected outside the authored design.

Saving gifts derives the legacy enabled flag from the item count. Public catalogues and personal reservation eligibility do not rely on that flag. Publication, payment, demo status, host/event scoping, revoked tokens and optimistic versions remain enforced.

The public catalogue exposes products and availability, never guest identities. Personal invite visits remember the original bearer in a host-only HTTP-only, Secure, SameSite=Lax cookie. The cookie is validated against the current event before reuse. Without one, viewing requires no identity; choosing can request access on the registered WhatsApp number. The response is generic and never returns an invitation token. A database claim suppresses repeats for five minutes across replicas; event/IP limits also apply. Unknown send outcomes are not retried automatically. Selection is carried in the message link and requires an explicit reservation confirmation.

New personal paths use /para/{name}-{32-character-base64url-token}. All original 192 bits are preserved. The name is decorative and old paths remain accepted; legacy page visits redirect after validating the invite and host. API token URLs remain unchanged. Public pages and personal response pages link back to the invitation.

## Rollout

Deploy the backend with PERSONAL_INVITATION_LINKS unset/false first; outgoing invitation and followup URLs keep the legacy working path. Deploy both studios and release the dedicated invitation renderer. Wait until published events reconcile to the new image, then enable PERSONAL_INVITATION_LINKS=true and redeploy the API to issue personal named links. Existing WhatsApp messages, tokens, answers and gift claims are not rewritten.
