import { AccentColor } from '../../types';
import { accentColorPresets } from '../../hooks/useAccentColor';

interface ColorPickerProps {
  value: AccentColor;
  onChange: (color: AccentColor) => void;
  disabled?: boolean;
}

const colorLabels: Record<AccentColor, string> = {
  pink: 'Rose',
  blue: 'Bleu',
  green: 'Vert',
  purple: 'Violet',
  orange: 'Orange',
  teal: 'Turquoise',
  red: 'Rouge',
  yellow: 'Jaune',
};

export function ColorPicker({ value, onChange, disabled }: ColorPickerProps) {
  const colors = Object.keys(accentColorPresets) as AccentColor[];

  return (
    <div className="flex flex-wrap gap-2">
      {colors.map((color) => {
        const preset = accentColorPresets[color];
        const isSelected = value === color;

        return (
          <button
            key={color}
            type="button"
            disabled={disabled}
            title={colorLabels[color]}
            onClick={() => onChange(color)}
            className="relative w-8 h-8 rounded-full transition-all duration-200 focus:outline-none"
            style={{
              backgroundColor: preset.main,
              boxShadow: isSelected
                ? `0 0 0 2px var(--color-bg-primary), 0 0 0 4px ${preset.main}, 0 0 12px ${preset.glow}`
                : 'none',
              transform: isSelected ? 'scale(1.1)' : 'scale(1)',
              opacity: disabled ? 0.5 : 1,
              cursor: disabled ? 'not-allowed' : 'pointer',
            }}
          >
            {isSelected && (
              <svg
                className="absolute inset-0 m-auto w-4 h-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                style={{ color: color === 'yellow' ? '#000' : '#fff' }}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={3}
                  d="M5 13l4 4L19 7"
                />
              </svg>
            )}
          </button>
        );
      })}
    </div>
  );
}
