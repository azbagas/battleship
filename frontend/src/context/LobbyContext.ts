import { createContext, useContext } from 'react'
import { PlacedShip, RoomStatePayload, WaitingRoomView } from '../types/protocol'

export interface ReadyState {
  hostReady: boolean
  guestReady: boolean
}

export interface LobbyContextValue {
  rooms: WaitingRoomView[]
  totalRooms: number
  maxRooms: number
  activeRoom: RoomStatePayload | null
  isCreating: boolean
  joiningRoomId: string | null
  isCancelling: boolean
  lobbyError: string | null
  isRoomCapReached: boolean
  createRoom: () => void
  joinRoom: (roomId: string) => void
  cancelRoom: () => void
  clearLobbyError: () => void

  // Ready-up controls
  readyState: ReadyState | null
  isTogglingReady: boolean
  isLeavingRoom: boolean
  toggleReady: () => void
  leaveRoom: () => void

  // Placement phase controls
  isFleetConfirmed: boolean
  isOpponentConfirmed: boolean
  ownFleet: PlacedShip[]
  placementDeadline: string | null
  submitFleet: (ships: PlacedShip[]) => void
}

export const LobbyContext = createContext<LobbyContextValue | undefined>(undefined)

export function useLobby(): LobbyContextValue {
  const context = useContext(LobbyContext)
  if (!context) {
    throw new Error('useLobby must be used within a LobbyProvider')
  }
  return context
}

