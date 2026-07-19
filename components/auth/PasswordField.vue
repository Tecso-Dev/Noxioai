<script setup lang="ts">
const props = withDefaults(defineProps<{
  modelValue: string
  id: string
  label: string
  autocomplete?: string
  describedby?: string
  invalid?: boolean
  maxlength?: number
}>(), {
  autocomplete: 'current-password',
  describedby: undefined,
  invalid: false,
  maxlength: 256
})

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const visible = ref(false)
</script>

<template>
  <div class="auth-field">
    <label :for="id">{{ label }}</label>
    <div class="auth-password-wrap">
      <input
        :id="id"
        :value="props.modelValue"
        :type="visible ? 'text' : 'password'"
        :autocomplete="autocomplete"
        :aria-describedby="describedby"
        :aria-invalid="invalid || undefined"
        :maxlength="maxlength"
        required
        spellcheck="false"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      >
      <button
        type="button"
        class="auth-password-toggle"
        :aria-label="visible ? $t('auth.hidePassword') : $t('auth.showPassword')"
        :aria-pressed="visible"
        @click="visible = !visible"
      >
        <svg v-if="!visible" viewBox="0 0 24 24" aria-hidden="true"><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z"/><circle cx="12" cy="12" r="2.5"/></svg>
        <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="m4 4 16 16M10.6 6.1A9.5 9.5 0 0 1 12 6c6 0 9.5 6 9.5 6a15 15 0 0 1-2.1 2.7M7.2 7.2C4.2 9 2.5 12 2.5 12s3.5 6 9.5 6c1.4 0 2.7-.3 3.8-.8M9.9 9.9a3 3 0 0 0 4.2 4.2"/></svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.auth-field { display: grid; gap: .5rem; }
.auth-field label { color: #c3cedb; font-size: .76rem; font-weight: 600; }
.auth-password-wrap { position: relative; }
.auth-password-wrap input { padding-inline-end: 3.25rem; }
.auth-password-toggle { align-items: center; background: transparent; border: 0; color: #71839a; cursor: pointer; display: flex; justify-content: center; padding: .65rem; position: absolute; inset-block: 50% auto; inset-inline-end: .45rem; translate: 0 -50%; }
.auth-password-toggle:hover { color: var(--cyan); }
.auth-password-toggle svg { fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.5; block-size: 1.15rem; inline-size: 1.15rem; }
.auth-password-toggle:focus-visible { border-radius: .25rem; outline: 2px solid var(--cyan); outline-offset: 1px; }
</style>
