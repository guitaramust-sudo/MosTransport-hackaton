import { registerRootComponent } from 'expo'

// Three.js checks this Node API while its CommonJS bundle is initialized.
// Hermes exposes `process`, but does not implement `emitWarning`.
const runtimeProcess = (globalThis as typeof globalThis & {
  process?: { emitWarning?: (...args: unknown[]) => void }
}).process

if (runtimeProcess && typeof runtimeProcess.emitWarning !== 'function') {
  runtimeProcess.emitWarning = () => undefined
}

// Keep this require below the runtime shim: static imports are hoisted.
const App = require('./App').default

registerRootComponent(App)
