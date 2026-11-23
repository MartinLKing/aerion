// Bumped whenever contacts change, so every save-contact button on screen
// looks its address up again (Emailio addition).
// @ts-ignore - wailsjs path
import { EventsOn } from '../../../wailsjs/runtime/runtime'

let version = $state(0)
let listening = false

export function contactSavesVersion(): number {
  return version
}

export function contactSaved(): void {
  version++
}

/** Follows saves made elsewhere, such as in a pop-out composer. */
export function listenForContactChanges(): void {
  if (listening) return
  listening = true
  EventsOn('contacts:changed', contactSaved)
}
