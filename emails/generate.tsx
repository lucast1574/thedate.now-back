import * as React from 'react';
import { mkdir, writeFile } from 'node:fs/promises';
import { render, toPlainText } from 'react-email';
import { TransactionalEmail, brands, messages, MessageKind } from './template';

const output = new URL('../internal/mail/templates/', import.meta.url);
await mkdir(output, { recursive: true });
for (const brand of brands) {
  for (const kind of Object.keys(messages) as MessageKind[]) {
    const html = await render(<TransactionalEmail brand={brand} kind={kind} />);
    await writeFile(new URL(`${brand.id}-${kind}.html`, output), html);
    await writeFile(new URL(`${brand.id}-${kind}.txt`, output), toPlainText(html));
  }
}
console.log(`Compiled ${brands.length * Object.keys(messages).length} branded React Email templates for Go.`);
