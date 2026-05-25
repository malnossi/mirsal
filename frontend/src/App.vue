<script lang="ts" setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useTransferStore } from './stores/transfer'
import { storeToRefs } from 'pinia'

// SVG Icons from @mdi/js
import {
  mdiSend,
  mdiDownload,
  mdiFolder,
  mdiFile,
  mdiClose,
  mdiCheckCircle,
  mdiAlertCircle,
  mdiClipboardOutline,
  mdiCog,
  mdiContentCopy,
  mdiFolderOpen,
  mdiRefresh,
  mdiOpenInNew,
  mdiShieldLockOutline,
  mdiArrowRight
} from '@mdi/js'

// Initialize Pinia store
const store = useTransferStore()
const {
  currentState,
  role,
  code,
  filePath,
  fileName,
  fileSize,
  isDir,
  errorMessage,
  saveDir,
  progress
} = storeToRefs(store)

// Component local state
const activeTab = ref<'send' | 'receive'>('send')
const codeInput = ref('')
const dragOver = ref(false)
const showSettings = ref(false)
const newSaveDir = ref('')
const copySuccess = ref(false)
const clipboardCode = ref('')

onMounted(async () => {
  await store.initStore()
  newSaveDir.value = store.saveDir
  
  // Listen for window focus to check clipboard for valid wormhole codes
  window.addEventListener('focus', checkClipboard)
  checkClipboard()
})

onUnmounted(() => {
  store.cleanupStore()
  window.removeEventListener('focus', checkClipboard)
})

// Auto-check clipboard for valid wormhole codes (e.g. 5-purple-dishwasher)
const checkClipboard = async () => {
  if (activeTab.value !== 'receive' || currentState.value !== 'IDLE') return
  try {
    const text = await navigator.clipboard.readText()
    const trimmed = text.trim()
    // Magic wormhole code regex: number-word-word...
    const match = trimmed.match(/^\d+-[a-zA-Z]+-[a-zA-Z]+(-[a-zA-Z]+)*$/)
    if (match && match[0] !== codeInput.value) {
      clipboardCode.value = match[0]
    } else {
      clipboardCode.value = ''
    }
  } catch (e) {
    // Clipboard reading might be blocked or empty, ignore
    clipboardCode.value = ''
  }
}

// Watch tab change to check clipboard
watch(activeTab, (newTab) => {
  if (newTab === 'receive') {
    checkClipboard()
  } else {
    clipboardCode.value = ''
  }
})

// Drag and Drop files
const handleDragOver = () => {
  dragOver.value = true
}

const handleDragLeave = () => {
  dragOver.value = false
}

const handleDrop = (e: DragEvent) => {
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (files && files.length > 0) {
    const file = files[0]
    // Wails webviews append the full filesystem path in the standard file.path attribute
    if ('path' in file) {
      store.filePath = (file as any).path
      store.fileName = file.name
      // Fallback check, Go side will os.Stat and determine if Dir or File precisely
      store.isDir = file.type === '' && file.size === 0
    }
  }
}

// Select via dialogs
const triggerFileSelect = async () => {
  await store.chooseFile()
}

const triggerDirectorySelect = async () => {
  await store.chooseDirectory()
}

// Copy code to clipboard
const copyCodeToClipboard = async () => {
  if (!store.code) return
  try {
    await navigator.clipboard.writeText(store.code)
    copySuccess.value = true
    setTimeout(() => {
      copySuccess.value = false
    }, 2000)
  } catch (e) {
    console.error('Failed to copy to clipboard', e)
  }
}

// Use clipboard helper to autofill code
const useClipboardCode = () => {
  if (clipboardCode.value) {
    codeInput.value = clipboardCode.value
    clipboardCode.value = ''
  }
}

// Start transfer operations
const handleSend = async () => {
  if (store.filePath) {
    await store.startSend()
  }
}

const handleReceive = async () => {
  if (codeInput.value) {
    await store.startReceive(codeInput.value.trim())
  }
}

const handleCancel = async () => {
  await store.cancel()
}

const handleReset = () => {
  codeInput.value = ''
  store.reset()
}

// Settings handlers
const changeSaveDir = async () => {
  const selected = await store.chooseSaveFolder()
  if (selected) {
    newSaveDir.value = selected
  }
}
</script>

