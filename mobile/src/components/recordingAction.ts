type Phase='idle'|'recording'|'transcribing';
/** Tap recording controls: the second tap stops; never restart during transcription. */
export function recordingAction(phase:Phase):'start'|'stop'|'wait'{
  return phase==='recording'?'stop':phase==='transcribing'?'wait':'start';
}
