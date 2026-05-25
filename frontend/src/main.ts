import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'

// Vuetify
import 'vuetify/styles'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg'

const vuetify = createVuetify({
  components,
  directives,
  icons: {
    defaultSet: 'mdi',
    aliases,
    sets: {
      mdi,
    },
  },
  theme: {
    defaultTheme: 'dark',
    themes: {
      dark: {
        dark: true,
        colors: {
          primary: '#8A2BE2',    // Electric Purple
          secondary: '#00FFFF',  // Cyan / Neon Blue
          background: '#0E0D1A', // Space Indigo Dark
          surface: '#1A182E',    // Deep Purple Grey
          error: '#FF3366',      // Coral Red
          success: '#00FF88',    // Emerald Green
          warning: '#FFAA00'     // Amber
        }
      }
    }
  }
})

const app = createApp(App)
app.use(createPinia())
app.use(vuetify)
app.mount('#app')
