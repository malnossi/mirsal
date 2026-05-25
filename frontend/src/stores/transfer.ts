import { defineStore } from 'pinia'
import { ref, onMounted, onUnmounted } from 'vue'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import {
  Send,
  Receive,
  CancelTransfer,
  SelectFile,
  SelectDirectory,
  GetDefaultSaveDir,
  OpenFolder
} from '../../wailsjs/go/main/App'

export type TransferState =
  | 'IDLE'
  | 'COMPRESSING'
  | 'CONNECTING'
  | 'WAITING'
  | 'ACTIVE'
  | 'DECOMPRESSING'
  | 'COMPLETED'
  | 'FAILED'

export interface ProgressInfo {
  bytes: number
  total: number
  percent: number
  speed: number
  eta: number
  finished: boolean
}

export const useTransferStore = defineStore('transfer', () => {
  // State variables
  const currentState = ref<TransferState>('IDLE')
  const role = ref<'send' | 'receive' | null>(null)
  const code = ref<string>('')
  const filePath = ref<string>('')
  const fileName = ref<string>('')
  const fileSize = ref<number>(0)
  const isDir = ref<boolean>(false)
  const errorMessage = ref<string>('')
  const saveDir = ref<string>('')
  const completedPath = ref<string>('')

  // Progress variables
  const progress = ref<ProgressInfo>({
    bytes: 0,
    total: 0,
    percent: 0,
    speed: 0,
    eta: 0,
    finished: false
  })

  // Format Helpers
  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const formatSpeed = (bytesPerSec: number): string => {
    return `${formatBytes(bytesPerSec)}/s`
  }

  const formatDuration = (seconds: number): string => {
    if (seconds === Infinity || isNaN(seconds) || seconds <= 0) return 'estimating...'
    if (seconds < 60) return `${Math.round(seconds)}s`
    const mins = Math.floor(seconds / 60)
    const secs = Math.round(seconds % 60)
    return `${mins}m ${secs}s`
  }

  // Wails Event Listeners
  const handleProgressEvent = (data: any) => {
    progress.value = {
      bytes: data.bytes || 0,
      total: data.total || 0,
      percent: data.percent || 0,
      speed: data.speed || 0,
      eta: data.eta || 0,
      finished: data.finished || false
    }
  }

  const handleStatusEvent = (data: any) => {
    console.log('Wails Status Event:', data)
    const status = data.status

    if (status === 'compressing') {
      currentState.value = 'COMPRESSING'
    } else if (status === 'connecting') {
      currentState.value = 'CONNECTING'
      role.value = data.role
    } else if (status === 'waiting') {
      currentState.value = 'WAITING'
      role.value = data.role
      code.value = data.code || ''
      fileName.value = data.fileName || ''
      fileSize.value = data.fileSize || 0
      isDir.value = data.isDir || false
    } else if (status === 'active') {
      currentState.value = 'ACTIVE'
      role.value = data.role
      fileName.value = data.fileName || ''
      fileSize.value = data.fileSize || 0
      isDir.value = data.isDir || false
    } else if (status === 'decompressing') {
      currentState.value = 'DECOMPRESSING'
    } else if (status === 'completed') {
      currentState.value = 'COMPLETED'
      completedPath.value = data.path || ''
      progress.value.percent = 100
      progress.value.finished = true
    } else if (status === 'failed') {
      currentState.value = 'FAILED'
      errorMessage.value = data.error || 'Transfer failed'
    }
  }

  // Actions
  const initStore = async () => {
    // Subscribe to Wails events
    EventsOn('transfer:progress', handleProgressEvent)
    EventsOn('transfer:status', handleStatusEvent)

    // Load default save directory
    try {
      const defaultDir = await GetDefaultSaveDir()
      saveDir.value = defaultDir
    } catch (e) {
      console.error('Failed to get default save dir:', e)
    }
  }

  const cleanupStore = () => {
    EventsOff('transfer:progress')
    EventsOff('transfer:status')
  }

  const reset = () => {
    currentState.value = 'IDLE'
    role.value = null
    code.value = ''
    filePath.value = ''
    fileName.value = ''
    fileSize.value = 0
    isDir.value = false
    errorMessage.value = ''
    completedPath.value = ''
    progress.value = {
      bytes: 0,
      total: 0,
      percent: 0,
      speed: 0,
      eta: 0,
      finished: false
    }
  }

  // Choose file using native dialog
  const chooseFile = async () => {
    try {
      const path = await SelectFile()
      if (path) {
        filePath.value = path
        const baseName = path.split(/[/\\]/).pop() || path
        fileName.value = baseName
        isDir.value = false
        return path
      }
    } catch (e) {
      console.error('Failed to select file:', e)
    }
    return ''
  }

  // Choose directory using native dialog
  const chooseDirectory = async () => {
    try {
      const path = await SelectDirectory()
      if (path) {
        filePath.value = path
        const baseName = path.split(/[/\\]/).pop() || path
        fileName.value = baseName
        isDir.value = true
        return path
      }
    } catch (e) {
      console.error('Failed to select directory:', e)
    }
    return ''
  }

  // Choose custom save folder
  const chooseSaveFolder = async () => {
    try {
      const path = await SelectDirectory()
      if (path) {
        saveDir.value = path
        return path
      }
    } catch (e) {
      console.error('Failed to select save folder:', e)
    }
    return ''
  }

  // Start sending
  const startSend = async () => {
    const pathToSend = filePath.value
    const isDirToSend = isDir.value
    const nameToSend = fileName.value
    if (!pathToSend) return

    reset()

    filePath.value = pathToSend
    isDir.value = isDirToSend
    fileName.value = nameToSend

    role.value = 'send'
    currentState.value = 'CONNECTING'
    try {
      // Send calls Go app.Send and returns generated code immediately
      const generatedCode = await Send(pathToSend)
      code.value = generatedCode
    } catch (e: any) {
      currentState.value = 'FAILED'
      errorMessage.value = e.toString() || 'Failed to start sending'
    }
  }

  // Start receiving
  const startReceive = async (inputCode: string) => {
    if (!inputCode) return
    reset()
    role.value = 'receive'
    currentState.value = 'CONNECTING'
    code.value = inputCode
    try {
      // Receive is asynchronous in Go and sends events, returns immediately
      await Receive(inputCode, saveDir.value)
    } catch (e: any) {
      currentState.value = 'FAILED'
      errorMessage.value = e.toString() || 'Failed to start receiving'
    }
  }

  // Cancel active transfer
  const cancel = async () => {
    try {
      await CancelTransfer()
      reset()
    } catch (e) {
      console.error('Failed to cancel transfer:', e)
    }
  }

  // Open the download destination folder
  const openSaveLocation = async () => {
    const path = completedPath.value || saveDir.value
    if (!path) return
    try {
      await OpenFolder(path)
    } catch (e) {
      console.error('Failed to open folder:', e)
    }
  }

  return {
    // State
    currentState,
    role,
    code,
    filePath,
    fileName,
    fileSize,
    isDir,
    errorMessage,
    saveDir,
    completedPath,
    progress,

    // Getters & helpers
    formatBytes,
    formatSpeed,
    formatDuration,

    // Actions
    initStore,
    cleanupStore,
    reset,
    chooseFile,
    chooseDirectory,
    chooseSaveFolder,
    startSend,
    startReceive,
    cancel,
    openSaveLocation
  }
})
