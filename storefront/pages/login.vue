<template>
  <div class="min-h-[70vh] flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
    <div class="w-full max-w-md">
      <!-- Tabs -->
      <div class="flex border-b border-gray-200 mb-8">
        <button
          :class="[
            'flex-1 py-3 text-sm font-medium border-b-2 transition-colors',
            activeTab === 'login'
              ? 'border-indigo-600 text-indigo-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300',
          ]"
          @click="activeTab = 'login'"
        >
          Sign In
        </button>
        <button
          :class="[
            'flex-1 py-3 text-sm font-medium border-b-2 transition-colors',
            activeTab === 'register'
              ? 'border-indigo-600 text-indigo-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300',
          ]"
          @click="activeTab = 'register'"
        >
          Create Account
        </button>
      </div>

      <!-- Login form -->
      <form
        v-if="activeTab === 'login'"
        class="space-y-5"
        @submit.prevent="handleLogin"
      >
        <div>
          <h2 class="text-2xl font-bold text-gray-900">Welcome back</h2>
          <p class="mt-1 text-sm text-gray-500">Sign in to your account to continue.</p>
        </div>

        <div>
          <label for="login-email" class="block text-sm font-medium text-gray-700 mb-1">
            Email address
          </label>
          <input
            id="login-email"
            v-model="loginForm.email"
            type="email"
            required
            autocomplete="email"
            class="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
            placeholder="you@example.com"
          />
        </div>

        <div>
          <label for="login-password" class="block text-sm font-medium text-gray-700 mb-1">
            Password
          </label>
          <input
            id="login-password"
            v-model="loginForm.password"
            type="password"
            required
            autocomplete="current-password"
            class="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
            placeholder="Enter your password"
          />
        </div>

        <p v-if="loginError" class="text-sm text-red-600">{{ loginError }}</p>

        <button
          type="submit"
          :disabled="submitting"
          class="w-full flex justify-center py-2.5 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {{ submitting ? 'Signing in...' : 'Sign In' }}
        </button>
      </form>

      <!-- Register form -->
      <form
        v-else
        class="space-y-5"
        @submit.prevent="handleRegister"
      >
        <div>
          <h2 class="text-2xl font-bold text-gray-900">Create an account</h2>
          <p class="mt-1 text-sm text-gray-500">Join us to track your orders and save your details.</p>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label for="reg-firstname" class="block text-sm font-medium text-gray-700 mb-1">
              First name
            </label>
            <input
              id="reg-firstname"
              v-model="registerForm.firstName"
              type="text"
              autocomplete="given-name"
              class="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              placeholder="First name"
            />
          </div>
          <div>
            <label for="reg-lastname" class="block text-sm font-medium text-gray-700 mb-1">
              Last name
            </label>
            <input
              id="reg-lastname"
              v-model="registerForm.lastName"
              type="text"
              autocomplete="family-name"
              class="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              placeholder="Last name"
            />
          </div>
        </div>

        <div>
          <label for="reg-email" class="block text-sm font-medium text-gray-700 mb-1">
            Email address
          </label>
          <input
            id="reg-email"
            v-model="registerForm.email"
            type="email"
            required
            autocomplete="email"
            class="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
            placeholder="you@example.com"
          />
        </div>

        <div>
          <label for="reg-password" class="block text-sm font-medium text-gray-700 mb-1">
            Password
          </label>
          <input
            id="reg-password"
            v-model="registerForm.password"
            type="password"
            required
            autocomplete="new-password"
            minlength="8"
            class="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
            placeholder="At least 8 characters"
          />
        </div>

        <div>
          <label for="reg-confirm" class="block text-sm font-medium text-gray-700 mb-1">
            Confirm password
          </label>
          <input
            id="reg-confirm"
            v-model="registerForm.confirmPassword"
            type="password"
            required
            autocomplete="new-password"
            class="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
            placeholder="Re-enter your password"
          />
        </div>

        <p v-if="registerError" class="text-sm text-red-600">{{ registerError }}</p>

        <button
          type="submit"
          :disabled="submitting"
          class="w-full flex justify-center py-2.5 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {{ submitting ? 'Creating account...' : 'Create Account' }}
        </button>
      </form>

      <!-- Guest checkout note -->
      <p class="mt-8 text-center text-sm text-gray-500">
        You can also
        <NuxtLink to="/products" class="text-indigo-600 hover:text-indigo-800 font-medium">
          continue as a guest
        </NuxtLink>
        and check out without an account.
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
useHead({
  title: 'Sign In - ForgeCommerce',
})

const authStore = useAuthStore()
const router = useRouter()

const activeTab = ref<'login' | 'register'>('login')
const submitting = ref(false)
const loginError = ref<string | null>(null)
const registerError = ref<string | null>(null)

const loginForm = reactive({
  email: '',
  password: '',
})

const registerForm = reactive({
  email: '',
  password: '',
  confirmPassword: '',
  firstName: '',
  lastName: '',
})

async function handleLogin() {
  loginError.value = null
  submitting.value = true

  try {
    await authStore.login(loginForm.email, loginForm.password)
    router.push('/')
  } catch (e: any) {
    loginError.value = e.message || 'Invalid email or password'
  } finally {
    submitting.value = false
  }
}

async function handleRegister() {
  registerError.value = null

  if (registerForm.password !== registerForm.confirmPassword) {
    registerError.value = 'Passwords do not match'
    return
  }

  if (registerForm.password.length < 8) {
    registerError.value = 'Password must be at least 8 characters'
    return
  }

  submitting.value = true

  try {
    await authStore.register(
      registerForm.email,
      registerForm.password,
      registerForm.firstName || undefined,
      registerForm.lastName || undefined
    )
    router.push('/')
  } catch (e: any) {
    registerError.value = e.message || 'Registration failed'
  } finally {
    submitting.value = false
  }
}

// Redirect if already authenticated.
onMounted(() => {
  if (authStore.isAuthenticated) {
    router.replace('/')
  }
})
</script>
