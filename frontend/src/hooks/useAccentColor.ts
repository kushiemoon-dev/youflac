import { useEffect } from 'react';
import { AccentColor } from '../types';

// Accent color presets with all variations
export const accentColorPresets: Record<AccentColor, {
  main: string;
  hover: string;
  subtle: string;
  glow: string;
}> = {
  pink: {
    main: '#f472b6',
    hover: '#ec4899',
    subtle: 'rgba(244, 114, 182, 0.15)',
    glow: 'rgba(244, 114, 182, 0.4)',
  },
  blue: {
    main: '#60a5fa',
    hover: '#3b82f6',
    subtle: 'rgba(96, 165, 250, 0.15)',
    glow: 'rgba(96, 165, 250, 0.4)',
  },
  green: {
    main: '#4ade80',
    hover: '#22c55e',
    subtle: 'rgba(74, 222, 128, 0.15)',
    glow: 'rgba(74, 222, 128, 0.4)',
  },
  purple: {
    main: '#a78bfa',
    hover: '#8b5cf6',
    subtle: 'rgba(167, 139, 250, 0.15)',
    glow: 'rgba(167, 139, 250, 0.4)',
  },
  orange: {
    main: '#fb923c',
    hover: '#f97316',
    subtle: 'rgba(251, 146, 60, 0.15)',
    glow: 'rgba(251, 146, 60, 0.4)',
  },
  teal: {
    main: '#2dd4bf',
    hover: '#14b8a6',
    subtle: 'rgba(45, 212, 191, 0.15)',
    glow: 'rgba(45, 212, 191, 0.4)',
  },
  red: {
    main: '#f87171',
    hover: '#ef4444',
    subtle: 'rgba(248, 113, 113, 0.15)',
    glow: 'rgba(248, 113, 113, 0.4)',
  },
  yellow: {
    main: '#fbbf24',
    hover: '#f59e0b',
    subtle: 'rgba(251, 191, 36, 0.15)',
    glow: 'rgba(251, 191, 36, 0.4)',
  },
};

// Apply accent color to CSS custom properties
export function applyAccentColor(color: AccentColor) {
  const preset = accentColorPresets[color] || accentColorPresets.pink;
  const root = document.documentElement;

  root.style.setProperty('--color-accent', preset.main);
  root.style.setProperty('--color-accent-hover', preset.hover);
  root.style.setProperty('--color-accent-subtle', preset.subtle);
  root.style.setProperty('--color-accent-glow', preset.glow);
}

// Hook to apply and manage accent color
export function useAccentColor(color: AccentColor | undefined) {
  useEffect(() => {
    if (color) {
      applyAccentColor(color);
    }
  }, [color]);
}
