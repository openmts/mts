import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { setOnAuthFailed } from './api/client'
import './index.css'

const app = createApp(App)
app.use(router)
app.mount('#app')

setOnAuthFailed(() => {
  router.push('/login')
})
