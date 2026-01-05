import { useState, useRef, useEffect } from 'react';

const ChevronDownIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="6 9 12 15 18 9" />
  </svg>
);

interface DropdownOption {
  value: string;
  label: string;
  description?: string;
}

interface DropdownProps {
  value: string;
  options: DropdownOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
}

export function Dropdown({ value, options, onChange, placeholder, disabled }: DropdownProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Close on click outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const selectedOption = options.find(o => o.value === value);

  return (
    <div className="dropdown" ref={ref}>
      <button
        type="button"
        className="dropdown-trigger"
        onClick={() => !disabled && setOpen(!open)}
        disabled={disabled}
        aria-expanded={open}
      >
        <span>{selectedOption?.label || placeholder || 'Select...'}</span>
        <ChevronDownIcon />
      </button>
      {open && (
        <div className="dropdown-menu">
          {options.map(option => (
            <div
              key={option.value}
              className={`dropdown-item ${option.value === value ? 'selected' : ''}`}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
            >
              <div>{option.label}</div>
              {option.description && (
                <div
                  className="text-xs mt-0.5"
                  style={{ color: 'var(--color-text-tertiary)' }}
                >
                  {option.description}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
