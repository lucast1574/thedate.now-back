# Gift product previews

Gift metadata is read-only and does not create a gift until the owner saves it.

- Shared links retain product, seller and variant identifiers. Known analytics fields are removed. Mercado Libre /up links retain pdp_filters and can recover wid from the share fragment.
- The parser reads at most 4 MiB of HTML, including Product JSON-LD in the body, ProductGroup/mainEntity and schema.org Product microdata. Ordinary body meta tags cannot replace head metadata.
- Product images take priority over generic site images. Relative image URLs are resolved and validated with the same public HTTPS rules.
- If HTML is incomplete, Shopify /products/{slug}.js and VTEX's public product-by-URL catalogue can provide an exact product. VTEX results must match the requested slug; recommendations cannot substitute another product.
- No JavaScript is executed. The client validates DNS and redirect destinations, rejects private addresses, limits response sizes, and applies a 12-second overall deadline. No shopper cookies or secrets are forwarded.
- Pasting a valid HTTPS link starts an automatic lookup after 700 ms. Explicit retry remains available. Responses for old links are ignored, and custom names/images are retained.

## Live verification, 2026-10-30

Public product pages returned a title and image for Falabella Peru (20926138), Ripley Peru (2086342969777) and Coolbox Peru (100431244). An obsolete Coolbox product returned no matching catalogue entry and was not replaced with a different gift.

The supplied Mercado Libre /up/MPEU3667914835 link (item MPE913032834) returned a generic Mercado Libre page. Its official item API returned 403 without authorized access. The reader rejects that generic title rather than claiming a successful import. Automatic extraction for such blocked pages requires an authorized provider/API integration; this implementation does not bypass access controls or guarantee every retailer/country.

Retailer identity labels are independent of extraction support. Supported public metadata formats work across stores without a store allowlist. Store changes and access restrictions can still require manual completion.
