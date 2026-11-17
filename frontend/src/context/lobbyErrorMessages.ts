import { ErrorCode, ErrorPayload } from '../types/protocol'

const LOBBY_ERROR_MESSAGES: Record<string, string> = {
  [ErrorCode.RoomNotAvailable]:
    'Game is no longer available. Another commander joined or the room was cancelled.',
  [ErrorCode.RoomCapReached]:
    'Maximum room capacity reached (10 rooms). Please wait for an open slot or join a waiting game.',
  [ErrorCode.RoomNotFound]: 'Room not found or no longer available.',
  [ErrorCode.CannotJoinOwnRoom]: 'You cannot join your own room.',
  [ErrorCode.AlreadyInRoom]: 'You already have an active room or match.',
  [ErrorCode.NotInWaitingRoom]: 'You are not currently hosting a waiting room.',
  [ErrorCode.NotInReadyUp]: 'Room is no longer in ready-up phase.',
  [ErrorCode.NotInPlacement]: 'Fleet placement has concluded.',
  [ErrorCode.FleetAlreadyLocked]: 'Fleet is already confirmed and locked.',
  [ErrorCode.InvalidPlacement]: 'Invalid ship positions. Check for overlaps or out-of-grid bounds.',
  [ErrorCode.NotYourTurn]: 'Stand by, Commander. It is not your turn to fire.',
  [ErrorCode.CellAlreadyTargeted]:
    'Coordinate has already been targeted. Select an uncharted grid cell.',
  [ErrorCode.GameOver]: 'The engagement has already concluded.',
  [ErrorCode.RematchNotAllowed]: 'Rematch is only available following a normal completed game.',
  [ErrorCode.OpponentNotPresent]: 'Opponent has already left the room. Rematch is unavailable.',
}

/** User-facing text for a server error; unknown codes fall back to the server message. */
export function lobbyErrorMessage(err: ErrorPayload): string {
  return LOBBY_ERROR_MESSAGES[err.code] ?? (err.message || 'An unexpected error occurred.')
}