<template>
  <v-app class="app-bg">
    <!-- Main layout with gorgeous space glassmorphic blur -->
    <v-main class="d-flex align-center justify-center py-6">
      <v-container class="d-flex flex-column align-center">
        <!-- Top Toolbar / Header -->
        <div class="w-100 max-width-card d-flex align-center justify-between mb-4 px-2">
          <div class="d-flex align-center gap-2">
            <v-icon :icon="mdiShieldLockOutline" color="primary" size="large" class="glow-icon"></v-icon>
            <h1 class="logo-title font-weight-bold">Mirsal</h1>
          </div>
          <div>
            <v-btn
              icon
              variant="text"
              color="grey-lighten-1"
              @click="showSettings = true"
              :disabled="currentState !== 'IDLE'"
            >
              <v-icon :icon="mdiCog"></v-icon>
            </v-btn>
          </div>
        </div>

        <!-- Glass Container -->
        <v-card class="glass-card w-100 max-width-card overflow-hidden" elevation="24">
          <!-- State: IDLE screen -->
          <div v-if="currentState === 'IDLE'">
            <!-- Segmented Tabs Send/Receive -->
            <v-tabs
              v-model="activeTab"
              grow
              bg-color="transparent"
              color="primary"
              slider-color="secondary"
              class="border-b"
            >
              <v-tab value="send" class="text-subtitle-1 py-4">
                <v-icon :icon="mdiSend" start></v-icon>
                Send
              </v-tab>
              <v-tab value="receive" class="text-subtitle-1 py-4">
                <v-icon :icon="mdiDownload" start></v-icon>
                Receive
              </v-tab>
            </v-tabs>

            <v-card-text class="pa-6">
              <!-- Send Tab Content -->
              <v-window v-model="activeTab">
                <v-window-item value="send">
                  <div
                    class="drag-drop-zone d-flex flex-column align-center justify-center pa-8 text-center"
                    :class="{ 'drag-over': dragOver, 'has-file': !!filePath }"
                    @dragover.prevent="handleDragOver"
                    @dragleave.prevent="handleDragLeave"
                    @drop.prevent="handleDrop"
                  >
                    <!-- Icon based on type -->
                    <div class="mb-4">
                      <v-icon
                        :icon="isDir ? mdiFolder : mdiFile"
                        :color="filePath ? 'secondary' : 'grey-darken-1'"
                        size="64"
                        class="animated-icon"
                      ></v-icon>
                    </div>

                    <div v-if="!filePath">
                      <h3 class="text-h6 mb-2">Drag and drop file or folder here</h3>
                      <p class="text-body-2 text-grey-darken-1 mb-6">Zero-knowledge secure transfer</p>
                      
                      <div class="d-flex gap-3 justify-center">
                        <v-btn
                          color="primary"
                          variant="tonal"
                          prepend-icon="mdi-file-outline"
                          @click="triggerFileSelect"
                        >
                          <v-icon :icon="mdiFile" start></v-icon>
                          Choose File
                        </v-btn>
                        <v-btn
                          color="secondary"
                          variant="tonal"
                          prepend-icon="mdi-folder-outline"
                          @click="triggerDirectorySelect"
                        >
                          <v-icon :icon="mdiFolder" start></v-icon>
                          Choose Folder
                        </v-btn>
                      </div>
                    </div>

                    <div v-else class="w-100">
                      <h3 class="text-h6 mb-1 text-truncate text-secondary">{{ fileName }}</h3>
                      <p class="text-body-2 text-grey-lighten-1 mb-6">
                        {{ isDir ? 'Folder Selected' : 'File Selected' }}
                      </p>

                      <div class="d-flex gap-3 justify-center">
                        <v-btn
                          color="primary"
                          variant="elevated"
                          size="large"
                          @click="handleSend"
                          append-icon="mdi-arrow-right"
                          class="glow-btn"
                        >
                          Send File
                          <v-icon :icon="mdiArrowRight" end></v-icon>
                        </v-btn>
                        <v-btn color="grey-darken-3" variant="flat" size="large" @click="store.reset()">
                          Clear
                        </v-btn>
                      </div>
                    </div>
                  </div>
                </v-window-item>

                <!-- Receive Tab Content -->
                <v-window-item value="receive">
                  <div class="d-flex flex-column align-center py-6 text-center">
                    <h2 class="text-h5 mb-2 font-weight-medium">Enter Wormhole Code</h2>
                    <p class="text-body-2 text-grey-darken-1 mb-6">Ask the sender for their 6-word code</p>

                    <v-text-field
                      v-model="codeInput"
                      label="Code (e.g. 5-purple-dishwasher)"
                      placeholder="e.g. 5-purple-dishwasher"
                      variant="outlined"
                      color="primary"
                      clearable
                      class="w-100 px-6 mb-4 max-width-input"
                      hide-details
                      @keyup.enter="handleReceive"
                    ></v-text-field>

                    <!-- Auto-Clipboard Detect chip -->
                    <v-slide-y-transition>
                      <v-chip
                        v-if="clipboardCode"
                        color="secondary"
                        variant="tonal"
                        class="mb-6 cursor-pointer bounce-chip px-4 py-2"
                        @click="useClipboardCode"
                      >
                        <v-icon :icon="mdiClipboardOutline" start class="mr-1"></v-icon>
                        Use detected code: <strong>{{ clipboardCode }}</strong>
                      </v-chip>
                    </v-slide-y-transition>

                    <v-btn
                      color="secondary"
                      variant="elevated"
                      size="large"
                      :disabled="!codeInput"
                      @click="handleReceive"
                      class="glow-btn"
                    >
                      <v-icon :icon="mdiDownload" start></v-icon>
                      Receive File
                    </v-btn>

                    <!-- Target Directory Indicator -->
                    <div class="mt-8 text-caption text-grey-darken-1 d-flex align-center gap-1">
                      <span>Downloading to: </span>
                      <span class="text-grey-lighten-1 text-truncate" style="max-width: 250px;">{{ saveDir }}</span>
                    </div>
                  </div>
                </v-window-item>
              </v-window>
            </v-card-text>
          </div>

          <!-- State: COMPRESSING screen -->
          <div v-else-if="currentState === 'COMPRESSING'" class="d-flex flex-column align-center justify-center pa-12 text-center">
            <v-progress-circular
              indeterminate
              color="primary"
              size="80"
              width="6"
              class="mb-6 active-glow"
            ></v-progress-circular>
            <h3 class="text-h5 text-primary mb-2">Compressing Folder...</h3>
            <p class="text-body-2 text-grey-darken-1">Preparing files for magic streaming</p>
          </div>

          <!-- State: CONNECTING / WAITING (Code Generated) screen -->
          <div v-else-if="currentState === 'CONNECTING' || currentState === 'WAITING'" class="pa-8 text-center d-flex flex-column align-center">
            <div v-if="currentState === 'CONNECTING'" class="my-6">
              <v-progress-circular
                indeterminate
                color="secondary"
                size="80"
                width="6"
                class="mb-6 active-glow"
              ></v-progress-circular>
              <h3 class="text-h5 text-secondary mb-2">Establishing Secure Wormhole...</h3>
              <p class="text-body-2 text-grey-darken-1">Exchanging zero-knowledge keys</p>
            </div>

            <div v-else class="w-100 d-flex flex-column align-center">
              <v-avatar color="primary-lighten-4" size="72" class="mb-4 bg-primary-transparent">
                <v-icon :icon="mdiShieldLockOutline" color="primary" size="40"></v-icon>
              </v-avatar>

              <h2 class="text-h5 mb-1 font-weight-medium">Secure Channel Open</h2>
              <p class="text-body-2 text-grey-darken-1 mb-6">Share this temporary code with the recipient</p>

              <!-- Code Box -->
              <div class="code-box d-flex align-center justify-center px-6 py-4 mb-6">
                <span class="code-text select-all">{{ code }}</span>
                <v-btn
                  icon
                  variant="text"
                  color="secondary"
                  class="ml-2"
                  @click="copyCodeToClipboard"
                  title="Copy Code"
                >
                  <v-icon :icon="mdiContentCopy"></v-icon>
                </v-btn>
              </div>

              <!-- File Info Card -->
              <div class="glass-subcard pa-4 mb-6 w-100 max-width-subcard d-flex align-center gap-3">
                <v-icon :icon="isDir ? mdiFolder : mdiFile" color="secondary" size="32"></v-icon>
                <div class="text-left overflow-hidden">
                  <div class="text-body-1 font-weight-bold text-truncate text-grey-lighten-2">{{ fileName }}</div>
                  <div class="text-body-2 text-grey-darken-1">{{ store.formatBytes(fileSize) }}</div>
                </div>
              </div>

              <!-- Waiting indicator -->
              <div class="d-flex align-center gap-2 mb-6">
                <span class="breathing-dot"></span>
                <span class="text-body-2 text-grey-darken-1">Waiting for receiver to connect...</span>
              </div>
            </div>

            <v-btn color="error" variant="outlined" size="large" @click="handleCancel" class="mt-2">
              <v-icon :icon="mdiClose" start></v-icon>
              Cancel Transfer
            </v-btn>
          </div>

          <!-- State: ACTIVE transfer screen (Progress) -->
          <div v-else-if="currentState === 'ACTIVE'" class="pa-8 text-center d-flex flex-column align-center">
            <!-- Circular Progress Container -->
            <div class="progress-container mb-6 position-relative">
              <v-progress-circular
                :model-value="progress.percent"
                color="secondary"
                size="160"
                width="12"
                bg-color="grey-darken-4"
                class="active-glow"
              >
                <div class="d-flex flex-column align-center justify-center">
                  <span class="text-h4 font-weight-bold text-secondary">{{ Math.round(progress.percent) }}%</span>
                  <span class="text-caption text-grey-darken-1 mt-1">transferred</span>
                </div>
              </v-progress-circular>
            </div>

            <h3 class="text-h5 text-truncate mb-1 max-width-subcard text-grey-lighten-2">{{ fileName }}</h3>
            <p class="text-body-2 text-grey-darken-1 mb-6">
              {{ role === 'send' ? 'Sending to peer...' : 'Receiving from peer...' }}
            </p>

            <!-- Real-time transfer stats -->
            <v-row class="w-100 max-width-subcard mb-6 px-2">
              <v-col cols="6" class="text-left border-r border-grey-darken-4">
                <div class="text-caption text-grey-darken-1">Speed</div>
                <div class="text-h6 font-weight-medium text-secondary">{{ store.formatSpeed(progress.speed) }}</div>
              </v-col>
              <v-col cols="6" class="text-left pl-4">
                <div class="text-caption text-grey-darken-1">Time Remaining</div>
                <div class="text-h6 font-weight-medium text-grey-lighten-2">{{ store.formatDuration(progress.eta) }}</div>
              </v-col>
            </v-row>

            <!-- Progress Bytes indicator -->
            <div class="text-body-2 text-grey-lighten-1 mb-8">
              {{ store.formatBytes(progress.bytes) }} of {{ store.formatBytes(fileSize || progress.total) }}
            </div>

            <v-btn color="error" variant="elevated" size="large" @click="handleCancel" class="px-8">
              <v-icon :icon="mdiClose" start></v-icon>
              Cancel Transfer
            </v-btn>
          </div>

          <!-- State: DECOMPRESSING screen -->
          <div v-else-if="currentState === 'DECOMPRESSING'" class="d-flex flex-column align-center justify-center pa-12 text-center">
            <v-progress-circular
              indeterminate
              color="secondary"
              size="80"
              width="6"
              class="mb-6 active-glow"
            ></v-progress-circular>
            <h3 class="text-h5 text-secondary mb-2">Unpacking Folder...</h3>
            <p class="text-body-2 text-grey-darken-1">Automatically extracting folder contents</p>
          </div>

          <!-- State: COMPLETED screen -->
          <div v-else-if="currentState === 'COMPLETED'" class="pa-10 text-center d-flex flex-column align-center">
            <v-avatar color="success-lighten-4" size="80" class="mb-6 bg-success-transparent">
              <v-icon :icon="mdiCheckCircle" color="success" size="48"></v-icon>
            </v-avatar>

            <h2 class="text-h4 mb-2 font-weight-bold text-success">Transfer Complete</h2>
            <p class="text-body-2 text-grey-darken-1 mb-8">
              {{ role === 'send' ? 'Your file was transferred successfully!' : 'File downloaded and saved successfully!' }}
            </p>

            <div class="glass-subcard pa-4 mb-8 w-100 max-width-subcard d-flex align-center gap-3">
              <v-icon :icon="isDir ? mdiFolder : mdiFile" color="success" size="32"></v-icon>
              <div class="text-left overflow-hidden">
                <div class="text-body-1 font-weight-bold text-truncate text-grey-lighten-2">{{ fileName }}</div>
                <div class="text-body-2 text-grey-darken-1">{{ store.formatBytes(fileSize || progress.total) }}</div>
              </div>
            </div>

            <div class="d-flex gap-3 justify-center w-100">
              <v-btn
                v-if="role === 'receive'"
                color="secondary"
                variant="elevated"
                size="large"
                @click="store.openSaveLocation"
                class="glow-btn"
              >
                <v-icon :icon="mdiFolderOpen" start></v-icon>
                Open Location
              </v-btn>
              <v-btn color="primary" variant="flat" size="large" @click="handleReset" class="px-6">
                <v-icon :icon="mdiRefresh" start></v-icon>
                Done
              </v-btn>
            </div>
          </div>

          <!-- State: FAILED screen -->
          <div v-else-if="currentState === 'FAILED'" class="pa-10 text-center d-flex flex-column align-center">
            <v-avatar color="error-lighten-4" size="80" class="mb-6 bg-error-transparent">
              <v-icon :icon="mdiAlertCircle" color="error" size="48"></v-icon>
            </v-avatar>

            <h2 class="text-h4 mb-2 font-weight-bold text-error">Transfer Failed</h2>
            <p class="text-body-1 text-grey-lighten-2 px-4 mb-8 max-width-subcard">
              {{ errorMessage }}
            </p>

            <v-btn color="primary" size="large" @click="handleReset" class="px-8 glow-btn">
              Try Again
            </v-btn>
          </div>
        </v-card>
      </v-container>
    </v-main>

    <!-- Settings Dialog -->
    <v-dialog v-model="showSettings" max-width="500px">
      <v-card class="glass-dialog pa-4">
        <v-card-title class="d-flex justify-between align-center border-b pb-3 mb-4">
          <span class="text-h6 font-weight-bold text-grey-lighten-1">Transfer Settings</span>
          <v-btn icon variant="text" size="small" @click="showSettings = false">
            <v-icon :icon="mdiClose"></v-icon>
          </v-btn>
        </v-card-title>

        <v-card-text>
          <div class="mb-4">
            <label class="text-subtitle-2 text-grey-lighten-1 d-block mb-2">Default Download Destination</label>
            <div class="d-flex gap-2">
              <v-text-field
                v-model="newSaveDir"
                readonly
                variant="outlined"
                density="comfortable"
                hide-details
                color="primary"
                class="flex-grow-1"
              ></v-text-field>
              <v-btn color="primary" variant="tonal" height="48" @click="changeSaveDir">
                Browse
              </v-btn>
            </div>
          </div>
        </v-card-text>

        <v-card-actions class="pt-4 border-t">
          <v-spacer></v-spacer>
          <v-btn color="secondary" variant="elevated" @click="showSettings = false">
            Save & Close
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Clipboard alert Snackbar -->
    <v-snackbar v-model="copySuccess" color="success" timeout="2000" class="custom-snackbar">
      Wormhole code copied to clipboard!
    </v-snackbar>
  </v-app>
