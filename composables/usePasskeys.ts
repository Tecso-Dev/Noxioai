type PublicKeyOptions = Record<string, any>

export interface AuthCapabilities {
  passkeys: boolean
  oauth: string[]
  password: { minimum: number; maximum: number }
}

function base64URLToBuffer(value: string): ArrayBuffer {
  const padded = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=')
  const decoded = atob(padded)
  const bytes = new Uint8Array(decoded.length)
  for (let index = 0; index < decoded.length; index += 1) bytes[index] = decoded.charCodeAt(index)
  return bytes.buffer
}

function bufferToBase64URL(value: ArrayBuffer | null): string | null {
  if (!value) return null
  const bytes = new Uint8Array(value)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function decodeCreationOptions(payload: PublicKeyOptions): PublicKeyCredentialCreationOptions {
  const options = structuredClone(payload.publicKey || payload)
  options.challenge = base64URLToBuffer(options.challenge)
  options.user.id = base64URLToBuffer(options.user.id)
  options.excludeCredentials = (options.excludeCredentials || []).map((credential: PublicKeyOptions) => ({
    ...credential,
    id: base64URLToBuffer(credential.id)
  }))
  return options
}

function decodeRequestOptions(payload: PublicKeyOptions): PublicKeyCredentialRequestOptions {
  const options = structuredClone(payload.publicKey || payload)
  options.challenge = base64URLToBuffer(options.challenge)
  options.allowCredentials = (options.allowCredentials || []).map((credential: PublicKeyOptions) => ({
    ...credential,
    id: base64URLToBuffer(credential.id)
  }))
  return options
}

function serializeCredential(credential: PublicKeyCredential): Record<string, any> {
  const response = credential.response
  const output: Record<string, any> = {
    id: credential.id,
    rawId: bufferToBase64URL(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    clientExtensionResults: credential.getClientExtensionResults()
  }
  if (response instanceof AuthenticatorAttestationResponse) {
    output.response = {
      clientDataJSON: bufferToBase64URL(response.clientDataJSON),
      attestationObject: bufferToBase64URL(response.attestationObject),
      transports: typeof response.getTransports === 'function' ? response.getTransports() : []
    }
  } else {
    const assertion = response as AuthenticatorAssertionResponse
    output.response = {
      clientDataJSON: bufferToBase64URL(assertion.clientDataJSON),
      authenticatorData: bufferToBase64URL(assertion.authenticatorData),
      signature: bufferToBase64URL(assertion.signature),
      userHandle: bufferToBase64URL(assertion.userHandle)
    }
  }
  return output
}

export function usePasskeys() {
  const api = useRuntimeConfig().public.apiBase
  const supported = computed(() => import.meta.client && 'PublicKeyCredential' in window && !!navigator.credentials)

  async function capabilities(): Promise<AuthCapabilities> {
    return $fetch<AuthCapabilities>(`${api}/api/auth/capabilities`, { credentials: 'include' })
  }

  async function conditionalMediationAvailable(): Promise<boolean> {
    if (!supported.value) return false
    const check = (window.PublicKeyCredential as any)?.isConditionalMediationAvailable
    if (typeof check !== 'function') return false
    try {
      return await check()
    } catch {
      return false
    }
  }

  async function login(remember = false, options?: { mediation?: CredentialMediationRequirement; signal?: AbortSignal }): Promise<void> {
    if (!supported.value) throw new Error('passkeys_unsupported')
    const request = await $fetch<PublicKeyOptions>(`${api}/api/auth/passkeys/login/start`, {
      method: 'POST', credentials: 'include', body: { remember }
    })
    const credential = await navigator.credentials.get({
      publicKey: decodeRequestOptions(request),
      mediation: options?.mediation,
      signal: options?.signal
    }) as PublicKeyCredential | null
    if (!credential) throw new Error('passkey_cancelled')
    await $fetch(`${api}/api/auth/passkeys/login/finish`, {
      method: 'POST', credentials: 'include', body: serializeCredential(credential)
    })
  }

  async function register(name = ''): Promise<void> {
    if (!supported.value) throw new Error('passkeys_unsupported')
    const options = await $fetch<PublicKeyOptions>(`${api}/api/auth/passkeys/register/start`, {
      method: 'POST', credentials: 'include'
    })
    const credential = await navigator.credentials.create({ publicKey: decodeCreationOptions(options) }) as PublicKeyCredential | null
    if (!credential) throw new Error('passkey_cancelled')
    const query = name ? `?name=${encodeURIComponent(name)}` : ''
    await $fetch(`${api}/api/auth/passkeys/register/finish${query}`, {
      method: 'POST', credentials: 'include', body: serializeCredential(credential)
    })
  }

  return { supported, capabilities, conditionalMediationAvailable, login, register }
}
