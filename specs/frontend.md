# Spec: Frontend

## Páginas e rotas

```
/                     → redirect para /dashboard se autenticado, senão /login
/login                → página de login (botão Google)
/dashboard            → lista de slots do Sérgio (requer auth)
/slots/new            → criar slot (requer auth + passkey registrada)
/slots/:id            → detalhe do slot (requer auth, dono)
/s/:id                → página pública de submit (Nicoly — sem conta)
/passkey/setup        → setup inicial da passkey (primeiro login)
```

## Componentes principais

### `<CryptoProvider>`
Context que expõe funções de crypto. Wrapa a WebCrypto API.
Nunca expõe chaves raw — apenas operações.

```typescript
interface CryptoContextValue {
  generateSlotKeypair: () => Promise<WrappedKeyBundle>
  encryptPayload: (data: string, recipientPublicKey: string) => Promise<EncryptedPayload>
  decryptPayload: (payload: EncryptedPayload, wrappedBundle: WrappedKeyBundle) => Promise<string>
  registerPasskey: (challenge: string) => Promise<PublicKeyCredential>
  authenticatePasskey: (challenge: string, credentialId: string) => Promise<PRFOutput>
  isPRFSupported: () => Promise<boolean>
}
```

### `<SlotCard>`
Card de slot na listagem. Mostra status com badge colorido.
- `pending` → cinza
- `filled` → amarelo (dados aguardando)
- `opened` → verde
- `expired` → vermelho

Ação principal por status: "Aguardando dados" / "Abrir dados" / "Já visualizado" / "Expirado".

### `<CreateSlotForm>`
Formulário de criação de slot.
Campos: label, email do remetente, TTL (select: 24h/48h/72h/7d).
Ao submeter: gera keypair, protege com passkey, envia ao backend.
Mostra link gerado para copiar/compartilhar.

### `<SubmitPage>` (Nicoly)
Fluxo em etapas:
1. Input de email
2. Input de OTP (enviado por email)
3. Input ou upload dos dados (textarea ou CSV)
4. Preview e confirmação
5. Encriptação local (mostra spinner "Encriptando no seu browser...")
6. Envio e confirmação

**UX crítica:** deixar claro que os dados são encriptados antes de sair do computador.

### `<OpenSlotModal>`
Abre ao clicar "Abrir dados" em um slot filled.
1. Solicita passkey (navigator.credentials.get com PRF)
2. Decripta no browser
3. Exibe dados com opção de download CSV
4. Aviso de TTL restante

## Gestão de estado

- `useState` / `useReducer` para estado local de formulários
- `React Query` para cache de dados do servidor
- JWT armazenado em memória (variável de módulo) — nunca localStorage
- Refresh automático antes de expirar (refetch 1min antes do exp)

## Segurança frontend

### CSP
Configurado no servidor Go, não no HTML. Header HTTP.
```
Content-Security-Policy:
  default-src 'none';
  script-src 'self' 'sha384-{HASH_DO_BUNDLE}';
  style-src 'self';
  connect-src 'self' {VITE_API_URL};
  font-src 'self';
  img-src 'self' data:;
  frame-ancestors 'none';
  base-uri 'none';
  form-action 'self'
```

### SRI
Vite gera hashes dos bundles. Build pipeline injeta no HTML template.

```typescript
// vite.config.ts
export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name].[hash].js',
      }
    }
  },
  plugins: [
    react(),
    sriPlugin({ algorithms: ['sha384'] }) // vite-plugin-sri
  ]
})
```

## Tratamento de erros

- Erros de crypto: mensagem genérica ao usuário, log no console (não no servidor)
- Erros de API: toast com mensagem do campo `error` da resposta
- Passkey não suportada: bloquear com instrução de browser compatível
- Slot expirado: página dedicada com mensagem clara

## Browser compatibility gate

Verificar no boot da aplicação:
```typescript
const isSupported =
  'crypto' in window &&
  'subtle' in window.crypto &&
  'credentials' in navigator &&
  typeof PublicKeyCredential !== 'undefined'

if (!isSupported) {
  // renderizar página de browser incompatível
  // não renderizar a aplicação
}
```