</template>

<style>
/* Curated Dark Neon Aesthetics & CSS Variables */
:root {
  --glass-bg: rgba(26, 24, 46, 0.65);
  --glass-border: rgba(255, 255, 255, 0.08);
  --glass-blur: blur(24px);
}

.app-bg {
  background: radial-gradient(circle at 10% 20%, rgb(18, 12, 38) 0%, rgb(10, 8, 20) 90%) !important;
  font-family: 'Outfit', 'Inter', sans-serif !important;
  min-height: 100vh;
}

.max-width-card {
  max-width: 520px;
}

.max-width-input {
  max-width: 400px;
}

.max-width-subcard {
  max-width: 400px;
}

/* Glassmorphism Styling */
.glass-card {
  background: var(--glass-bg) !important;
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border) !important;
  border-radius: 20px !important;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5), 0 0 50px rgba(138, 43, 226, 0.15) !important;
}

.glass-dialog {
  background: rgb(20, 18, 38) !important;
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border) !important;
  border-radius: 16px !important;
}

.glass-subcard {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid rgba(255, 255, 255, 0.05) !important;
  border-radius: 12px !important;
}

.logo-title {
  font-size: 1.8rem;
  letter-spacing: -0.05rem;
  background: linear-gradient(135deg, #a855f7 0%, #06b6d4 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* Drag and Drop Zone styling */
.drag-drop-zone {
  border: 2px dashed rgba(255, 255, 255, 0.15);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.01);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.drag-drop-zone.drag-over {
  border-color: #00FFFF;
  background: rgba(0, 255, 255, 0.05);
  box-shadow: 0 0 20px rgba(0, 255, 255, 0.1);
  transform: scale(0.99);
}

.drag-drop-zone.has-file {
  border-color: rgba(138, 43, 226, 0.4);
  background: rgba(138, 43, 226, 0.03);
}

/* Glowing Elements */
.glow-icon {
  filter: drop-shadow(0 0 8px rgba(138, 43, 226, 0.6));
}

.glow-btn {
  box-shadow: 0 4px 15px rgba(0, 255, 255, 0.25) !important;
  transition: all 0.2s ease-in-out !important;
}

.glow-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(0, 255, 255, 0.4) !important;
}

.active-glow {
  filter: drop-shadow(0 0 15px rgba(0, 255, 255, 0.3));
}

/* Animations */
.animated-icon {
  transition: transform 0.3s ease;
}

.drag-drop-zone:hover .animated-icon {
  transform: translateY(-5px) scale(1.05);
}

.bounce-chip {
  animation: float 2.5s ease-in-out infinite;
}

@keyframes float {
  0% { transform: translateY(0px); }
  50% { transform: translateY(-5px); }
  100% { transform: translateY(0px); }
}

/* Breathing Waiting indicator */
.breathing-dot {
  width: 10px;
  height: 10px;
  background-color: #8A2BE2;
  border-radius: 50%;
  box-shadow: 0 0 0 rgba(138, 43, 226, 0.4);
  animation: pulse 1.8s infinite;
}

@keyframes pulse {
  0% {
    transform: scale(0.9);
    box-shadow: 0 0 0 0 rgba(138, 43, 226, 0.7);
  }
  70% {
    transform: scale(1);
    box-shadow: 0 0 0 10px rgba(138, 43, 226, 0);
  }
  100% {
    transform: scale(0.9);
    box-shadow: 0 0 0 0 rgba(138, 43, 226, 0);
  }
}

/* Codebox Styling */
.code-box {
  background: rgba(0, 255, 255, 0.05);
  border: 1px solid rgba(0, 255, 255, 0.2);
  border-radius: 12px;
  max-width: 400px;
  box-shadow: inset 0 0 10px rgba(0, 255, 255, 0.05);
}

.code-text {
  font-family: 'Fira Code', 'Courier New', Courier, monospace;
  font-size: 1.4rem;
  letter-spacing: 0.05rem;
  font-weight: 700;
  color: #00FFFF;
  text-shadow: 0 0 8px rgba(0, 255, 255, 0.3);
}

/* Layout Utilities */
.gap-2 { gap: 8px; }
.gap-3 { gap: 12px; }
.border-b { border-bottom: 1px solid rgba(255, 255, 255, 0.08) !important; }
.border-t { border-top: 1px solid rgba(255, 255, 255, 0.08) !important; }
.border-r { border-right: 1px solid rgba(255, 255, 255, 0.08) !important; }
.cursor-pointer { cursor: pointer; }
.flex-grow-1 { flex-grow: 1; }
.gap-1 { gap: 4px; }

/* Custom Transparent Backgrounds for Status Avatars */
.bg-primary-transparent {
  background-color: rgba(138, 43, 226, 0.15) !important;
  border: 1px solid rgba(138, 43, 226, 0.2) !important;
}

.bg-success-transparent {
  background-color: rgba(0, 255, 136, 0.15) !important;
  border: 1px solid rgba(0, 255, 136, 0.2) !important;
}

.bg-error-transparent {
  background-color: rgba(255, 51, 102, 0.15) !important;
  border: 1px solid rgba(255, 51, 102, 0.2) !important;
}
</style>
