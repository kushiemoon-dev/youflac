import { useCallback, useEffect, useState } from 'react';
import * as Api from '../lib/api';

export type SoundType = 'complete' | 'error' | 'queueEmpty';

// Singleton for sound effects
let globalSoundEnabled = true;
let audioContext: AudioContext | null = null;

// Initialize audio context on first user interaction
function getAudioContext(): AudioContext | null {
  if (!audioContext) {
    try {
      audioContext = new AudioContext();
    } catch {
      return null;
    }
  }
  return audioContext;
}

// Sound configurations (using Web Audio API for synthesized sounds)
const soundConfigs: Record<SoundType, { frequencies: number[]; durations: number[]; type: OscillatorType }> = {
  complete: {
    frequencies: [523.25, 659.25, 783.99], // C5, E5, G5 (major chord arpeggio)
    durations: [0.1, 0.1, 0.15],
    type: 'sine',
  },
  error: {
    frequencies: [311.13, 277.18], // Eb4, C#4 (dissonant)
    durations: [0.15, 0.2],
    type: 'square',
  },
  queueEmpty: {
    frequencies: [440, 554.37, 659.25], // A4, C#5, E5
    durations: [0.1, 0.1, 0.2],
    type: 'sine',
  },
};

// Play a synthesized sound
function playSynthSound(config: typeof soundConfigs[SoundType]) {
  const ctx = getAudioContext();
  if (!ctx) return;

  // Resume context if suspended (required by browsers after first user interaction)
  if (ctx.state === 'suspended') {
    ctx.resume();
  }

  let startTime = ctx.currentTime;

  config.frequencies.forEach((freq, i) => {
    const oscillator = ctx.createOscillator();
    const gainNode = ctx.createGain();

    oscillator.type = config.type;
    oscillator.frequency.setValueAtTime(freq, startTime);

    // Envelope: quick attack, sustain, quick release
    const duration = config.durations[i];
    gainNode.gain.setValueAtTime(0, startTime);
    gainNode.gain.linearRampToValueAtTime(0.3, startTime + 0.01);
    gainNode.gain.linearRampToValueAtTime(0.3, startTime + duration - 0.02);
    gainNode.gain.linearRampToValueAtTime(0, startTime + duration);

    oscillator.connect(gainNode);
    gainNode.connect(ctx.destination);

    oscillator.start(startTime);
    oscillator.stop(startTime + duration);

    startTime += duration * 0.7; // Slight overlap for smoother sound
  });
}

// Play a sound globally
export function playSound(type: SoundType) {
  if (!globalSoundEnabled) return;

  const config = soundConfigs[type];
  if (config) {
    playSynthSound(config);
  }
}

// Update global sound enabled state
export function setSoundEnabled(enabled: boolean) {
  globalSoundEnabled = enabled;
}

// Hook for components that need to manage sound effects
export function useSoundEffects() {
  const [enabled, setEnabled] = useState(true);

  // Load config on mount
  useEffect(() => {
    Api.GetConfig()
      .then((config) => {
        const isEnabled = config.soundEffectsEnabled ?? true;
        setEnabled(isEnabled);
        globalSoundEnabled = isEnabled;
      })
      .catch(console.error);
  }, []);

  // Update global state when local state changes
  const updateEnabled = useCallback((value: boolean) => {
    setEnabled(value);
    globalSoundEnabled = value;
  }, []);

  return {
    enabled,
    setEnabled: updateEnabled,
    playSound,
  };
}
