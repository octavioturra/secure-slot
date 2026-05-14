# Spec: Camada de Criptografia

## Princípio fundamental

Toda operação criptográfica acontece no browser via WebCrypto API.
O servidor recebe e armazena apenas ciphertext opaco.

---

## 1. Geração de keypair (Sérgio, no momento de criar um slot)

```typescript
// Algoritmo: RSA-OAEP, 4096 bits, SHA-256
const keypair = await window.crypto.subtle.generateKey(
  { name: "RSA-OAEP", modulusLength: 4096, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256" },
  true, // extractable — necessário para exportar e proteger com passkey
  ["encrypt", "decrypt"]
)
```

**Exportação:**
- `publicKey` → exportada como SPKI, enviada ao servidor em base64
- `privateKey` → exportada como PKCS8, encriptada com AES-GCM derivado da passkey (ver seção 3)

---

## 2. Encriptação dos dados (Nicoly, no browser)

Hybrid encryption: AES encripta os dados, RSA encripta a chave AES.

```typescript
// Passo 1: gera chave AES efêmera
const aesKey = await window.crypto.subtle.generateKey(
  { name: "AES-GCM", length: 256 },
  true,
  ["encrypt", "decrypt"]
)

// Passo 2: encripta o payload com AES-GCM
const iv = window.crypto.getRandomValues(new Uint8Array(12))
const ciphertext = await window.crypto.subtle.encrypt(
  { name: "AES-GCM", iv },
  aesKey,
  payload // ArrayBuffer dos dados (ex: JSON dos CPFs)
)

// Passo 3: encripta a chave AES com a pubkey RSA do Sérgio
const rawAesKey = await window.crypto.subtle.exportKey("raw", aesKey)
const encryptedAesKey = await window.crypto.subtle.encrypt(
  { name: "RSA-OAEP" },
  recipientPublicKey, // importada do servidor
  rawAesKey
)

// Payload enviado ao servidor:
// { encryptedAesKey: base64, iv: base64, ciphertext: base64 }
```

---

## 3. Proteção da chave privada com passkey (WebAuthn + PRF extension)

A chave privada RSA não é armazenada em localStorage. É protegida por material derivado da passkey.

```typescript
// No registro da passkey — obter material PRF
const credential = await navigator.credentials.create({
  publicKey: {
    challenge: serverChallenge,
    rp: { name: "SecureSlot", id: window.location.hostname },
    user: { id: userId, name: userEmail, displayName: userName },
    pubKeyCredParams: [{ type: "public-key", alg: -7 }],
    extensions: {
      prf: { eval: { first: new TextEncoder().encode("secureslot-key-protection") } }
    }
  }
})

// Material PRF usado para derivar chave AES via HKDF
const prfOutput = credential.getClientExtensionResults().prf?.results?.first
const hkdfKey = await window.crypto.subtle.importKey("raw", prfOutput, "HKDF", false, ["deriveKey"])
const wrappingKey = await window.crypto.subtle.deriveKey(
  { name: "HKDF", hash: "SHA-256", salt: new Uint8Array(32), info: new TextEncoder().encode("private-key-wrap") },
  hkdfKey,
  { name: "AES-GCM", length: 256 },
  false,
  ["wrapKey", "unwrapKey"]
)

// Encripta a chave privada RSA com a wrappingKey derivada da passkey
const wrappedPrivateKey = await window.crypto.subtle.wrapKey("pkcs8", privateKey, wrappingKey, { name: "AES-GCM", iv })
// wrappedPrivateKey é armazenado no servidor associado ao slot
```

**Fallback se PRF não disponível:** alertar usuário que o browser não suporta proteção avançada de chave. Não degradar silenciosamente.

---

## 4. Decriptação dos dados (Sérgio, no browser)

```typescript
// Passo 1: autenticar com passkey e obter PRF para derivar wrappingKey (igual ao registro)
// Passo 2: unwrap da chave privada RSA
const privateKey = await window.crypto.subtle.unwrapKey(
  "pkcs8", wrappedPrivateKey, wrappingKey,
  { name: "RSA-OAEP", hash: "SHA-256" },
  { name: "RSA-OAEP" },
  false, ["decrypt"]
)

// Passo 3: decripta a chave AES com RSA
const rawAesKey = await window.crypto.subtle.decrypt({ name: "RSA-OAEP" }, privateKey, encryptedAesKey)
const aesKey = await window.crypto.subtle.importKey("raw", rawAesKey, { name: "AES-GCM" }, false, ["decrypt"])

// Passo 4: decripta o payload com AES-GCM
const plaintext = await window.crypto.subtle.decrypt({ name: "AES-GCM", iv }, aesKey, ciphertext)
```

---

## 5. Importação da pubkey do servidor

```typescript
async function importPublicKey(spkiBase64: string): Promise<CryptoKey> {
  const spki = Uint8Array.from(atob(spkiBase64), c => c.charCodeAt(0))
  return window.crypto.subtle.importKey(
    "spki", spki,
    { name: "RSA-OAEP", hash: "SHA-256" },
    false, ["encrypt"]
  )
}
```

---

## Casos de erro a tratar

| Erro | Comportamento |
|---|---|
| PRF extension não suportada | Mostrar aviso claro, bloquear criação de slot |
| Passkey falha na autenticação | Mensagem de erro, não expor detalhes técnicos |
| Decriptação falha | "Não foi possível abrir o slot" — nunca logar o ciphertext |
| Browser sem WebCrypto | Bloquear com mensagem de browser incompatível |

---

## Tipos TypeScript

```typescript
interface EncryptedPayload {
  encryptedAesKey: string  // base64, RSA-OAEP
  iv: string               // base64, 12 bytes
  ciphertext: string       // base64, AES-GCM
}

interface WrappedKeyBundle {
  wrappedPrivateKey: string  // base64, AES-GCM wrapped PKCS8
  wrapIv: string             // base64, 12 bytes
  publicKey: string          // base64, SPKI
  credentialId: string       // WebAuthn credential ID
}
```
