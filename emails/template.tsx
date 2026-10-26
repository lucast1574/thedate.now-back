import * as React from 'react';
import { Html, Head, Preview, Body, Container, Section, Heading, Text, Button, Img, Link, Hr } from 'react-email';

export type Brand = { id: string; name: string; accent: string; ink: string; tint: string; origin: string; studio: string; logo: string; tagline: string };
export const brands: Brand[] = [
  { id: 'general', name: 'The Date', accent: '#7952c8', ink: '#292336', tint: '#eee7fa', origin: 'https://thedate.now', studio: 'https://crea.thedate.now', logo: 'event-icon.png', tagline: 'Tu próxima fecha empieza aquí.' },
  { id: 'wedding', name: 'Save the Date', accent: '#98664f', ink: '#302b26', tint: '#eee4d8', origin: 'https://save.thedate.now', studio: 'https://studio.save.thedate.now', logo: 'wedding-icon.png', tagline: 'Una fecha. Todos tus momentos.' },
];

export const messages = {
  welcome: { eyebrow: 'Tu cuenta está lista', title: 'Hagamos que suceda.', body: 'Ya tienes tu espacio para preparar invitaciones, organizar a tus invitados y dar forma a tu próxima celebración.', action: 'Entrar a mi espacio' },
  ticket_received: { eyebrow: 'Soporte', title: 'Recibimos tu reporte.', body: 'Tu ticket quedó guardado. Nuestro equipo lo revisará y podrás seguir su estado desde tu cuenta.', action: 'Ver mis tickets' },
  ticket_resolved: { eyebrow: 'Soporte', title: 'Tu ticket está resuelto.', body: 'El equipo marcó tu reporte como resuelto. Entra a tu cuenta para consultar el historial y la respuesta.', action: 'Consultar respuesta' },
  invitation: { eyebrow: 'Una invitación para colaborar', title: 'Este evento también es tuyo.', body: '{{.Inviter}} te invita a colaborar en un evento. Accede con la cuenta a la que llegó este correo para aceptar la invitación.', action: 'Aceptar invitación' },
  payment_received: { eyebrow: 'Tu evento', title: 'Pago confirmado.', body: 'El pago de tu evento quedó confirmado. Entra a tu espacio para consultar los detalles y continuar con tu invitación.', action: 'Ver mi evento' },
};
export type MessageKind = keyof typeof messages;

export function TransactionalEmail({ brand, kind }: { brand: Brand; kind: MessageKind }) {
  const copy = messages[kind];
  const text = { color: '#5e5b65', fontSize: '16px', lineHeight: '27px', margin: '0 0 20px' };
  return <Html lang="es"><Head>
    <meta name="color-scheme" content="light" />
    <style>{'@media(max-width:600px){.mail-shell{padding:20px 12px!important}.mail-content{padding:28px 24px!important}.mail-title{font-size:32px!important;line-height:38px!important}}'}</style>
  </Head><Preview>{copy.title}</Preview>
    <Body style={{ background: '#f6f4f7', margin: 0, fontFamily: 'Arial, Helvetica, sans-serif' }}>
      <Section className="mail-shell" style={{ padding: '48px 20px' }}>
        <Container style={{ maxWidth: '600px', width: '100%', background: '#ffffff', border: '1px solid #e6e1e9', borderRadius: '16px', overflow: 'hidden' }}>
          <Section style={{ padding: '28px 32px', background: brand.tint, borderTop: `4px solid ${brand.accent}` }}>
            <Link href={brand.origin} style={{ color: brand.ink, textDecoration: 'none' }}>
              <Img src={`${brand.origin}/${brand.logo}`} width="40" height="40" alt="" style={{ display: 'inline-block', verticalAlign: 'middle', marginRight: '12px' }} />
              <span style={{ fontFamily: 'Georgia, serif', fontSize: '27px', fontWeight: 600 }}>{brand.name}</span>
            </Link>
          </Section>
          <Section className="mail-content" style={{ padding: '38px 40px' }}>
            <Text style={{ color: brand.accent, fontSize: '11px', letterSpacing: '1.4px', fontWeight: 700, textTransform: 'uppercase', margin: '0 0 16px' }}>{copy.eyebrow}</Text>
            <Heading className="mail-title" style={{ color: brand.ink, fontFamily: 'Georgia, serif', fontSize: '38px', lineHeight: '44px', fontWeight: 500, margin: '0 0 28px' }}>{copy.title}</Heading>
            <Text style={text}>{'Hola{{if .Name}} {{.Name}}{{end}},'}</Text>
            <Text style={text}>{copy.body}</Text>
            {kind === 'invitation' && <Text style={text}>El enlace vence en 7 días. Este acceso se limita al evento invitado y no permite ver otros eventos ni los pagos del organizador.</Text>}
            {kind !== 'welcome' && <Section style={{ padding: '20px', borderLeft: `3px solid ${brand.accent}`, background: brand.tint, margin: '24px 0' }}>
              <Text style={{ ...text, color: brand.ink, fontSize: '15px', margin: 0 }}>{'{{.Title}}'}</Text>
            </Section>}
            <Button href="{{.ActionURL}}" style={{ background: brand.accent, color: '#ffffff', padding: '15px 24px', borderRadius: '8px', fontWeight: 700, fontSize: '14px', textDecoration: 'none' }}>{copy.action}</Button>
            <Text style={{ ...text, fontSize: '12px', margin: '20px 0 0' }}>También puedes <Link href="{{.ActionURL}}" style={{ color: brand.accent }}>abrir este enlace</Link> en tu navegador.</Text>
            <Hr style={{ borderColor: '#e9e4ec', margin: '28px 0 22px' }} />
            <Text style={{ color: brand.ink, fontSize: '14px', margin: 0 }}>El equipo de {brand.name}</Text>
          </Section>
        </Container>
        <Text style={{ textAlign: 'center', color: '#77717d', fontSize: '12px', lineHeight: '20px', margin: '22px 0 0' }}>{brand.tagline}<br />Este correo corresponde únicamente a tu cuenta de {brand.name}.</Text>
      </Section>
    </Body>
  </Html>;
}
